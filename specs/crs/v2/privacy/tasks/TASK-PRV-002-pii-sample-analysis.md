# TASK-PRV-002 — PII Sample Data Analysis + DB Driver Integration

| Field            | Value                                      |
|------------------|----------------------------------------    |
| **Task ID**      | TASK-PRV-002                               |
| **Source**       | SOL-PRV-001 Phase 2 (CR-PRV-001)          |
| **Status**       | Pending                                    |
| **Priority**     | P0                                         |
| **Complexity**   | High                                       |
| **Sprint**       | Sprint 2                                   |

---

## Mô tả

Mở rộng PII Scanner với sample data analysis — query max 100 rows qua DB Driver plugin (L7) để deep-detect PII bằng data pattern matching.

## Scope

1. **L5 — `scanner.go` enhancement**: Sample analysis mode — dùng `dbFactory.GetDriver()` + `QueryConn()` để lấy sample rows
2. **L5 — `patterns.go` enhancement**: `AnalyzeSamples()` — regex patterns cho data values (email format, phone format, SSN patterns)
3. **L7 — DB Driver integration**: Read-only `QueryConn` cho sampling, `LIMIT 100`, hỗ trợ 22+ engines
4. **L4 — `api/v1/pii_discovery_service.go`**: gRPC service — `StartScan`, `GetScanJob`, `ListScanResults`, `ConfirmClassification`, `GetPIIInventorySummary`
5. **Proto**: `proto/v1/v1/pii_discovery_service.proto`

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/component/privacy/scanner.go` | MODIFY — Add sample analysis |
| `backend/component/privacy/patterns.go` | MODIFY — Add data pattern matching |
| `backend/api/v1/pii_discovery_service.go` | NEW — gRPC service |
| `proto/v1/v1/pii_discovery_service.proto` | NEW — Proto definitions |

## Acceptance Criteria

- [ ] Sample analysis queries max 100 rows per column
- [ ] Sample data NEVER persisted — processed in-memory only
- [ ] Confidence score combines column name + data analysis
- [ ] gRPC APIs functional: StartScan, GetScanJob, ListScanResults
- [ ] ConfirmClassification allows human override (is_confirmed flag)
- [ ] Read-only DB connections for sampling

## Dependencies

- TASK-PRV-001 (Pattern registry + store)

## Definition of Done

- Sample analysis tested with at least 3 DB engines (PostgreSQL, MySQL, MongoDB)
- gRPC service integration tested
- Sample data leak prevention verified
