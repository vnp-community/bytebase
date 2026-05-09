# Change Request: JIT (Just-In-Time) Access

| Field | Value |
|---|---|
| **CR ID** | CR-ENT-017 |
| **Feature ID** | SEC-20 |
| **Title** | JIT (Just-In-Time) Access |
| **Plan** | ENTERPRISE |
| **Priority** | P1 — High |
| **Status** | Draft |
| **Created** | 2026-05-08 |

---

## 1. Tổng quan

Cấp quyền tạm thời tới database resources. Users request access cho specific duration, quyền tự động revoke khi hết hạn.

## 2. Yêu cầu chức năng

### FR-001: Access Request Flow
- User request: resource, role, duration (1h-7d), justification
- Route to approver → Approved → Time-bound IAM binding → Auto-revoke

### FR-002: Approval Routing
- Auto-route to project owner / DBA
- Configurable routing rules (CEL)
- Self-approval prevention

### FR-003: Automatic Revocation
- Background runner checks expired grants (every 1 min)
- User notified 15 min before expiry
- Active sessions terminated on revocation

### FR-004: Access Extension
- Extension request requires new approval
- Maximum total duration configurable

## 3. Backend Changes

| Component | Thay đổi |
|---|---|
| `backend/api/v1/jit_access.go` | Access request CRUD + approval |
| `backend/component/iam/` | Time-bound binding evaluation |
| `backend/runner/` | Background expiry checker |
| `enterprise/feature.go` | `FeatureJITAccess` |

## 4. Database Changes

```sql
CREATE TABLE access_request (
    id BIGSERIAL PRIMARY KEY,
    workspace TEXT NOT NULL,
    requester_uid BIGINT NOT NULL,
    resource TEXT NOT NULL,
    role TEXT NOT NULL,
    duration_seconds BIGINT NOT NULL,
    justification TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'PENDING',
    grant_expires_at TIMESTAMPTZ,
    created_ts TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

## 5. Test Cases

| TC | Mô tả | Expected |
|---|---|---|
| TC-001 | Request 4h access | Request created, routed |
| TC-002 | Access expired | Binding removed |
| TC-003 | Self-approval | Rejected |
| TC-004 | Non-ENTERPRISE | Feature gated |
