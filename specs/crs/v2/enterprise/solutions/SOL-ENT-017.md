# Solution: CR-ENT-017 — JIT (Just-In-Time) Access

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-ENT-017                |
| **Solution**   | SOL-ENT-017               |
| **Status**     | Proposed                  |
| **Complexity** | Medium                    |

---

## 1. Tóm tắt giải pháp

Enhance `AccessGrantService` hiện có (`backend/api/v1/access_grant_service.go`, 17KB) để hỗ trợ full JIT access request → approval → time-bound grant → auto-revocation flow. Thêm background runner cho expired grant cleanup.

---

## 2. Architectural Alignment

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L4 — Service** | `access_grant_service.go` (existing, 17KB) | Access request CRUD + approval |
| **L5 — Component** | `component/iam/` | Time-bound IAM binding evaluation |
| **L6 — Runner** | `runner/jitaccess/` (NEW) | Background expiry checker (every 1 min) |
| **L8 — Store** | `access_request` table (NEW) | Request persistence |
| **L9 — Enterprise** | `feature.go` | `FeatureJITAccess` gate |

---

## 3. Chi tiết Implementation

### 3.1 JIT Access Flow

```
User → Request: resource + role + duration (1h-7d) + justification
  → Route to project owner / DBA (configurable CEL rules)
  → Self-approval prevention enforced
  → Approved → Create time-bound IAM binding (expires_at)
  → L5 IAM Manager evaluates binding with time check
  → L6 JIT Runner: scan expired grants every 1 min
    → Remove expired IAM bindings
    → Notify user 15 min before expiry
```

### 3.2 Schema Migration

```sql
CREATE TABLE access_request (
    id BIGSERIAL PRIMARY KEY,
    workspace TEXT NOT NULL,
    requester_uid BIGINT NOT NULL REFERENCES principal(id),
    resource TEXT NOT NULL,
    role TEXT NOT NULL,
    duration_seconds BIGINT NOT NULL,
    justification TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'PENDING'
        CHECK (status IN ('PENDING', 'APPROVED', 'REJECTED', 'EXPIRED')),
    approver_uid BIGINT REFERENCES principal(id),
    grant_expires_at TIMESTAMPTZ,
    created_ts TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_access_request_expiry ON access_request (grant_expires_at) WHERE status = 'APPROVED';
```

### 3.3 IAM Time-Bound Binding

```go
// component/iam/manager.go
func (m *Manager) CheckPermission(ctx context.Context, ...) (bool, error) {
    // Existing logic + time-bound check
    for _, binding := range iamPolicy.Bindings {
        if binding.ExpiresAt != nil && time.Now().After(*binding.ExpiresAt) {
            continue // expired binding
        }
        // ... existing permission check
    }
}
```

---

## 4. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-ENT-011 | JIT access requests custom roles |
| CR-ENT-012 | JIT unmask grants for masked columns |
| CR-ENT-003 | All JIT events audited |

---

## 5. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | Access request CRUD + approval | Sprint 1 |
| 2 | Time-bound IAM binding | Sprint 1 |
| 3 | JIT expiry runner | Sprint 2 |
| 4 | Extension requests | Sprint 2 |
