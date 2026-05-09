# T-008-02: Health Route Registration

| Field | Value |
|---|---|
| **Task ID** | T-008-02 |
| **Solution** | SOL-ARCH-008 |
| **Priority** | P1 |
| **Depends On** | T-008-01 |
| **Target File** | `backend/server/echo_routes.go` |
| **Type** | Modify existing |

---

## Objective

Replace shallow `/healthz` with deep health check and add `/readyz`, `/livez` endpoints.

## Implementation

```go
// BEFORE (line 75-77):
e.GET("/healthz", func(c *echo.Context) error {
    return c.String(http.StatusOK, "OK")
})

// AFTER:
checker := newHealthChecker(s)
e.GET("/healthz", checker.healthzHandler)
e.GET("/readyz", checker.readyzHandler)
e.GET("/livez", checker.livezHandler)
```

**Note**: `configureEchoRouters` needs access to `*Server` — add parameter if not present.

## Acceptance Criteria

- [ ] Old `/healthz` replaced with deep check
- [ ] `/readyz` and `/livez` registered
- [ ] `go build ./backend/...` passes
- [ ] `/healthz` backward-compatible: still returns 200 when healthy
