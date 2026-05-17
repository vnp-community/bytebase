# T-005-01: Component Registry

| Field | Value |
|---|---|
| **Task ID** | T-005-01 |
| **Solution** | SOL-ARCH-005 |
| **Priority** | P2 |
| **Depends On** | None |
| **Target File** | `backend/server/components.go` |
| **Type** | New file |
| **Status** | ✅ **DONE** |
| **Completed** | 2026-05-09 |

---

## Objective

Implement `ComponentRegistry` for tracking component health with Critical/Important/Optional classification. Used by health checks and graceful bootstrap.

## Implementation — DELIVERED

### File: `backend/server/components.go` (144 lines)

### Types

```go
type ComponentClass int
const (
    Critical  ComponentClass = iota  // abort if fails
    Important                        // retry in background
    Optional                         // disable if fails
)

type ComponentStatus struct {
    Name      string         `json:"name"`
    Class     ComponentClass `json:"-"`
    Status    string         `json:"status"`     // healthy|degraded|disabled|failed
    Error     error          `json:"-"`
    ErrorMsg  string         `json:"error,omitempty"`
    StartedAt time.Time      `json:"started_at"`
}

type ComponentRegistry struct {
    mu         sync.RWMutex
    components map[string]*ComponentStatus
}
```

### API

| Method | Description |
|--------|-------------|
| `NewComponentRegistry()` | Creates empty registry |
| `Register(name, class)` | Registers component with criticality class |
| `SetHealthy(name)` | Marks component as healthy |
| `SetDegraded(name, err)` | Marks component as degraded (running with issues) |
| `SetFailed(name, err)` | Marks component as failed |
| `SetDisabled(name, err)` | Marks component as disabled (intentionally off) |
| `IsReady() bool` | Returns `true` only if ALL `Critical` components are healthy |
| `HealthReport()` | Returns full snapshot of all component statuses |

### Thread Safety

- All state mutations guarded by `sync.RWMutex`
- `HealthReport()` returns a copy, not a reference to internal state

## Acceptance Criteria

- [x] `ComponentRegistry` with Register/SetHealthy/SetFailed/SetDisabled ✅
- [x] `IsReady()` returns false if any Critical component is not healthy ✅
- [x] Thread-safe (RWMutex) ✅
- [x] `ComponentClass.String()` for logging ✅
- [x] `go build ./backend/server/...` passes ✅

## Verification

```
$ go build ./backend/server/... → ✅ PASS
$ wc -l backend/server/components.go → 144
$ grep -c 'func ' backend/server/components.go → 9
```
