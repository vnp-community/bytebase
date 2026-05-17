# Solution: CR-PRV-004 — Data Retention & Automated Purging

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-PRV-004                |
| **Solution**   | SOL-PRV-004               |
| **Status**     | Proposed                  |
| **Complexity** | High                      |

---

## 1. Tóm tắt giải pháp

Mở rộng **DataCleaner runner** hiện có tại L6 (`runner/cleaner/`) với policy-driven retention management. DataCleaner đã có cơ chế periodic cleanup (TDD §5) — giải pháp thêm configurable retention policies tại L8 Store, legal hold mechanism tại L4 Service, và compliance dashboard tại L1. Tận dụng kiến trúc monolith: tất cả data tables nằm cùng PostgreSQL → purging đơn giản bằng batch DELETE với transaction.

---

## 2. Architectural Alignment

```
L4 Service (retention_policy_service.go + legal_hold_service.go)
  │  CRUD policies + legal holds
  ▼
L8 Store (retention_policy, legal_hold)
  │
  ▼
L6 Runner (cleaner/ — enhanced DataCleaner)
  │  periodic: check retention policies → batch purge
  │  ├─ Check legal holds → skip held data
  │  ├─ Batch DELETE expired data
  │  ├─ Verify purge count
  │  └─ Audit log purge results
  ▼
L3 Security (Audit Interceptor) — log purge events
```

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L6 — Runner** | `runner/cleaner/` | Extend existing DataCleaner with policy-driven purging |
| **L4 — Service** | `api/v1/retention_policy_service.go` | Retention policy CRUD |
| **L4 — Service** | `api/v1/legal_hold_service.go` | Legal hold management |
| **L8 — Store** | `store/retention_policy.go` | Retention policy + legal hold persistence |
| **L9 — Enterprise** | `feature.go` | `FeatureDataRetention` gate |
| **L1 — Presentation** | `RetentionDashboard.tsx` | Compliance dashboard |

---

## 3. Chi tiết Implementation

### 3.1 L6 — Enhanced DataCleaner

**File**: `backend/runner/cleaner/retention_cleaner.go` (new file in existing cleaner/)

```go
type RetentionCleaner struct {
    store *store.Store
}

// Purge targets — mapped to existing Bytebase metadata tables
var purgeTargets = []PurgeTarget{
    {Table: "query_history",     RetentionKey: "query_history",      TimestampCol: "created_ts"},
    {Table: "audit_log",         RetentionKey: "audit_logs",         TimestampCol: "created_ts"},
    {Table: "web_refresh_token", RetentionKey: "session_data",       TimestampCol: "updated_ts"},
    {Table: "task_run_log",      RetentionKey: "change_history",     TimestampCol: "created_ts"},
    {Table: "pii_scan_result",   RetentionKey: "pii_scan_results",   TimestampCol: "scanned_at"},
}

func (c *RetentionCleaner) Run(ctx context.Context) error {
    policies, _ := c.store.ListRetentionPolicies(ctx)
    legalHolds, _ := c.store.ListActiveLegalHolds(ctx)
    
    for _, target := range purgeTargets {
        policy := findPolicy(policies, target.RetentionKey)
        if policy == nil || policy.RetentionDays == 0 { // 0 = unlimited
            continue
        }
        
        cutoff := time.Now().AddDate(0, 0, -policy.RetentionDays)
        
        // Check legal holds — skip held data
        holdFilter := buildLegalHoldFilter(legalHolds, target.Table)
        
        // Batch purge (1000 per batch to avoid lock contention)
        totalPurged := 0
        for {
            purged, _ := c.store.PurgeExpiredData(ctx, PurgeRequest{
                Table:        target.Table,
                TimestampCol: target.TimestampCol,
                Cutoff:       cutoff,
                BatchSize:    policy.BatchSize, // default 1000
                HoldFilter:   holdFilter,
            })
            totalPurged += purged
            if purged < policy.BatchSize {
                break // no more to purge
            }
            time.Sleep(100 * time.Millisecond) // yield between batches
        }
        
        // Audit log
        if totalPurged > 0 {
            c.store.CreateAuditLog(ctx, &store.AuditLogMessage{
                Method:   "RetentionPurge",
                Resource: target.Table,
                Status:   "SUCCESS",
                Details:  fmt.Sprintf("Purged %d records older than %v", totalPurged, cutoff),
            })
        }
    }
    return nil
}
```

### 3.2 Integration with Existing DataCleaner

**File**: `backend/runner/cleaner/cleaner.go` (modify existing)

```go
// Existing DataCleaner already runs periodic cleanup.
// Add RetentionCleaner as additional cleanup step.
func (c *DataCleaner) Run(ctx context.Context) {
    ticker := time.NewTicker(c.cleanupInterval) // existing: periodic
    for {
        select {
        case <-ticker.C:
            c.cleanupExpiredData(ctx)      // existing cleanup
            c.retentionCleaner.Run(ctx)    // NEW: policy-driven retention
        case <-ctx.Done():
            return
        }
    }
}
```

### 3.3 L4 — Legal Hold Service

**File**: `backend/api/v1/legal_hold_service.go`

```go
type LegalHoldService struct {
    store *store.Store
}

func (s *LegalHoldService) CreateLegalHold(ctx context.Context, req *v1pb.CreateLegalHoldRequest) (*v1pb.LegalHold, error) {
    // Requires bb.legalHold.create permission (workspace admin only)
    hold, _ := s.store.CreateLegalHold(ctx, &store.LegalHoldMessage{
        Scope:       req.Scope,      // USER, PROJECT, DATABASE
        ScopeUID:    req.ScopeUid,
        Reason:      req.Reason,
        HoldUntil:   req.HoldUntil,  // nil = indefinite
        CreatedBy:   ctx.Value(userKey),
    })
    return convertLegalHold(hold), nil
}
```

### 3.4 L8 — Database Schema

```sql
CREATE TABLE retention_policy (
    id BIGSERIAL PRIMARY KEY,
    workspace_uid BIGINT NOT NULL,
    data_type TEXT NOT NULL, -- 'query_history', 'audit_logs', etc.
    retention_days INT NOT NULL, -- 0 = unlimited
    min_retention_days INT NOT NULL DEFAULT 0, -- regulatory minimum
    batch_size INT NOT NULL DEFAULT 1000,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_ts TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_ts TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_uid, data_type)
);

CREATE TABLE legal_hold (
    id BIGSERIAL PRIMARY KEY,
    workspace_uid BIGINT NOT NULL,
    scope TEXT NOT NULL CHECK (scope IN ('USER', 'PROJECT', 'DATABASE', 'WORKSPACE')),
    scope_uid BIGINT,
    reason TEXT NOT NULL,
    hold_until TIMESTAMPTZ, -- NULL = indefinite
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_by BIGINT NOT NULL,
    released_by BIGINT,
    created_ts TIMESTAMPTZ NOT NULL DEFAULT now(),
    released_ts TIMESTAMPTZ
);

-- Default retention policies (seeded via migration)
INSERT INTO retention_policy (workspace_uid, data_type, retention_days, min_retention_days) VALUES
    (0, 'query_history', 90, 1),
    (0, 'audit_logs', 365, 90),
    (0, 'session_data', 30, 1),
    (0, 'pii_scan_results', 180, 30);
```

---

## 4. Security Considerations

| Concern | Mitigation |
|---------|-----------|
| Premature purge | Minimum retention days enforced per regulation |
| Legal hold bypass | Legal hold check is non-overridable in purge pipeline |
| Purge verification | Row count logged + audit trail |
| Admin abuse | Legal hold create/release requires workspace admin + audit |
| Data recovery | Purged data irrecoverable — warnings shown in UI |

---

## 5. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-PRV-003 (DSR) | Legal hold blocks Right to Erasure |
| CR-ENT-003 (Audit Log) | Audit logs subject to retention policy |
| CR-PRV-005 (Privacy Audit) | Purge events logged to audit trail |

---

## 6. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | Retention policy CRUD + Store + migration | Sprint 1 |
| 2 | Enhanced DataCleaner with retention policies | Sprint 2 |
| 3 | Legal hold service + approval workflow | Sprint 3 |
| 4 | Retention dashboard + compliance reports | Sprint 3 |
