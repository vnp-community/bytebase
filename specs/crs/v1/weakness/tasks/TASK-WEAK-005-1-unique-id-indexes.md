# TASK-WEAK-005-1: Unique ID Index Migration

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-005 |
| Priority | P0 |
| Depends On | — |
| Est. | S (~30 LoC SQL) |

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

- [ ] Migration applies cleanly on existing data
- [ ] id-only lookups use unique index (EXPLAIN shows Index Scan)
- [ ] Composite PK (project, id) still works for project-scoped queries
- [ ] Down migration drops constraints cleanly
