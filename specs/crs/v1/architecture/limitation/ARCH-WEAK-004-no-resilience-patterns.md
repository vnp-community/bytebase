# ARCH-WEAK-004 — Missing Resilience Patterns

| Field          | Value                                      |
|----------------|--------------------------------------------|
| **Category**   | Weakness (Needs Fix)                       |
| **Layer**      | L4-L6 (Service, Component, Runner)         |
| **Impact**     | Reliability, Cascading Failures            |
| **Severity**   | High                                       |

---

## 1. Description

Codebase thiếu các resilience patterns cơ bản: **circuit breaker**, **bulkhead**, **rate limiting**, **structured retry with backoff**. Verified qua source scan:

### Evidence

```bash
# Search for resilience patterns across entire backend
$ grep -rn 'circuit|breaker|backoff|ratelimit|rate_limit' backend/ --include='*.go' -l
→ migrator/migrator.go            # retry in schema migration only
→ runner/approval/runner.go       # simple retry
→ plugin/webhook/dingtalk/...     # retry in webhook delivery
→ plugin/parser/pg/...            # retry in parser (unrelated)

# Context timeouts in runner layer
$ grep -rn 'context.WithTimeout' backend/runner/ --include='*.go' | wc -l
→ 5   ← only 5 timeouts across 8 runners
```

### Missing Patterns

| Pattern | Found in Code | Where Needed |
|---------|:------------:|:------------:|
| Circuit Breaker | ❌ | DB connections, external API calls, webhook delivery |
| Bulkhead | ❌ | Runner isolation, per-tenant resource limits |
| Rate Limiting | ❌ | API endpoints, webhook dispatch |
| Structured Retry (exp backoff) | Partial (webhook only) | DB reconnect, schema sync, task execution |
| Timeout Propagation | Partial (5 usages) | All runner operations |
| Fallback | ❌ | Cache miss → DB, DB down → cached response |

---

## 2. Failure Cascade Example

```
Schema Sync Runner starts syncing 200 instances
  → Opens 200 DB connections simultaneously
    → Exhausts connection pool (50 max)
      → API requests queue for connections
        → API latency spikes to 30s+
          → Frontend shows timeout errors
            → Users retry → more load
              → System becomes unresponsive

No circuit breaker to stop at step 2.
No bulkhead to reserve API connections at step 3.
No rate limit to prevent step 6.
```

---

## 3. Specific Gap Analysis

### 3.1 Database Reconnection (db_connection.go:137-178)

```go
func (m *DBConnectionManager) reloadConnection(ctx context.Context, filePath string) {
    time.Sleep(100 * time.Millisecond)  // ← FIXED delay, no exponential backoff
    newURL, err := readURLFromFile(filePath)
    if err != nil { return }             // ← NO retry
    newDB, err := createConnectionWithTracer(ctx, newURL)
    if err != nil { return }             // ← NO retry, NO circuit breaker
}
```

### 3.2 Schema Sync — No Concurrency Limit

```go
// schemasync/syncer.go — syncs ALL instances without rate limiting
// If 500 instances configured, all sync simultaneously
```

### 3.3 Webhook Delivery — No Circuit Breaker

```go
// webhook/manager.go — delivers to ALL configured webhooks
// If Slack is down, keeps trying every event → wastes resources
// No circuit breaker to stop after N failures
```

---

## 4. Consequences

| Consequence | Description |
|------------|-------------|
| **Cascading Failure** | One slow dependency affects all operations |
| **Resource Exhaustion** | Uncontrolled concurrency → pool exhaustion |
| **No Self-Healing** | System doesn't automatically recover from degraded state |
| **Unpredictable Latency** | No timeout guarantees for runner operations |
| **Retry Storms** | Failed operations retried without backoff → amplify load |
