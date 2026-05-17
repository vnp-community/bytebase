# TASK-106: Observability Infrastructure

| Field | Value |
|-------|-------|
| Task ID | TASK-106 |
| Phase | 1 — Infrastructure |
| Estimated | 1 day |
| Dependencies | TASK-000 |
| Status | ✅ DONE |

## Objective

Create shared observability packages: OTel tracer init, ServiceMetrics, structured logging context, and production middleware.

## Files to Create

### `backend/component/otel/tracer.go`
- `InitTracer(serviceName, sampleRate)` — initialize OTel TracerProvider
- OTLP exporter (configurable via env: `OTEL_EXPORTER_OTLP_ENDPOINT`)
- Fallback: noop tracer if endpoint not configured

### `backend/component/metrics/service_metrics.go`
- `ServiceMetrics` struct with Prometheus counters/histograms/gauges
- `NewServiceMetrics(registry, serviceName)` constructor
- Metrics: request_total, request_duration, active_requests, error_total, nats_messages, circuit_state, panic_total

### `backend/component/middleware/production.go`
- `PanicRecoveryMiddleware(serviceName)` — recover panics, log stack, increment counter
- `MetricsMiddleware(metrics, serviceName)` — record request count/duration/active
- `LoggingMiddleware(serviceName)` — structured log with trace_id, duration
- `InternalAuthMiddleware(secret)` — HMAC token validation
- `TimeoutMiddleware(timeouts)` — per-route deadline enforcement

### `backend/component/errors/codes.go`
- `ServiceError` struct with Code, Message, Service, TraceID, RetryAfter
- Error code constants: `DCM_*`, `SQL_*`, `ADMIN_*`, `GW_*`, `NATS_*`
- `WrapError(err, service, traceID)` helper

## Acceptance Criteria

- [ ] OTel tracer initializes (noop if no exporter)
- [ ] ServiceMetrics registers all counters/histograms
- [ ] All 5 middleware functions compile
- [ ] ServiceError JSON serialization works
- [ ] `go build ./backend/component/...` passes
