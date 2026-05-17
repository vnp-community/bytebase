# Solution: Policy Decision Audit & Analytics

| Field | Value |
|---|---|
| **SOL ID** | SOL-POL-008 |
| **CR Reference** | CR-POL-008 |
| **Status** | Proposed |
| **Created** | 2026-05-17 |
| **Dependencies** | SOL-POL-001, SOL-POL-006 |

---

## 1. Architecture Mapping

| CR Component | Target Layer | Rationale |
|---|---|---|
| Decision Logger | **L5 — Component** | Shared logging component, like existing Webhook |
| Decision Store | **L8 — Store** | New table `policy_decision_log`, follows store pattern |
| Analytics Service | **L4 — Service** | New gRPC service for decision analytics API |
| Prometheus Metrics | **L10 — Infra** | Registered on existing `/metrics` endpoint |
| OPA Log Exporter | **L7 — Plugin** | Plugin for external log shipping |
| DataCleaner extension | **L6 — Runner** | Retention cleanup in existing cleaner runner |

---

## 2. Package Structure

```
backend/component/policy/audit/
├── logger.go          ← DecisionLogger: async logging of policy decisions
├── analytics.go       ← AnalyticsService: aggregation queries
├── metrics.go         ← Prometheus: eval_total, duration, denied_total
└── opa_export.go      ← OPA Decision Log format exporter

backend/store/
└── policy_decision_log.go  ← CRUD + analytics queries for decision_log table

proto/v1/
└── policy_audit.proto      ← gRPC API for decision log querying
```

---

## 3. Key Design Decisions

### 3.1 Decision Logger — Async, High-Throughput

Uses Bus.PolicyDecisionChan (buffer: 5000) for async logging to avoid blocking evaluation:

```go
type DecisionLogger struct {
    store    *store.Store
    bus      *bus.Bus
    metrics  *DecisionMetrics
    batcher  *LogBatcher     // Batch inserts for performance
}

func NewDecisionLogger(store *store.Store, bus *bus.Bus) *DecisionLogger {
    logger := &DecisionLogger{
        store:   store,
        bus:     bus,
        metrics: NewDecisionMetrics(),
        batcher: NewLogBatcher(100, 1*time.Second), // batch 100 entries or flush every 1s
    }
    return logger
}

// Run starts the async log consumer goroutine.
func (l *DecisionLogger) Run(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            l.batcher.Flush(ctx) // Flush remaining entries on shutdown
            return
        case log := <-l.bus.PolicyDecisionChan:
            l.batcher.Add(log)
            l.metrics.Record(log)
            if l.batcher.ShouldFlush() {
                l.batcher.Flush(ctx)
            }
        }
    }
}
```

**Integration into PolicyManager** (SOL-POL-001):

```go
// In PolicyManager.Evaluate(), after getting decision:
m.bus.PolicyDecisionChan <- PolicyDecisionLog{
    RequestID:    requestIDFromCtx(ctx),
    SubjectType:  req.Subject.Type,
    SubjectID:    req.Subject.ID,
    Action:       req.Action,
    ResourceType: req.Resource.Type,
    ResourceID:   req.Resource.ID,
    Engine:       decision.Engine,
    PolicyID:     decision.PolicyID,
    Allowed:      decision.Allowed,
    Reason:       decision.Reason,
    EvalTimeMs:   decision.EvaluationTime.Milliseconds(),
    WorkspaceID:  workspaceIDFromCtx(ctx),
}
```

### 3.2 Decision Log Store

```sql
CREATE TABLE policy_decision_log (
    id TEXT NOT NULL,
    workspace TEXT NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT now(),
    request_id TEXT NOT NULL,
    subject_type TEXT NOT NULL,
    subject_id TEXT NOT NULL,
    action TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    engine TEXT NOT NULL,
    policy_id TEXT,
    policy_version INTEGER,
    allowed BOOLEAN NOT NULL,
    reason TEXT,
    evaluation_time_ms INTEGER,
    obligations JSONB NOT NULL DEFAULT '[]',
    project TEXT,
    environment TEXT,
    PRIMARY KEY (workspace, id)
);

-- Performance indexes for analytics queries
CREATE INDEX idx_decision_log_time
    ON policy_decision_log(workspace, timestamp DESC);
CREATE INDEX idx_decision_log_subject
    ON policy_decision_log(workspace, subject_id, timestamp DESC);
CREATE INDEX idx_decision_log_resource
    ON policy_decision_log(workspace, resource_type, resource_id);
CREATE INDEX idx_decision_log_denied
    ON policy_decision_log(workspace, allowed, timestamp DESC)
    WHERE NOT allowed;
CREATE INDEX idx_decision_log_engine
    ON policy_decision_log(workspace, engine, timestamp DESC);
```

**Design notes**:
- Composite PK `(workspace, id)` — consistent with existing store patterns (TDD §4.3)
- Partial index `WHERE NOT allowed` — optimizes denial queries (most common audit query)
- `obligations` as JSONB — consistent with `policy.payload` pattern (TDD §4.4)

### 3.3 Log Batcher — High-Volume Performance

```go
type LogBatcher struct {
    entries    []*PolicyDecisionLog
    maxBatch   int           // Default: 100
    flushInterval time.Duration // Default: 1s
    lastFlush  time.Time
    mu         sync.Mutex
}

func (b *LogBatcher) Flush(ctx context.Context) error {
    b.mu.Lock()
    entries := b.entries
    b.entries = make([]*PolicyDecisionLog, 0, b.maxBatch)
    b.lastFlush = time.Now()
    b.mu.Unlock()

    if len(entries) == 0 {
        return nil
    }

    // Batch INSERT using pgx/v5 CopyFrom for maximum throughput
    return store.BatchInsertDecisionLogs(ctx, entries)
}
```

**Target**: Handle 10K decisions/second without impacting evaluation latency.

### 3.4 Analytics Queries

```go
type AnalyticsService struct {
    store *store.Store
}

// TopDeniedPolicies returns most frequently denying policies.
func (a *AnalyticsService) TopDeniedPolicies(ctx, workspace, timeRange) ([]*PolicyDenialStat, error) {
    // SELECT policy_id, COUNT(*) as denied_count
    // FROM policy_decision_log
    // WHERE workspace = $1 AND NOT allowed AND timestamp > $2
    // GROUP BY policy_id ORDER BY denied_count DESC LIMIT 20
}

// DenialRateByEngine returns denial rates per engine.
func (a *AnalyticsService) DenialRateByEngine(ctx, workspace, timeRange) ([]*EngineStat, error)

// AccessPatterns returns subject-resource-action usage patterns.
func (a *AnalyticsService) AccessPatterns(ctx, workspace, timeRange) ([]*AccessPattern, error)

// PolicyCoverage identifies resources without policy evaluations.
func (a *AnalyticsService) PolicyCoverage(ctx, workspace, timeRange) (*CoverageReport, error)

// ComplianceAudit returns filtered decision trail for compliance reporting.
func (a *AnalyticsService) ComplianceAudit(ctx, workspace, filter) ([]*PolicyDecisionLog, error)
```

### 3.5 Prometheus Metrics

```go
var (
    policyEvalTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "bytebase_policy_evaluation_total",
            Help: "Total policy evaluations",
        },
        []string{"engine", "policy_category", "decision", "pep"},
    )
    policyEvalDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "bytebase_policy_evaluation_duration_seconds",
            Help:    "Policy evaluation duration",
            Buckets: []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1},
        },
        []string{"engine"},
    )
    policyDecisionDenied = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "bytebase_policy_decision_denied_total",
            Help: "Total denied policy decisions",
        },
        []string{"engine", "policy_id", "action"},
    )
)
```

### 3.6 OPA Decision Log Compatibility

Export decision logs in OPA Decision Log format for integration with Styra DAS, ELK, Splunk:

```go
type OPALogExporter struct {
    store    *store.Store
    endpoint string         // External log sink URL
    client   *http.Client
    batcher  *ExportBatcher
}

func (e *OPALogExporter) Export(log *PolicyDecisionLog) *OPADecisionLog {
    return &OPADecisionLog{
        Labels:     map[string]string{"id": "bytebase", "version": "1.0"},
        DecisionID: log.ID,
        Path:       "data/bytebase/authz/allow",
        Input:      buildOPAInput(log),
        Result:     log.Allowed,
        Timestamp:  log.Timestamp.Format(time.RFC3339),
        Metrics: map[string]any{
            "timer_rego_query_eval_ns": log.EvalTimeMs * 1_000_000,
        },
    }
}
```

### 3.7 Data Retention & Cleanup

Extends existing DataCleaner runner (architecture.md §7):

```go
// In backend/runner/cleaner/cleaner.go — extend cleanupTasks
func (c *DataCleaner) cleanPolicyDecisionLogs(ctx context.Context) error {
    // 1. Aggregate old logs (> 90 days) into summary tables
    // 2. Delete detailed logs older than retention period
    // 3. Log cleanup stats
    retentionDays := c.config.PolicyDecisionLogRetention // Default: 90

    deleted, err := c.store.CleanPolicyDecisionLogs(ctx, retentionDays)
    slog.Info("Cleaned policy decision logs",
        "deleted", deleted,
        "retention_days", retentionDays)
    return err
}
```

---

## 4. Integration with Existing AuditLog

The Decision Logger works **alongside** the existing AuditInterceptor, not replacing it:

| System | Scope | Granularity |
|---|---|---|
| AuditInterceptor (`audit.go`, 25KB) | API request/response | Per-API call |
| Decision Logger (this solution) | Policy evaluation | Per-policy decision |

A single API request may trigger multiple policy evaluations (e.g., access check + masking check). The `request_id` field links decision logs back to the parent audit log entry.

---

## 5. Runner Integration

Decision Logger runs as part of the Runner layer (L6):

```
Server Bootstrap:
  5.5  policyManager = policy.NewManager(...)
  5.6  pepRegistry = pep.NewRegistry(...)
  5.7  decisionLogger = audit.NewDecisionLogger(store, bus)  ← NEW

Runner startup:
  go decisionLogger.Run(ctx)  // Async log consumer goroutine
```

---

## 6. Performance & Scalability

| Metric | Target |
|---|---|
| Logging overhead per evaluation | < 10μs (async, channel-based) |
| Batch insert throughput | 10K entries/second (pgx CopyFrom) |
| Analytics query (top denied) | < 100ms for 30-day window |
| Retention cleanup | < 5min for 1M expired entries |
| Storage estimate | ~200 bytes/entry → 1.7GB/day at 10K decisions/sec |

---

## 7. Compliance Reporting

Pre-built compliance report templates:

| Report | Content |
|---|---|
| **GDPR Access Audit** | Who accessed PII data, when, masking applied |
| **SOX Change Control** | Schema changes with approval chain, policy enforcement |
| **PCI-DSS Access Review** | Cardholder data access decisions, denial reasons |
| **Custom** | Filtered by time range, subject, resource, engine |
