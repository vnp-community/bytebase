# Change Request: Data Classification

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-ENT-013                                               |
| **Feature ID**     | SEC-16                                                   |
| **Title**          | Data Classification                                      |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-08                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Triển khai hệ thống **Data Classification** cho phép phân loại dữ liệu theo sensitivity level (PII, Sensitive, Confidential, Public, etc.) ở column-level. Classification drives tự động Data Masking (CR-ENT-012) và Risk Assessment (CR-ENT-006).

### 1.2 Mục tiêu
- Column-level data classification tags
- Auto-classification suggestions (heuristic-based)
- Classification drives masking policy automatically
- Compliance reporting (GDPR, HIPAA data inventory)

---

## 2. Yêu cầu chức năng

### FR-001: Classification Schema
- **Mô tả**: Define hierarchical classification schema.
- **Default Schema**:

| Level          | Code   | Mô tả                                  | Default Masking |
|----------------|--------|------------------------------------------|-----------------|
| **PUBLIC**     | L0     | Non-sensitive, publicly available        | NONE            |
| **INTERNAL**   | L1     | Internal use only                        | NONE            |
| **SENSITIVE**  | L2     | Business-sensitive data                  | PARTIAL         |
| **CONFIDENTIAL**| L3    | PII, financial data                      | PARTIAL         |
| **RESTRICTED** | L4     | Highly regulated (healthcare, legal)     | FULL            |

- **Sub-categories**:
  - `PII` — Personally Identifiable Information (email, phone, SSN)
  - `PHI` — Protected Health Information
  - `PCI` — Payment Card Industry data
  - `FINANCIAL` — Financial records
  - `CREDENTIALS` — Passwords, keys, tokens
- **Acceptance Criteria**:
  - AC-1: Custom classification levels configurable
  - AC-2: Sub-categories tagable per column
  - AC-3: Classification schema exportable (JSON/YAML)

### FR-002: Column Classification Assignment
- **Mô tả**: Assign classification tags to database columns.
- **Assignment Methods**:
  - Manual: Admin/DBA sets classification per column via UI
  - Bulk: CSV import for mass classification
  - API: Programmatic classification via gRPC/REST API
- **Acceptance Criteria**:
  - AC-1: Classification visible in Schema Diagram
  - AC-2: Classification badge on SQL Editor column headers
  - AC-3: Bulk operations for entire table/database
  - AC-4: Classification changes audited

### FR-003: Auto-Classification Suggestions
- **Mô tả**: Heuristic-based auto-detection of sensitive columns.
- **Detection Heuristics**:

| Pattern                            | Suggested Classification |
|------------------------------------|--------------------------|
| Column name: `*email*`, `*mail*`   | PII / L3                |
| Column name: `*phone*`, `*mobile*` | PII / L3                |
| Column name: `*ssn*`, `*social*`   | PII / L4                |
| Column name: `*password*`, `*hash*`| CREDENTIALS / L4         |
| Column name: `*card*`, `*credit*`  | PCI / L4                |
| Column name: `*salary*`, `*balance*`| FINANCIAL / L3          |
| Column name: `*address*`, `*zip*`  | PII / L3                |
| Column name: `*dob*`, `*birth*`    | PII / L3                |
| Data pattern: email regex          | PII / L3                |
| Data pattern: credit card regex    | PCI / L4                |

- **Acceptance Criteria**:
  - AC-1: Auto-scan runs on schema sync
  - AC-2: Suggestions shown as "unconfirmed" — require human approval
  - AC-3: Scan results configurable (enable/disable patterns)
  - AC-4: No false positive should auto-apply (always human-in-the-loop)

### FR-004: Classification Reporting
- **Mô tả**: Compliance reporting cho classified data.
- **Reports**:
  - Data inventory: all classified columns across databases
  - Unclassified columns report (compliance gap)
  - Classification distribution by level/category
  - Per-regulation view (GDPR data map, HIPAA PHI inventory)
- **Acceptance Criteria**:
  - AC-1: Dashboard showing classification coverage
  - AC-2: Export reports (CSV, PDF)
  - AC-3: Filter by database, project, classification level

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component                    | File/Package                           | Thay đổi                                          |
|------------------------------|----------------------------------------|----------------------------------------------------|
| Classification Service       | `backend/api/v1/classification_service.go` | Classification CRUD                           |
| Schema Sync                  | `backend/runner/schemasync/`           | Auto-classification on sync                        |
| Database Catalog             | `backend/store/catalog.go`             | Store column classification                        |
| Feature Gate                 | `enterprise/feature.go`               | Define `FeatureDataClassification`                 |

### 3.2 Database Changes

```sql
-- Classification stored in database catalog (existing schema metadata)
-- Column classification in database_schema JSONB or separate table

CREATE TABLE column_classification (
    id BIGSERIAL PRIMARY KEY,
    database_uid BIGINT NOT NULL,
    schema_name TEXT NOT NULL,
    table_name TEXT NOT NULL,
    column_name TEXT NOT NULL,
    classification_level TEXT NOT NULL,
    sub_categories TEXT[] DEFAULT '{}',
    is_auto_detected BOOLEAN NOT NULL DEFAULT false,
    confirmed BOOLEAN NOT NULL DEFAULT false,
    confirmed_by BIGINT REFERENCES principal(id),
    created_ts TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_ts TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (database_uid, schema_name, table_name, column_name)
);
```

---

## 4. Test Cases

| Test ID    | Mô tả                                                  | Expected Result                       |
|------------|----------------------------------------------------------|---------------------------------------|
| TC-001     | Classify column as PII/L3                               | Classification saved, masking applied |
| TC-002     | Auto-detect email column                                | Suggestion: PII/L3 (unconfirmed)     |
| TC-003     | Confirm auto-detected classification                    | Status: confirmed                     |
| TC-004     | Classification report export                            | CSV with all classified columns       |
| TC-005     | Unclassified columns report                             | Lists columns without classification  |
| TC-006     | Bulk classify via CSV import                            | All columns classified                |
| TC-007     | Non-ENTERPRISE: classification hidden                   | Feature gated                         |

---

## 5. Rollout Plan

| Phase   | Mô tả                              | Timeline       |
|---------|--------------------------------------|----------------|
| Phase 1 | Classification schema + CRUD         | Sprint 1       |
| Phase 2 | Manual classification UI             | Sprint 1       |
| Phase 3 | Auto-classification engine           | Sprint 2       |
| Phase 4 | Masking integration                  | Sprint 2       |
| Phase 5 | Reporting + compliance dashboard     | Sprint 3       |
