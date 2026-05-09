# TASK-AI-003-5: DriverRegistry Interface

| Field | Value |
|-------|-------|
| Solution | SOL-AI-003 |
| Priority | P2 |
| Depends On | TASK-AI-003-4 |
| Status | ✅ DONE |
| Completed | 2025-05-09 |
| Est. | S (~50 LoC) |

## Objective

Create a `DriverRegistry` interface exposing which database drivers are compiled into the current binary.

## Delivered

**File**: `backend/server/driver_registry.go`

```go
type DriverRegistry interface {
    AvailableEngines() []storepb.Engine
    IsEngineAvailable(engine storepb.Engine) bool
}
```

Concrete `runtimeRegistry` delegates to `db.RegisteredEngines()` and `db.IsEngineRegistered()`.

## Verification

```bash
go build ./backend/server/...  # ✅ PASS
```

## Acceptance Criteria

- [x] Interface + concrete impl compiled
- [x] Returns correct engines for current build profile
- [x] No existing callers — additive only
