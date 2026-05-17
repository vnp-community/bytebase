# TASK-AI-005-1: EventBus Interface + Typed Event Structs

| Field | Value |
|-------|-------|
| Solution | SOL-AI-005 |
| Priority | P1 |
| Depends On | — |
| Status | ✅ DONE |
| Completed | 2026-05-09 |
| Verified | 2026-05-11 |
| Est. | S (~100 LoC interface + ~40 LoC events) |

## Objective

Define the `EventBus` interface and typed event structs. Additive — no existing code modified.

## Delivered

### `backend/component/bus/interface.go` (65 lines)

`EventBus` interface with 15 methods:

- **Send**: `TicklePlanCheck()`, `TickleTaskRun()`, `RequestApprovalCheck()`, `RequestRolloutCreation()`, `RequestPlanCompletionCheck()`
- **Cancel registry**: `RegisterTaskRunCancel()`, `CancelTaskRun()`, `DeregisterTaskRunCancel()`, `RegisterPlanCheckCancel()`, `CancelPlanCheck()`, `DeregisterPlanCheckCancel()`
- **Read channels**: `PlanCheckChan()`, `TaskRunChan()`, `ApprovalChan()`, `RolloutCreationChan()`, `PlanCompletionChan()`

### Typed Event Structs (in `bus.go`)

4 typed event structs: `PlanRef`, `TaskRunRef`, `IssueRef`, `PlanCheckRunRef`

> **Note**: Spec called for separate `events.go` file. Implementation co-located event structs in `bus.go` for cohesion — functionally equivalent.

### Verification (2026-05-11 re-verified)

```bash
go build ./backend/component/bus/...  # ✅ PASS
go vet ./backend/component/bus/...    # ✅ PASS
```

## Acceptance Criteria

- [x] `EventBus` interface defined with all 15 methods
- [x] 4 typed event structs defined
- [x] `go build` passes
- [x] No existing code modified (additive)
