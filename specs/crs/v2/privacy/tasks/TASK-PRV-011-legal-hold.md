# TASK-PRV-011 — Legal Hold + Retention Dashboard

| Field            | Value                                      |
|------------------|----------------------------------------    |
| **Task ID**      | TASK-PRV-011                               |
| **Source**       | SOL-PRV-004 Phase 3–4 (CR-PRV-004)        |
| **Status**       | Pending                                    |
| **Priority**     | P1                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 3                                   |

---

## Mô tả

Xây dựng Legal Hold mechanism và Retention Compliance Dashboard.

## Scope

1. **L4 — `api/v1/legal_hold_service.go`**: Legal hold CRUD — create, release, list
2. **L8 — Migration**: Tạo bảng `legal_hold`
3. **Legal hold scopes**: USER, PROJECT, DATABASE, WORKSPACE
4. **Hold integration**: RetentionCleaner checks legal holds trước khi purge — skip held data
5. **Hold approval**: Create/release requires admin approval + audit
6. **L1 — `RetentionDashboard.tsx`**: Compliance dashboard — data volumes by retention status, overdue alerts, active legal holds, storage trends

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/api/v1/legal_hold_service.go` | NEW — Legal hold service |
| `backend/store/retention_policy.go` | MODIFY — Add legal hold CRUD |
| `backend/store/migration/` | NEW — DDL migration |
| `backend/runner/cleaner/retention_cleaner.go` | MODIFY — Legal hold check |
| `frontend/src/react/pages/privacy/RetentionDashboard.tsx` | NEW — Dashboard |

## Acceptance Criteria

- [ ] Legal hold prevents ALL automated purging for in-scope data
- [ ] Hold scopes: per user, project, database, workspace
- [ ] Hold duration: indefinite or time-limited
- [ ] Create/release requires admin approval
- [ ] Hold status visible in dashboard
- [ ] Real-time metrics: data volumes, overdue alerts
- [ ] Export compliance reports

## Dependencies

- TASK-PRV-010 (Retention policy engine)

## Definition of Done

- Legal hold blocks purge verified
- Dashboard functional
- Compliance reports reviewed
