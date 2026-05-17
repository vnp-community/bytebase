# Solution: CR-ENT-018 — Request Role Workflow

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-ENT-018                |
| **Solution**   | SOL-ENT-018               |
| **Status**     | Proposed                  |
| **Complexity** | Medium                    |

---

## 1. Tóm tắt giải pháp

Xây dựng self-service role request flow tích hợp với IAM Manager (L5). Workflow: user request → auto-route to owner/admin → approve → IAM binding created tự động. Liên kết với JIT Access (CR-ENT-017) cho time-bound grants.

---

## 2. Architectural Alignment

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L4 — Service** | `role_request.go` (NEW) | Request CRUD + approval routing |
| **L5 — Component** | `component/iam/` | Auto-grant IAM binding on approval |
| **L8 — Store** | `role_request` table (NEW) | Persistence |
| **L9 — Enterprise** | `feature.go` | `FeatureRequestRoleWorkflow` gate |

---

## 3. Chi tiết Implementation

### 3.1 Request Flow

```
User → Select project/workspace + desired role + justification
  → Auto-route to:
    - Project roles → project owner
    - Workspace roles → workspace admin
    - Elevated roles (workspaceAdmin) → multi-level approval
  → Approved → System creates IAM binding automatically
  → Rejected → Notification with reason
```

### 3.2 Schema Migration

```sql
CREATE TABLE role_request (
    id BIGSERIAL PRIMARY KEY,
    workspace TEXT NOT NULL,
    requester_uid BIGINT NOT NULL REFERENCES principal(id),
    target_resource TEXT NOT NULL,  -- projects/{id} or workspaces/{id}
    requested_role TEXT NOT NULL,
    justification TEXT NOT NULL,
    duration_type TEXT NOT NULL DEFAULT 'PERMANENT'
        CHECK (duration_type IN ('PERMANENT', 'TIME_BOUND')),
    duration_seconds BIGINT,  -- for TIME_BOUND
    status TEXT NOT NULL DEFAULT 'PENDING'
        CHECK (status IN ('PENDING', 'APPROVED', 'REJECTED')),
    approver_uid BIGINT REFERENCES principal(id),
    approved_at TIMESTAMPTZ,
    rejection_reason TEXT,
    created_ts TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 3.3 Auto-Grant on Approval

```go
func (s *RoleRequestService) ApproveRequest(ctx context.Context, req *ApproveRequestRequest) error {
    roleReq, _ := s.store.GetRoleRequest(ctx, req.RequestId)
    // Create IAM binding
    binding := &store.IAMBinding{
        Role:    roleReq.RequestedRole,
        Members: []string{principalName(roleReq.RequesterUID)},
    }
    if roleReq.DurationType == "TIME_BOUND" {
        binding.ExpiresAt = time.Now().Add(time.Duration(roleReq.DurationSeconds) * time.Second)
    }
    return s.iamManager.AddBinding(ctx, roleReq.TargetResource, binding)
}
```

---

## 4. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-ENT-011 | Users can request custom roles |
| CR-ENT-017 | Time-bound option links to JIT access |
| CR-ENT-003 | All role requests audited |

---

## 5. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | Request CRUD + routing | Sprint 1 |
| 2 | Approval + auto-grant | Sprint 1 |
| 3 | Admin dashboard | Sprint 2 |
| 4 | Time-bound integration | Sprint 2 |
