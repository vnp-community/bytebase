package server

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v5"
)

// HealthStatus represents overall system health.
type HealthStatus struct {
	Status     string                      `json:"status"` // healthy, degraded, unhealthy
	Version    string                      `json:"version,omitempty"`
	Uptime     int64                       `json:"uptime_seconds"`
	Components map[string]*ComponentHealth `json:"components"`
}

// ComponentHealth represents a single component's health.
type ComponentHealth struct {
	Status    string `json:"status"`              // healthy, degraded, unhealthy, disabled
	LatencyMs int64  `json:"latency_ms,omitempty"`
	Message   string `json:"message,omitempty"`
}

// healthChecker performs health checks with result caching.
type healthChecker struct {
	s         *Server
	mu        sync.RWMutex
	lastCheck *HealthStatus
	lastTime  time.Time
	cacheTTL  time.Duration
}

func newHealthChecker(s *Server) *healthChecker {
	return &healthChecker{
		s:        s,
		cacheTTL: 5 * time.Second,
	}
}

// check performs health checks with caching to prevent check storms.
func (h *healthChecker) check(ctx context.Context) *HealthStatus {
	h.mu.RLock()
	if h.lastCheck != nil && time.Since(h.lastTime) < h.cacheTTL {
		cached := h.lastCheck
		h.mu.RUnlock()
		return cached
	}
	h.mu.RUnlock()

	status := h.performChecks(ctx)

	h.mu.Lock()
	h.lastCheck = status
	h.lastTime = time.Now()
	h.mu.Unlock()

	return status
}

func (h *healthChecker) performChecks(ctx context.Context) *HealthStatus {
	components := make(map[string]*ComponentHealth)
	overallStatus := "healthy"

	// 1. Database connectivity — most critical dependency
	dbHealth := h.checkDatabase(ctx)
	components["database"] = dbHealth
	if dbHealth.Status == "unhealthy" {
		overallStatus = "unhealthy"
	}

	// 2. Component registry health
	if h.s.registry != nil {
		for name, comp := range h.s.registry.HealthReport() {
			ch := &ComponentHealth{Status: comp.Status}
			if comp.ErrorMsg != "" {
				ch.Message = comp.ErrorMsg
			}
			components[name] = ch

			if comp.Class == Critical && comp.Status != "healthy" {
				overallStatus = "unhealthy"
			} else if comp.Status == "disabled" || comp.Status == "degraded" {
				if overallStatus == "healthy" {
					overallStatus = "degraded"
				}
			}
		}
	}

	// 3. Connection pool stats
	if h.s.store != nil {
		db := h.s.store.GetDB()
		if db != nil {
			stats := db.Stats()
			poolMsg := fmt.Sprintf("active=%d idle=%d max=%d wait=%d",
				stats.InUse, stats.Idle, stats.MaxOpenConnections, stats.WaitCount)
			poolStatus := "healthy"
			if stats.MaxOpenConnections > 0 && stats.InUse >= stats.MaxOpenConnections-2 {
				poolStatus = "degraded"
				if overallStatus == "healthy" {
					overallStatus = "degraded"
				}
			}
			components["pool"] = &ComponentHealth{
				Status:  poolStatus,
				Message: poolMsg,
			}
		}
	}

	return &HealthStatus{
		Status:     overallStatus,
		Version:    h.s.profile.Version,
		Uptime:     time.Now().Unix() - h.s.startedTS,
		Components: components,
	}
}

func (h *healthChecker) checkDatabase(ctx context.Context) *ComponentHealth {
	if h.s.store == nil {
		return &ComponentHealth{Status: "unhealthy", Message: "store not initialized"}
	}

	db := h.s.store.GetDB()
	if db == nil {
		return &ComponentHealth{Status: "unhealthy", Message: "database connection not available"}
	}

	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	start := time.Now()
	err := db.PingContext(ctx)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return &ComponentHealth{
			Status:    "unhealthy",
			LatencyMs: latency,
			Message:   err.Error(),
		}
	}
	return &ComponentHealth{
		Status:    "healthy",
		LatencyMs: latency,
	}
}

// healthzHandler returns full health check JSON. Returns 200 or 503.
func (h *healthChecker) healthzHandler(c *echo.Context) error {
	status := h.check(c.Request().Context())
	code := http.StatusOK
	if status.Status == "unhealthy" {
		code = http.StatusServiceUnavailable
	}
	return c.JSON(code, status)
}

// readyzHandler checks if the server is ready to receive traffic.
// Returns 200 when all Critical components are healthy, 503 otherwise.
func (h *healthChecker) readyzHandler(c *echo.Context) error {
	if h.s.registry != nil && !h.s.registry.IsReady() {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"status": "not_ready",
		})
	}
	// Quick DB ping
	if h.s.store != nil {
		db := h.s.store.GetDB()
		if db != nil {
			ctx, cancel := context.WithTimeout(c.Request().Context(), 1*time.Second)
			defer cancel()
			if err := db.PingContext(ctx); err != nil {
				return c.JSON(http.StatusServiceUnavailable, map[string]string{
					"status": "db_unreachable",
				})
			}
		}
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ready"})
}

// livezHandler is a lightweight liveness probe.
// Only checks if the process is alive and the database is reachable.
func (h *healthChecker) livezHandler(c *echo.Context) error {
	if h.s.store != nil {
		db := h.s.store.GetDB()
		if db != nil {
			ctx, cancel := context.WithTimeout(c.Request().Context(), 1*time.Second)
			defer cancel()
			if err := db.PingContext(ctx); err != nil {
				return c.String(http.StatusServiceUnavailable, "db_unreachable")
			}
		}
	}
	return c.String(http.StatusOK, "ok")
}
