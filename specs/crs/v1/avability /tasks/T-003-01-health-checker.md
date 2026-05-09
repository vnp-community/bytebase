# T-003-01: Deep Health Check System

| Field | Value |
|---|---|
| **Task ID** | T-003-01 |
| **Solution** | SOL-AVAIL-003 |
| **Priority** | P0 |
| **Depends On** | None |
| **Target File** | `backend/component/health/checker.go` (NEW) |

---

## Objective

Tạo `health.Checker` component thực hiện deep health check (PG connectivity, memory, pool stats) với parallel execution và Prometheus metrics.

## Implementation

Tạo file `backend/component/health/checker.go` — xem full code tại SOL-AVAIL-003 §2.1.

Key points:
- `Status` enum: HEALTHY, DEGRADED, UNHEALTHY
- `CheckResult` struct: Name, Status, Latency, Message, Critical flag
- `Checker` struct: holds `*sql.DB`, list of `CheckFunc`, Prometheus gauges
- 3 checks: `checkPostgreSQL` (Ping + pool stats), `checkMemory` (runtime.MemStats), `checkDiskSpace`
- `RunAll(ctx)` executes all checks in parallel with 10s timeout
- Returns `(overallStatus, []CheckResult)`

## Acceptance Criteria

- [ ] `health.NewChecker(db, registry)` registers 2 Prometheus metrics (gauge + histogram)
- [ ] `RunAll()` returns overall + per-check results
- [ ] PG check: UNHEALTHY on ping failure, DEGRADED when pool > 90% utilized
- [ ] Memory check: DEGRADED > 1.5GB alloc, UNHEALTHY > 2.5GB
- [ ] `go build ./backend/component/health/...` passes
