# TASK-SEC-001 — Session & Token Blacklist DB Migration

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-001                               |
| **Source**       | SOL-SEC-001 §3.4                           |
| **Status**       | Pending                                    |
| **Priority**     | P0                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 1                                   |

---

## Mô tả

Tạo DB migration cho `user_session`, `token_blacklist` tables và extend `web_refresh_token` với family tracking columns.

## Scope

1. **Migration file**: Tạo `user_session` table (id, user_uid FK, fingerprint, device_info JSONB, ip_address, created_ts, last_active, expires_at, revoked)
2. **Migration file**: Tạo `token_blacklist` table (jti PK, expires_at)
3. **Indexes**: `idx_session_user`, `idx_session_expiry` (partial WHERE NOT revoked), `idx_blacklist_expiry`
4. **ALTER**: `web_refresh_token` ADD `family_id TEXT`, `rotation_count INT DEFAULT 0`
5. **Store**: `store/session.go` — CRUD: CreateSession, CountActiveSessions, TerminateOldestSession, GetActiveSessions, RevokeAllSessions
6. **Store**: `store/token_blacklist.go` — InsertBlacklistedToken, IsBlacklisted, PurgeExpired

## Acceptance Criteria

- [ ] Migration chạy thành công trên PostgreSQL 14+
- [ ] Store methods có unit tests
- [ ] Indexes verified qua EXPLAIN ANALYZE

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/migrator/migration/` | New migration file |
| `backend/store/session.go` | New file |
| `backend/store/token_blacklist.go` | New file |
| `backend/store/web_refresh_token.go` | Add family_id, rotation_count |

## Definition of Done

- Migration idempotent, rollback-safe
- Store layer 100% covered by unit tests
