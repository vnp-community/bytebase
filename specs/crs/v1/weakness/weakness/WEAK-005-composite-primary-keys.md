# WEAK-005 — Composite Primary Keys Create Complex Query Patterns

| Metadata       | Value                                      |
|----------------|--------------------------------------------|
| ID             | WEAK-005                                   |
| Category       | Database Design                            |
| Severity       | MEDIUM                                     |
| Affected Layer | L8 (Data Access — Store)                   |
| Source Files   | `backend/store/*.go`, `backend/migrator/migration/LATEST.sql` |

---

## Mô tả

Nhiều bảng metadata sử dụng composite primary key `(project, id)` thay vì single-column PK. Tạo complexity cho queries và ORM-style data access.

## Chi tiết

### Tables với Composite PK

```sql
CREATE TABLE plan (project TEXT NOT NULL, id BIGSERIAL, PRIMARY KEY (project, id));
CREATE TABLE issue (project TEXT NOT NULL, id BIGSERIAL, PRIMARY KEY (project, id));
CREATE TABLE task (project TEXT NOT NULL, id BIGSERIAL, PRIMARY KEY (project, id));
CREATE TABLE task_run (project TEXT NOT NULL, id BIGSERIAL, PRIMARY KEY (project, id));
CREATE TABLE plan_check_run (project TEXT NOT NULL, id BIGSERIAL, PRIMARY KEY (project, id));
CREATE TABLE release (project TEXT NOT NULL, id BIGSERIAL, PRIMARY KEY (project, id));
CREATE TABLE db_group (project TEXT NOT NULL, id BIGSERIAL, PRIMARY KEY (project, id));
CREATE TABLE task_run_log (project TEXT NOT NULL, id BIGSERIAL, PRIMARY KEY (project, id));
```

### Problems

1. **Mọi FK reference phải include project** — increases join complexity.
2. **URL patterns phải encode project** — e.g., `projects/{project}/plans/{plan}`.
3. **Cross-project queries** khó thực hiện — partition boundary.
4. **Auto-increment behavior** — `BIGSERIAL` tăng globally nhưng PK là (project, id) → id không unique alone.
5. **Migration history** — changelog, revision must track both project and resource ID.

### Code Evidence

```go
// Store methods phải luôn nhận project parameter
func (s *Store) GetPlan(ctx, find *FindPlanMessage) // FindPlanMessage includes ProjectID
func (s *Store) GetIssue(ctx, find *FindIssueMessage) // FindIssueMessage includes ProjectID
```

## Impact

- Queries without project filter **cannot use PK index** → full table scan risk.
- Foreign key references more verbose → larger index sizes.
- API resource naming forced to include project in all paths.

## Khuyến nghị

1. Document clear guidelines cho composite PK usage.
2. Ensure all queries include project filter → prevent performance regression.
3. Consider adding unique constraint on `id` alone for simpler lookups.
