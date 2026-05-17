# TASK-PRV-003 — SchemaSync Hook + Incremental PII Scanning

| Field            | Value                                      |
|------------------|----------------------------------------    |
| **Task ID**      | TASK-PRV-003                               |
| **Source**       | SOL-PRV-001 Phase 3 (CR-PRV-001)          |
| **Status**       | Pending                                    |
| **Priority**     | P1                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 2                                   |

---

## Mô tả

Hook PII Scanner vào SchemaSync runner (L6) để tự động trigger incremental scan khi schema thay đổi.

## Scope

1. **L6 — `runner/schemasync/syncer.go`** (modify): Thêm `piiScanner` field, gọi `Scan()` async sau khi schema sync hoàn thành
2. **Column diff detection**: `diffColumns(oldSchema, newSchema)` — phát hiện new table, new column, column rename, type change
3. **Incremental scan**: Chỉ scan changed columns, skip confirmed classifications
4. **Feature gate check**: Chỉ trigger khi `FeaturePIIDiscovery` enabled

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/runner/schemasync/syncer.go` | MODIFY — Add PII scan hook |

## Acceptance Criteria

- [ ] Incremental scan triggered on: new table, new column, column rename, type change
- [ ] Scan chạy async (goroutine), không block schema sync
- [ ] ≤ 30s cho typical schema change
- [ ] Skip columns đã có `is_confirmed=true`
- [ ] Feature gate check trước khi trigger

## Dependencies

- TASK-PRV-001, TASK-PRV-002

## Definition of Done

- Integration test: schema sync → PII scan triggered
- Performance test: scan overhead ≤ 30s
