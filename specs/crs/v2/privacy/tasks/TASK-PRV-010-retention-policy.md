# TASK-PRV-010 — Retention Policy Engine + DataCleaner Extension

| Field            | Value                                      |
|------------------|----------------------------------------    |
| **Task ID**      | TASK-PRV-010                               |
| **Source**       | SOL-PRV-004 Phase 1–2 (CR-PRV-004)        |
| **Status**       | Pending                                    |
| **Priority**     | P1                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 1–2                                 |

---

## Mô tả

Xây dựng Retention Policy engine và mở rộng DataCleaner runner (L6) hiện có với policy-driven automated purging.

## Scope

1. **L4 — `api/v1/retention_policy_service.go`**: Retention policy CRUD
2. **L8 — Migration**: Tạo bảng `retention_policy` + seed default policies
3. **L8 — `store/retention_policy.go`**: Policy CRUD store
4. **L6 — `runner/cleaner/retention_cleaner.go`**: RetentionCleaner — policy-driven purging engine
5. **Integration**: Hook RetentionCleaner vào existing DataCleaner.Run() loop
6. **Purge targets**: `query_history` (90d), `audit_log` (365d), `web_refresh_token` (30d), `pii_scan_result` (180d)
7. **Batch purging**: 1000 records/batch, 100ms yield between batches
8. **Audit trail**: Purge events logged
9. **L9 — `feature.go`**: `FeatureDataRetention` gate

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/api/v1/retention_policy_service.go` | NEW — Policy CRUD |
| `backend/store/retention_policy.go` | NEW — Store |
| `backend/store/migration/` | NEW — DDL + seed data |
| `backend/runner/cleaner/retention_cleaner.go` | NEW — Purging engine |
| `backend/runner/cleaner/cleaner.go` | MODIFY — Hook retention cleaner |
| `backend/enterprise/feature.go` | ADD — `FeatureDataRetention` |

## Acceptance Criteria

- [ ] Per-workspace + per-project retention settings
- [ ] Minimum retention enforced (cannot set below regulatory minimum)
- [ ] Batch purging: configurable batch size, yield between batches
- [ ] Purge results logged to audit trail
- [ ] Policy changes audited
- [ ] Default policies seeded via migration
- [ ] Dry-run mode: preview what will be purged

## Dependencies

- None (extends existing DataCleaner)

## Definition of Done

- Purging tested with >10K records
- Batch performance validated
- Audit trail verified
