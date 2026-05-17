# TASK-SEC-011 — Rate Limiter Component

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-011                               |
| **Source**       | SOL-SEC-003 §3.1, SOL-SEC-013 §3.1        |
| **Status**       | Pending                                    |
| **Priority**     | P0                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 1                                   |

---

## Mô tả

Implement shared RateLimitEngine component (L5) sử dụng cho cả brute-force protection (SOL-SEC-003) và API rate limiting (SOL-SEC-013). Sliding window / token bucket algorithm.

## Scope

1. **TokenBucket**: `component/ratelimit/bucket.go` — capacity, rate, tokens, lastTime, `Allow() (bool, *RateLimitInfo)`
2. **RateLimitEngine**: `component/ratelimit/engine.go` — globalLimiter, ipLimiters (`sync.Map`), userLimiters, endpointConfigs
3. **Endpoint configs**: Default limits per sensitive endpoint (Login: 10/min, Query: 120/min, AdminExecute: 30/min, Export: 5/min)
4. **RateLimitInfo**: Remaining, Limit, RetryAfter
5. **Memory management**: Periodic cleanup of idle buckets (no request > 10min → remove)

## Acceptance Criteria

- [ ] TokenBucket.Allow() correctly rate limits
- [ ] Per-IP, per-endpoint, global tiers work independently
- [ ] Idle bucket cleanup prevents memory leak
- [ ] Thread-safe (sync.Map + Mutex)
- [ ] Unit tests + benchmark tests

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/component/ratelimit/bucket.go` | New file |
| `backend/component/ratelimit/engine.go` | New file |

## Definition of Done

- Benchmarked: < 100ns per Allow() call
