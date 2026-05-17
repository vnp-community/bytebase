# TASK-ENT-006 — Audit Log Retention & Immutability

| Field            | Value                                      |
|------------------|--------------------------------------------|
| **Task ID**      | TASK-ENT-006                               |
| **Source**       | SOL-ENT-003 (CR-ENT-003)                  |
| **Status**       | Done                                       |
| **Priority**     | P1                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 3                                   |

---

## Mô tả

Triển khai configurable retention policy và immutability enforcement cho audit logs.

## Scope

1. **L6 — Retention Purge**: `runner/cleaner/audit_cleaner.go` — purge expired audit logs
   - ENTERPRISE: 365 ngày, TEAM: 90 ngày
   - Configurable qua workspace settings
   - `PurgeAuditLogsBefore(ctx, cutoff)` store method
2. **Immutability Enforcement**:
   - API level: Không expose `UpdateAuditLog` / `DeleteAuditLog` endpoints
   - Database level: Application user chỉ có `INSERT` và `SELECT`
   - Purge: Chỉ `DataCleaner` runner (system context) có quyền `DELETE`
3. **Compliance validation**: E2E tests verify immutability

## Acceptance Criteria

- [x] Retention purge chạy đúng schedule
- [x] ENTERPRISE: 365 ngày retention
- [x] TEAM: 90 ngày retention
- [x] Configurable retention period qua settings
- [x] No UPDATE/DELETE APIs exposed
- [x] Database permissions restrict app user to INSERT/SELECT only
- [x] Only DataCleaner system context can DELETE
- [x] E2E compliance tests pass

## Dependencies

- TASK-ENT-004, TASK-ENT-005

## Definition of Done

- [x] Retention runner functional
- [x] Immutability verified via integration tests
- [x] Compliance audit checklist completed
