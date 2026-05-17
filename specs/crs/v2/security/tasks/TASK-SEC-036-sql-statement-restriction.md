# TASK-SEC-036 — SQL Statement Type Restriction + Read-Only Pool

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-036                               |
| **Source**       | SOL-SEC-016 §3.1, §3.2                    |
| **Status**       | Pending                                    |
| **Priority**     | P0                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 1                                   |

---

## Mô tả

Implement SQL statement type restriction trong SQLService (L4) và read-only connection pool trong DBFactory (L5).

## Scope

1. **Statement type check**: Parse SQL via existing ANTLR parser (L7), determine type (DDL/DML/DQL/DCL), check against role-based policy
2. **Row limit enforcement**: `addRowLimit(stmt, maxRows, engine)` — append LIMIT clause
3. **Read-only pool**: `DBFactory.GetReadOnlyDriver()` — use read-only DataSource or `ReadOnlyWrapper` with `SET TRANSACTION READ ONLY`
4. **ReadOnlyWrapper**: `.Execute()` → error, `.QueryConn()` → read-only transaction
5. **Policy**: Per-environment query policy (which statement types allowed per role)

## Acceptance Criteria

- [ ] DDL blocked for non-DBA users
- [ ] Row limit enforced
- [ ] Read-only pool prevents writes
- [ ] SELECT uses read-only connection
- [ ] Policy configurable per environment

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/api/v1/sql_service.go` | Statement type check |
| `backend/component/dbfactory/factory.go` | Read-only pool |

## Definition of Done

- Statement restrictions verified per role
