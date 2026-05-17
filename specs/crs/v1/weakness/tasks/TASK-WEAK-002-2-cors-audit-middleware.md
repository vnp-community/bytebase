# TASK-WEAK-002-2: CORS Audit Middleware + Dev Mode Warning

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-002 |
| Priority | P1 |
| Depends On | TASK-WEAK-002-1 |
| Est. | S (~60 LoC) |
| Status | ✅ Done |
| Completed | 2026-05-12 |
| Notes | Created `backend/server/cors_audit.go` with `bytebase_cors_rejected_total{origin}` counter and `bytebase_dev_mode` gauge. Combined with TASK-002-1 in single file. |

## Objective

Log rejected CORS requests with Prometheus counter. Add dev mode startup gauge metric.

## Files

| Action | Path |
|--------|------|
| CREATE | `backend/server/cors_audit.go` |

## Specification

### `cors_audit.go`

```go
var corsRejectedCounter = prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "bytebase_cors_rejected_total",
        Help: "CORS requests rejected by origin validation",
    },
    []string{"origin"},
)

var devModeGauge = prometheus.NewGauge(prometheus.GaugeOpts{
    Name: "bytebase_dev_mode",
    Help: "1 if server running in development mode",
})
```

Log rejected origins at `slog.Warn` level. Counter enables alerting on CORS attack patterns.

Dev mode gauge: set to 1 in server startup when `BB_MODE=dev`.

## Acceptance Criteria

- [ ] Rejected CORS requests logged with origin
- [ ] `bytebase_cors_rejected_total{origin}` Prometheus counter
- [ ] `bytebase_dev_mode` gauge = 1 in dev, 0 in release
