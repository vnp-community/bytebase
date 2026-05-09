# T-003-02: Health Endpoints

| Field | Value |
|---|---|
| **Task ID** | T-003-02 |
| **Solution** | SOL-AVAIL-003 |
| **Depends On** | T-003-01 |
| **Target File** | `backend/server/echo_routes.go` (Modify) |

---

## Objective

Thêm `/healthz/deep` và `/readyz` endpoints vào Echo router. Giữ nguyên `/healthz` (plain "OK") cho liveness probe.

## Context — Current Code (line 75-77)

```go
e.GET("/healthz", func(c *echo.Context) error {
    return c.String(http.StatusOK, "OK")
})
```

## Implementation

Thêm sau `/healthz` (giữ nguyên dòng hiện tại):

```go
// Deep health check (monitoring/alerting)
e.GET("/healthz/deep", func(c *echo.Context) error {
    overall, checks := s.healthChecker.RunAll(c.Request().Context())
    status := http.StatusOK
    if overall == health.StatusUnhealthy {
        status = http.StatusServiceUnavailable
    }
    return c.JSON(status, map[string]any{
        "status":    overall,
        "checks":    checks,
        "node":      s.profile.ReplicaID,
        "version":   s.profile.Version,
        "uptime":    time.Since(time.Unix(s.startedTS, 0)).String(),
        "timestamp": time.Now().UTC(),
    })
})

// Readiness probe (K8s)
e.GET("/readyz", func(c *echo.Context) error {
    overall, checks := s.healthChecker.RunAll(c.Request().Context())
    status := http.StatusOK
    if overall == health.StatusUnhealthy {
        status = http.StatusServiceUnavailable
    }
    return c.JSON(status, map[string]any{"status": overall, "checks": checks})
})
```

> **Note**: `s.healthChecker` cần được khởi tạo trong `NewServer()` — thêm field vào Server struct.

## Acceptance Criteria

- [ ] `/healthz` unchanged → returns "OK" (200)
- [ ] `/healthz/deep` returns JSON with checks, node, version, uptime
- [ ] `/readyz` returns 503 if any critical check UNHEALTHY
- [ ] `go build ./backend/server/...` passes
