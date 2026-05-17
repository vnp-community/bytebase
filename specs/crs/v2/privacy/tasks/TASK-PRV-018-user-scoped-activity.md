# TASK-PRV-018 — User-Scoped Activity + Data Minimization

| Field            | Value                                      |
|------------------|----------------------------------------    |
| **Task ID**      | TASK-PRV-018                               |
| **Source**       | SOL-PRV-008 Phase 1–2 (CR-PRV-008)        |
| **Status**       | Pending                                    |
| **Priority**     | P2                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 1–2                                 |

---

## Mô tả

Implement user-scoped activity queries và data minimization cho query text storage.

## Scope

1. **L8 — `store/activity.go`** (modify): Scoped activity queries — non-admin chỉ thấy own activities
2. **L8 — `store/query_history.go`** (modify): User-scoped query history — default own only
3. **Admin access**: Requires explicit justification (logged as meta-audit)
4. **Aggregated views**: Management dashboards show counts/trends, not individual records
5. **L4 — `api/v1/sql_service.go`** (modify): Data minimization — store query hash + structure, not full literal values
6. **Minimization rules**: Login (timestamp+success/fail, not IP geo), Session (duration, not clicks), Telemetry (opt-in only)

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/store/activity.go` | MODIFY — Add visibility scoping |
| `backend/store/query_history.go` | MODIFY — User-scoped queries |
| `backend/api/v1/sql_service.go` | MODIFY — Data minimization |

## Acceptance Criteria

- [ ] Default: user data only visible to owner
- [ ] Admin access requires justification (logged)
- [ ] Aggregated/anonymized views for management dashboards
- [ ] Query text: store hash + redacted structure, not full literals
- [ ] Login: store timestamp + success/fail only
- [ ] Telemetry collection requires user consent
- [ ] User notification khi admin accesses their data (optional)

## Dependencies

- TASK-PRV-012 (Query redactor — reuse for minimization)

## Definition of Done

- Visibility scoping tested with admin/user roles
- Minimization verified: no raw PII in query_history
- Aggregated views validated
