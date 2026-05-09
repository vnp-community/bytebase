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

- [ ] All `EngineSupportX()` functions delegate to registry
- [ ] Functions marked with `// DEPRECATED` comment
- [ ] API returns correct capabilities per engine
- [ ] Feature matrix endpoint returns all 22 engines
- [ ] Existing callers of `EngineSupportX()` unaffected
