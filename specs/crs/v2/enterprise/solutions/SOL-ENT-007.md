# Solution: CR-ENT-007 — Approval Workflow

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-ENT-007                |
| **Solution**   | SOL-ENT-007               |
| **Status**     | Proposed                  |
| **Complexity** | High                      |

---

## 1. Tóm tắt giải pháp

Mở rộng **Approval Runner** hiện có (L6) để hỗ trợ custom multi-step approval templates với CEL-based condition matching. Tận dụng `Bus.ApprovalCheckChan` đã có. Thêm approval template CRUD, sequential step execution, và emergency bypass.

---

## 2. Architectural Alignment

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L6 — Runner** | `runner/approval/` | Core approval flow engine (enhance existing) |
| **L4 — Service** | `approval_template_service.go` (NEW) | Template CRUD |
| **L5 — Component** | `bus/` | `ApprovalCheckChan` already exists |
| **L5 — Component** | `webhook/` | Approval notifications |
| **L8 — Store** | `store/approval.go` (NEW) | Templates, flows, steps |
| **L9 — Enterprise** | `feature.go` | `FeatureApprovalWorkflow` gate |

---

## 3. Chi tiết Implementation

### 3.1 Approval Flow Logic

```
Issue Created → Risk Assessment (CR-ENT-006)
  → Bus.ApprovalCheckChan → Approval Runner
    → Match template via CEL conditions (priority order)
    → Create ApprovalFlow with steps from template
    → Step 1: Notify approvers → Wait
    → Step 1 Approved → Step 2: Notify → Wait
    → All Approved → Issue status = APPROVED → Rollout can proceed
```

### 3.2 Template Structure

```go
type ApprovalTemplate struct {
    Name      string
    Steps     []ApprovalStep
    Condition string  // CEL expression
    Priority  int
}

type ApprovalStep struct {
    Title     string
    Type      ApprovalType  // ANY_OF | ALL_OF
    Approvers []Approver    // role, group, or user
}
```

### 3.3 Schema Migration

```sql
CREATE TABLE approval_template (
    id BIGSERIAL PRIMARY KEY, workspace TEXT NOT NULL,
    name TEXT NOT NULL, config JSONB NOT NULL,
    condition_expression TEXT, priority INT NOT NULL DEFAULT 0,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_ts TIMESTAMPTZ NOT NULL DEFAULT now(), updated_ts TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE approval_flow (
    id BIGSERIAL PRIMARY KEY, issue_uid BIGINT NOT NULL,
    template_uid BIGINT REFERENCES approval_template(id),
    status TEXT NOT NULL DEFAULT 'PENDING',
    created_ts TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE approval_step (
    id BIGSERIAL PRIMARY KEY, flow_uid BIGINT NOT NULL REFERENCES approval_flow(id),
    step_index INT NOT NULL, status TEXT NOT NULL DEFAULT 'PENDING',
    approver_uid BIGINT REFERENCES principal(id),
    approved_at TIMESTAMPTZ, rejected_reason TEXT,
    created_ts TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 3.4 Emergency Bypass

Workspace admin có thể bypass approval flow — yêu cầu explicit reason, logged as `CRITICAL` audit entry.

---

## 4. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-ENT-006 | Risk level drives template condition matching |
| CR-ENT-019 | Environment tier used in approval conditions |
| CR-ENT-011 | Custom roles as approver targets |

---

## 5. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | Approval template CRUD | Sprint 1 |
| 2 | Approval flow engine | Sprint 2 |
| 3 | CEL condition matching | Sprint 2 |
| 4 | Notifications + escalation | Sprint 3 |
| 5 | Emergency bypass | Sprint 3 |
| 6 | E2E testing | Sprint 4 |
