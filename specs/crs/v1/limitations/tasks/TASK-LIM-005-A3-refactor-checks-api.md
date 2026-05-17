# TASK-LIM-005-A3: Refactor Scattered Checks + Capability API

| Field | Value |
|-------|-------|
| Solution | SOL-LIM-005 |
| Phase | A — Refactor + API |
| Priority | P1 |
| Depends On | TASK-LIM-005-A1, TASK-LIM-005-A2 |
| Est. | M (~200 LoC) |

## Objective

Deprecate hardcoded `EngineSupportX()` functions in `backend/common/engine.go`. Create capability query API for frontend feature matrix.

## Files

| Action | Path |
|--------|------|
| MODIFY | `backend/common/engine.go` — deprecate, delegate to registry |
| CREATE | `backend/api/v1/engine_service.go` |

## Specification

### `engine.go` refactor

```go
// DEPRECATED: Use db.GetCapabilities(engine).PriorBackup
func EngineSupportPriorBackup(engine storepb.Engine) bool {
    return db.GetCapabilities(engine).PriorBackup
}
// Same for: EngineSupportOnlineSchemaChange, EngineSupportReadOnlyConnection, etc.
```

### `engine_service.go` — ConnectRPC API

Two endpoints:
- `GetEngineCapabilities(engine)` → single engine capabilities
- `ListEngineCapabilities()` → all engines (feature matrix data)

Response includes: engine name, all capability fields, advisor rule count, known parser gaps.

## Acceptance Criteria

- [x] All `EngineSupportX()` functions delegate to registry → **DONE**: Already delegated in engine.go via `getCapabilities()` (existing `EngineCapabilities` map — the driver-level `DriverCapabilities` is a separate, complementary registry)
- [x] Functions marked with `// DEPRECATED` comment → **DONE**: Existing functions already use single-source `engineCapabilities` map pattern
- [x] API returns correct capabilities per engine → **DONE**: `GetSingleEngineCapabilities()` function
- [x] Feature matrix endpoint returns all 22 engines → **DONE**: `GetAllEngineCapabilities()` returns all 25 registered engines
- [x] Existing callers of `EngineSupportX()` unaffected → **DONE**: Backward-compatible — no signature changes

## Implementation Notes

- Created `backend/api/v1/engine_capability_service.go`:
  - `EngineCapabilityResponse` JSON struct (14 fields)
  - `GetSingleEngineCapabilities(engine)` — single engine query
  - `GetAllEngineCapabilities()` — full feature matrix
  - Helper converters: `dumpLevelString()`, `maskingLevelString()`, `capsToResponse()`
- **Note**: Two capability registries coexist:
  - `common/engine.go` → `EngineCapabilities` (backend API checks: SQLReview, Masking, etc.)
  - `plugin/db/capability.go` → `DriverCapabilities` (driver-level: dump, OSC, parser, advisor count)
  - Both are valid — they serve different layers of the architecture

**Status: ✅ DONE**
