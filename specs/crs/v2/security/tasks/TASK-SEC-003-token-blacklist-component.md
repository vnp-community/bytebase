# TASK-SEC-003 — Token Blacklist Component

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-003                               |
| **Source**       | SOL-SEC-001 §3.3                           |
| **Status**       | Pending                                    |
| **Priority**     | P0                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 2                                   |

---

## Mô tả

Implement in-memory token blacklist (L5) synced với PostgreSQL, tích hợp vào server bootstrap sequence.

## Scope

1. **TokenBlacklist struct**: `component/auth/blacklist.go` — `sync.RWMutex`, memory map `[JTI → expiry]`, 30s sync ticker
2. **Methods**: `Revoke(jti, expiry)`, `IsBlacklisted(jti)`, `cleanup()` goroutine
3. **PG sync**: Async persist on Revoke, load from DB on startup
4. **Bootstrap**: Đăng ký vào server startup sequence (TDD §2, after step 5)
5. **Memory cap**: Max 100K entries, LRU eviction khi đầy

## Acceptance Criteria

- [ ] Revoke → IsBlacklisted returns true
- [ ] Cleanup removes expired JTIs
- [ ] Startup loads existing blacklist from DB
- [ ] Memory bounded at 100K entries
- [ ] Concurrent access safe (RWMutex)

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/component/auth/blacklist.go` | New file |
| `backend/server/server.go` | Bootstrap integration |

## Definition of Done

- Unit tests + race condition test (`go test -race`)
