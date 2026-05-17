# T-008-02: Health Route Registration

| Field | Value |
|---|---|
| **Task ID** | T-008-02 |
| **Solution** | SOL-ARCH-008 |
| **Priority** | P1 |
| **Depends On** | T-008-01 |
| **Target File** | `backend/server/echo_routes.go` |
| **Type** | Modify existing |
| **Status** | ✅ **DONE** |
| **Completed** | 2026-05-09 |

---

## Objective

Replace shallow `/healthz` with deep health check and add `/readyz`, `/livez` endpoints.

## Implementation — DELIVERED

### File: `backend/server/echo_routes.go` — Lines 77-80

```go
// BEFORE (shallow):
// e.GET("/healthz", func(c *echo.Context) error {
//     return c.String(http.StatusOK, "OK")
// })

// AFTER (deep):
checker := newHealthChecker(s)
e.GET("/healthz", checker.healthzHandler)
e.GET("/readyz", checker.readyzHandler)
e.GET("/livez", checker.livezHandler)
```

### Endpoint Details

| Endpoint | Purpose | Response | Use Case |
|----------|---------|----------|----------|
| `GET /healthz` | Full health check | 200 + JSON (all components) or 503 | Monitoring dashboards, Grafana |
| `GET /readyz` | Readiness probe | 200 or 503 (no body) | K8s readinessProbe |
| `GET /livez` | Liveness probe | 200 or 503 (no body) | K8s livenessProbe |

### Backward Compatibility

- `/healthz` still returns 200 when healthy → existing monitoring scripts work unchanged
- New: returns JSON body with component-level details when queried
- New: returns 503 instead of 200 when unhealthy (was always 200 before)

## Acceptance Criteria

- [x] Old `/healthz` replaced with deep check ✅
- [x] `/readyz` and `/livez` registered ✅
- [x] `go build ./backend/server/...` passes ✅
- [x] `/healthz` backward-compatible: still returns 200 when healthy ✅

## Verification

```
$ go build ./backend/server/... → ✅ PASS
$ grep -n 'healthz\|readyz\|livez' backend/server/echo_routes.go
  77: checker := newHealthChecker(s)
  78: e.GET("/healthz", checker.healthzHandler)
  79: e.GET("/readyz", checker.readyzHandler)
  80: e.GET("/livez", checker.livezHandler)
```
