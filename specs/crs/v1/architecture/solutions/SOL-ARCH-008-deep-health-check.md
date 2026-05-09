# Solution: Deep Health Check — CR-ARCH-008

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **Solution ID**    | SOL-ARCH-008                                             |
| **CR Reference**   | CR-ARCH-008                                              |
| **Title**          | Component-Level Health Reporting + K8s Probes            |
| **Affected Layers**| L2 (API Gateway)                                         |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-09                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Architecture Context

Per [architecture.md](../../architecture.md) §2 (L2 — Echo Routes):
- `/healthz` → `return c.String(http.StatusOK, "OK")` (echo_routes.go:75-77)

Per [TDD.md](../../TDD.md) §3.2:
- Echo v5 serves all HTTP endpoints including health check

---

## 2. Current Implementation (echo_routes.go:75-77)

```go
e.GET("/healthz", func(c *echo.Context) error {
    return c.String(http.StatusOK, "OK")
})
```

**Problem**: Always 200, no dependency verification.

---

## 3. Solution Design

### 3.1 Health Handler

**New file**: `backend/server/health.go`

```go
package server

import (
    "context"
    "database/sql"
    "encoding/json"
    "log/slog"
    "net/http"
    "sync"
    "time"

    "github.com/labstack/echo/v5"
)

// HealthStatus represents overall system health.
type HealthStatus struct {
    Status     string                       `json:"status"`  // healthy, degraded, unhealthy
    Version    string                       `json:"version"`
    Uptime     int64                        `json:"uptime_seconds"`
    Components map[string]*ComponentHealth  `json:"components"`
}

// ComponentHealth represents a single component's health.
type ComponentHealth struct {
    Status    string `json:"status"`              // healthy, degraded, unhealthy, disabled
    LatencyMs int64  `json:"latency_ms,omitempty"`
    Message   string `json:"message,omitempty"`
}

// healthChecker performs cached health checks.
type healthChecker struct {
    s          *Server
    mu         sync.RWMutex
    lastCheck  *HealthStatus
    lastTime   time.Time
    cacheTTL   time.Duration
}

func newHealthChecker(s *Server) *healthChecker {
    return &healthChecker{
        s:        s,
        cacheTTL: 5 * time.Second,
    }
}

// check performs health checks with caching.
func (h *healthChecker) check(ctx context.Context) *HealthStatus {
    h.mu.RLock()
    if h.lastCheck != nil && time.Since(h.lastTime) < h.cacheTTL {
        cached := h.lastCheck
        h.mu.RUnlock()
        return cached
    }
    h.mu.RUnlock()

    // Perform actual checks
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

    // 1. Database connectivity
    dbHealth := h.checkDatabase(ctx)
    components["database"] = dbHealth
    if dbHealth.Status == "unhealthy" {
        overallStatus = "unhealthy"
    }

    // 2. Component registry health
    if h.s.registry != nil {
        for name, comp := range h.s.registry.HealthReport() {
            ch := &ComponentHealth{Status: comp.Status}
            if comp.Error != nil {
                ch.Message = comp.Error.Error()
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
        stats := db.Stats()
        components["pool"] = &ComponentHealth{
            Status: "healthy",
            Message: fmt.Sprintf("active=%d idle=%d max=%d",
                stats.InUse, stats.Idle, stats.MaxOpenConnections),
        }
        if stats.InUse >= stats.MaxOpenConnections-2 {
            components["pool"].Status = "degraded"
            if overallStatus == "healthy" {
                overallStatus = "degraded"
            }
        }
    }

    // 4. License status
    if h.s.licenseService != nil {
        components["license"] = &ComponentHealth{
            Status: "healthy",
            Message: fmt.Sprintf("plan=%s", h.s.licenseService.GetCurrentPlan()),
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

    ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
    defer cancel()

    start := time.Now()
    err := h.s.store.GetDB().PingContext(ctx)
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

// === HTTP Handlers ===

// healthzHandler — full health check
func (h *healthChecker) healthzHandler(c *echo.Context) error {
    status := h.check(c.Request().Context())
    code := http.StatusOK
    if status.Status == "unhealthy" {
        code = http.StatusServiceUnavailable
    } else if status.Status == "degraded" {
        code = http.StatusServiceUnavailable
    }
    return c.JSON(code, status)
}

// readyzHandler — is server ready for traffic?
func (h *healthChecker) readyzHandler(c *echo.Context) error {
    if h.s.registry != nil && !h.s.registry.IsReady() {
        return c.JSON(http.StatusServiceUnavailable, map[string]string{
            "status": "not_ready",
        })
    }
    // Quick DB ping
    ctx, cancel := context.WithTimeout(c.Request().Context(), 1*time.Second)
    defer cancel()
    if err := h.s.store.GetDB().PingContext(ctx); err != nil {
        return c.JSON(http.StatusServiceUnavailable, map[string]string{
            "status": "db_unreachable",
        })
    }
    return c.JSON(http.StatusOK, map[string]string{"status": "ready"})
}

// livezHandler — lightweight liveness probe
func (h *healthChecker) livezHandler(c *echo.Context) error {
    ctx, cancel := context.WithTimeout(c.Request().Context(), 1*time.Second)
    defer cancel()
    if err := h.s.store.GetDB().PingContext(ctx); err != nil {
        return c.String(http.StatusServiceUnavailable, "db_unreachable")
    }
    return c.String(http.StatusOK, "ok")
}
```

### 3.2 Route Registration

**Modified file**: `backend/server/echo_routes.go`

```go
func configureEchoRouters(e *echo.Echo, ..., s *Server) {
    // ... existing middleware ...

    checker := newHealthChecker(s)

    // Replace shallow healthz with deep check
    e.GET("/healthz", checker.healthzHandler)
    e.GET("/readyz", checker.readyzHandler)
    e.GET("/livez", checker.livezHandler)
}
```

---

## 4. File Change Manifest

| File | Layer | Action | Impact |
|------|-------|--------|--------|
| `backend/server/health.go` | L2 | **NEW** | Health check implementation |
| `backend/server/echo_routes.go` | L2 | **MODIFY** | Register /healthz, /readyz, /livez |

---

## 5. Kubernetes Probe Configuration

```yaml
livenessProbe:
  httpGet:
    path: /livez
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 10
  timeoutSeconds: 2
readinessProbe:
  httpGet:
    path: /readyz
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 5
  timeoutSeconds: 3
```

---

## 6. Rollback Plan

Revert echo_routes.go → restore `return c.String(http.StatusOK, "OK")`.
