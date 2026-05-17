# TASK-107: Resilience Infrastructure

| Field | Value |
|-------|-------|
| Task ID | TASK-107 |
| Phase | 1 — Infrastructure |
| Estimated | 0.5 day |
| Dependencies | TASK-106 |
| Status | ✅ DONE |

## Objective

Create circuit breaker, retry, and feature flag infrastructure.

## Files to Create

### `backend/gateway/circuitbreaker.go`
- `ServiceCircuitBreakers` — per-service circuit breaker (sony/gobreaker)
- `OnStateChange` callback → log + update Prometheus gauge
- Configurable via `ServiceConfig.CircuitBreaker`

### `backend/component/retry/retry.go`
- `WithRetry(fn, opts)` — generic retry with exponential backoff
- Used by NATSBus for publish retries

### `backend/component/config/service_config.go`
- `ServiceConfig` struct with timeouts, circuit breaker, NATS, observability settings
- `FeatureFlags` struct with runtime toggles
- Load from env vars or profile config

## Dependencies to Add

```bash
go get github.com/sony/gobreaker@latest
go get github.com/avast/retry-go@latest
```

## Acceptance Criteria

- [ ] Circuit breaker opens after 50% failure rate
- [ ] Retry with exponential backoff works
- [ ] Feature flags default to safe values (all false)
- [ ] `go build ./backend/...` passes
