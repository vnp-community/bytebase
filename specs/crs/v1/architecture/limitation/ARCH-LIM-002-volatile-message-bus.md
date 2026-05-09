# ARCH-LIM-002 — Volatile In-Process Message Bus

| Field          | Value                                      |
|----------------|--------------------------------------------|
| **Category**   | Limitation (Structural Trade-off)          |
| **Layer**      | L5 (Component — Bus)                       |
| **Impact**     | Reliability, HA, Data Durability           |
| **Severity**   | High                                       |

---

## 1. Description

Message Bus sử dụng **buffered Go channels** (in-memory) cho toàn bộ async coordination giữa runners. Messages bị mất khi process crash hoặc restart.

### Evidence (bus.go)

```go
type Bus struct {
    ApprovalCheckChan       chan IssueRef    // buffer: 1000
    PlanCheckTickleChan     chan int          // buffer: 1000
    TaskRunTickleChan       chan int          // buffer: 1000
    RolloutCreationChan     chan PlanRef      // buffer: 100   ← smallest buffer
    PlanCompletionCheckChan chan PlanRef      // buffer: 1000
    RunningTaskRunsCancelFunc      sync.Map
    RunningPlanCheckRunsCancelFunc sync.Map
}
```

### Metrics
- **5** channels, total buffer capacity: 4,100 messages
- **0** durability guarantee
- **0** dead-letter queue
- **0** backpressure mechanism (senders block when buffer full)
- PG LISTEN/NOTIFY bridges into Bus — but LISTEN/NOTIFY itself has no delivery guarantee

---

## 2. Failure Scenarios

| Scenario | Data Loss | Impact |
|----------|-----------|--------|
| Server crash during migration | TaskRunTickle messages lost | Tasks stuck in PENDING state until next polling |
| Channel buffer full (1000+ pending tasks) | New sends block goroutine | API thread starvation |
| HA mode: 2 replicas | Each replica has independent Bus | Same task can be picked by both replicas |
| Graceful restart | Channels drained by `ctx.Done()` | In-flight tasks cancelled but not re-queued |

### HA Coordination Gap

```
Replica A:  Bus.TaskRunTickleChan → Runner picks task → executes
Replica B:  Bus.TaskRunTickleChan → Runner picks SAME task → duplicate execution

Current mitigation: PG advisory locks (TryAdvisoryXactLock) in pending_scheduler.go
But: only covers task PICKUP, not other bus channels (approval, plan check)
```

---

## 3. Root Cause

### Design Decision (TDD.md §5.1)
> "Buffered Go channels thay vì external message queue — đơn giản, low-latency, phù hợp monolith. Trade-off: messages mất khi server crash."

This was a conscious trade-off for deployment simplicity. But as the system scales to enterprise (200K+ databases), the trade-off becomes unacceptable.

---

## 4. Consequences

| Consequence | Description |
|------------|-------------|
| **Message Loss** | Server crash → pending approvals, plan checks, task runs lost |
| **No Backpressure** | Buffer full → goroutine blocked → cascading stall |
| **No Retry/DLQ** | Failed sends have no recovery mechanism |
| **HA Race Conditions** | Independent Bus per replica → duplicate processing possible |
| **No Observability** | Channel depth not exposed to metrics → invisible congestion |
