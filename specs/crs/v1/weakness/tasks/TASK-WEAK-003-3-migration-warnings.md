# TASK-WEAK-003-3: Migration Executor Warning Propagation

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-003 |
| Priority | P1 |
| Depends On | — |
| Est. | M (~100 LoC) |
| Status | ✅ Done |
| Completed | 2026-05-12 |
| Notes | Proto `TaskRunResult.warnings` field added (field 10). All 4 migration methods updated to accumulate warnings. `bytebase_schema_sync_errors_total` and `bytebase_changelog_update_errors_total` counters added. |

## Objective

Surface schema sync and changelog update failures as warnings in `TaskRunResult` instead of silently logging and continuing.

## Files

| Action | Path |
|--------|------|
| MODIFY | `proto/store/task_run.proto` — add `repeated string warnings` field |
| MODIFY | `backend/runner/taskrun/database_migrate_executor.go` — populate warnings |
| CREATE | `backend/metrics/error_metrics.go` — new Prometheus counters |

## Specification

### Proto change

```protobuf
message TaskRunResult {
    bool has_prior_backup = 1;
    repeated string warnings = 2;  // NEW: non-fatal issues
}
```

### Executor change (lines 267-282)

Replace fire-and-forget logging with warning accumulation:
```go
if err != nil {
    result.Warnings = append(result.Warnings, fmt.Sprintf("schema sync failed: %v", err))
    schemaSyncErrorsCounter.Inc()
}
if err := exec.store.UpdateChangelog(ctx, update); err != nil {
    result.Warnings = append(result.Warnings, fmt.Sprintf("changelog update failed: %v", err))
    changelogUpdateErrorsCounter.Inc()
}
```

### Metrics

- `bytebase_schema_sync_errors_total` counter
- `bytebase_changelog_update_errors_total` counter

## Acceptance Criteria

- [ ] Proto `TaskRunResult.warnings` field added (backward compatible)
- [ ] Schema sync failure → warning in result (not silent)
- [ ] Changelog update failure → warning in result
- [ ] Prometheus counters exported for both error types
