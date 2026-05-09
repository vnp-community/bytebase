# T-011-03: Runtime Engine Discovery API

| Field | Value |
|---|---|
| **Task ID** | T-011-03 |
| **Solution** | SOL-ARCH-011 |
| **Priority** | P3 |
| **Depends On** | T-011-01 |
| **Target Files** | `backend/api/v1/actuator_service.go`, `backend/plugin/db/registry.go` |
| **Type** | Modify existing |

---

## Objective

Expose which DB engines are available in the running binary via the Actuator API. Frontend can adapt UI based on compiled engines.

## Implementation

### 1. Registry — add `RegisteredEngines()`

```go
// plugin/db/registry.go
func RegisteredEngines() []storepb.Engine {
    registryMu.RLock()
    defer registryMu.RUnlock()
    engines := make([]storepb.Engine, 0, len(registry))
    for engine := range registry {
        engines = append(engines, engine)
    }
    return engines
}
```

### 2. Actuator — include in response

```go
// api/v1/actuator_service.go
func (s *ActuatorService) GetActuatorInfo(...) (*v1pb.ActuatorInfo, error) {
    info := &v1pb.ActuatorInfo{
        // ... existing fields ...
        AvailableEngines: db.RegisteredEngines(),
    }
    return info, nil
}
```

## Acceptance Criteria

- [ ] `RegisteredEngines()` returns list of compiled engines
- [ ] `/v1/actuator/info` includes `available_engines`
- [ ] Full build shows all 23; minimal shows PG only
- [ ] `go build ./backend/...` passes
