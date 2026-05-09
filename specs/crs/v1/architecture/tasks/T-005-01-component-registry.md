# T-005-01: Component Registry

| Field | Value |
|---|---|
| **Task ID** | T-005-01 |
| **Solution** | SOL-ARCH-005 |
| **Priority** | P2 |
| **Depends On** | None |
| **Target File** | `backend/server/components.go` |
| **Type** | New file |

---

## Objective

Implement `ComponentRegistry` for tracking component health with Critical/Important/Optional classification. Used by health checks and graceful bootstrap.

## Implementation

```go
package server

type ComponentClass int
const (
    Critical  ComponentClass = iota  // abort if fails
    Important                        // retry in background
    Optional                         // disable if fails
)

type ComponentStatus struct {
    Name    string
    Class   ComponentClass
    Status  string    // healthy|degraded|disabled|failed
    Error   error
    StartedAt time.Time
}

type ComponentRegistry struct {
    mu         sync.RWMutex
    components map[string]*ComponentStatus
}

func NewComponentRegistry() *ComponentRegistry
func (r *ComponentRegistry) Register(name string, class ComponentClass)
func (r *ComponentRegistry) SetHealthy(name string)
func (r *ComponentRegistry) SetFailed(name string, err error)
func (r *ComponentRegistry) SetDisabled(name string, err error)
func (r *ComponentRegistry) IsReady() bool             // all Critical healthy?
func (r *ComponentRegistry) HealthReport() map[string]*ComponentStatus
```

## Acceptance Criteria

- [ ] `ComponentRegistry` with Register/SetHealthy/SetFailed/SetDisabled
- [ ] `IsReady()` returns false if any Critical component is not healthy
- [ ] Thread-safe (RWMutex)
- [ ] Unit tests for state transitions
