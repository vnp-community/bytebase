# T-009-04: Apply Resilience to Webhook/Sync/DB

| Field | Value |
|---|---|
| **Task ID** | T-009-04 |
| **Solution** | SOL-ARCH-009 |
| **Priority** | P0 |
| **Depends On** | T-009-01, T-009-02, T-009-03 |
| **Target Files** | `backend/component/webhook/manager.go`, `backend/runner/schemasync/syncer.go`, `backend/store/db_connection.go` |
| **Type** | Modify existing |

---

## Objective

Apply resilience patterns to 3 critical points:
1. **Webhook**: circuit breaker around HTTP calls to Slack/DingTalk/Teams
2. **Schema sync**: bulkhead limiting to 10 concurrent instance syncs
3. **DB reconnect**: replace `time.Sleep(100ms)` with exponential backoff

## Implementation

### 1. Webhook — circuit breaker

```go
// webhook/manager.go
type Manager struct {
    // ... existing fields ...
    circuitBreaker *resilience.CircuitBreaker
}

func (m *Manager) Send(ctx context.Context, msg *WebhookMessage) error {
    return m.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
        return m.doSend(ctx, msg)
    })
}
```

### 2. Schema sync — bulkhead

```go
// schemasync/syncer.go
type Syncer struct {
    // ... existing fields ...
    bulkhead *resilience.Bulkhead
}
// In syncAllInstances: wrap each instance sync in bulkhead.Execute()
```

### 3. DB reconnect — retry

```go
// db_connection.go:138 — replace time.Sleep(100ms) with:
err := resilience.Retry(ctx, "db_reconnect", resilience.RetryConfig{
    MaxRetries: 5, InitialDelay: 100*time.Millisecond, MaxDelay: 30*time.Second,
}, func(ctx context.Context) error { /* reconnect logic */ })
```

## Acceptance Criteria

- [ ] Webhook circuit breaker: 5 failures → open → 30s reset
- [ ] Schema sync bulkhead: max 10 concurrent
- [ ] DB reconnect: exponential backoff (no fixed sleep)
- [ ] `go build ./backend/...` passes
- [ ] Existing tests pass
