# TASK-ENT-005 — Audit Log Search & Export APIs

| Field            | Value                                      |
|------------------|--------------------------------------------|
| **Task ID**      | TASK-ENT-005                               |
| **Source**       | SOL-ENT-003 (CR-ENT-003)                  |
| **Status**       | Done                                       |
| **Priority**     | P0                                         |
| **Complexity**   | High                                       |
| **Sprint**       | Sprint 2                                   |

---

## Mô tả

Triển khai `SearchAuditLogs` và `ExportAuditLogs` gRPC APIs với CEL filter support, cursor-based pagination, và performance indexes.

## Scope

### Backend
1. **Proto**: Định nghĩa `SearchAuditLogsRequest/Response`, `ExportAuditLogsRequest/Response` trong `audit_log_service.proto`
2. **L4 — Service**: Implement `SearchAuditLogs()` — CEL filter parsing, cursor pagination, max 1000 per page
3. **L4 — Service**: Implement `ExportAuditLogs()` — JSON/CSV export, async processing cho >10K records
4. **L8 — Store**: Optimized queries + pagination trong `store/audit_log.go`
5. **Database Indexes** (migration):
   - `idx_audit_log_created_ts` — `created_ts DESC`
   - `idx_audit_log_actor` — `user_uid`
   - `idx_audit_log_method` — `method`
   - `idx_audit_log_search` — composite `(created_ts DESC, user_uid, method)`
6. **Permission**: `bb.auditLogs.search` required
7. **Rate Limiting**: Export endpoint rate limited

### Frontend
8. **AuditLog.vue**: Full search UI với filters
9. **AuditFilter.vue**: CEL-based filter builder
10. **AuditExport.vue**: Export dialog (JSON/CSV format selection)

## Acceptance Criteria

- [x] CEL filter parsing hoạt động (actor, timestamp, method, status)
- [x] Cursor-based pagination functional, max 1000/page
- [x] Export JSON/CSV functional
- [x] Async export cho >10K records
- [x] Database indexes created via migration
- [x] Permission enforcement cho search API
- [x] Rate limiting trên export endpoint
- [x] Frontend search/filter/export UI functional

## Dependencies

- TASK-ENT-004 (Audit Interceptor Expansion)

## Definition of Done

- [x] APIs functional + tested
- [x] Indexes verified with EXPLAIN ANALYZE
- [x] Frontend UI completed
