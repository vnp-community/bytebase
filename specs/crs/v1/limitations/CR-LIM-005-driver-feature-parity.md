# Change Request: Database Driver Feature Parity Enhancement

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-LIM-005                                               |
| **Limitation ID**  | LIM-005                                                  |
| **Title**          | Database Driver Feature Parity Enhancement               |
| **Category**       | Feature Coverage / Plugin                                |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-08                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Nâng cao feature parity giữa 22 database engine drivers bằng cách: mở rộng **SQL Advisor** cho SQL-capable engines thiếu coverage, thêm **Online Schema Change cho PostgreSQL**, cải thiện **parser accuracy**, và xây dựng **feature matrix UI** cho user transparency.

### 1.2 Bối cảnh
Hiện tại 14/22 engines thiếu SQL Advisor rules. Online Schema Change chỉ hỗ trợ MySQL (gh-ost). Prior backup chỉ hỗ trợ một số engines. Nhiều parser có acknowledged TODO/FIXME gaps (Oracle, Spanner, BigQuery, etc.). Users không có visibility rõ ràng về feature support per engine.

### 1.3 Mục tiêu
- SQL Advisor coverage cho ClickHouse, BigQuery, Spanner, StarRocks (SQL-capable engines)
- Online Schema Change cho PostgreSQL (pg-osc hoặc pgroll integration)
- Parser FIXME/TODO resolution cho top-priority engines
- Feature matrix UI hiển thị rõ ràng capability per engine
- Driver conformance test suite đảm bảo quality cho tất cả drivers

---

## 2. Yêu cầu chức năng

### FR-001: SQL Advisor Expansion — ClickHouse
- **Mô tả**: Implement SQL review rules cho ClickHouse.
- **Rule Categories**:
  | Category           | Rules                                                  | Priority |
  |--------------------|--------------------------------------------------------|----------|
  | Naming Convention  | Table/column naming patterns                           | P0       |
  | Schema Design      | Engine selection, partition key validation              | P0       |
  | Query Safety       | SELECT *, LIMIT requirements, JOIN optimization        | P0       |
  | Data Type          | Nullable vs LowCardinality, String vs FixedString      | P1       |
  | Index              | ORDER BY key usage, skip index recommendations         | P1       |
- **Target**: 30+ rules (vs 200+ cho PG/MySQL)
- **Acceptance Criteria**:
  - AC-1: ≥ 30 ClickHouse-specific lint rules operational
  - AC-2: Rules integrated into SQL Review pipeline
  - AC-3: Rule documentation available

### FR-002: SQL Advisor Expansion — BigQuery
- **Mô tả**: Implement SQL review rules cho BigQuery.
- **Rule Categories**:
  | Category           | Rules                                                  | Priority |
  |--------------------|--------------------------------------------------------|----------|
  | Cost Optimization  | SELECT *, partition pruning, clustering usage           | P0       |
  | Schema Design      | Nested/repeated fields, partition requirements          | P0       |
  | Query Safety       | Full table scans, slot utilization hints                | P0       |
  | Naming Convention  | Dataset/table/column naming                            | P1       |
- **Target**: 25+ rules
- **Acceptance Criteria**:
  - AC-1: ≥ 25 BigQuery-specific lint rules
  - AC-2: Cost-aware rules flag expensive query patterns

### FR-003: SQL Advisor Expansion — Spanner & StarRocks
- **Spanner**: 20+ rules — interleaved table design, index coverage, hotspot detection
- **StarRocks**: 15+ rules — materialized view usage, distribution key, bucket sizing
- **Acceptance Criteria**:
  - AC-1: Rules validated against engine-specific best practices
  - AC-2: Integration with existing advisor pipeline

### FR-004: Online Schema Change — PostgreSQL
- **Mô tả**: Integrate PostgreSQL online schema change tool (pgroll hoặc pg-osc).
- **Supported Operations**:
  - ADD COLUMN (non-nullable with default)
  - ALTER COLUMN TYPE
  - ADD INDEX CONCURRENTLY
  - DROP COLUMN (with expand-and-contract pattern)
- **Implementation**: Plugin wrapper around `pgroll` CLI
- **Acceptance Criteria**:
  - AC-1: Zero-downtime ALTER TABLE cho supported operations
  - AC-2: Progress tracking via API/UI
  - AC-3: Automatic rollback on failure
  - AC-4: Works with PostgreSQL 14+

### FR-005: Parser Accuracy Improvements
- **Mô tả**: Resolve documented FIXME/TODO gaps in SQL parsers.
- **Priority Fixes**:
  | Engine       | Fix                                          | Effort   |
  |--------------|----------------------------------------------|----------|
  | Oracle/PLSQL | Bind variables, USING clause, xmltable       | 2 sprints|
  | Spanner      | Dashed path expressions, recursive CTE       | 1 sprint |
  | BigQuery     | UNION alias handling                         | 1 sprint |
  | Redshift     | Cross-database query support                 | 1 sprint |
  | CockroachDB  | Expression extraction completion             | 1 sprint |
  | StarRocks    | Parser compat, index information             | 1 sprint |
- **Acceptance Criteria**:
  - AC-1: Each fix has regression test suite
  - AC-2: FIXME/TODO markers removed after resolution
  - AC-3: Parser accuracy ≥ 95% on engine test corpus

### FR-006: Feature Matrix UI
- **Mô tả**: Hiển thị feature support matrix per engine trên UI và documentation.
- **Display Location**: Instance creation wizard + Documentation site
- **Matrix Dimensions**:
  - SQL Review (rule count)
  - Schema Dump (full/partial/none)
  - Prior Backup (yes/no)
  - Online Schema Change (yes/no)
  - Data Masking (column/document/none)
  - Schema Sync (yes/no)
  - Change History (yes/no)
- **Acceptance Criteria**:
  - AC-1: Matrix auto-generated from driver capability registry
  - AC-2: Displayed during instance creation
  - AC-3: API endpoint: `GET /v1/engines/{engine}/capabilities`

### FR-007: Driver Conformance Test Suite
- **Mô tả**: Automated test suite validating all drivers against capability interface.
- **Test Categories**:
  - Connection lifecycle (Open/Close/Ping)
  - Query execution (Execute/Query)
  - Schema introspection (SyncInstance/SyncDBSchema)
  - Schema export (Dump)
  - Engine-specific features
- **Acceptance Criteria**:
  - AC-1: Each driver has ≥ 80% conformance test coverage
  - AC-2: Test suite runs in CI pipeline
  - AC-3: Conformance report generated per release

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component                    | File/Package                              | Thay đổi                                          |
|------------------------------|-------------------------------------------|----------------------------------------------------|
| ClickHouse Advisor           | `backend/plugin/advisor/clickhouse/`      | New advisor rules package                          |
| BigQuery Advisor             | `backend/plugin/advisor/bigquery/`        | New advisor rules package                          |
| Spanner Advisor              | `backend/plugin/advisor/spanner/`         | New advisor rules package                          |
| StarRocks Advisor            | `backend/plugin/advisor/starrocks/`       | New advisor rules package                          |
| PG Online Schema Change      | `backend/plugin/db/pg/online_schema.go`   | pgroll integration wrapper                         |
| Engine Capability Registry   | `backend/plugin/db/capability.go`         | Feature capability declaration per engine          |
| Capability API               | `backend/api/v1/engine_service.go`        | Expose capabilities per engine                     |
| Parser Fixes (Oracle)        | `backend/plugin/parser/plsql/`            | FIXME resolution                                   |
| Parser Fixes (Spanner)       | `backend/plugin/parser/spanner/`          | FIXME resolution                                   |
| Conformance Tests            | `backend/plugin/db/conformance_test.go`   | Cross-engine conformance test suite                |

### 3.2 Frontend Changes

| Component           | File                                    | Thay đổi                                    |
|---------------------|-----------------------------------------|----------------------------------------------| 
| Feature Matrix      | `frontend/src/components/FeatureMatrix` | Engine capability matrix component           |
| Instance Wizard     | `frontend/src/views/InstanceCreate`     | Show capabilities during creation            |

---

## 4. Phụ thuộc

| Dependency           | Mô tả                                                    |
|----------------------|-----------------------------------------------------------|
| pgroll               | PostgreSQL online schema migration tool                   |
| ANTLR v4 grammars    | Parser grammars per engine (existing)                     |
| Test databases       | Docker containers for each engine in CI                   |

---

## 5. Test Cases

| Test ID    | Mô tả                                                  | Expected Result                        |
|------------|----------------------------------------------------------|----------------------------------------|
| TC-001     | ClickHouse: run 30+ advisor rules on test SQL           | All rules produce expected output      |
| TC-002     | BigQuery: cost-aware rules detect SELECT *               | Warning generated                      |
| TC-003     | PG online schema change: ADD COLUMN on live table        | Zero-downtime, column added            |
| TC-004     | PG online schema change: failure rollback                | Table unchanged after failure          |
| TC-005     | Oracle parser: bind variables parsed correctly           | No parser errors                       |
| TC-006     | Feature matrix API for PostgreSQL                        | Full capability list returned          |
| TC-007     | Feature matrix API for Redis                             | Limited capability list returned       |
| TC-008     | Driver conformance: all 22 engines pass base tests       | ≥ 80% coverage each                   |

---

## 6. Rollout Plan

| Phase   | Mô tả                                          | Timeline       |
|---------|--------------------------------------------------|----------------|
| Phase 1 | Engine capability registry + Feature matrix UI   | Sprint 1-2     |
| Phase 2 | ClickHouse SQL Advisor (30+ rules)               | Sprint 2-4     |
| Phase 3 | BigQuery SQL Advisor (25+ rules)                 | Sprint 4-6     |
| Phase 4 | PostgreSQL Online Schema Change (pgroll)         | Sprint 5-7     |
| Phase 5 | Spanner + StarRocks advisors                     | Sprint 7-8     |
| Phase 6 | Parser FIXME resolution                          | Sprint 8-10    |
| Phase 7 | Conformance test suite                           | Sprint 10-11   |

---

## 7. Risks & Mitigations

| Risk                                    | Impact | Mitigation                                          |
|-----------------------------------------|--------|------------------------------------------------------|
| pgroll compatibility issues             | MEDIUM | Extensive testing on PG 14/15/16                     |
| Advisor false positives                 | MEDIUM | Community feedback loop, rule tuning                 |
| Parser regressions                      | HIGH   | Regression test corpus per engine                    |
| CI time increase (22 engine containers) | LOW    | Parallel test execution, selective runs              |
