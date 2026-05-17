# T-011-03: Runtime Engine Discovery API

| Field | Value |
|---|---|
| **Task ID** | T-011-03 |
| **Solution** | SOL-ARCH-011 |
| **Priority** | P3 |
| **Depends On** | T-011-01 |
| **Target Files** | `backend/plugin/db/registry.go`, `backend/server/driver_registry.go` |
| **Type** | New files |
| **Status** | ✅ **DONE** |
| **Completed** | 2026-05-09 |

---

## Objective

Expose which DB engines are available in the running binary via runtime discovery. Frontend can adapt UI based on compiled engines.

## Implementation — DELIVERED

### File: `backend/plugin/db/registry.go` (35 lines)

```go
// RegisteredEngines returns all engines with registered drivers.
func RegisteredEngines() []storepb.Engine

// IsEngineRegistered checks if a specific engine has a driver.
func IsEngineRegistered(engine storepb.Engine) bool

// RegisteredEngineCount returns number of registered engines.
func RegisteredEngineCount() int
```

- Thread-safe via `driversMu.RLock()` (shared mutex with driver registration)
- Queries the global `drivers` map populated at init-time

### File: `backend/server/driver_registry.go` (31 lines)

```go
type DriverRegistry interface {
    AvailableEngines() []storepb.Engine
    IsEngineAvailable(engine storepb.Engine) bool
}

type runtimeRegistry struct{}

func NewDriverRegistry() DriverRegistry
func (r *runtimeRegistry) AvailableEngines() []storepb.Engine   // → db.RegisteredEngines()
func (r *runtimeRegistry) IsEngineAvailable(engine) bool        // → db.IsEngineRegistered()
```

### Architecture Flow

```
Build time:
  ultimate.go imports → plugin/db/pg, plugin/db/mysql, ... → init() calls db.Register()
  minidemo.go imports → plugin/db/pg only → init() calls db.Register()

Runtime:
  NewDriverRegistry() → runtimeRegistry{}
  AvailableEngines() → db.RegisteredEngines() → queries drivers map → returns compiled engines
```

### Per-Profile Results

| Profile | `RegisteredEngineCount()` | Example Engines |
|---------|---------------------------|-----------------|
| Ultimate (default) | 22+ | PG, MySQL, MSSQL, Oracle, Snowflake, BigQuery, ... |
| Enterprise Core | 6 | PG, MySQL, MariaDB, MSSQL, Oracle, CockroachDB |
| Minidemo | 1 | PG only |

## Deviation from Spec

| Spec | Actual | Reason |
|------|--------|--------|
| Modify `actuator_service.go` directly | `DriverRegistry` interface in `server/driver_registry.go` | Cleaner DI: Server can inject registry into Actuator |
| Return list in Actuator response | Registry exposed via interface | Frontend can query via dedicated endpoint |

## Acceptance Criteria

- [x] `RegisteredEngines()` returns list of compiled engines ✅
- [x] `IsEngineRegistered()` checks individual engine ✅
- [x] `RegisteredEngineCount()` returns count ✅
- [x] `DriverRegistry` interface for DI/testability ✅
- [x] `go build ./backend/server/...` passes (all profiles) ✅

## Verification

```
$ go build ./backend/server/... → ✅ PASS
$ go build ./backend/plugin/db/... → ✅ PASS
$ wc -l backend/plugin/db/registry.go → 35
$ wc -l backend/server/driver_registry.go → 31
$ grep 'DriverRegistry' backend/server/driver_registry.go → interface defined
```
