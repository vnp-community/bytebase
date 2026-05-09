# TASK-AI-005-3: Runner Migration to EventBus

| Field | Value |
|-------|-------|
| Solution | SOL-AI-005 |
| Priority | P1 |
| Depends On | AI-005-2 |
| Status | ✅ DONE |
| Completed | 2025-05-10 |
| Est. | L |

## Delivered

Migrated all 10 files from direct `bus.Bus` field access to `EventBus` interface methods:

### Files Modified (Runner Layer)
| File | Changes |
|------|---------|
| `runner/approval/runner.go` | `ApprovalCheckChan` → `ApprovalChan()`, `RolloutCreationChan <-` → `RequestRolloutCreation()` |
| `runner/plancheck/scheduler.go` | `PlanCheckTickleChan` → `PlanCheckChan()`, `RunningPlanCheckRunsCancelFunc.Store/Delete` → `Register/DeregisterPlanCheckCancel()`, `ApprovalCheckChan <-` → `RequestApprovalCheck()` |
| `runner/notifylistener/listener.go` | `RunningPlanCheckRunsCancelFunc.Load` + cast → `CancelPlanCheck()`, `RunningTaskRunsCancelFunc.Load` + cast → `CancelTaskRun()` |
| `runner/taskrun/pending_scheduler.go` | `TaskRunTickleChan` reads/sends → `TaskRunChan()` / `TickleTaskRun()` |
| `runner/taskrun/running_scheduler.go` | `RunningTaskRunsCancelFunc.Store/Delete` → `Register/DeregisterTaskRunCancel()`, `PlanCompletionCheckChan <-` → `RequestPlanCompletionCheck()` |
| `runner/taskrun/scheduler.go` | `PlanCompletionCheckChan` → `PlanCompletionChan()`, `RolloutCreationChan` → `RolloutCreationChan()` |
| `runner/taskrun/rollout_creator.go` | `chan bus.PlanRef` → `<-chan bus.PlanRef`, `TaskRunTickleChan <-` → `TickleTaskRun()` |

### Files Modified (API Layer)
| File | Changes |
|------|---------|
| `api/v1/rollout_service.go` | `TaskRunTickleChan <-` → `TickleTaskRun()` |
| `api/v1/rollout_service_execute.go` | `TaskRunTickleChan`, `PlanCompletionCheckChan`, `RunningTaskRunsCancelFunc` → EventBus methods |
| `api/v1/plan_service.go` | `PlanCheckTickleChan <-` → `TicklePlanCheck()`, `RunningPlanCheckRunsCancelFunc.Load` → `CancelPlanCheck()` |
| `api/v1/issue_hook.go` | `ApprovalCheckChan <-` → `RequestApprovalCheck()` |
| `api/v1/issue_service_lifecycle.go` | `RolloutCreationChan <-` → `RequestRolloutCreation()` |

## Key Safety Improvements
- Eliminated unsafe `cancel.(context.CancelFunc)()` type assertions
- Non-blocking sends prevent goroutine hangs
- Receive-only channel types (`<-chan`) prevent accidental sends from consumers

## Verification

```bash
go build ./backend/component/bus/...    # ✅ PASS
go build ./backend/runner/...           # ✅ PASS
go build ./backend/api/v1/...           # ✅ PASS
go vet ./backend/component/bus/... ./backend/runner/... ./backend/api/v1/...  # ✅ PASS
```
