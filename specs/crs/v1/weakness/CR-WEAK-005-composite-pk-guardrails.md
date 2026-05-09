# Change Request: Composite Primary Key Guardrails

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-WEAK-005                                              |
| **Weakness ID**    | WEAK-005                                                 |
| **Title**          | Composite PK Safety — Unique ID Constraint & Query Guard |
| **Category**       | Database Design / Data Access                            |
| **Priority**       | P2 — Medium                                              |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-08                                               |
| **Author**         | VNP AI Ops Team                                          |
| **PRD Refs**       | DCM-01 (Issue/Plan/Rollout), DCM-12 (Changelog)         |

---

## 1. Tổng quan

### 1.1 Mô tả
Thêm guardrails cho composite PK pattern `(project, id)`: unique constraint trên `id` alone, store-layer query validation, và developer documentation.

### 1.2 Bối cảnh
8 tables sử dụng composite PK `(project, id)` với `BIGSERIAL` id. `id` column tăng globally unique nhưng PK constraint chỉ enforce `(project, id)` uniqueness. Queries thiếu project filter sẽ **full table scan** thay vì PK lookup. Store methods yêu cầu project param nhưng không validate nó.

### 1.3 Mục tiêu
- Unique constraint trên `id` alone cho simpler lookups
- Store-layer validation reject queries thiếu project filter
- Developer documentation cho composite PK conventions
- Query performance monitoring

---

## 2. Yêu cầu chức năng

### FR-001: Unique Index trên ID Column
- **Mô tả**: Add `UNIQUE` constraint trên `id` column cho tất cả tables có composite PK.
- **Migration**:
  ```sql
  -- Vì BIGSERIAL đã globally unique (monotonically increasing),
  -- thêm explicit unique constraint cho defensive querying
  ALTER TABLE plan ADD CONSTRAINT plan_id_unique UNIQUE (id);
  ALTER TABLE issue ADD CONSTRAINT issue_id_unique UNIQUE (id);
  ALTER TABLE task ADD CONSTRAINT task_id_unique UNIQUE (id);
  ALTER TABLE task_run ADD CONSTRAINT task_run_id_unique UNIQUE (id);
  ALTER TABLE plan_check_run ADD CONSTRAINT plan_check_run_id_unique UNIQUE (id);
  ALTER TABLE release ADD CONSTRAINT release_id_unique UNIQUE (id);
  ALTER TABLE db_group ADD CONSTRAINT db_group_id_unique UNIQUE (id);
  ALTER TABLE task_run_log ADD CONSTRAINT task_run_log_id_unique UNIQUE (id);
  ```
- **Acceptance Criteria**:
  - AC-1: Migration applies without data conflicts (BIGSERIAL already unique)
  - AC-2: `GetByID(id)` queries can use unique index efficiently
  - AC-3: Existing composite PK unchanged — no breaking change
  - AC-4: Rollback migration drops unique constraints cleanly

### FR-002: Store Query Validation
- **Mô tả**: Add runtime validation khi FindMessage thiếu ProjectID.
- **Logic**:
  ```go
  func (s *Store) GetPlan(ctx context.Context, find *FindPlanMessage) (*PlanMessage, error) {
      if find.ProjectID == nil && find.ID == nil {
          return nil, errors.New("GetPlan requires at least ProjectID or ID")
      }
      // Log warning nếu chỉ có ID mà không có ProjectID
      if find.ProjectID == nil && find.ID != nil {
          slog.Warn("GetPlan called without ProjectID — using id-only lookup",
              slog.Int64("id", *find.ID))
      }
      // ...
  }
  ```
- **Acceptance Criteria**:
  - AC-1: Queries without both ProjectID and ID return error
  - AC-2: Queries with only ID log warning (vẫn hoạt động nhờ unique index)
  - AC-3: No existing code paths break (audit all callers first)

### FR-003: Developer Documentation
- **Mô tả**: Document composite PK conventions trong contributing guide.
- **Content**:
  - Khi nào dùng composite PK vs single PK
  - Bắt buộc include project filter trong mọi query
  - JSONB field naming (camelCase from protobuf)
  - Index strategy cho composite PK tables
- **Acceptance Criteria**:
  - AC-1: Documentation file tại `docs/dev/composite-pk-conventions.md`
  - AC-2: Linked from main CONTRIBUTING.md

### FR-004: Query Performance Monitoring
- **Mô tả**: Add slow query logging cho composite PK tables.
- **Logic**: `pg_stat_statements` monitoring + alert cho sequential scans trên PK tables.
- **Acceptance Criteria**:
  - AC-1: Grafana dashboard cho query performance per table
  - AC-2: Alert khi sequential scan ratio > 5% cho PK tables

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component           | File                                    | Thay đổi                              |
|---------------------|-----------------------------------------|----------------------------------------|
| Store Validation    | `backend/store/plan.go`                 | Add ProjectID/ID validation            |
| Store Validation    | `backend/store/issue.go`                | Add ProjectID/ID validation            |
| Store Validation    | `backend/store/task.go`                 | Add ProjectID/ID validation            |
| Store Validation    | `backend/store/task_run.go`             | Add ProjectID/ID validation            |
| Documentation       | `docs/dev/composite-pk-conventions.md`  | New file                               |

### 3.2 Database Changes

New migration file:
```sql
-- backend/migrator/migration/prod/NEXT_VERSION/0001_add_unique_id_constraints.sql
ALTER TABLE plan ADD CONSTRAINT plan_id_unique UNIQUE (id);
ALTER TABLE issue ADD CONSTRAINT issue_id_unique UNIQUE (id);
ALTER TABLE task ADD CONSTRAINT task_id_unique UNIQUE (id);
ALTER TABLE task_run ADD CONSTRAINT task_run_id_unique UNIQUE (id);
ALTER TABLE plan_check_run ADD CONSTRAINT plan_check_run_id_unique UNIQUE (id);
ALTER TABLE release ADD CONSTRAINT release_id_unique UNIQUE (id);
ALTER TABLE db_group ADD CONSTRAINT db_group_id_unique UNIQUE (id);
ALTER TABLE task_run_log ADD CONSTRAINT task_run_log_id_unique UNIQUE (id);
```

---

## 4. Test Cases

| Test ID    | Mô tả                                                  | Expected Result                    |
|------------|---------------------------------------------------------|------------------------------------|
| TC-001     | Migration applies on empty DB                          | All unique constraints created     |
| TC-002     | Migration applies on production-like data              | No conflicts (BIGSERIAL unique)    |
| TC-003     | GetPlan with ProjectID + ID                             | Normal lookup (PK index)           |
| TC-004     | GetPlan with only ID                                    | Works (unique index) + warning log |
| TC-005     | GetPlan with neither ProjectID nor ID                   | Returns error                      |
| TC-006     | Rollback migration                                      | Unique constraints dropped         |
| TC-007     | pg_stat_statements shows no seq scans on PK tables      | Index usage verified               |

---

## 5. Rollout Plan

| Phase   | Mô tả                                        | Timeline     |
|---------|------------------------------------------------|--------------|
| Phase 1 | Unique index migration                        | Sprint 1     |
| Phase 2 | Store validation (plan, issue first)          | Sprint 1     |
| Phase 3 | Remaining store validations                   | Sprint 2     |
| Phase 4 | Developer documentation                       | Sprint 2     |
| Phase 5 | Query performance dashboard                   | Sprint 3     |

---

## 6. Risks & Mitigations

| Risk                                       | Impact | Mitigation                                |
|--------------------------------------------|--------|-------------------------------------------|
| Unique constraint conflicts on upgrade     | LOW    | BIGSERIAL already globally unique         |
| Store validation breaks existing callers   | MEDIUM | Audit all callers before validation       |
| Warning log noise from id-only queries     | LOW    | Rate-limit warnings, track and fix callers|
