# T-008-01: Health Check Handler

| Field | Value |
|---|---|
| **Task ID** | T-008-01 |
| **Solution** | SOL-ARCH-008 |
| **Priority** | P1 |
| **Depends On** | None |
| **Target File** | `backend/server/health.go` |
| **Type** | New file |

---

## Objective

Implement deep health check handler with component-level JSON reporting, DB ping, pool stats, and 5-second result caching.

## Implementation

```go
package server

type HealthStatus struct {
    Status     string                      `json:"status"` // healthy|degraded|unhealthy
    Version    string                      `json:"version"`
    Uptime     int64                       `json:"uptime_seconds"`
    Components map[string]*ComponentHealth `json:"components"`
}

type ComponentHealth struct {
    Status    string `json:"status"`
    LatencyMs int64  `json:"latency_ms,omitempty"`
    Message   string `json:"message,omitempty"`
}

type healthChecker struct { /* server ref, cache, TTL */ }

func (h *healthChecker) healthzHandler(c *echo.Context) error  // 200 or 503
func (h *healthChecker) readyzHandler(c *echo.Context) error   // readiness
func (h *healthChecker) livezHandler(c *echo.Context) error    // lightweight
```

**Checks**: DB ping (1s timeout), pool utilization, license status.

## Acceptance Criteria

- [ ] `/healthz` → JSON with component health, 200 or 503
- [ ] `/readyz` → 200 after init complete, 503 during startup
- [ ] `/livez` → 200 if DB reachable, < 10ms response
- [ ] 5-second cache prevents check storms
- [ ] `go build ./backend/server/...` passes
