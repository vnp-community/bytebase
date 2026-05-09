# Change Request: Data Masking

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-ENT-012                                               |
| **Feature ID**     | SEC-15                                                   |
| **Title**          | Column-Level Data Masking                                |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-08                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Triển khai **column-level data masking** cho dữ liệu nhạy cảm, tự động áp dụng masking rules khi query results được trả về qua SQL Editor. Hỗ trợ masking cho cả SQL databases và NoSQL document databases.

### 1.2 Mục tiêu
- Automatic masking cho sensitive columns (PII, financial, health)
- Multiple masking levels: NONE, PARTIAL, FULL
- Policy-based masking rules (per column, per classification)
- Unmask grants cho authorized users
- Support 22+ database engines

---

## 2. Yêu cầu chức năng

### FR-001: Masking Levels
- **Mô tả**: Ba mức masking cho dữ liệu.
- **Levels**:

| Level       | Mô tả                                   | Ví dụ Input          | Ví dụ Output          |
|-------------|------------------------------------------|----------------------|-----------------------|
| **NONE**    | Hiển thị đầy đủ                         | `john@example.com`   | `john@example.com`   |
| **PARTIAL** | Masking một phần (configurable pattern)  | `john@example.com`   | `j***@example.com`   |
| **FULL**    | Masking toàn bộ                         | `john@example.com`   | `******`             |

- **Acceptance Criteria**:
  - AC-1: Masking áp dụng tại query result level (source data unchanged)
  - AC-2: Partial masking pattern configurable per data type
  - AC-3: Full masking replaces with fixed-length asterisks

### FR-002: Masking Policy Configuration
- **Mô tả**: Admin configure masking rules.
- **Rule Types**:
  - **Column-level**: Specific masking per `table.column`
  - **Classification-based**: Auto-mask based on Data Classification (CR-ENT-013)
  - **Global policy**: Default masking for all sensitive-classified data
- **Configuration Example**:
  ```yaml
  masking_rules:
    - column: "users.email"
      level: PARTIAL
      pattern: "{first_char}***@{domain}"
    - column: "users.ssn"
      level: FULL
    - classification: "PII"
      level: PARTIAL
    - classification: "FINANCIAL"
      level: FULL
  ```
- **Acceptance Criteria**:
  - AC-1: Column-level rules override classification-based
  - AC-2: Rules scoped per project or global
  - AC-3: Admin UI for managing masking rules
  - AC-4: Masking policy changes require audit log

### FR-003: Unmask Grants
- **Mô tả**: Authorized users can request/receive unmask access.
- **Grant Mechanisms**:
  - Permanent grant: user always sees unmasked data for specific columns
  - Temporary grant: time-limited unmask access (JIT-style)
  - Request workflow: user requests → approver grants
- **Acceptance Criteria**:
  - AC-1: Unmask grant overrides masking policy
  - AC-2: Temporary grants auto-expire
  - AC-3: Grant/revoke actions audited

### FR-004: Masking Engine
- **Mô tả**: Runtime masking pipeline.
- **Pipeline** (from TDD Section 9):
  ```
  SQL Query → Parse AST → Identify columns → Evaluate masking policy
    → Check access grants → Apply masking algorithm → Return results
  ```
- **Supported Data Types**:
  - String (email, name, address, phone)
  - Numeric (SSN, credit card, account number)
  - Date/Time (birth date)
  - JSON/JSONB (nested paths for NoSQL)
  - Binary (masked as `[MASKED]`)
- **Acceptance Criteria**:
  - AC-1: Masking transparent to SQL syntax (works with JOINs, GROUP BY)
  - AC-2: Masked data NOT usable for WHERE clause filtering
  - AC-3: Performance overhead < 10% for typical queries
  - AC-4: Document masking for MongoDB, CosmosDB, Elasticsearch

### FR-005: Masking Algorithms
- **Mô tả**: Built-in masking algorithms.
- **Algorithms**:

| Algorithm           | Mô tả                            | Ví dụ                                    |
|--------------------|-----------------------------------|------------------------------------------|
| `MASK_FULL`        | Replace all chars                 | `hello` → `*****`                        |
| `MASK_FIRST_N`     | Mask first N chars               | `john` → `***n`                          |
| `MASK_LAST_N`      | Mask last N chars                | `john` → `j***`                          |
| `MASK_EMAIL`       | Mask email prefix                | `john@ex.com` → `j***@ex.com`           |
| `MASK_PHONE`       | Keep country code + last 4       | `+1-555-1234` → `+1-***-1234`           |
| `MASK_CREDIT_CARD` | Keep last 4 digits               | `4111-1111-1111-1111` → `****-****-****-1111` |
| `MASK_CUSTOM`      | Custom regex pattern             | Configurable                              |

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component                    | File/Package                         | Thay đổi                                          |
|------------------------------|--------------------------------------|----------------------------------------------------|
| Masking Evaluator            | `backend/component/masker/`          | Core masking engine (exists, enhance)              |
| Document Masking             | `backend/component/masker/document_masking.go` | NoSQL document masking          |
| SQL Service                  | `backend/api/v1/sql_service.go`      | Apply masking to query results                     |
| Masking Policy Service       | `backend/api/v1/org_policy_service.go` | Masking policy CRUD                              |
| Feature Gate                 | `enterprise/feature.go`              | Define `FeatureDataMasking`                        |

### 3.2 Frontend Changes

| Component             | File                                        | Thay đổi                                    |
|-----------------------|---------------------------------------------|----------------------------------------------|
| SQL Result Grid       | `frontend/src/components/SQLResultTable.vue` | Visual indicator for masked columns          |
| Masking Policy UI     | `frontend/src/views/MaskingPolicy.vue`       | Masking rules management                    |
| Column Settings       | `frontend/src/components/ColumnSettings.vue` | Per-column masking config                   |

---

## 4. Test Cases

| Test ID    | Mô tả                                                  | Expected Result                       |
|------------|----------------------------------------------------------|---------------------------------------|
| TC-001     | Query masked column as regular user                     | Data masked per policy                |
| TC-002     | Query masked column with unmask grant                   | Data shown unmasked                   |
| TC-003     | PARTIAL mask on email column                            | `j***@example.com`                    |
| TC-004     | FULL mask on SSN column                                 | `*********`                           |
| TC-005     | JOIN query with masked column                           | Masking preserved in results          |
| TC-006     | MongoDB document with nested PII                        | Nested fields masked                  |
| TC-007     | Temporary unmask grant expired                          | Data masked again                     |
| TC-008     | Non-ENTERPRISE: masking not applied                     | Feature gated                         |

---

## 5. Rollout Plan

| Phase   | Mô tả                              | Timeline       |
|---------|--------------------------------------|----------------|
| Phase 1 | Masking policy CRUD                  | Sprint 1       |
| Phase 2 | SQL masking engine                   | Sprint 2       |
| Phase 3 | Masking algorithms                   | Sprint 2       |
| Phase 4 | Unmask grants                        | Sprint 3       |
| Phase 5 | Document masking (NoSQL)             | Sprint 4       |
| Phase 6 | Performance optimization             | Sprint 4       |
