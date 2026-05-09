# Change Request: Approval Workflow

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-ENT-007                                               |
| **Feature ID**     | SEC-09                                                   |
| **Title**          | Custom Approval Workflow                                 |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-08                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Triển khai hệ thống **Approval Workflow tùy chỉnh** cho database changes, cho phép define multi-level approval flows dựa trên risk level, environment, project, và SQL statement type. Tích hợp chặt chẽ với Risk Assessment (CR-ENT-006).

### 1.2 Mục tiêu
- Define approval templates với multiple approval steps
- Map approval templates tới conditions (risk level, environment, etc.)
- Support approval groups (DBA group, Manager group, etc.)
- Enforce approval trước khi rollout execution
- Integration với IM notifications (Slack, Teams)

---

## 2. Yêu cầu chức năng

### FR-001: Approval Templates
- **Mô tả**: Admin có thể tạo approval templates với multiple steps.
- **Template Structure**:
  ```yaml
  name: "Production Schema Change"
  steps:
    - title: "DBA Review"
      type: ANY_OF          # ANY_OF | ALL_OF
      approvers:
        - role: workspaceDBA
        - group: dba-team@example.com
    - title: "Manager Approval"
      type: ANY_OF
      approvers:
        - role: projectOwner
        - user: manager@example.com
    - title: "Security Review"
      type: ALL_OF
      approvers:
        - group: security-team@example.com
  ```
- **Approval Types**:
  - `ANY_OF`: Chỉ cần 1 người trong nhóm approve
  - `ALL_OF`: Tất cả trong nhóm phải approve
- **Acceptance Criteria**:
  - AC-1: Support unlimited approval steps
  - AC-2: Support mixed approval types per step
  - AC-3: Approvers có thể là: user, role, group
  - AC-4: Template validation — không cho phép empty steps

### FR-002: Approval Condition Matching
- **Mô tả**: Map approval templates tới conditions.
- **Conditions** (CEL-based):
  ```cel
  // Match: Production + HIGH risk
  risk.level >= "HIGH" && environment.tier == "PRODUCTION"
  
  // Match: Any DROP statement
  statement.type == "DROP"
  
  // Match: Specific project
  project.name == "projects/critical-app"
  ```
- **Matching Priority**: First match wins (ordered by priority)
- **Acceptance Criteria**:
  - AC-1: Multiple conditions có thể map tới cùng template
  - AC-2: Default template nếu không match condition nào
  - AC-3: Condition evaluation dùng CEL engine

### FR-003: Approval Flow Execution
- **Mô tả**: Execute approval flow khi issue được submit.
- **Flow**:
  ```
  Issue Created → Risk Assessment → Match Approval Template
    → Step 1: Notify approvers → Wait for approval
    → Step 2: Notify approvers → Wait for approval
    → ...
    → All steps approved → Issue transitions to approved
    → Rollout can proceed
  ```
- **States**: PENDING → APPROVED / REJECTED
- **Acceptance Criteria**:
  - AC-1: Steps execute sequentially (step N+1 unlocks after step N approved)
  - AC-2: Rejection at any step blocks entire flow
  - AC-3: Rejector must provide reason
  - AC-4: Approver can delegate to another user
  - AC-5: Auto-escalation after configurable timeout

### FR-004: Approval Notifications
- **Mô tả**: Notify approvers qua multiple channels.
- **Channels**: Email, Slack, DingTalk, Feishu, Microsoft Teams
- **Acceptance Criteria**:
  - AC-1: Notification sent immediately khi step becomes active
  - AC-2: Reminder notification after configurable timeout (e.g., 4 hours)
  - AC-3: Notification chứa: issue summary, risk level, approval link

### FR-005: Approval Bypass & Emergency
- **Mô tả**: Emergency bypass cho critical situations.
- **Acceptance Criteria**:
  - AC-1: Workspace admin có thể bypass approval (với audit trail)
  - AC-2: Bypass requires explicit reason
  - AC-3: Bypass event logged as CRITICAL audit entry

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component                    | File/Package                           | Thay đổi                                          |
|------------------------------|----------------------------------------|----------------------------------------------------|
| Approval Runner              | `backend/runner/approval/`             | Core approval flow engine                          |
| Approval Template Service    | `backend/api/v1/`                      | CRUD approval templates                            |
| Issue Service                | `backend/api/v1/issue_service.go`      | Integrate approval status                          |
| Message Bus                  | `backend/bus/`                         | `ApprovalCheckChan` already exists                 |
| Webhook Manager              | `backend/component/webhook/`           | Approval notifications                             |

### 3.2 Database Changes

```sql
CREATE TABLE approval_template (
    id BIGSERIAL PRIMARY KEY,
    workspace TEXT NOT NULL,
    name TEXT NOT NULL,
    config JSONB NOT NULL,  -- ApprovalTemplateConfig proto JSON
    condition_expression TEXT,  -- CEL expression
    priority INT NOT NULL DEFAULT 0,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_ts TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_ts TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE approval_flow (
    id BIGSERIAL PRIMARY KEY,
    issue_uid BIGINT NOT NULL,
    template_uid BIGINT REFERENCES approval_template(id),
    status TEXT NOT NULL DEFAULT 'PENDING',
    created_ts TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE approval_step (
    id BIGSERIAL PRIMARY KEY,
    flow_uid BIGINT NOT NULL REFERENCES approval_flow(id),
    step_index INT NOT NULL,
    status TEXT NOT NULL DEFAULT 'PENDING',
    approver_uid BIGINT REFERENCES principal(id),
    approved_at TIMESTAMPTZ,
    rejected_reason TEXT,
    created_ts TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

---

## 4. Test Cases

| Test ID    | Mô tả                                                  | Expected Result                       |
|------------|----------------------------------------------------------|---------------------------------------|
| TC-001     | HIGH risk issue → 2-step approval template matched       | Correct template applied              |
| TC-002     | Step 1 approved, step 2 becomes PENDING                  | Sequential execution                  |
| TC-003     | Step rejected → entire flow rejected                     | Issue blocked, reason logged          |
| TC-004     | Emergency bypass by admin                                | Bypassed with audit trail             |
| TC-005     | Auto-escalation after timeout                            | Escalation notification sent          |
| TC-006     | ALL_OF type: 2/3 approved                                | Step still PENDING                    |
| TC-007     | ANY_OF type: 1/3 approved                                | Step APPROVED                         |

---

## 5. Rollout Plan

| Phase   | Mô tả                              | Timeline       |
|---------|--------------------------------------|----------------|
| Phase 1 | Approval template CRUD               | Sprint 1       |
| Phase 2 | Approval flow engine                 | Sprint 2       |
| Phase 3 | Condition matching (CEL)             | Sprint 2       |
| Phase 4 | Notifications + escalation           | Sprint 3       |
| Phase 5 | Emergency bypass                     | Sprint 3       |
| Phase 6 | E2E testing                          | Sprint 4       |
