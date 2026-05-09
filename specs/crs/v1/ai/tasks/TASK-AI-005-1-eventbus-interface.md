# TASK-AI-005-1: EventBus Interface + Typed Event Structs

| Field | Value |
|-------|-------|
| Solution | SOL-AI-005 |
| Priority | P1 |
| Depends On | — |
| Est. | S (~100 LoC interface + ~40 LoC events) |
| **Status** | **✅ DONE** (2026-05-09) |

## Objective

Define the `EventBus` interface and typed event structs in new files. Additive — no existing code modified.

## Files

| Action | Path |
|--------|------|
| CREATE | `backend/component/bus/interface.go` — EventBus interface |
| CREATE | `backend/component/bus/events.go` — ApprovalEvent, RolloutEvent, PlanCompletionEvent |

## Specification

### interface.go

```go
type EventBus interface {
    TicklePlanCheck()
    TickleTaskRun()
    RequestApprovalCheck(projectID string, issueUID int64)
    RequestRolloutCreation(projectID string, planUID int64)
    RequestPlanCompletionCheck(projectID string, planUID int64)
    RegisterTaskRunCancel(taskRunUID int64, cancel context.CancelFunc)
    CancelTaskRun(taskRunUID int64) bool
    DeregisterTaskRunCancel(taskRunUID int64)
    RegisterPlanCheckCancel(planCheckRunUID int64, cancel context.CancelFunc)
    CancelPlanCheck(planCheckRunUID int64) bool
    DeregisterPlanCheckCancel(planCheckRunUID int64)
    PlanCheckChan() <-chan struct{}
    TaskRunChan() <-chan struct{}
    ApprovalChan() <-chan ApprovalEvent
    RolloutCreationChan() <-chan RolloutEvent
    PlanCompletionChan() <-chan PlanCompletionEvent
}
```

### events.go

3 typed event structs with `ProjectID`, timestamp, source.

### Verification

```bash
go build ./backend/component/bus/...
```

## Acceptance Criteria

- [ ] `EventBus` interface defined with all methods
- [ ] 3 typed event structs defined
- [ ] `go build` passes
- [ ] No existing code modified
