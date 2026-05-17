# TASK-SEC-025 — Security Event Bus + Classification

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-025                               |
| **Source**       | SOL-SEC-010 §3.1, §3.2                    |
| **Status**       | Pending                                    |
| **Priority**     | P0                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 1                                   |

---

## Mô tả

Extend existing Bus (L5) với SecurityEventChan. Implement event classification trong Audit Interceptor. Create security_event và siem_target DB tables.

## Scope

1. **Bus extension**: `component/bus/bus.go` — ADD `SecurityEventChan chan *SecurityEvent` (buffer: 5000)
2. **SecurityEvent struct**: ID, Category (AUTH/ACCESS/DATA/CONFIG/SCHEMA/SYSTEM), Severity (CRITICAL-INFO), Timestamp, Actor, Resource, Action, Detail, SourceIP, GeoLocation
3. **Audit Interceptor**: `audit.go` — `classifySecurityEvent(method, err)` → emit to SecurityEventChan (non-blocking select)
4. **Migration**: `security_event` table (id, category, severity, action, actor_uid, actor_email, resource, detail JSONB, source_ip, geo_country, geo_city, created_ts)
5. **Migration**: `siem_target` table (id, name, type, config JSONB, filter JSONB, is_active)
6. **Store**: `store/security_event.go` — CreateSecurityEvent, SearchEvents, CountByCategory

## Acceptance Criteria

- [ ] SecurityEventChan integrated into existing Bus
- [ ] Auth events classified correctly (login_failed, admin_execute, iam_policy_change)
- [ ] Non-blocking emit (channel full → log warning, don't block request)
- [ ] Store CRUD unit tests

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/component/bus/bus.go` | SecurityEventChan |
| `backend/api/v1/audit.go` | Event classification + emit |
| `backend/migrator/migration/` | security_event, siem_target |
| `backend/store/security_event.go` | New file |

## Definition of Done

- Bus extension backward-compatible
- Classification rules tested
