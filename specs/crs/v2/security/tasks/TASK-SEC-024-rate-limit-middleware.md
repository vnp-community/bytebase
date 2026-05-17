# TASK-SEC-024 — Rate Limit Middleware + DDoS Protection

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-024                               |
| **Source**       | SOL-SEC-013 §3.2-§3.5                     |
| **Status**       | Pending                                    |
| **Priority**     | P0                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 1                                   |

---

## Mô tả

Implement rate limit middleware (L2) và DDoS concurrent connection limiter trong Echo middleware stack. Depends on RateLimitEngine (TASK-SEC-011).

## Scope

1. **Middleware**: `rateLimitMiddleware()` — insert at position 3 in Echo middleware stack (after IP policy, before security headers)
2. **Multi-tier**: Global → Per-IP → Per-endpoint checks, sequentially
3. **Response headers**: `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `Retry-After`
4. **429 response**: JSON `{"error": "rate limit exceeded"}` with Retry-After header
5. **DDoS middleware**: `ddosProtectionMiddleware()` — `sync.Map` tracking concurrent connections per IP, max default 100
6. **Adaptive**: Read CPU/memory metrics, reduce capacity when > 80% load

## Acceptance Criteria

- [ ] Global rate limit enforced
- [ ] Per-IP rate limit enforced
- [ ] Per-endpoint limits (Login: 10/min, Export: 5/min)
- [ ] Response headers set correctly
- [ ] Concurrent connection limit works
- [ ] Adaptive throttling under high load

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/server/echo_routes.go` | Rate limit + DDoS middleware |

## Definition of Done

- Middleware position verified in stack
- Load test validated
