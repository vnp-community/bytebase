# TASK-PRV-014 — Export Policy Engine + Rate Limiter

| Field            | Value                                      |
|------------------|----------------------------------------    |
| **Task ID**      | TASK-PRV-014                               |
| **Source**       | SOL-PRV-006 Phase 1, 3 (CR-PRV-006)       |
| **Status**       | Pending                                    |
| **Priority**     | P0                                         |
| **Complexity**   | High                                       |
| **Sprint**       | Sprint 1–2                                 |

---

## Mô tả

Xây dựng Export Policy Engine và Rate Limiter cho DLP — ngăn chặn data exfiltration qua export.

## Scope

1. **L5 — `component/export/policy.go`**: ExportPolicyEngine — `Evaluate()` method, classification-based decision (AutoApprove / ApprovalNeeded / Blocked)
2. **L5 — `component/export/rate_limiter.go`**: ExportRateLimiter — in-memory counters, per-user/per-day limits
3. **Rate limits**: Max rows/request (10K), max exports/day (10), max volume/day (100MB)
4. **L8 — Migration**: Tạo bảng `export_policy` + `export_audit`
5. **L8 — `store/export_policy.go`**: Policy + rate limit store
6. **L4 — `api/v1/sql_service.go`** (modify): Integrate policy check + rate limiter vào Export flow
7. **L9 — `feature.go`**: `FeatureExportDLP` gate

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/component/export/policy.go` | NEW — Export policy engine |
| `backend/component/export/rate_limiter.go` | NEW — Rate limiter |
| `backend/store/export_policy.go` | NEW — Store |
| `backend/store/migration/` | NEW — DDL migration |
| `backend/api/v1/sql_service.go` | MODIFY — Integrate DLP |
| `backend/enterprise/feature.go` | ADD — `FeatureExportDLP` |

## Acceptance Criteria

- [ ] Auto-approve cho non-sensitive data exports
- [ ] Approval required cho PII/L2+ classified columns
- [ ] Hard block cho L4 restricted data
- [ ] Rate limits: max rows, max exports/day, max volume/day
- [ ] Rate limits configurable per role/project
- [ ] Limit breach: real-time alert + export blocked
- [ ] Counter per user UID (not per session)

## Dependencies

- TASK-ENT-016 (Data Classification) — for classification-based decisions
- TASK-ENT-015 (Data Masking) — for masked export mode

## Definition of Done

- Policy engine tested with L1–L4 classifications
- Rate limiter tested with concurrent users
- Export blocked when limits exceeded
