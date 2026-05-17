# Change Request: SQL Injection Deep Defense

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-SEC-016                                               |
| **Feature ID**     | DCM-06, SQL-01                                           |
| **Title**          | SQL Injection Deep Defense                               |
| **Plan**           | ALL                                                      |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-13                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Triển khai defense-in-depth chống SQL injection cho toàn bộ Bytebase platform. Đặc biệt quan trọng vì Bytebase SQL Editor cho phép execute SQL trực tiếp — cần phân biệt giữa legitimate SQL execution và SQL injection attacks trên metadata layer.

### 1.2 Bối cảnh
Bytebase có hai attack surfaces cho SQL injection:
1. **Metadata layer**: SQL injection vào Bytebase's own PostgreSQL metadata store
2. **Managed databases**: SQL injection thông qua SQL Editor → managed database instances

Bytebase đã có SQL Review (DCM-06, 200+ rules) cho managed databases. CR này focus vào metadata layer protection và enhanced controls cho managed database access.

---

## 2. Yêu cầu chức năng

### FR-001: Metadata Store Protection
- **Acceptance Criteria**:
  - AC-1: 100% parameterized queries trong store layer (pgx/v5 prepared statements)
  - AC-2: Static analysis tool integration (gosec, sqlc) trong CI pipeline
  - AC-3: No string concatenation cho SQL construction (compile-time check)
  - AC-4: Input validation layer trước store operations (protovalidate)
  - AC-5: Database user least privilege (separate read/write roles)

### FR-002: SQL Editor Security Controls
- **Acceptance Criteria**:
  - AC-1: Statement type restriction per permission (SELECT-only cho viewers)
  - AC-2: Dangerous statement detection (DROP, TRUNCATE, DELETE without WHERE)
  - AC-3: Multi-statement execution control (disable by default for non-admin)
  - AC-4: Query execution timeout enforcement (per environment)
  - AC-5: Row limit enforcement cho query results
  - AC-6: Read-only connection pool cho SQL Editor queries

### FR-003: SQL AST Validation
- **Acceptance Criteria**:
  - AC-1: Parse SQL to AST before execution (ANTLR parsers exist)
  - AC-2: Block SQL with suspicious patterns (stacked queries, UNION-based)
  - AC-3: Configurable deny-list of SQL functions/keywords per environment
  - AC-4: Admin bypass for legitimate operations

### FR-004: Query Logging & Analysis
- **Acceptance Criteria**:
  - AC-1: Log all SQL queries executed via SQL Editor (with user context)
  - AC-2: Detect anomalous query patterns (unusual tables, functions)
  - AC-3: Query classification: DDL/DML/DQL/DCL
  - AC-4: Alert on DCL (permission changes) via SQL Editor

---

## 3. Yêu cầu kỹ thuật

| Component                    | File/Package                                | Thay đổi                                   |
|------------------------------|---------------------------------------------|---------------------------------------------|
| Store Layer                  | `backend/store/`                            | Parameterized query audit                   |
| SQL Service                  | `backend/api/v1/sql_service.go`             | Statement type restriction, AST validation  |
| SQL Advisor                  | `backend/plugin/advisor/`                   | Security-focused lint rules                 |
| CI Pipeline                  | `.github/workflows/`                        | gosec + static analysis integration         |
| Query Audit                  | `backend/api/v1/audit.go`                   | SQL query logging enrichment                |

---

## 4. Test Cases

| Test ID | Mô tả                                               | Expected Result                  |
|---------|------------------------------------------------------|----------------------------------|
| TC-001  | String concat SQL in store layer (CI check)          | Build fails                      |
| TC-002  | Viewer executes DROP statement                       | Permission denied                |
| TC-003  | Multi-statement with DELETE in SQL Editor             | Blocked for non-admin            |
| TC-004  | UNION-based injection pattern                        | AST validation blocks            |
| TC-005  | Query timeout exceeded                               | Query killed, user notified      |
| TC-006  | DCL via SQL Editor (GRANT/REVOKE)                    | Alert + audit log                |
| TC-007  | Read-only connection pool for Editor                 | Write operations rejected at DB  |

---

## 5. Rollout Plan

| Phase   | Mô tả                              | Timeline       |
|---------|--------------------------------------|----------------|
| Phase 1 | Metadata store audit + CI checks     | Sprint 1       |
| Phase 2 | SQL Editor statement restrictions    | Sprint 2       |
| Phase 3 | AST validation + query logging       | Sprint 3       |
| Phase 4 | Anomaly detection                    | Sprint 4       |
