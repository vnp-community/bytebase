# TASK-AI-003-4: engine.go Map Refactor (11 switches → map)

| Field | Value |
|-------|-------|
| Solution | SOL-AI-003 |
| Priority | P0 |
| Depends On | TASK-AI-003-3 |
| Status | ✅ DONE |
| Completed | 2026-05-09 |
| Verified | 2026-05-10 |
| Est. | M (replace 493 LOC with ~120 LOC) |

## Objective

Replace 11 separate switch statements in `backend/common/engine.go` with a single `EngineCapabilities` struct and `map[storepb.Engine]EngineCapabilities`. Add `init()` exhaustiveness check.

## Delivered

**File**: `backend/common/engine.go` — **280 LOC** (down from ~493, 43% reduction)

### Structure

1. **`EngineCapabilities` struct** — 11 fields (10 bool + 1 string)
2. **`engineCapabilities` map** — 40 engine references, single source of truth
3. **`init()` exhaustiveness check** (line 149) — panics if engine missing from map
4. **10 thin wrapper functions** — `EngineSupportSQLReview()`, `EngineSupportQueryNewACL()`, etc.

### Verification (2026-05-10 re-verified)

```bash
go test ./backend/common/... -run TestEngine -v -count=1  # ✅ PASS (1.964s)
go build ./backend/...                                     # ✅ PASS
go vet ./backend/common/...                                # ✅ PASS
```

## Acceptance Criteria

- [x] All 11 switch statements replaced with map lookups
- [x] `init()` panics if engine missing from map
- [x] engine_test.go (from 003-3) passes unchanged
- [x] All 3 build profiles compile
- [x] LOC reduced from ~493 to ~280 (43% reduction)
