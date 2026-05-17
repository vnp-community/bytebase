# TASK-ENT-014 — Custom Roles & Permission Catalog

| Field            | Value                                      |
|------------------|--------------------------------------------|
| **Task ID**      | TASK-ENT-014                               |
| **Source**       | SOL-ENT-011 (CR-ENT-011)                  |
| **Status**       | Done                                       |
| **Priority**     | P1                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 1–3                                 |

---

## Mô tả

Mở rộng IAM Manager (L5) để resolve custom role permissions. Tận dụng `role` table và `GetRoleSnapshot()` đã có.

## Scope

### Phase 1 — Sprint 1: Custom Role CRUD + Permission Catalog
1. **L4 — RoleService Enhancement**: Custom role CRUD with validation
2. **Schema Migration**: Add `is_custom`, `scope` columns to `role` table
3. **Role Naming**: `roles/custom.{name}` convention
4. **Permission Catalog API**: Expose all permissions grouped by category (`bb.databases.*`, `bb.instances.*`, etc.) with descriptions
5. **L1 — Frontend**: Permission catalog UI, role creation form
6. **L9 — Feature Gate**: `FeatureCustomRoles`

### Phase 2 — Sprint 2: IAM Integration
7. **L5 — IAM Manager Enhancement**: `resolvePermissions()` — lookup custom roles from store
8. **L3 — ACL**: Custom role permissions evaluated in ACL check
9. **Validation Rules**: Cannot delete if assigned, permission dependencies enforced, scope validation

### Phase 3 — Sprint 3: Comparison + Audit
10. **Role Comparison UI**: Side-by-side comparison of role permissions
11. **Audit**: Role creation/update/delete logged

## Acceptance Criteria

- [x] Custom role CRUD functional (create, update, delete, list)
- [x] `roles/custom.{name}` naming convention enforced
- [x] Permission catalog API returns all permissions with metadata
- [x] IAM Manager resolves custom role permissions correctly
- [x] ACL interceptor evaluates custom role permissions
- [x] Cannot delete role with active assignments
- [x] Permission dependencies enforced
- [x] Scope validation: workspace-scope ≠ project-level assignment
- [x] Role comparison UI functional
- [x] All changes audited

## Dependencies

- CR-ENT-017 (JIT Access) — JIT requests custom roles
- CR-ENT-018 (Request Role Workflow) — requests custom roles
- CR-ENT-007 (Approval Workflow) — custom roles as approver targets

## Definition of Done

- [x] Custom roles fully integrated into IAM pipeline
- [x] Permission catalog comprehensive
- [x] Validation rules all tested
