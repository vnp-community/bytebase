# TASK-WEAK-005-1: Unique ID Index Migration

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-005 |
| Priority | P0 |
| Depends On | — |
| Est. | S (~30 LoC SQL) |
| Status | ✅ Done |

## Objective

Add UNIQUE constraints on `id` column for 8 composite PK tables. BIGSERIAL IDs are globally unique — constraint always succeeds on existing data.

## Files

| Action | Path |
|--------|------|
| CREATE | `backend/migrator/migration/prod/NEXT/0001_add_unique_id_constraints.sql` |

## Specification

```sql
ALTER TABLE plan ADD CONSTRAINT plan_id_unique UNIQUE (id);
ALTER TABLE issue ADD CONSTRAINT issue_id_unique UNIQUE (id);
ALTER TABLE task ADD CONSTRAINT task_id_unique UNIQUE (id);
ALTER TABLE task_run ADD CONSTRAINT task_run_id_unique UNIQUE (id);
ALTER TABLE plan_check_run ADD CONSTRAINT plan_check_run_id_unique UNIQUE (id);
ALTER TABLE release ADD CONSTRAINT release_id_unique UNIQUE (id);
ALTER TABLE db_group ADD CONSTRAINT db_group_id_unique UNIQUE (id);
ALTER TABLE task_run_log ADD CONSTRAINT task_run_log_id_unique UNIQUE (id);
```

Include down migration dropping all constraints.

## Acceptance Criteria

- [x] Migration applies cleanly on existing data
- [x] id-only lookups use unique index (EXPLAIN shows Index Scan)
- [x] Composite PK (project, id) still works for project-scoped queries
- [x] Down migration drops constraints cleanly

## Implementation Notes

Created migration: `backend/migrator/migration/3.18/0003_add_unique_id_constraints.sql`

Added UNIQUE constraints for 5 tables with composite PK `(project, id)` and bigint id:
- plan, issue, task, task_run, plan_check_run

Note: `release`, `db_group`, and `task_run_log` were excluded because they don't have a
single bigint `id` column (release uses `(project, train, iteration)`, db_group uses
`(project, resource_id)` text, task_run_log uses `(project, task_run_id, created_at)`).

LATEST.sql updated to include the new constraints.
