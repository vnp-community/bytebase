# T-003-05: Self-Healing Runner

| Field | Value |
|---|---|
| **Task ID** | T-003-05 |
| **Solution** | SOL-AVAIL-003 |
| **Depends On** | T-003-01 |
| **Target Files** | `backend/runner/selfheal/runner.go` (NEW), `backend/server/server.go` (Modify) |

---

## Objective

Tạo runner tự động khắc phục: purge cache khi pool exhaustion, force GC khi memory pressure.

## Implementation

Xem SOL-AVAIL-003 §2.5. Tóm tắt:
- 30s ticker loop
- Gọi `healthChecker.RunAll()`
- PG DEGRADED → `store.DeleteCache()`
- Memory DEGRADED → `store.DeleteCache()` + `runtime.GC()` + `debug.FreeOSMemory()`
- Wire vào `Server.Run()` (HA mode only)

## Acceptance Criteria

- [ ] Runner follows standard pattern: `Run(ctx, *sync.WaitGroup)`
- [ ] Uses existing `store.DeleteCache()` for pool healing
- [ ] Only runs in HA mode (`profile.HA`)
- [ ] `go build ./backend/...` passes
