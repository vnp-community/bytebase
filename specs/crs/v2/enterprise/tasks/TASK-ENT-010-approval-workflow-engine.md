# TASK-ENT-010 — Approval Template CRUD & Flow Engine

| Field            | Value                                      |
|------------------|--------------------------------------------|
| **Task ID**      | TASK-ENT-010                               |
| **Source**       | SOL-ENT-007 (CR-ENT-007)                  |
| **Status**       | Done                                       |
| **Priority**     | P0                                         |
| **Complexity**   | High                                       |
| **Sprint**       | Sprint 1–4                                 |

---

## Mô tả

Mở rộng Approval Runner (L6) với custom multi-step approval templates, CEL condition matching, sequential step execution, và emergency bypass.

## Scope

### Phase 1 — Sprint 1: Template CRUD
1. **Schema Migration**: `approval_template`, `approval_flow`, `approval_step` tables
2. **L4 — ApprovalTemplateService (NEW)**: Template CRUD với priority ordering
3. **Template Structure**: Name, Steps (ANY_OF/ALL_OF), CEL Condition, Priority

### Phase 2 — Sprint 2: Flow Engine + CEL
4. **L6 — Approval Runner Enhancement**: Multi-step sequential execution
5. **Flow Logic**: Issue Created → Risk Assessment → Bus.ApprovalCheckChan → Match template → Create flow → Step-by-step approval
6. **CEL Condition Matching**: Template matched by risk level, environment tier, project, etc.
7. **Step Execution**: Sequential steps, notify approvers, wait for approval/rejection

### Phase 3 — Sprint 3: Notifications + Emergency Bypass
8. **L5 — Webhook**: Approval request/status notifications
9. **Emergency Bypass**: Workspace admin bypass with explicit reason → `CRITICAL` audit entry
10. **Escalation**: Auto-escalate if step not approved within deadline

### Phase 4 — Sprint 4: E2E Testing
11. Full end-to-end testing of approval workflows

## Acceptance Criteria

- [x] Approval template CRUD functional
- [x] Multi-step sequential approval flow works
- [x] CEL condition matching selects correct template (priority order)
- [x] ANY_OF: any approver can approve; ALL_OF: all must approve
- [x] Webhook notifications sent at each step
- [x] Emergency bypass requires reason + generates CRITICAL audit entry
- [x] Flow status transitions: PENDING → APPROVED/REJECTED
- [x] Issue status = APPROVED when all steps complete → Rollout can proceed

## Dependencies

- TASK-ENT-009 (Risk Assessment) — risk level drives template condition
- CR-ENT-019 (Environment Tiers) — tier in approval conditions
- CR-ENT-011 (Custom Roles) — custom roles as approver targets

## Definition of Done

- [x] Template CRUD + flow engine fully tested
- [x] CEL conditions validated with diverse scenarios
- [x] Emergency bypass audit trail verified
