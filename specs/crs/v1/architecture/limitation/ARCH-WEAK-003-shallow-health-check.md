# ARCH-WEAK-003 — Shallow Health Check

| Field          | Value                                      |
|----------------|--------------------------------------------|
| **Category**   | Weakness (Needs Fix)                       |
| **Layer**      | L2 (API Gateway)                           |
| **Impact**     | Operational Visibility, HA Failover        |
| **Severity**   | Medium                                     |

---

## 1. Description

Health check endpoint `/healthz` returns HTTP 200 unconditionally — it does not verify database connectivity, runner health, or any critical dependency.

### Evidence (echo_routes.go:75)

```go
e.GET("/healthz", func(c *echo.Context) error {
    return c.String(http.StatusOK, "OK!")   // ← ALWAYS returns 200
})
```

**No checks performed**: DB ping, cache status, runner goroutine health, license validity, disk space.

---

## 2. Consequences

| Scenario | Current | Desired |
|----------|---------|---------|
| PG connection lost | `/healthz` → 200 OK | `/healthz` → 503 + `"db": "unhealthy"` |
| All runners panicked | `/healthz` → 200 OK | `/healthz` → 503 + `"runners": "degraded"` |
| Disk full (embedded PG) | `/healthz` → 200 OK | `/healthz` → 503 + `"storage": "critical"` |
| License expired | `/healthz` → 200 OK | `/healthz` → 200 + `"license": "expired"` |

### HA Impact

Load balancers and orchestrators (k8s, Nomad) rely on health checks for:
- **Liveness probe**: restart unhealthy instances
- **Readiness probe**: remove from traffic routing

A shallow health check means **broken instances continue receiving traffic**.

---

## 3. Missing Architecture

```
CURRENT:
  /healthz → return "OK!" (always)

NEEDED:
  /healthz → Aggregate health:
    ├── DB ping (critical)
    ├── Runner goroutine count (important)
    ├── Connection pool stats (informational)
    ├── License status (informational)
    └── Memory usage (informational)

  /readyz  → Readiness:
    ├── DB connected
    ├── Schema migrated
    └── All runners started

  /livez   → Liveness:
    └── Process alive + DB ping
```

---

## 4. Kubernetes Integration Gap

```yaml
# Cannot configure meaningful probes today
livenessProbe:
  httpGet:
    path: /healthz    # Always 200 — useless
readinessProbe:
  httpGet:
    path: /healthz    # Always 200 — useless
```
