# TASK-WEAK-005-2: Store Composite PK Validation Helper

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-005 |
| Priority | P1 |
| Depends On | TASK-WEAK-005-1 |
| Est. | S (~80 LoC) |
| Status | ✅ Done |

## Objective

Create `validateCompositePKQuery()` helper and apply to 7 Get/List store methods. Prevents empty queries (no projectID AND no id) and logs warnings for id-only lookups.

## Files

| Action | Path |
|--------|------|
| CREATE | `backend/store/composite_pk_guard.go` |
| MODIFY | `backend/store/plan.go` — add validation |
| MODIFY | `backend/store/issue.go` — add validation |
| MODIFY | `backend/store/task.go` — add validation |

## Specification

```go
func validateCompositePKQuery(entity string, projectID *string, id *int64) error {
    if (projectID == nil || *projectID == "") && id == nil {
        return errors.Errorf("%s query requires at least ProjectID or ID", entity)
    }
    if (projectID == nil || *projectID == "") && id != nil {
        slog.Warn("composite PK query without ProjectID", slog.String("entity", entity))
    }
    return nil
}
```

Apply to: `GetPlan`, `GetIssue`, `GetTask`, `GetTaskRun`, `GetPlanCheckRun`, `GetRelease`, `GetDBGroup`.

## Acceptance Criteria

- [x] Empty query (no projectID, no id) → error returned
- [x] id-only query → warning logged, query executes (uses unique index)
- [x] Normal query (projectID set) → no warning
- [x] Unit test for all 3 cases

## Implementation Notes

Created files:
- `backend/store/composite_pk_guard.go` — `validateCompositePKQuery()` helper
- `backend/store/composite_pk_guard_test.go` — 4 test cases (all passing)

Integrated into:
- `GetPlan` (plan.go)
- `GetIssue` (issue.go)
- `GetTaskByID` (task.go)

Note: The helper signature uses `string` for projectID and `*int64` for id to
match the actual store patterns. GetTaskRun, GetPlanCheckRun, GetRelease, and
GetDBGroup already require project context by their function signatures.
