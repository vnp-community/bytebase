# Solution: CR-ENT-013 — Data Classification

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-ENT-013                |
| **Solution**   | SOL-ENT-013               |
| **Status**     | Proposed                  |
| **Complexity** | Medium                    |

---

## 1. Tóm tắt giải pháp

Xây dựng hệ thống Data Classification tại column-level với hierarchical schema (L0-L4), auto-detection heuristics tích hợp vào SchemaSync runner (L6), và compliance reporting. Classification drives Data Masking (CR-ENT-012) tự động.

---

## 2. Architectural Alignment

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L4 — Service** | `classification_service.go` (NEW) | Classification CRUD + reporting |
| **L6 — Runner** | `runner/schemasync/` | Auto-classification on schema sync |
| **L8 — Store** | `column_classification` (NEW) | Classification persistence |
| **L9 — Enterprise** | `feature.go` | `FeatureDataClassification` gate |
| **L1 — Presentation** | Schema Diagram, SQL Editor | Classification badges |

---

## 3. Chi tiết Implementation

### 3.1 Classification Schema

| Level | Code | Default Masking |
|-------|------|----------------|
| PUBLIC | L0 | NONE |
| INTERNAL | L1 | NONE |
| SENSITIVE | L2 | PARTIAL |
| CONFIDENTIAL | L3 | PARTIAL |
| RESTRICTED | L4 | FULL |

Sub-categories: `PII`, `PHI`, `PCI`, `FINANCIAL`, `CREDENTIALS`

### 3.2 Auto-Classification Heuristics

```go
// runner/schemasync/classifier.go
var classificationPatterns = []struct {
    Pattern  *regexp.Regexp
    Level    string
    Category string
}{
    {regexp.MustCompile(`(?i)(email|mail)`), "L3", "PII"},
    {regexp.MustCompile(`(?i)(phone|mobile)`), "L3", "PII"},
    {regexp.MustCompile(`(?i)(ssn|social)`), "L4", "PII"},
    {regexp.MustCompile(`(?i)(password|hash|secret)`), "L4", "CREDENTIALS"},
    {regexp.MustCompile(`(?i)(card|credit)`), "L4", "PCI"},
    {regexp.MustCompile(`(?i)(salary|balance)`), "L3", "FINANCIAL"},
}
```

Suggestions stored as `is_auto_detected=true, confirmed=false` — require human approval.

### 3.3 Schema Migration

```sql
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

### 3.4 Integration with SchemaSync

SchemaSync runner already syncs schema metadata periodically. Add auto-classification step after schema sync completes.

### 3.5 Compliance Reporting

- Data inventory: all classified columns across databases
- Unclassified columns report (gap analysis)
- Per-regulation view (GDPR/HIPAA mapping)

---

## 4. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-ENT-012 | Classification drives auto-masking policy |
| CR-ENT-006 | Table classification feeds risk scoring (15% weight) |

---

## 5. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | Classification schema + CRUD | Sprint 1 |
| 2 | Manual classification UI | Sprint 1 |
| 3 | Auto-classification engine | Sprint 2 |
| 4 | Masking integration | Sprint 2 |
| 5 | Reporting + compliance dashboard | Sprint 3 |
