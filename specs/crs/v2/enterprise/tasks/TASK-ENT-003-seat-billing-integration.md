# TASK-ENT-003 — Seat Count Billing Integration (Stripe)

| Field            | Value                                      |
|------------------|--------------------------------------------|
| **Task ID**      | TASK-ENT-003                               |
| **Source**       | SOL-ENT-002 (CR-ENT-002)                  |
| **Status**       | Done                                       |
| **Priority**     | P1                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 3                                   |

---

## Mô tả

Implement Stripe usage-based billing sync cho seat count. Report seat count tự động sau mỗi user create/deactivate event.

## Scope

1. Implement `SubscriptionService.ReportSeatUsage()` — update Stripe usage record
2. Trigger: Event-driven sau mỗi user create/deactivate
3. Error handling: retry logic cho Stripe API failures
4. Idempotency: ensure seat count sync chính xác

## Acceptance Criteria

- [x] Seat count synced to Stripe sau mỗi user change
- [x] Retry logic cho API failures
- [x] Audit log entry cho billing sync events
- [x] Integration tests với Stripe test mode

## Dependencies

- TASK-ENT-002 (SeatEnforcer)

## Definition of Done

- [x] Stripe integration tested in sandbox
- [x] Retry mechanism validated
