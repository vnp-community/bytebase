# TASK-AI-005-3: Runner Migration to EventBus

| Field | Value |
|-------|-------|
| Solution | SOL-AI-005 |
| Priority | P1 |
| Depends On | AI-005-2 |
| Status | ✅ DONE |
| Completed | 2026-05-10 |
| Verified | 2026-05-11 |
| Est. | L |

## Delivered

Migrated all 12 files from direct `*bus.Bus` field access to `bus.EventBus` interface methods.

### Files Modified (Runner Layer)

| File | Changes |
|------|---------|
| `runner/approval/runner.go` | `ApprovalCheckChan` → `ApprovalChan()`, `RolloutCreationChan <-` → `RequestRolloutCreation()` |
| `runner/plancheck/scheduler.go` | `PlanCheckTickleChan` → `PlanCheckChan()`, cancel funcs → `Register/DeregisterPlanCheckCancel()` |
| `runner/notifylistener/listener.go` | Unsafe `cancel.(context.CancelFunc)()` → `CancelPlanCheck()`/`CancelTaskRun()` |
| `runner/taskrun/pending_scheduler.go` | `TaskRunTickleChan` → `TaskRunChan()` / `TickleTaskRun()` |
| `runner/taskrun/running_scheduler.go` | cancel funcs → `Register/DeregisterTaskRunCancel()` |
| `runner/taskrun/scheduler.go` | `PlanCompletionCheckChan` → `PlanCompletionChan()` |
| `runner/taskrun/rollout_creator.go` | `TickleTaskRun()` via interface |

### Files Modified (API Layer)

| File | Changes |
|------|---------|
| `api/v1/rollout_service.go` | `TickleTaskRun()` |
| `api/v1/rollout_service_execute.go` | EventBus methods for task run lifecycle |
| `api/v1/plan_service.go` | `TicklePlanCheck()`, `CancelPlanCheck()` |
| `api/v1/issue_hook.go` | `RequestApprovalCheck()` |
| `api/v1/issue_service_lifecycle.go` | `RequestRolloutCreation()` |

### Migration Status

- `*bus.Bus` remaining references: **0** (in production code)
- `bus.EventBus` usage count: **27** references across runner/api/server layers

### Key Safety Improvements

- Eliminated unsafe `cancel.(context.CancelFunc)()` type assertions
- Non-blocking sends prevent goroutine hangs
- Receive-only channel types (`<-chan`) prevent accidental sends from consumers

## Verification (2026-05-11 re-verified)

```bash
go build ./backend/component/bus/...  # ✅ PASS
go build ./backend/runner/...         # ✅ PASS
go build ./backend/api/v1/...        # ✅ PASS
go build ./backend/server/...        # ✅ PASS
go vet ./backend/runner/...          # ✅ PASS
```

## Acceptance Criteria

- [x] Zero remaining `*bus.Bus` references in production code
- [x] 27 `bus.EventBus` references across all layers
- [x] Unsafe type assertions eliminated
- [x] All builds pass
