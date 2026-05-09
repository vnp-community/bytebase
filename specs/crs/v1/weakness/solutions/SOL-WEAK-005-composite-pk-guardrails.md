# Solution: Composite PK Guardrails — CR-WEAK-005

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **Solution ID**    | SOL-WEAK-005                                             |
| **CR Reference**   | CR-WEAK-005                                              |
| **Title**          | Unique ID Index + Store Query Validation                 |
| **Affected Layers**| L8 (Data Access — Store), L10 (Infrastructure — Migrator)|
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-09                                               |

---

## 1. Architecture Context

Per TDD.md §4.3: Tables use composite PK `(project, id)` with `BIGSERIAL` id. Design rationale: project-level data isolation.

Per architecture.md §9 (L8): Store layer provides typed methods requiring `FindMessage` structs with `ProjectID` field. All upper layers (L3-L7) depend on Store.

**9 tables with composite PK**: plan, issue, task, task_run, plan_check_run, release, db_group, plan_webhook_delivery, task_run_log.

---

## 2. Solution Design

### 2.1 Unique Index Migration

**New migration**: `backend/migrator/migration/prod/NEXT/0001_add_unique_id_constraints.sql`

```sql
-- BIGSERIAL is globally unique (monotonically increasing), so these
-- constraints will always succeed on existing data.
-- Unique indexes enable efficient id-only lookups without project filter.
ALTER TABLE plan ADD CONSTRAINT plan_id_unique UNIQUE (id);
ALTER TABLE issue ADD CONSTRAINT issue_id_unique UNIQUE (id);
ALTER TABLE task ADD CONSTRAINT task_id_unique UNIQUE (id);
ALTER TABLE task_run ADD CONSTRAINT task_run_id_unique UNIQUE (id);
ALTER TABLE plan_check_run ADD CONSTRAINT plan_check_run_id_unique UNIQUE (id);
ALTER TABLE release ADD CONSTRAINT release_id_unique UNIQUE (id);
ALTER TABLE db_group ADD CONSTRAINT db_group_id_unique UNIQUE (id);
ALTER TABLE task_run_log ADD CONSTRAINT task_run_log_id_unique UNIQUE (id);
```

**Rollback**: `backend/migrator/migration/prod/NEXT/0001_add_unique_id_constraints.down.sql`

```sql
ALTER TABLE plan DROP CONSTRAINT IF EXISTS plan_id_unique;
ALTER TABLE issue DROP CONSTRAINT IF EXISTS issue_id_unique;
-- ...etc
```

### 2.2 Store Query Validation Helper

**New file**: `backend/store/composite_pk_guard.go`

```go
package store

import (
    "log/slog"
    "github.com/pkg/errors"
)

// validateCompositePKQuery ensures at least one of projectID or id is provided
// for tables with composite PK (project, id).
// Returns error if neither is provided. Logs warning if only id is used.
func validateCompositePKQuery(entity string, projectID *string, id *int64) error {
    if (projectID == nil || *projectID == "") && id == nil {
        return errors.Errorf("%s query requires at least ProjectID or ID", entity)
    }
    if (projectID == nil || *projectID == "") && id != nil {
        slog.Warn("composite PK query without ProjectID — using id-only lookup (unique index)",
            slog.String("entity", entity),
            slog.Int64("id", *id),
        )
    }
    return nil
}
```

**Usage in store methods** (e.g., `plan.go`):

```go
func (s *Store) GetPlan(ctx context.Context, find *FindPlanMessage) (*PlanMessage, error) {
    if err := validateCompositePKQuery("plan", find.ProjectID, find.ID); err != nil {
        return nil, err
    }
    // ...existing query logic...
}
```

Apply to: `GetPlan`, `GetIssue`, `GetTask`, `GetTaskRun`, `GetPlanCheckRun`, `GetRelease`, `GetDBGroup`.

### 2.3 Developer Documentation

**New file**: `docs/dev/composite-pk-conventions.md`

Content: when to use composite PK, required project filter, JSONB naming, index strategy.

---

## 3. File Change Manifest

| File | Action | Impact |
|------|--------|--------|
| `backend/migrator/migration/prod/NEXT/...` | NEW | Unique constraints |
| `backend/store/composite_pk_guard.go` | NEW | Validation helper |
| `backend/store/plan.go` | MODIFY | Add validation call |
| `backend/store/issue.go` | MODIFY | Add validation call |
| `backend/store/task.go` | MODIFY | Add validation call |
| `docs/dev/composite-pk-conventions.md` | NEW | Developer guide |

## 4. Test Strategy

```go
func TestValidateCompositePKQuery(t *testing.T) {
    // Both nil → error
    // Only ID → warning + success
    // Both provided → success
    // Only ProjectID → success
}

func TestMigration_UniqueConstraint(t *testing.T) {
    // Apply migration → verify constraints exist
    // Insert duplicate id (different project) → should still work (PK is composite)
    // Verify id-only lookup uses unique index
}
```

## 5. Rollback

Drop unique constraints via down migration. Remove validation calls. No data loss.
