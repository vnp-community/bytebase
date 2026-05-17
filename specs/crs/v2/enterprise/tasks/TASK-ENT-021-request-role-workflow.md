# TASK-ENT-021 — Request Role Workflow

| Field            | Value                                      |
|------------------|--------------------------------------------|
| **Task ID**      | TASK-ENT-021                               |
| **Source**       | SOL-ENT-018 (CR-ENT-018)                  |
| **Status**       | Done                                       |
| **Priority**     | P2                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 1–2                                 |

---

## Mô tả

Xây dựng self-service role request flow tích hợp với IAM Manager. Workflow: user request → auto-route → approve → IAM binding auto-created.

## Scope

### Phase 1 — Sprint 1: Request CRUD + Auto-Grant
1. **Schema Migration**: `role_request` table — requester, target_resource, requested_role, justification, duration_type (PERMANENT/TIME_BOUND), status
2. **L4 — RoleRequestService (NEW)**: `role_request.go` — request CRUD
3. **Auto-Routing**:
   - Project roles → project owner
   - Workspace roles → workspace admin
   - Elevated roles (workspaceAdmin) → multi-level approval
4. **Auto-Grant on Approval**: `ApproveRequest()` → create IAM binding automatically
5. **Time-Bound Option**: Link to JIT access (CR-ENT-017) for temporary roles
6. **L9 — Feature Gate**: `FeatureRequestRoleWorkflow`

### Phase 2 — Sprint 2: Dashboard + Time-Bound
7. **Admin Dashboard**: View pending/approved/rejected requests
8. **Time-Bound Integration**: When `duration_type=TIME_BOUND`, set `ExpiresAt` on IAM binding
9. **Notifications**: Notification on approval/rejection with reason

## Acceptance Criteria

- [x] Role request CRUD functional
- [x] Auto-routing: correct approver based on role scope
- [x] Auto-grant: IAM binding created automatically on approval
- [x] Rejection includes reason, notification sent
- [x] Time-bound roles expire correctly
- [x] Admin dashboard shows all requests with filters
- [x] Self-approval prevention (if same as routing target)
- [x] All role requests audited

## Dependencies

- CR-ENT-011 (Custom Roles) — users can request custom roles
- CR-ENT-017 (JIT Access) — time-bound option links to JIT
- CR-ENT-003 (Audit Log) — all role requests audited

## Definition of Done

- [x] Self-service flow tested end-to-end
- [x] Auto-routing logic validated
- [x] Admin dashboard functional
