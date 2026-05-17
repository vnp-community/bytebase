# Change Request: Vault Audit Trail & Access Logging

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-VLT-006                                               |
| **Title**          | Vault Audit Trail & Access Logging                       |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P2 — Medium                                              |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-17                                               |
| **Author**         | VNP AI Ops Team                                          |
| **Dependencies**   | CR-VLT-001                                               |

---

## 1. Tổng quan

### 1.1 Mô tả
Xây dựng **Vault Audit Trail** để ghi nhận chi tiết mọi thao tác với secrets — ai đọc, ai ghi, khi nào, từ đâu. Integration với existing Audit Log system của Bytebase (L3 Audit Interceptor) và bổ sung vault-specific audit events.

### 1.2 Bối cảnh
Bytebase đã có Audit Log system (SEC-07, SEC-10) ghi lại API requests. Tuy nhiên:
- Audit log hiện tại ghi **API call level** — không biết request đó có access secret hay không
- Không track **which specific secret** was accessed
- Không distinguish giữa "user viewed instance config" và "system resolved DB password for connection"
- Compliance requirements (PCI-DSS, SOC2) yêu cầu granular secret access logging

### 1.3 Mục tiêu
- Ghi nhận mọi secret read/write/delete operation
- Distinguish user-initiated vs system-initiated access
- Integrate với existing audit_log table
- Exportable audit reports cho compliance
- Dashboard cho secret access patterns

---

## 2. Yêu cầu chức năng

### FR-001: Secret Access Event Model

```go
type SecretAccessEvent struct {
    EventID     string           // UUID
    Timestamp   time.Time
    
    // Who
    PrincipalID int              // User/Service account ID
    PrincipalType string         // USER, SERVICE_ACCOUNT, SYSTEM
    IPAddress   string           // Client IP (for user actions)
    
    // What
    Operation   SecretOperation  // GET, SET, DELETE, LIST, ROTATE
    SecretRef   SecretRef        // Category + Path + Key
    Provider    string           // vault-kv-v2, aws-sm, etc.
    
    // Context
    TriggerType TriggerType      // USER_API, SYSTEM_CONNECTION, RUNNER_TASK, MIGRATION
    RequestID   string           // Correlation with API audit log
    
    // Result
    Success     bool
    ErrorMsg    string
    Latency     time.Duration
}

type SecretOperation int
const (
    SecretOpGet SecretOperation = iota
    SecretOpSet
    SecretOpDelete
    SecretOpList
    SecretOpRotate
    SecretOpMigrate
    SecretOpHealthCheck
)

type TriggerType int
const (
    TriggerUserAPI        TriggerType = iota  // User clicked "Test Connection"
    TriggerSystemConn                          // DBFactory resolving credentials
    TriggerRunnerTask                          // TaskRun executor connecting to DB
    TriggerMigration                           // Vault migration engine
    TriggerRotation                            // Secret rotation
    TriggerHealthCheck                         // Vault health monitor
)
```

### FR-002: Audit Hook in SecretManager

Instrument SecretManager để auto-log mọi operation:

```go
func (m *SecretManager) Resolve(ctx context.Context, ref SecretRef) (string, error) {
    start := time.Now()
    
    // Execute actual resolution
    value, err := m.doResolve(ctx, ref)
    
    // Log access event
    m.logAccess(ctx, SecretAccessEvent{
        Operation:   SecretOpGet,
        SecretRef:   ref,
        Provider:    m.primary.Name(),
        TriggerType: extractTriggerType(ctx),
        Success:     err == nil,
        ErrorMsg:    errorString(err),
        Latency:     time.Since(start),
        PrincipalID: extractPrincipalID(ctx),
        IPAddress:   extractIPAddress(ctx),
    })
    
    return value, err
}
```

### FR-003: Access Pattern Analytics

Phân tích patterns truy cập secret:

```go
type SecretAccessAnalytics struct {
    // Per-secret metrics
    TotalReads      map[string]int64  // path → read count
    UniqueAccessors map[string]int    // path → unique user count
    LastAccessed    map[string]time.Time
    
    // Anomaly detection
    UnusualAccessors []AnomalyEvent  // User accessing secret they never accessed before
    HighFrequency    []AnomalyEvent  // Unusual spike in access frequency
    OffHoursAccess   []AnomalyEvent  // Access outside business hours
}

type AnomalyEvent struct {
    SecretRef   SecretRef
    PrincipalID int
    Timestamp   time.Time
    Type        AnomalyType
    Details     string
}
```

### FR-004: Compliance Reporting

Export-ready reports cho audit/compliance:

```go
type ComplianceReport struct {
    Period      TimeRange           // Report period
    Summary     ComplianceSummary
    Details     []SecretAccessEvent
    Anomalies   []AnomalyEvent
}

type ComplianceSummary struct {
    TotalSecrets          int
    VaultBackedSecrets    int
    PlaintextSecrets      int    // Non-compliant
    TotalAccessEvents     int
    UniqueAccessors       int
    RotatedSecrets        int
    ExpiredSecrets        int
    AnomalyCount          int
}
```

**Supported export formats**:
- JSON (machine-readable)
- CSV (spreadsheet analysis)
- PDF (management reporting) — via existing Export component

### FR-005: Audit Log Integration

Extend existing `audit_log` table với secret-specific fields:

```sql
-- Option A: Add columns to existing audit_log
ALTER TABLE audit_log ADD COLUMN secret_access JSONB;

-- Option B: Separate table (preferred for performance)
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
    latency_ms INT,
    CONSTRAINT fk_principal FOREIGN KEY (principal_id) REFERENCES principal(id)
);

CREATE INDEX idx_vault_access_log_time ON vault_access_log(workspace, timestamp DESC);
CREATE INDEX idx_vault_access_log_principal ON vault_access_log(principal_id, timestamp DESC);
CREATE INDEX idx_vault_access_log_path ON vault_access_log(secret_path, timestamp DESC);
```

### FR-006: Access Policy Enforcement

Optional: restrict secret access based on policies:

```go
type SecretAccessPolicy struct {
    // Who can access which categories
    AllowedCategories map[SecretCategory][]string  // category → list of role names
    
    // Time-based restrictions
    BusinessHoursOnly bool
    BusinessHoursStart int  // 8 (8:00 AM)
    BusinessHoursEnd   int  // 18 (6:00 PM)
    
    // Rate limiting per principal
    MaxReadsPerMinute  int  // Default: 100
    MaxWritesPerMinute int  // Default: 10
}
```

---

## 3. Yêu cầu kỹ thuật

| Component                          | File/Package                                         | Thay đổi                                    |
|------------------------------------|------------------------------------------------------|----------------------------------------------|
| SecretAccessEvent types            | `backend/component/secret/audit/types.go`            | New: event model + analytics types           |
| Audit Hook                        | `backend/component/secret/audit/hook.go`             | New: instrumentation for SecretManager       |
| Access Logger                     | `backend/component/secret/audit/logger.go`           | New: async event writer to vault_access_log  |
| Analytics Engine                  | `backend/component/secret/audit/analytics.go`        | New: access pattern analysis                 |
| Compliance Reporter               | `backend/component/secret/audit/compliance.go`       | New: compliance report generation            |
| Access Policy Enforcer            | `backend/component/secret/audit/policy.go`           | New: policy-based access control             |
| Vault Access Store                | `backend/store/vault_access_log.go`                  | New: vault_access_log CRUD                   |
| Audit API                         | `backend/api/v1/vault_audit_service.go`              | New: audit query + compliance report API     |
| Proto: Audit messages             | `proto/v1/v1/vault_audit_service.proto`              | New: audit API proto definitions             |
| Database Schema                   | `backend/migrator/migration/*/`                      | Table: `vault_access_log`                    |
| Data Cleaner integration          | `backend/runner/cleaner/cleaner.go`                  | Modify: add vault_access_log retention       |
| UI: Audit Dashboard               | `frontend/src/views/Setting/VaultAudit.vue`          | New: access log viewer + analytics UI        |

### 3.1 Async Event Writer

Secret access logging **must not** impact request latency:

```go
type AsyncAccessLogger struct {
    eventChan chan SecretAccessEvent  // Buffered: 10000
    store     *store.Store
    batchSize int                     // Default: 100
    flushInterval time.Duration       // Default: 5s
}

func (l *AsyncAccessLogger) Log(event SecretAccessEvent) {
    select {
    case l.eventChan <- event:
        // Queued
    default:
        // Channel full — log warning, drop event
        vaultAuditDroppedCounter.Inc()
    }
}

func (l *AsyncAccessLogger) Run(ctx context.Context) {
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
            l.flush(ctx, batch)
            return
        }
    }
}
```

### 3.2 Data Retention

- Default retention: 90 days (configurable)
- Integration với existing DataCleaner runner
- Partitioned by month for efficient cleanup
- Compliance mode: retention up to 7 years

---

## 4. Security Considerations

| Concern                          | Mitigation                                                    |
|----------------------------------|---------------------------------------------------------------|
| Audit log tampering              | Append-only table with row checksums; optional write to external SIEM |
| Secret values in audit log       | **Never** log actual secret values — only path, key, operation |
| High-volume logging impact       | Async batched writer; configurable verbosity levels           |
| Audit log storage growth         | Automatic retention cleanup; partitioned tables               |
| Anomaly detection false positives| Configurable thresholds; learning period before alerting      |
| Access log as information leak   | Audit log itself requires WORKSPACE_ADMIN to read             |

---

## 5. Test Cases

| Test ID | Mô tả                                                | Expected Result                           |
|---------|--------------------------------------------------------|-------------------------------------------|
| TC-001  | GetSecret logs access event                           | Event in vault_access_log with correct fields |
| TC-002  | SetSecret logs write event                            | Operation=SET logged                       |
| TC-003  | System connection (DBFactory) logged as SYSTEM_CONNECTION | TriggerType correct                     |
| TC-004  | User API call logged with principal + IP              | Full context captured                      |
| TC-005  | 1000 events/second → async writer handles            | No dropped events, <5s flush delay        |
| TC-006  | Event channel full → graceful degradation            | Events dropped with metric increment       |
| TC-007  | Compliance report for 30-day period                   | Summary + details correct                  |
| TC-008  | Anomaly: new user accesses critical secret           | Anomaly event generated                    |
| TC-009  | Retention cleanup removes events older than 90 days  | Old events removed, recent kept            |
| TC-010  | Audit log never contains secret values               | Scan all events — no plaintext secrets     |

---

## 6. Rollout Plan

| Phase   | Mô tả                                     | Timeline       |
|---------|--------------------------------------------|----------------|
| Phase 1 | Event model + Async Logger                 | Sprint 1       |
| Phase 2 | SecretManager instrumentation              | Sprint 2       |
| Phase 3 | API + UI Dashboard                         | Sprint 3       |
| Phase 4 | Analytics + Anomaly Detection              | Sprint 4       |
| Phase 5 | Compliance Reporting + Data Retention      | Sprint 5       |
