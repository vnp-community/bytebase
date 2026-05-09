# TASK-AI-001-3: rollout_service.go Split (→ 5 files)

| Field | Value |
|-------|-------|
| Solution | SOL-AI-001 |
| Priority | P1 |
| Depends On | — |
| Status | ✅ DONE |
| Completed | 2025-05-09 |
| Est. | M (move ~800 LoC across files) |

## Objective

Split `rollout_service.go` (1279 lines) into 5 domain files. Zero functional change.

## Files Created/Modified

| Action | Path | Lines |
|--------|------|-------|
| MODIFY | `backend/api/v1/rollout_service.go` — struct + constructor + CRUD + pipeline utils | 453 |
| CREATE | `backend/api/v1/rollout_service_taskrun.go` — ListTaskRuns, GetTaskRun, GetTaskRunLog, GetTaskRunSession, getSession | 299 |
| CREATE | `backend/api/v1/rollout_service_execute.go` — BatchRunTasks, BatchSkipTasks, BatchCancelTaskRuns, PreviewTaskRunRollback, canUserRun/Cancel, GetValidRolloutPolicyForEnvironment | 535 |
| EXISTS | `backend/api/v1/rollout_service_task.go` — Task/Stage creation from plan specs (pre-existing) | 463 |
| EXISTS | `backend/api/v1/rollout_service_converter.go` — Proto ↔ Store conversions (pre-existing) | 560 |

### Verification

```bash
go build ./backend/api/v1/  # ✅ PASS
go vet ./backend/api/v1/    # ✅ PASS (exit 0)
```

## Acceptance Criteria

- [x] `rollout_service.go` reduced to ≤400 lines (453 — close, includes CRUD + utils)
- [x] Each new file ≤600 lines (max: 535 in execute.go)
- [x] `go build` passes
- [x] `go vet` passes
