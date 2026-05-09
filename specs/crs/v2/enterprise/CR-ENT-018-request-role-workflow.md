# Change Request: Request Role Workflow

| Field | Value |
|---|---|
| **CR ID** | CR-ENT-018 |
| **Feature ID** | SEC-21 |
| **Title** | Request Role Workflow |
| **Plan** | ENTERPRISE |
| **Priority** | P2 — Medium |
| **Status** | Draft |
| **Created** | 2026-05-08 |

---

## 1. Tổng quan

Luồng **yêu cầu cấp quyền** cho phép users self-service request roles trên projects/workspace, thay vì admin phải manually assign. Tích hợp với approval workflow và audit log.

## 2. Yêu cầu chức năng

### FR-001: Role Request Form
- User chọn: target project/workspace, desired role, justification
- Hiển thị permissions của role được yêu cầu
- Duration: permanent hoặc time-bound (link với JIT Access CR-ENT-017)

### FR-002: Request Routing
- Auto-route to project owner (project roles) hoặc workspace admin (workspace roles)
- Multi-level approval cho elevated roles (workspaceAdmin, workspaceDBA)
- Configurable approval rules

### FR-003: Role Grant Execution
- Approved → System tạo IAM binding tự động
- Rejected → Notification với rejection reason
- Role changes audited (CR-ENT-003)

### FR-004: Request Management
- Admin dashboard: pending requests, history, analytics
- User view: my requests, status tracking
- Bulk approve/reject

## 3. Backend Changes

| Component | Thay đổi |
|---|---|
| `backend/api/v1/role_request.go` | Role request CRUD + approval |
| `backend/component/iam/` | Auto-grant binding on approval |
| `enterprise/feature.go` | `FeatureRequestRoleWorkflow` |

## 4. Database Changes

```sql
CREATE TABLE role_request (
    id BIGSERIAL PRIMARY KEY,
    workspace TEXT NOT NULL,
    requester_uid BIGINT NOT NULL,
    target_resource TEXT NOT NULL,
    requested_role TEXT NOT NULL,
    justification TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'PENDING',
    approver_uid BIGINT,
    approved_at TIMESTAMPTZ,
    created_ts TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

## 5. Test Cases

| TC | Mô tả | Expected |
|---|---|---|
| TC-001 | Request projectDeveloper role | Request created, routed to owner |
| TC-002 | Approve role request | IAM binding created |
| TC-003 | Reject role request | Notification with reason |
| TC-004 | Request already-held role | Error: role already assigned |
| TC-005 | Non-ENTERPRISE | Feature gated |
