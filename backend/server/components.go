package server

import (
	"fmt"
	"sync"
	"time"
)

// ComponentClass defines the criticality of a server component.
type ComponentClass int

const (
	// Critical — server MUST abort if initialization fails.
	Critical ComponentClass = iota
	// Important — server starts degraded, component retries in background.
	Important
	// Optional — server starts without it, feature disabled.
	Optional
)

// String returns a human-readable class name.
func (c ComponentClass) String() string {
	switch c {
	case Critical:
		return "critical"
	case Important:
		return "important"
	case Optional:
		return "optional"
	default:
		return fmt.Sprintf("unknown(%d)", int(c))
	}
}

// ComponentStatus tracks the health of an individual component.
type ComponentStatus struct {
	Name      string         `json:"name"`
	Class     ComponentClass `json:"-"`
	ClassStr  string         `json:"class"`
	Status    string         `json:"status"` // "initializing", "healthy", "degraded", "disabled", "failed"
	Error     error          `json:"-"`
	ErrorMsg  string         `json:"error,omitempty"`
	StartedAt time.Time      `json:"started_at,omitempty"`
}

// ComponentRegistry tracks all server components and their health.
type ComponentRegistry struct {
	mu         sync.RWMutex
	components map[string]*ComponentStatus
}

// NewComponentRegistry creates a new component registry.
func NewComponentRegistry() *ComponentRegistry {
	return &ComponentRegistry{
		components: make(map[string]*ComponentStatus),
	}
}

// Register adds a component to the registry with the given class.
func (r *ComponentRegistry) Register(name string, class ComponentClass) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.components[name] = &ComponentStatus{
		Name:     name,
		Class:    class,
		ClassStr: class.String(),
		Status:   "initializing",
	}
}

// SetHealthy marks a component as healthy.
func (r *ComponentRegistry) SetHealthy(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.components[name]; ok {
		c.Status = "healthy"
		c.Error = nil
		c.ErrorMsg = ""
		c.StartedAt = time.Now()
	}
}

// SetDegraded marks a component as degraded (partially working).
func (r *ComponentRegistry) SetDegraded(name string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.components[name]; ok {
		c.Status = "degraded"
		c.Error = err
		if err != nil {
			c.ErrorMsg = err.Error()
		}
	}
}

// SetFailed marks a component as failed.
func (r *ComponentRegistry) SetFailed(name string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.components[name]; ok {
		c.Status = "failed"
		c.Error = err
		if err != nil {
			c.ErrorMsg = err.Error()
		}
	}
}

// SetDisabled marks a component as disabled (init failed, feature unavailable).
func (r *ComponentRegistry) SetDisabled(name string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.components[name]; ok {
		c.Status = "disabled"
		c.Error = err
		if err != nil {
			c.ErrorMsg = err.Error()
		}
	}
}

// IsReady returns true if all Critical components are healthy.
func (r *ComponentRegistry) IsReady() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, c := range r.components {
		if c.Class == Critical && c.Status != "healthy" {
			return false
		}
	}
	return true
}

// HealthReport returns a snapshot of all component statuses.
func (r *ComponentRegistry) HealthReport() map[string]*ComponentStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string]*ComponentStatus, len(r.components))
	for k, v := range r.components {
		cp := *v
		result[k] = &cp
	}
	return result
}
