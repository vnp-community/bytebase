# TASK-WEAK-004-3: rollout_service.go Split (→ 4 files)

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-004 |
| Priority | P1 |
| Depends On | — |
| Est. | M (move ~900 LoC across files) |

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

- [ ] `rollout_service.go` reduced to ≤400 lines
- [ ] `go build ./backend/api/v1/...` passes
- [ ] All existing integration tests pass
