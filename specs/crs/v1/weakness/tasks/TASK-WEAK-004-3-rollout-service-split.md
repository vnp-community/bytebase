# TASK-WEAK-004-3: rollout_service.go Split (→ 4 files)

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-004 |
| Priority | P1 |
| Depends On | — |
| Est. | M (move ~900 LoC across files) |
| Status | ✅ Done |
| Completed | 2026-05-12 |

## Objective

Split `rollout_service.go` (1278 lines) into 4 domain files. Same package, same struct.

## Files

| Action | Path |
|--------|------|
| MODIFY | `backend/api/v1/rollout_service.go` — keep struct + CRUD |
| CREATE | `backend/api/v1/rollout_task.go` — Task/Stage creation from plan specs |
| CREATE | `backend/api/v1/rollout_execution.go` — RunTask, CancelTask, RetryTask |
| CREATE | `backend/api/v1/rollout_converter.go` — Proto ↔ Store conversions |

## Acceptance Criteria

- [x] `rollout_service.go` reduced to 453 lines (≤500 ✓)
- [x] `go build ./backend/api/v1/...` passes
- [x] All existing integration tests pass

## Implementation Notes

- Split into 5 files: `rollout_service.go` (453), `rollout_service_converter.go` (560), `rollout_service_execute.go` (533), `rollout_service_task.go` (463), `rollout_service_taskrun.go` (299)
- All files well within 1500-line lint threshold
