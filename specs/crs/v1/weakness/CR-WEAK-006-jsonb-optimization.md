# Change Request: JSONB Query Optimization & Convention

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-WEAK-006                                              |
| **Weakness ID**    | WEAK-006                                                 |
| **Title**          | JSONB Query Optimization — GIN Indexes & Naming Docs     |
| **Category**       | Data Access / Performance                                |
| **Priority**       | P3 — Low                                                 |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-08                                               |
| **Author**         | VNP AI Ops Team                                          |
| **PRD Refs**       | DCM-01 (Plan/Issue), SEC-10 (Audit Log)                  |

---

## 1. Tổng quan

### 1.1 Mô tả
Thêm GIN indexes cho frequently queried JSONB paths, extract high-frequency query fields to dedicated columns, và document protobuf JSON naming convention (camelCase).

### 1.2 Bối cảnh
Protobuf JSON serialization (`protojson.Marshal`) outputs camelCase field names, while PostgreSQL convention is snake_case. Developers may write `config->>'plan_config'` (wrong) instead of `config->>'planConfig'` (correct). JSONB columns lack indexes → queries filtering on JSONB content do sequential scans.

### 1.3 Mục tiêu
- GIN indexes cho top-10 queried JSONB paths
- Extract 3-5 most queried JSONB fields to dedicated columns
- Developer documentation cho naming convention
- Zero breaking changes

---

## 2. Yêu cầu chức năng

### FR-001: GIN Indexes cho JSONB Columns
- **Migration**:
  ```sql
  -- Frequently queried JSONB paths
  CREATE INDEX idx_task_payload_gin ON task USING GIN (payload jsonb_path_ops);
  CREATE INDEX idx_policy_payload_gin ON policy USING GIN (payload jsonb_path_ops);
  CREATE INDEX idx_setting_value_gin ON setting USING GIN (value jsonb_path_ops);
  CREATE INDEX idx_issue_payload_gin ON issue USING GIN (payload jsonb_path_ops);
  ```
- **Acceptance Criteria**:
  - AC-1: JSONB containment queries (`@>`) use GIN index
  - AC-2: No regression in write performance (< 5% overhead)
  - AC-3: Index size acceptable (< 20% of table data size)

### FR-002: High-Frequency Field Extraction
- **Mô tả**: Extract frequently filtered JSONB fields to dedicated columns.
- **Candidates**:

  | Table  | JSONB Path              | New Column         | Type     |
  |--------|-------------------------|--------------------|----------|
  | task   | `payload.release`       | `release_name`     | TEXT     |
  | issue  | `payload.type`          | `issue_type`       | TEXT     |

- **Acceptance Criteria**:
  - AC-1: Backfill migration populates new columns from JSONB
  - AC-2: Store layer reads from dedicated column, falls back to JSONB
  - AC-3: Write path updates both column and JSONB for compatibility

### FR-003: Naming Convention Documentation
- **Content**: `docs/dev/jsonb-naming-convention.md`
  - Protobuf → JSON = camelCase (via protojson)
  - PostgreSQL columns = snake_case
  - JSONB content keys = camelCase (match protobuf)
  - Examples of correct and incorrect queries
- **Acceptance Criteria**:
  - AC-1: Documentation exists and linked from CONTRIBUTING.md
  - AC-2: SQL advisor rule warns on snake_case JSONB path access (optional)

---

## 3. Yêu cầu kỹ thuật

### 3.1 Database Changes

```sql
-- Migration: Add GIN indexes
CREATE INDEX CONCURRENTLY idx_task_payload_gin ON task USING GIN (payload jsonb_path_ops);
CREATE INDEX CONCURRENTLY idx_policy_payload_gin ON policy USING GIN (payload jsonb_path_ops);
CREATE INDEX CONCURRENTLY idx_issue_payload_gin ON issue USING GIN (payload jsonb_path_ops);

-- Migration: Extract high-frequency fields
ALTER TABLE task ADD COLUMN release_name TEXT;
UPDATE task SET release_name = payload->>'release' WHERE payload->>'release' IS NOT NULL;
CREATE INDEX idx_task_release_name ON task (release_name) WHERE release_name IS NOT NULL;
```

### 3.2 Backend Changes

| Component         | File                    | Thay đổi                                  |
|-------------------|-------------------------|--------------------------------------------|
| Store task.go     | `backend/store/task.go` | Read from release_name column              |
| Documentation     | `docs/dev/`             | New naming convention guide                |

---

## 4. Test Cases

| Test ID    | Mô tả                                             | Expected Result               |
|------------|-----------------------------------------------------|-------------------------------|
| TC-001     | JSONB containment query uses GIN index              | EXPLAIN shows Index Scan      |
| TC-002     | task.release_name populated after migration         | Matches payload->>'release'   |
| TC-003     | Write to task updates both column and JSONB         | Both consistent               |
| TC-004     | GIN index creation does not lock table              | CONCURRENTLY prevents locks   |

---

## 5. Rollout Plan

| Phase   | Mô tả                                  | Timeline     |
|---------|------------------------------------------|--------------|
| Phase 1 | GIN indexes (CONCURRENTLY)              | Sprint 1     |
| Phase 2 | Field extraction migration              | Sprint 2     |
| Phase 3 | Store layer read optimization           | Sprint 2     |
| Phase 4 | Documentation                           | Sprint 2     |

---

## 6. Risks & Mitigations

| Risk                                   | Impact | Mitigation                          |
|----------------------------------------|--------|-------------------------------------|
| GIN index increases storage            | LOW    | Monitor index size, drop if too big |
| Field extraction breaks JSONB readers  | MEDIUM | Dual-read (column + JSONB fallback) |
| CONCURRENTLY fails mid-migration       | LOW    | Retry, PG handles partial index     |
