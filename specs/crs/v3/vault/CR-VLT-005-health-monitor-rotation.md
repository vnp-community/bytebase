# Change Request: Vault Health Monitor & Secret Rotation

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-VLT-005                                               |
| **Title**          | Vault Health Monitor & Secret Rotation                   |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-17                                               |
| **Author**         | VNP AI Ops Team                                          |
| **Dependencies**   | CR-VLT-001, CR-VLT-002                                  |

---

## 1. Tổng quan

### 1.1 Mô tả
Xây dựng **Vault Health Monitor** và **Secret Rotation Engine** để:
- Giám sát sức khỏe vault provider liên tục
- Tự động rotate secrets theo policy (expiry, schedule)
- Alerting khi vault unreachable hoặc secrets sắp hết hạn
- Integration với existing IM webhook notification system

### 1.2 Bối cảnh
Sau khi migrate secrets sang vault, cần đảm bảo:
1. **Vault availability** — nếu vault down, Bytebase không thể kết nối DB instances
2. **Secret freshness** — credentials cần được rotate định kỳ theo compliance (PCI-DSS: 90 ngày)
3. **Lease management** — HashiCorp Vault dynamic secrets có TTL, cần renew
4. **Proactive alerting** — DBA cần biết trước khi secret hết hạn

### 1.3 Mục tiêu
- Continuous vault health monitoring (mỗi 30 giây)
- Secret expiration tracking + notification
- Automated secret rotation cho supported providers
- Circuit breaker khi vault unreachable
- Dashboard cho vault health status

---

## 2. Yêu cầu chức năng

### FR-001: Vault Health Monitor Runner

Background runner giám sát vault connectivity:

```go
type VaultHealthMonitor struct {
    secretManager  *SecretManager
    store          *store.Store
    webhookManager *webhook.Manager
    interval       time.Duration  // Default: 30s
    
    // Circuit breaker state
    consecutiveFails int
    lastHealthy      time.Time
    status           VaultHealthStatus
}

type VaultHealthStatus int
const (
    VaultHealthy VaultHealthStatus = iota
    VaultDegraded    // Slow response (>500ms)
    VaultUnhealthy   // Failed health check
    VaultUnreachable // Circuit breaker tripped (5+ consecutive fails)
)

func (m *VaultHealthMonitor) Run(ctx context.Context) {
    ticker := time.NewTicker(m.interval)
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            m.check(ctx)
        }
    }
}

func (m *VaultHealthMonitor) check(ctx context.Context) {
    start := time.Now()
    err := m.secretManager.Primary().Healthy(ctx)
    latency := time.Since(start)
    
    if err != nil {
        m.consecutiveFails++
        if m.consecutiveFails >= 5 {
            m.status = VaultUnreachable
            m.alertVaultDown(ctx)
        } else {
            m.status = VaultUnhealthy
        }
    } else if latency > 500*time.Millisecond {
        m.status = VaultDegraded
        m.consecutiveFails = 0
    } else {
        if m.status != VaultHealthy {
            m.alertVaultRecovered(ctx)
        }
        m.status = VaultHealthy
        m.consecutiveFails = 0
        m.lastHealthy = time.Now()
    }
    
    // Update Prometheus metrics
    vaultHealthGauge.Set(float64(m.status))
    vaultLatencyHistogram.Observe(latency.Seconds())
}
```

### FR-002: Secret Expiration Tracker

Track secret expiry dates và notify trước khi hết hạn:

```go
type SecretExpirationTracker struct {
    store          *store.Store
    secretManager  *SecretManager
    webhookManager *webhook.Manager
    interval       time.Duration  // Default: 1h
}

type SecretExpiration struct {
    Ref       SecretRef
    ExpiresAt time.Time
    DaysLeft  int
    Status    ExpirationStatus  // OK, WARNING (30d), CRITICAL (7d), EXPIRED
}

// Check flow:
// 1. Enumerate all vault-backed secrets
// 2. For providers that support metadata (Vault, AWS SM):
//    - Read secret metadata including expiry/version
// 3. For providers without metadata:
//    - Track rotation date from Bytebase internal tracking
// 4. Generate alerts for WARNING/CRITICAL/EXPIRED secrets
```

**Notification rules**:
| Days to expiry | Level | Action |
|---|---|---|
| 30 | WARNING | IM notification to workspace admins |
| 7 | CRITICAL | IM notification + email to workspace admins |
| 0 | EXPIRED | IM notification + email + Bytebase banner alert |

### FR-003: Secret Rotation Engine

Tự động rotate secrets cho supported scenarios:

```go
type RotationPolicy struct {
    Category    SecretCategory
    MaxAgeDays  int           // Max age before rotation required
    Schedule    string        // Cron expression for scheduled rotation
    AutoRotate  bool          // Auto-rotate or notify-only
}

type RotationExecutor struct {
    secretManager  *SecretManager
    store          *store.Store
    dbFactory      *dbfactory.Factory
    webhookManager *webhook.Manager
}
```

**Supported rotation scenarios**:

| Scenario | Auto-Rotate | Description |
|---|---|---|
| Vault dynamic secrets | ✅ | Renew lease before TTL expires |
| DB password (manual) | ❌ | Notify DBA, provide rotation script |
| DB password (auto) | ✅ | Generate new password → ALTER USER → update vault → verify connection |
| SMTP password | ❌ | Notify admin, manual update required |
| AI API key | ❌ | Notify admin, regenerate from provider |
| IDP client secret | ❌ | Notify admin, requires IdP coordination |

**Auto-rotation flow cho DB passwords**:
```
Rotation Trigger (schedule or expiry warning)
  │
  ├─ 1. Generate new strong password
  ├─ 2. Connect to DB instance using ADMIN datasource
  ├─ 3. Execute ALTER USER/ROLE SET PASSWORD (engine-specific)
  │     ├─ PostgreSQL: ALTER ROLE {user} WITH PASSWORD '{new}'
  │     ├─ MySQL: ALTER USER '{user}'@'%' IDENTIFIED BY '{new}'
  │     ├─ Oracle: ALTER USER {user} IDENTIFIED BY "{new}"
  │     └─ MSSQL: ALTER LOGIN {user} WITH PASSWORD = '{new}'
  ├─ 4. Verify: Connect with new password
  ├─ 5. Update vault with new password
  ├─ 6. Update Bytebase rotation tracking
  ├─ 7. Create audit log entry
  └─ 8. Send notification to DBA
```

### FR-004: Vault Lease Manager

Cho HashiCorp Vault dynamic secrets:

```go
type LeaseManager struct {
    vaultClient    *vault.Client
    activLeases    map[string]*LeaseInfo
    mu             sync.RWMutex
    renewInterval  time.Duration  // Default: lease_duration / 3
}

type LeaseInfo struct {
    LeaseID    string
    SecretRef  SecretRef
    TTL        time.Duration
    Renewable  bool
    CreatedAt  time.Time
    ExpiresAt  time.Time
    RenewCount int
}

// Auto-renew leases before expiry
func (l *LeaseManager) Run(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case <-time.After(l.renewInterval):
            l.renewAll(ctx)
        }
    }
}
```

### FR-005: Health Dashboard API

```protobuf
service VaultHealthService {
    rpc GetVaultHealth(GetVaultHealthRequest) returns (VaultHealthResponse);
    rpc ListSecretExpirations(ListSecretExpirationsRequest) returns (ListSecretExpirationsResponse);
    rpc ListRotationPolicies(ListRotationPoliciesRequest) returns (ListRotationPoliciesResponse);
    rpc SetRotationPolicy(SetRotationPolicyRequest) returns (RotationPolicy);
    rpc TriggerRotation(TriggerRotationRequest) returns (RotationResult);
    rpc GetVaultMetrics(GetVaultMetricsRequest) returns (VaultMetricsResponse);
}
```

---

## 3. Yêu cầu kỹ thuật

| Component                          | File/Package                                         | Thay đổi                                    |
|------------------------------------|------------------------------------------------------|----------------------------------------------|
| Health Monitor Runner              | `backend/runner/vaulthealth/monitor.go`              | New: vault health check runner               |
| Expiration Tracker Runner          | `backend/runner/vaulthealth/expiration.go`           | New: secret expiration tracking              |
| Rotation Executor                  | `backend/component/secret/rotation/executor.go`     | New: secret rotation engine                  |
| Rotation Policy Store              | `backend/store/vault_rotation.go`                    | New: rotation policy + tracking persistence  |
| Lease Manager                      | `backend/component/secret/rotation/lease.go`        | New: Vault lease renewal                     |
| Health API                         | `backend/api/v1/vault_health_service.go`             | New: health + rotation gRPC service          |
| Prometheus Metrics                 | `backend/component/secret/metrics.go`                | Add: health, latency, rotation counters      |
| Proto: VaultHealth messages        | `proto/v1/v1/vault_health_service.proto`             | New: health API proto definitions            |
| Database Schema                    | `backend/migrator/migration/*/`                      | Tables: `vault_rotation_policy`, `vault_rotation_log`, `vault_lease` |
| Bus integration                    | `backend/component/bus/bus.go`                       | Add: VaultRotationChan for rotation events   |
| UI: Vault Health Dashboard         | `frontend/src/views/Setting/VaultHealth.vue`         | New: health status + rotation management UI  |

### 3.1 Database Schema

```sql
CREATE TABLE vault_rotation_policy (
    id SERIAL PRIMARY KEY,
    workspace TEXT NOT NULL,
    category VARCHAR(50) NOT NULL,
    scope TEXT,                        -- NULL = all scopes
    max_age_days INT DEFAULT 90,
    schedule VARCHAR(100),             -- Cron expression
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
    status VARCHAR(50) NOT NULL,         -- SUCCESS, FAILED
    old_version INT,
    new_version INT,
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

### 3.2 Prometheus Metrics

```go
var (
    vaultHealthGauge = prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "bytebase_vault_health_status",
        Help: "Vault health status: 0=healthy, 1=degraded, 2=unhealthy, 3=unreachable",
    })
    vaultLatencyHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{
        Name:    "bytebase_vault_request_duration_seconds",
        Help:    "Vault request latency distribution",
        Buckets: prometheus.ExponentialBuckets(0.01, 2, 10),
    })
    vaultSecretOpsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
        Name: "bytebase_vault_secret_operations_total",
        Help: "Total vault secret operations",
    }, []string{"operation", "provider", "status"})
    vaultRotationTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
        Name: "bytebase_vault_rotation_total",
        Help: "Total secret rotations",
    }, []string{"category", "type", "status"})
    vaultExpiringSecretsGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
        Name: "bytebase_vault_expiring_secrets",
        Help: "Number of secrets expiring within threshold",
    }, []string{"threshold"})
)
```

---

## 4. Security Considerations

| Concern                          | Mitigation                                                    |
|----------------------------------|---------------------------------------------------------------|
| Auto-rotation failure leaves invalid password | Verify new password works before updating vault; keep old password as rollback |
| Rotation during active connections | Graceful: update vault → existing connections use cached → new connections use new password |
| Lease renewal race condition     | Mutex on lease operations; idempotent renewal                |
| Health check false positive      | 3-check sliding window before status change; configurable thresholds |
| Rotation audit trail             | All rotations logged to vault_rotation_log + audit_log       |
| DBA notification fatigue         | Configurable notification schedule; batch alerts             |

---

## 5. Test Cases

| Test ID | Mô tả                                                | Expected Result                           |
|---------|--------------------------------------------------------|-------------------------------------------|
| TC-001  | Vault healthy → check passes                         | Status=Healthy, metric=0                   |
| TC-002  | Vault slow (>500ms) → degraded                      | Status=Degraded, metric=1                  |
| TC-003  | 5 consecutive vault failures → unreachable           | Status=Unreachable, alert sent             |
| TC-004  | Vault recovers after unreachable                     | Status=Healthy, recovery alert sent        |
| TC-005  | Secret expires in 30 days → WARNING notification     | IM notification sent to workspace admins   |
| TC-006  | Auto-rotate PostgreSQL password                      | New password generated, ALTER ROLE, vault updated, connection verified |
| TC-007  | Auto-rotate fails (DB unreachable)                   | Old password preserved, error logged, alert sent |
| TC-008  | Vault lease renewal                                  | Lease renewed, expiry extended             |
| TC-009  | Lease expired → re-auth                             | New lease obtained automatically           |
| TC-010  | Rotation policy: 90-day max age                      | Secrets older than 90 days flagged/rotated |

---

## 6. Rollout Plan

| Phase   | Mô tả                                     | Timeline       |
|---------|--------------------------------------------|----------------|
| Phase 1 | Health Monitor runner + Prometheus metrics | Sprint 1       |
| Phase 2 | Expiration Tracker + notifications         | Sprint 2       |
| Phase 3 | Rotation Engine (manual trigger)           | Sprint 3       |
| Phase 4 | Auto-rotation + Lease Manager              | Sprint 4       |
| Phase 5 | Health Dashboard UI + integration testing  | Sprint 5       |
