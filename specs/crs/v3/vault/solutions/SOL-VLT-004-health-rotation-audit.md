# SOL-VLT-004: Health Monitor, Secret Rotation & Audit Trail

| Field | Value |
|---|---|
| **SOL ID** | SOL-VLT-004 |
| **CR References** | CR-VLT-005, CR-VLT-006 |
| **Layer** | L5 (Component) + L6 (Runner) + L4 (API) + L8 (Store) |
| **Dependencies** | SOL-VLT-001 |
| **Estimated Effort** | 5 sprints per CR (10 total) |

---

## Part A: Vault Health Monitor & Secret Rotation (CR-VLT-005)

### A.1 Health Monitor Runner

```
runner/vaulthealth/
├── monitor.go      # Health check runner (30s interval)
└── expiration.go   # Secret expiration tracker (1h interval)

component/secret/rotation/
├── executor.go     # Rotation engine
├── lease.go        # Vault lease manager
└── policy.go       # Rotation policy types
```

#### A.1.1 Health Monitor

```go
// runner/vaulthealth/monitor.go
// Same pattern as runner/heartbeat/ — periodic background check

type Monitor struct {
    secretManager  *secret.SecretManager
    webhookManager *webhook.Manager
    interval       time.Duration // 30s
    // Circuit breaker state
    consecutiveFails int
    lastHealthy      time.Time
    status           HealthStatus
}

type HealthStatus int
const (
    Healthy     HealthStatus = iota // All checks pass
    Degraded                        // Slow response (>500ms)
    Unhealthy                       // Health check failed
    Unreachable                     // 5+ consecutive failures → circuit breaker
)

func (m *Monitor) Run(ctx context.Context) {
    ticker := time.NewTicker(m.interval)
    for {
        select {
        case <-ctx.Done(): return
        case <-ticker.C:   m.check(ctx)
        }
    }
}

func (m *Monitor) check(ctx context.Context) {
    start := time.Now()
    err := m.secretManager.Primary().Healthy(ctx)
    latency := time.Since(start)

    switch {
    case err != nil:
        m.consecutiveFails++
        if m.consecutiveFails >= 5 {
            m.status = Unreachable
            m.alertVaultDown(ctx) // IM notification via webhook manager
        } else {
            m.status = Unhealthy
        }
    case latency > 500*time.Millisecond:
        m.status = Degraded
        m.consecutiveFails = 0
    default:
        if m.status != Healthy {
            m.alertVaultRecovered(ctx) // Recovery notification
        }
        m.status = Healthy
        m.consecutiveFails = 0
        m.lastHealthy = time.Now()
    }

    // Prometheus metrics
    vaultHealthGauge.Set(float64(m.status))
    vaultLatencyHistogram.Observe(latency.Seconds())
}
```

#### A.1.2 Secret Expiration Tracker

```go
// runner/vaulthealth/expiration.go

type ExpirationTracker struct {
    store          *store.Store
    secretManager  *secret.SecretManager
    webhookManager *webhook.Manager
    interval       time.Duration // 1h
}

// Notification rules:
// 30 days → WARNING (IM only)
// 7 days  → CRITICAL (IM + email)
// 0 days  → EXPIRED (IM + email + banner alert)
```

### A.2 Secret Rotation Engine

#### A.2.1 Rotation Policy

```go
// rotation/policy.go
type RotationPolicy struct {
    ID          int
    WorkspaceID string
    Category    SecretCategory
    Scope       string // nil = all scopes
    MaxAgeDays  int    // Default: 90 (PCI-DSS)
    Schedule    string // Cron expression
    AutoRotate  bool   // Auto or notify-only
    Enabled     bool
}
```

#### A.2.2 Supported Rotation Scenarios

| Scenario | Auto | Flow |
|---|---|---|
| DB password (PostgreSQL) | ✅ | Generate → ALTER ROLE → update vault → verify connection |
| DB password (MySQL) | ✅ | Generate → ALTER USER → update vault → verify connection |
| DB password (Oracle/MSSQL) | ✅ | Engine-specific ALTER → update vault → verify |
| Vault dynamic secrets | ✅ | Renew lease before TTL |
| SMTP password | ❌ | Notify admin, manual update |
| AI API key | ❌ | Notify admin, regenerate from provider |
| IDP client secret | ❌ | Notify admin, requires IdP coordination |

#### A.2.3 Auto-Rotation Flow

```
Rotation Trigger (schedule or expiry warning)
  │
  ├─ 1. Generate new password (crypto/rand, 32 chars)
  ├─ 2. Connect to target DB via ADMIN DataSource
  ├─ 3. Execute ALTER statement (engine-specific):
  │     ├─ PostgreSQL: ALTER ROLE {user} WITH PASSWORD '{new}'
  │     ├─ MySQL: ALTER USER '{user}'@'%' IDENTIFIED BY '{new}'
  │     ├─ Oracle: ALTER USER {user} IDENTIFIED BY "{new}"
  │     └─ MSSQL: ALTER LOGIN {user} WITH PASSWORD = '{new}'
  ├─ 4. Verify: connect with new password
  ├─ 5. Update vault with new password
  ├─ 6. Invalidate SecretManager cache
  ├─ 7. Create audit log + rotation_log entry
  └─ 8. Notify DBA via webhook
```

### A.3 Vault Lease Manager

```go
// rotation/lease.go — HashiCorp Vault dynamic secrets only

type LeaseManager struct {
    vaultClient   *api.Client
    activeLeases  map[string]*LeaseInfo
    mu            sync.RWMutex
    renewInterval time.Duration // lease_duration / 3
}

type LeaseInfo struct {
    LeaseID   string
    Ref       SecretRef
    TTL       time.Duration
    Renewable bool
    ExpiresAt time.Time
}

// Auto-renew leases before expiry
func (l *LeaseManager) Run(ctx context.Context) {
    for {
        select {
        case <-ctx.Done(): return
        case <-time.After(l.renewInterval):
            l.renewAll(ctx) // Renew expiring leases
        }
    }
}
```

### A.4 Database Schema (Rotation)

```sql
CREATE TABLE vault_rotation_policy (
    id SERIAL PRIMARY KEY,
    workspace TEXT NOT NULL,
    category VARCHAR(50) NOT NULL,
    scope TEXT,
    max_age_days INT DEFAULT 90,
    schedule VARCHAR(100),
    auto_rotate BOOLEAN DEFAULT FALSE,
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE vault_rotation_log (
    id SERIAL PRIMARY KEY,
    policy_id INT REFERENCES vault_rotation_policy(id),
    secret_path TEXT NOT NULL,
    secret_key VARCHAR(255) NOT NULL,
    rotation_type VARCHAR(50) NOT NULL,  -- AUTO, MANUAL, LEASE_RENEW
    status VARCHAR(50) NOT NULL,
    error TEXT,
    rotated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    rotated_by INT REFERENCES principal(id)
);

CREATE TABLE vault_lease (
    id SERIAL PRIMARY KEY,
    workspace TEXT NOT NULL,
    lease_id TEXT NOT NULL UNIQUE,
    secret_path TEXT NOT NULL,
    secret_key VARCHAR(255) NOT NULL,
    ttl_seconds INT NOT NULL,
    renewable BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    last_renewed_at TIMESTAMPTZ,
    renew_count INT DEFAULT 0
);
CREATE INDEX idx_vault_lease_expires ON vault_lease(expires_at);
```

### A.5 Prometheus Metrics

```go
var (
    vaultHealthGauge = prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "bytebase_vault_health_status",
        Help: "0=healthy, 1=degraded, 2=unhealthy, 3=unreachable",
    })
    vaultLatencyHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{
        Name:    "bytebase_vault_request_duration_seconds",
        Buckets: prometheus.ExponentialBuckets(0.01, 2, 10),
    })
    vaultRotationTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
        Name: "bytebase_vault_rotation_total",
    }, []string{"category", "type", "status"})
    vaultExpiringSecrets = prometheus.NewGaugeVec(prometheus.GaugeOpts{
        Name: "bytebase_vault_expiring_secrets",
    }, []string{"threshold"}) // "30d", "7d", "expired"
)
```

---

## Part B: Vault Audit Trail & Access Logging (CR-VLT-006)

### B.1 Component Architecture

```
component/secret/audit/
├── types.go       # SecretAccessEvent, AnomalyEvent types
├── hook.go        # Instrumentation hook for SecretManager
├── logger.go      # Async batched event writer
├── analytics.go   # Access pattern analysis
└── compliance.go  # Compliance report generator

store/
└── vault_access_log.go  # DB persistence
```

### B.2 Secret Access Event Model

```go
// audit/types.go
type SecretAccessEvent struct {
    EventID       string
    Timestamp     time.Time
    PrincipalID   int
    PrincipalType string         // USER, SERVICE_ACCOUNT, SYSTEM
    IPAddress     string
    Operation     SecretOperation // GET, SET, DELETE, LIST, ROTATE
    SecretRef     SecretRef
    Provider      string
    TriggerType   TriggerType    // USER_API, SYSTEM_CONNECTION, RUNNER_TASK
    RequestID     string         // Correlation with API audit log
    Success       bool
    ErrorMsg      string
    Latency       time.Duration
}

type TriggerType int
const (
    TriggerUserAPI      TriggerType = iota // User action (Test Connection, etc.)
    TriggerSystemConn                       // DBFactory credential resolution
    TriggerRunnerTask                       // TaskRun executor
    TriggerMigration                        // Vault migration engine
    TriggerRotation                         // Secret rotation
    TriggerHealthCheck                      // Vault health monitor
)
```

### B.3 Audit Hook in SecretManager

```go
// SecretManager.Resolve() — instrumented with audit hook
func (m *SecretManager) Resolve(ctx context.Context, ref SecretRef) (string, error) {
    start := time.Now()
    value, err := m.doResolve(ctx, ref)

    // Async audit log (non-blocking)
    m.auditLogger.Log(SecretAccessEvent{
        Operation:   OpGet,
        SecretRef:   ref,
        Provider:    m.primary.Name(),
        TriggerType: extractTriggerType(ctx), // From context metadata
        Success:     err == nil,
        Latency:     time.Since(start),
        PrincipalID: extractPrincipalID(ctx),
        IPAddress:   extractIPAddress(ctx),
    })

    return value, err
}
```

### B.4 Async Event Writer

```go
// audit/logger.go — Non-blocking batched writer
// CRITICAL: audit logging must NOT impact request latency

type AsyncLogger struct {
    eventChan     chan SecretAccessEvent // Buffered: 10000
    store         *store.Store
    batchSize     int                   // 100
    flushInterval time.Duration         // 5s
}

func (l *AsyncLogger) Log(event SecretAccessEvent) {
    select {
    case l.eventChan <- event:
        // Queued successfully
    default:
        // Channel full — drop event, increment counter
        auditDroppedCounter.Inc()
    }
}

func (l *AsyncLogger) Run(ctx context.Context) {
    batch := make([]SecretAccessEvent, 0, l.batchSize)
    ticker := time.NewTicker(l.flushInterval)
    for {
        select {
        case event := <-l.eventChan:
            batch = append(batch, event)
            if len(batch) >= l.batchSize {
                l.flush(ctx, batch)
                batch = batch[:0]
            }
        case <-ticker.C:
            if len(batch) > 0 {
                l.flush(ctx, batch)
                batch = batch[:0]
            }
        case <-ctx.Done():
            l.flush(ctx, batch) // Final flush on shutdown
            return
        }
    }
}

func (l *AsyncLogger) flush(ctx context.Context, batch []SecretAccessEvent) {
    // Batch INSERT into vault_access_log table
    // Use store.BatchCreateVaultAccessLogs()
}
```

### B.5 Database Schema (Audit)

```sql
CREATE TABLE vault_access_log (
    id BIGSERIAL PRIMARY KEY,
    workspace TEXT NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    principal_id INT,
    principal_type VARCHAR(50),
    ip_address INET,
    operation VARCHAR(50) NOT NULL,
    secret_category VARCHAR(50) NOT NULL,
    secret_path TEXT NOT NULL,
    secret_key VARCHAR(255),
    provider VARCHAR(100),
    trigger_type VARCHAR(50),
    request_id TEXT,
    success BOOLEAN NOT NULL,
    error_msg TEXT,
    latency_ms INT
);

CREATE INDEX idx_vault_access_time ON vault_access_log(workspace, timestamp DESC);
CREATE INDEX idx_vault_access_principal ON vault_access_log(principal_id, timestamp DESC);
CREATE INDEX idx_vault_access_path ON vault_access_log(secret_path, timestamp DESC);
```

### B.6 Data Retention

- Default: 90 days (configurable)
- Integration with existing `runner/cleaner/cleaner.go` DataCleaner
- Compliance mode: up to 7 years retention

```go
// runner/cleaner/cleaner.go — Add vault_access_log cleanup
func (c *Cleaner) cleanVaultAccessLogs(ctx context.Context) {
    retention := c.getVaultLogRetention() // From workspace setting
    _, _ = c.store.DeleteVaultAccessLogsOlderThan(ctx, retention)
}
```

### B.7 Compliance Reporting

```go
// audit/compliance.go
type ComplianceReport struct {
    Period          TimeRange
    TotalSecrets    int
    VaultBacked     int
    PlaintextCount  int  // Non-compliant
    TotalAccesses   int
    UniqueAccessors int
    RotatedSecrets  int
    ExpiredSecrets  int
    Anomalies       int
}

// Export formats: JSON, CSV
// Delivered via existing Export component (component/export/)
```

### B.8 API

```protobuf
service VaultHealthService {
    rpc GetVaultHealth(GetVaultHealthRequest) returns (VaultHealthResponse);
    rpc ListSecretExpirations(ListSecretExpirationsRequest) returns (ListSecretExpirationsResponse);
    rpc ListRotationPolicies(ListRotationPoliciesRequest) returns (ListRotationPoliciesResponse);
    rpc SetRotationPolicy(SetRotationPolicyRequest) returns (RotationPolicy);
    rpc TriggerRotation(TriggerRotationRequest) returns (RotationResult);
}

service VaultAuditService {
    rpc ListVaultAccessLogs(ListVaultAccessLogsRequest) returns (ListVaultAccessLogsResponse);
    rpc GetComplianceReport(GetComplianceReportRequest) returns (ComplianceReport);
    rpc ExportComplianceReport(ExportComplianceReportRequest) returns (ExportComplianceReportResponse);
}
```

---

## 3. Security Constraints (Both Parts)

| Constraint | Implementation |
|---|---|
| Audit log never contains secret values | Only path, key, operation logged — never actual values |
| Auto-rotation failure safety | Verify new password works BEFORE updating vault |
| Rotation during active connections | Graceful: vault updated → cached credentials continue → new connections use new password |
| Audit log as information leak | `vault_access_log` requires WORKSPACE_ADMIN to read |
| Health check false positive | 3-check sliding window before status transition |
| High-volume logging impact | Async batched writer; configurable verbosity levels |

---

## 4. Bus Integration

```go
// component/bus/bus.go — Add vault channels
type Bus struct {
    // ... existing channels ...
    VaultRotationChan chan RotationEvent  // buffer: 100
    VaultAlertChan    chan AlertEvent     // buffer: 100
}
```

Rotation events flow through Bus to notify other components (webhook manager, audit logger).
