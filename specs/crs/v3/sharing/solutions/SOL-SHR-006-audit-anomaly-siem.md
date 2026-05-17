# SOL-SHR-006 — Audit Trail, Anomaly Detection & SIEM Integration

| Metadata | Value |
|---|---|
| Solution ID | SOL-SHR-006 |
| CRs | CR-SHR-004 (Sharing Audit & Compliance), CR-SHR-105 (Extended Audit & SIEM) |
| Arch Layers | L3 (Security), L5 (Component), L6 (Runner), L7 (Plugin), L8 (Store) |
| Priority | P1 — High |
| Sprints | 7–12 |
| Dependencies | SOL-SHR-001 (events source), SOL-SHR-003 (share access events) |

---

## 1. Phân tích kiến trúc hiện tại

### 1.1 Audit Interceptor (L3)

```go
// backend/api/v1/audit.go (25,157 bytes)
// Position: 4th in interceptor chain (after ACL)
// Captures: request metadata, response status
// Writes: AuditLog entry async
```

Existing audit chỉ log API requests — **không capture sharing-specific events** (access from public endpoint, credential detection, distribution).

### 1.2 AuditLog Store (L8)

```go
// store/audit_log.go
type AuditLogMessage struct {
    ID          int64
    WorkspaceID string
    Method      string     // gRPC method
    Resource    string     // Resource path
    User        string     // Actor
    Severity    string
    Payload     string     // JSON details
    CreatedAt   time.Time
}
```

### 1.3 DataCleaner Runner (L6)

`runner/cleaner/` — periodic cleanup runner. Pattern for retention management.

---

## 2. Giải pháp chi tiết

### 2.1 Module Structure

```
backend/
├── component/audit/                   ← L5: Audit business logic
│   ├── sharing_emitter.go            ← Structured event emission
│   ├── integrity.go                   ← HMAC event chain
│   └── anomaly/
│       ├── detector.go               ← Rule-based anomaly detection
│       └── rules.go                  ← Anomaly rule definitions
│
├── component/audit/compliance/        ← Compliance reporting
│   ├── reporter.go                   ← Report generator
│   └── templates.go                  ← SOC2, ISO27001, PCI-DSS templates
│
├── plugin/siem/                       ← L7: SIEM exporters
│   ├── registry.go                   ← Exporter registration
│   ├── syslog/exporter.go           ← RFC 5424 Syslog
│   ├── webhook/exporter.go          ← HTTP POST
│   ├── kafka/exporter.go            ← Kafka producer
│   └── elasticsearch/exporter.go    ← ES Bulk API
│
├── runner/audit/                      ← L6: Background processing
│   ├── anomaly_runner.go             ← Periodic anomaly scanning
│   └── retention_runner.go           ← Data retention/archival
│
└── store/
    ├── sharing_audit_event.go        ← L8: Event CRUD
    ├── anomaly_alert.go              ← Alert CRUD
    └── siem_config.go                ← SIEM config CRUD
```

### 2.2 Audit Event Emitter

```go
// backend/component/audit/sharing_emitter.go
package audit

import (
    "context"
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "time"
)

// SharingAuditEmitter creates structured, tamper-proof audit events.
type SharingAuditEmitter struct {
    store         *store.Store
    hmacKey       []byte          // Derived from auth_secret
    lastEventHash string          // HMAC chain state
    siemExporters []SIEMExporter  // Active exporters
    anomalyDetector *anomaly.Detector
}

type SharingAuditEvent struct {
    ID              string            `json:"id"`
    WorkspaceID     string            `json:"workspace_id"`
    Timestamp       time.Time         `json:"timestamp"`
    ActorType       string            `json:"actor_type"`       // "user", "system", "api"
    ActorID         string            `json:"actor_id"`
    ActorIP         string            `json:"actor_ip,omitempty"`
    Action          string            `json:"action"`           // "share.created", "share.accessed", etc.
    Category        string            `json:"category"`         // "sharing", "distribution", "encryption"
    ResourceType    string            `json:"resource_type"`    // "shared_credential", "share_link"
    ResourceID      string            `json:"resource_id"`
    Details         map[string]any    `json:"details"`
    Result          string            `json:"result"`           // "success", "denied", "error"
    RiskLevel       string            `json:"risk_level"`       // "low", "medium", "high", "critical"
    ComplianceTags  []string          `json:"compliance_tags"`  // ["PCI-DSS-8.2.6", "SOC2-CC6.1"]
    PreviousHash    string            `json:"previous_event_hash"`
    EventHash       string            `json:"event_hash"`
}

// Emit creates an audit event with HMAC chain integrity.
func (e *SharingAuditEmitter) Emit(ctx context.Context, event *SharingAuditEvent) error {
    // 1. Set timestamp
    event.Timestamp = time.Now()
    event.ID = generateEventID()
    
    // 2. Compute HMAC chain hash
    event.PreviousHash = e.lastEventHash
    event.EventHash = e.computeHash(event)
    e.lastEventHash = event.EventHash
    
    // 3. Auto-tag compliance
    event.ComplianceTags = e.tagCompliance(event)
    
    // 4. Persist to store
    if err := e.store.CreateSharingAuditEvent(ctx, event); err != nil {
        return err
    }
    
    // 5. Feed to anomaly detector (async)
    go e.anomalyDetector.Evaluate(context.Background(), event)
    
    // 6. Forward to SIEM exporters (async)
    for _, exporter := range e.siemExporters {
        go exporter.Export(context.Background(), event)
    }
    
    return nil
}

// computeHash creates HMAC-SHA256(key, event_data + previous_hash).
func (e *SharingAuditEmitter) computeHash(event *SharingAuditEvent) string {
    mac := hmac.New(sha256.New, e.hmacKey)
    mac.Write([]byte(event.ID))
    mac.Write([]byte(event.Timestamp.Format(time.RFC3339Nano)))
    mac.Write([]byte(event.Action))
    mac.Write([]byte(event.ResourceID))
    mac.Write([]byte(event.PreviousHash))
    return hex.EncodeToString(mac.Sum(nil))
}

// VerifyChain re-computes and validates the HMAC chain.
func (e *SharingAuditEmitter) VerifyChain(ctx context.Context, startID, endID string) (*ChainVerifyResult, error) {
    events, _ := e.store.ListSharingAuditEvents(ctx, &store.FindAuditEventMessage{
        StartID: startID, EndID: endID,
    })
    
    result := &ChainVerifyResult{TotalEvents: len(events)}
    prevHash := ""
    
    for _, event := range events {
        expected := e.computeHashFromEvent(event, prevHash)
        if event.EventHash != expected {
            result.BrokenAt = event.ID
            result.Valid = false
            return result, nil
        }
        prevHash = event.EventHash
    }
    
    result.Valid = true
    return result, nil
}

// tagCompliance auto-assigns compliance tags based on event type.
func (e *SharingAuditEmitter) tagCompliance(event *SharingAuditEvent) []string {
    var tags []string
    switch event.Action {
    case "share.created":
        tags = append(tags, "PCI-DSS-8.2.6", "ISO27001-A.9.4.1")
    case "share.accessed":
        tags = append(tags, "PCI-DSS-10.2", "SOC2-CC6.1")
    case "key.rotated":
        tags = append(tags, "SOC2-CC6.6", "PCI-DSS-3.6")
    case "credential.distributed":
        tags = append(tags, "PCI-DSS-8.2", "SOC2-CC6.1")
    }
    return tags
}
```

### 2.3 Anomaly Detection Engine

```go
// backend/component/audit/anomaly/detector.go
package anomaly

import (
    "context"
    "time"
    lru "github.com/hashicorp/golang-lru/v2/expirable"
)

// Detector evaluates audit events against anomaly rules.
type Detector struct {
    rules   []*Rule
    store   *store.Store
    // Sliding window counters (in-memory, TTL-based — matches TDD cache pattern)
    counters *lru.LRU[string, *WindowCounter]
}

type Rule struct {
    ID          string
    Pattern     string            // Rule identifier
    Window      time.Duration     // Time window
    Threshold   int               // Max events in window
    RiskLevel   string            // "high", "critical"
    Action      AnomalyAction     // Alert, auto-revoke, block
}

type AnomalyAction int
const (
    ActionAlert AnomalyAction = iota
    ActionAutoRevoke
    ActionBlock
)

func NewDetector(store *store.Store) *Detector {
    d := &Detector{
        store:    store,
        counters: lru.NewLRU[string, *WindowCounter](4096, nil, 1*time.Hour),
    }
    d.loadDefaultRules()
    return d
}

func (d *Detector) loadDefaultRules() {
    d.rules = []*Rule{
        {ID: "ANM-001", Pattern: "share.created", Window: 10 * time.Minute, Threshold: 5, RiskLevel: "high", Action: ActionAlert},
        {ID: "ANM-002", Pattern: "share.accessed:geo_anomaly", Window: 1 * time.Hour, Threshold: 3, RiskLevel: "critical", Action: ActionAutoRevoke},
        {ID: "ANM-003", Pattern: "share.access_denied", Window: 5 * time.Minute, Threshold: 10, RiskLevel: "high", Action: ActionBlock},
        {ID: "ANM-004", Pattern: "distribution.unregistered_target", Window: 0, Threshold: 1, RiskLevel: "critical", Action: ActionBlock},
        {ID: "ANM-005", Pattern: "credential.bulk_export", Window: 1 * time.Minute, Threshold: 10, RiskLevel: "high", Action: ActionAlert},
        {ID: "ANM-006", Pattern: "share.accessed:off_hours", Window: 0, Threshold: 1, RiskLevel: "medium", Action: ActionAlert},
    }
}

// Evaluate checks an event against all rules.
func (d *Detector) Evaluate(ctx context.Context, event *SharingAuditEvent) {
    for _, rule := range d.rules {
        if d.matchesPattern(event, rule.Pattern) {
            key := fmt.Sprintf("%s:%s:%s", rule.ID, event.ActorID, event.WorkspaceID)
            
            counter, ok := d.counters.Get(key)
            if !ok {
                counter = &WindowCounter{Window: rule.Window}
                d.counters.Add(key, counter)
            }
            
            counter.Increment()
            
            if counter.Count() >= rule.Threshold {
                d.triggerAnomaly(ctx, rule, event)
                counter.Reset()
            }
        }
    }
}

func (d *Detector) triggerAnomaly(ctx context.Context, rule *Rule, event *SharingAuditEvent) {
    alert := &store.AnomalyAlertMessage{
        WorkspaceID: event.WorkspaceID,
        RuleID:      rule.ID,
        ActorID:     event.ActorID,
        RiskLevel:   rule.RiskLevel,
        Description: fmt.Sprintf("Anomaly %s detected: %s", rule.ID, rule.Pattern),
        RelatedEvents: []string{event.ID},
    }
    
    d.store.CreateAnomalyAlert(ctx, alert)
    
    // Execute auto-action
    switch rule.Action {
    case ActionAutoRevoke:
        // Auto-revoke the share
        d.store.RevokeShareByEvent(ctx, event.ResourceID)
    case ActionBlock:
        // Block the actor temporarily
        d.store.BlockActor(ctx, event.ActorID, 1*time.Hour)
    }
}

// WindowCounter is a thread-safe sliding window counter.
type WindowCounter struct {
    Window    time.Duration
    events    []time.Time
    mu        sync.Mutex
}

func (w *WindowCounter) Increment() {
    w.mu.Lock()
    defer w.mu.Unlock()
    now := time.Now()
    w.events = append(w.events, now)
    // Prune events outside window
    cutoff := now.Add(-w.Window)
    i := 0
    for i < len(w.events) && w.events[i].Before(cutoff) {
        i++
    }
    w.events = w.events[i:]
}

func (w *WindowCounter) Count() int {
    w.mu.Lock()
    defer w.mu.Unlock()
    return len(w.events)
}
```

### 2.4 SIEM Exporter Plugin System

```go
// backend/plugin/siem/registry.go
package siem

type SIEMExporter interface {
    Type() string
    Export(ctx context.Context, event *audit.SharingAuditEvent) error
    Healthy(ctx context.Context) error
    Close() error
}

type ExporterFactory func(config map[string]interface{}) (SIEMExporter, error)

var exporters = make(map[string]ExporterFactory)

func Register(exporterType string, factory ExporterFactory) {
    exporters[exporterType] = factory
}

func Open(exporterType string, config map[string]interface{}) (SIEMExporter, error) {
    factory, ok := exporters[exporterType]
    if !ok {
        return nil, fmt.Errorf("siem: unknown exporter %q", exporterType)
    }
    return factory(config)
}
```

### 2.5 Syslog Exporter (RFC 5424)

```go
// backend/plugin/siem/syslog/exporter.go
package syslog

import (
    "fmt"
    "log/syslog"
    siem "github.com/bytebase/bytebase/backend/plugin/siem"
)

func init() {
    siem.Register("syslog", func(config map[string]interface{}) (siem.SIEMExporter, error) {
        return NewSyslogExporter(config)
    })
}

type Exporter struct {
    writer *syslog.Writer
}

func NewSyslogExporter(config map[string]interface{}) (*Exporter, error) {
    protocol := config["protocol"].(string) // "tcp", "udp"
    address := config["address"].(string)   // "siem.company.com:514"
    
    writer, err := syslog.Dial(protocol, address, syslog.LOG_INFO|syslog.LOG_AUTH, "bytebase-sharing")
    if err != nil {
        return nil, err
    }
    return &Exporter{writer: writer}, nil
}

func (e *Exporter) Type() string { return "syslog" }

func (e *Exporter) Export(ctx context.Context, event *audit.SharingAuditEvent) error {
    // Format as RFC 5424 structured data
    msg := fmt.Sprintf("[bytebase-sharing action=\"%s\" actor=\"%s\" resource=\"%s\" risk=\"%s\" result=\"%s\"] %s",
        event.Action, event.ActorID, event.ResourceID, event.RiskLevel, event.Result,
        event.Details,
    )
    
    switch event.RiskLevel {
    case "critical":
        return e.writer.Crit(msg)
    case "high":
        return e.writer.Warning(msg)
    default:
        return e.writer.Info(msg)
    }
}
```

### 2.6 Webhook Exporter (Generic HTTP POST)

```go
// backend/plugin/siem/webhook/exporter.go
package webhook

func init() {
    siem.Register("webhook", func(config map[string]interface{}) (siem.SIEMExporter, error) {
        return NewWebhookExporter(config)
    })
}

type Exporter struct {
    client  *http.Client
    url     string
    headers map[string]string
}

func (e *Exporter) Export(ctx context.Context, event *audit.SharingAuditEvent) error {
    body, _ := json.Marshal(event)
    req, _ := http.NewRequestWithContext(ctx, "POST", e.url, bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    for k, v := range e.headers {
        req.Header.Set(k, v)
    }
    resp, err := e.client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    if resp.StatusCode >= 400 {
        return fmt.Errorf("siem webhook: HTTP %d", resp.StatusCode)
    }
    return nil
}
```

### 2.7 Compliance Reporter

```go
// backend/component/audit/compliance/reporter.go
package compliance

// Reporter generates compliance reports from audit data.
type Reporter struct {
    store *store.Store
}

type ComplianceReport struct {
    Standard    string            // "SOC2", "ISO27001", "PCI-DSS"
    Period      DateRange
    Sections    []ReportSection
    Summary     ReportSummary
    GeneratedAt time.Time
}

// GenerateSOC2Report generates SOC 2 CC6 credential audit report.
func (r *Reporter) GenerateSOC2Report(ctx context.Context, wsID string, period DateRange) (*ComplianceReport, error) {
    report := &ComplianceReport{
        Standard:    "SOC2",
        Period:      period,
        GeneratedAt: time.Now(),
    }
    
    // CC6.1 — Logical and Physical Access Controls
    accessEvents, _ := r.store.ListSharingAuditEvents(ctx, &store.FindAuditEventMessage{
        WorkspaceID:    wsID,
        ComplianceTag:  "SOC2-CC6.1",
        StartTime:      period.Start,
        EndTime:        period.End,
    })
    report.Sections = append(report.Sections, ReportSection{
        Title:  "CC6.1 — Secret Access Controls",
        Events: len(accessEvents),
        Details: summarizeAccessPatterns(accessEvents),
    })
    
    // CC6.6 — Key Management
    keyEvents, _ := r.store.ListSharingAuditEvents(ctx, &store.FindAuditEventMessage{
        ComplianceTag: "SOC2-CC6.6",
        StartTime:     period.Start,
        EndTime:       period.End,
    })
    report.Sections = append(report.Sections, ReportSection{
        Title:  "CC6.6 — Key Lifecycle Management",
        Events: len(keyEvents),
        Details: summarizeKeyRotations(keyEvents),
    })
    
    // Summary
    anomalies, _ := r.store.CountAnomalyAlerts(ctx, wsID, period)
    report.Summary = ReportSummary{
        TotalEvents:      len(accessEvents) + len(keyEvents),
        AnomaliesDetected: anomalies,
        ComplianceScore:   calculateScore(anomalies, len(accessEvents)),
    }
    
    return report, nil
}
```

### 2.8 Retention Runner (L6)

```go
// backend/runner/audit/retention_runner.go
// Follows DataCleaner pattern (periodic cleanup).

type RetentionRunner struct {
    store    *store.Store
    interval time.Duration
    policies map[string]time.Duration // workspace → retention period
}

func (r *RetentionRunner) Run(ctx context.Context) {
    ticker := time.NewTicker(r.interval) // Every 24 hours
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            r.processRetention(ctx)
        }
    }
}

func (r *RetentionRunner) processRetention(ctx context.Context) {
    // Default retention: 3 years (PCI-DSS requirement)
    defaultRetention := 3 * 365 * 24 * time.Hour
    
    cutoff := time.Now().Add(-defaultRetention)
    
    // 1. Archive old events to cold storage (optional S3/GCS)
    r.store.ArchiveSharingAuditEvents(ctx, cutoff)
    
    // 2. Delete archived events from hot storage
    r.store.DeleteArchivedSharingAuditEvents(ctx, cutoff)
    
    // 3. Log retention action
    slog.Info("audit: retention processed", "cutoff", cutoff)
}
```

### 2.9 Database Migration

```sql
-- Sharing audit events (HMAC chain)
CREATE TABLE sharing_audit_event (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    actor_type TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    actor_ip INET,
    action TEXT NOT NULL,
    category TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    details JSONB NOT NULL DEFAULT '{}',
    result TEXT NOT NULL,
    risk_level TEXT DEFAULT 'low',
    compliance_tags TEXT[],
    previous_event_hash TEXT,
    event_hash TEXT NOT NULL
);

CREATE INDEX idx_sae_workspace_time ON sharing_audit_event(workspace_id, timestamp DESC);
CREATE INDEX idx_sae_actor ON sharing_audit_event(actor_id, timestamp DESC);
CREATE INDEX idx_sae_resource ON sharing_audit_event(resource_id);
CREATE INDEX idx_sae_action ON sharing_audit_event(action, timestamp DESC);
CREATE INDEX idx_sae_compliance ON sharing_audit_event USING GIN(compliance_tags);
CREATE INDEX idx_sae_risk ON sharing_audit_event(risk_level, timestamp DESC)
    WHERE risk_level IN ('high', 'critical');

-- Anomaly alerts
CREATE TABLE anomaly_alert (
    id SERIAL PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    rule_id TEXT NOT NULL,
    triggered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    actor_id TEXT NOT NULL,
    risk_level TEXT NOT NULL,
    description TEXT NOT NULL,
    related_events TEXT[],
    status TEXT DEFAULT 'open',
    resolved_at TIMESTAMPTZ,
    resolved_by INT REFERENCES principal(id),
    resolution_note TEXT
);

CREATE INDEX idx_anomaly_workspace ON anomaly_alert(workspace_id, status, triggered_at DESC);

-- SIEM configuration
CREATE TABLE siem_config (
    id SERIAL PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    name TEXT NOT NULL,
    exporter_type TEXT NOT NULL,
    config JSONB NOT NULL,
    event_filter JSONB,
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

---

## 3. Integration Points

### 3.1 AuditInterceptor Extension (L3)

```go
// backend/api/v1/audit.go — extend for sharing events
// The sharing emitter is called from:
// 1. SharingService handlers (create, revoke, list)
// 2. ShareAccess public endpoint (access, OTP verify)
// 3. Distribution runner (distribute, verify, rollback)
// 4. Envelope encryptor (seal, open, rekey)
```

### 3.2 Compliance API

```protobuf
// proto/v1/audit_service.proto — extend
rpc GenerateComplianceReport(GenerateReportRequest) returns (ComplianceReport) {
    option (google.api.http) = {
        post: "/v1/{workspace=workspaces/*}/audit/compliance-report"
        body: "*"
    };
}
rpc VerifyAuditChain(VerifyChainRequest) returns (ChainVerifyResult) {
    option (google.api.http) = {
        post: "/v1/{workspace=workspaces/*}/audit/verify-chain"
    };
}
rpc ListAnomalyAlerts(ListAlertsRequest) returns (ListAlertsResponse) {
    option (google.api.http) = {
        get: "/v1/{workspace=workspaces/*}/audit/anomaly-alerts"
    };
}
```

---

## 4. Test Strategy

| Test | Description | Method |
|---|---|---|
| HMAC chain creation | 10 events → valid chain | Unit |
| HMAC tamper detection | Modify event → chain broken | Unit |
| Anomaly rule ANM-001 | >5 shares in 10 min → alert | Unit |
| Anomaly auto-revoke | ANM-002 → share revoked | Integration |
| Syslog export | Event → RFC 5424 message | Unit (mock syslog) |
| Webhook export | Event → HTTP POST | HTTP test server |
| SOC2 report generation | Query → formatted report | Integration |
| Retention cleanup | Old events archived/deleted | Unit |
