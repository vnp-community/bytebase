# TASK-WEAK-005-2: Store Composite PK Validation Helper

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-005 |
| Priority | P1 |
| Depends On | TASK-WEAK-005-1 |
| Est. | S (~80 LoC) |

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

- [ ] Empty query (no projectID, no id) → error returned
- [ ] id-only query → warning logged, query executes (uses unique index)
- [ ] Normal query (projectID set) → no warning
- [ ] Unit test for all 3 cases
