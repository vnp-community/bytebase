# TASK-PRV-001 — PII Pattern Registry + Column Name Scanner

| Field            | Value                                      |
|------------------|----------------------------------------    |
| **Task ID**      | TASK-PRV-001                               |
| **Source**       | SOL-PRV-001 Phase 1 (CR-PRV-001)          |
| **Status**       | Pending                                    |
| **Priority**     | P0                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 1                                   |

---

## Mô tả

Xây dựng PII pattern registry và column name matching engine — nền tảng cho toàn bộ PII Discovery system.

## Scope

1. **L5 — `component/privacy/patterns.go`**: PII pattern registry với 10+ built-in regex patterns (email, phone, national_id, credentials, financial, full_name, address, dob, ip_device)
2. **L5 — `component/privacy/scanner.go`**: PIIScanner struct + `Scan()` method — column name matching only (Phase 1)
3. **L8 — Migration**: Tạo bảng `pii_scan_result` + `pii_scan_job` + indexes
4. **L8 — `store/pii_inventory.go`**: CRUD cho scan results + jobs (`UpsertPIIScanResults`, `ListPIIScanResults`, `CreateScanJob`)
5. **L9 — `feature.go`**: `FeaturePIIDiscovery` gate

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/component/privacy/patterns.go` | NEW — Pattern registry |
| `backend/component/privacy/scanner.go` | NEW — Scanner engine (column name mode) |
| `backend/store/pii_inventory.go` | NEW — Store CRUD |
| `backend/store/migration/` | NEW — DDL migration |
| `backend/enterprise/feature.go` | ADD — `FeaturePIIDiscovery` |

## Acceptance Criteria

- [ ] 10+ PII category patterns: EMAIL, PHONE, NATIONAL_ID, CREDENTIALS, FINANCIAL, FULL_NAME, ADDRESS, DOB, IP_DEVICE, HEALTH
- [ ] Column name matching returns confidence score (0–1.0)
- [ ] `pii_scan_result` + `pii_scan_job` tables created with proper indexes
- [ ] Store CRUD functional: upsert, list, filter by category/database
- [ ] Feature gate blocks non-ENTERPRISE access
- [ ] Unit tests cho PatternRegistry.MatchColumns()

## Dependencies

- None (foundation task)

## Definition of Done

- Pattern registry tested with edge cases (multilingual column names)
- Migration script validated
- Scanner unit tests pass
