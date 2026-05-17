# TASK-ENT-016 — Data Classification System

| Field            | Value                                      |
|------------------|--------------------------------------------|
| **Task ID**      | TASK-ENT-016                               |
| **Source**       | SOL-ENT-013 (CR-ENT-013)                  |
| **Status**       | Done                                       |
| **Priority**     | P1                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 1–3                                 |

---

## Mô tả

Xây dựng hệ thống Data Classification tại column-level với hierarchical schema (L0-L4), auto-detection heuristics, và compliance reporting.

## Scope

### Phase 1 — Sprint 1: Classification Schema + CRUD + UI
1. **Classification Levels**: PUBLIC (L0), INTERNAL (L1), SENSITIVE (L2), CONFIDENTIAL (L3), RESTRICTED (L4)
2. **Sub-Categories**: PII, PHI, PCI, FINANCIAL, CREDENTIALS
3. **Schema Migration**: `column_classification` table — database_uid, schema, table, column, level, sub_categories, auto_detected flag, confirmed flag
4. **L4 — ClassificationService (NEW)**: CRUD + reporting APIs
5. **L1 — Frontend**: Manual classification UI on Schema Diagram + SQL Editor
6. **L9 — Feature Gate**: `FeatureDataClassification`

### Phase 2 — Sprint 2: Auto-Classification + Masking Integration
7. **L6 — SchemaSync Integration**: Auto-classification after schema sync via pattern matching:
   - `email|mail` → L3/PII
   - `phone|mobile` → L3/PII
   - `ssn|social` → L4/PII
   - `password|hash|secret` → L4/CREDENTIALS
   - `card|credit` → L4/PCI
   - `salary|balance` → L3/FINANCIAL
8. **Auto-Detected Suggestions**: `is_auto_detected=true, confirmed=false` — require human approval
9. **Masking Integration**: Classification level drives auto-masking policy (CR-ENT-012)

### Phase 3 — Sprint 3: Compliance Reporting
10. **Data Inventory**: All classified columns across databases
11. **Gap Analysis**: Unclassified columns report
12. **Compliance View**: GDPR/HIPAA regulation mapping

## Acceptance Criteria

- [x] 5 classification levels (L0-L4) with sub-categories
- [x] Manual classification CRUD functional
- [x] Auto-classification patterns detect common sensitive data
- [x] Auto-detected suggestions require human confirmation
- [x] Classification drives masking policy automatically
- [x] Data inventory report comprehensive
- [x] Unclassified columns gap analysis functional
- [x] Schema Diagram shows classification badges
- [x] SQL Editor shows classification info

## Dependencies

- CR-ENT-012 (Data Masking) — classification drives auto-masking
- CR-ENT-006 (Risk Assessment) — table classification feeds scoring (15%)

## Definition of Done

- [x] Classification CRUD + auto-detection tested
- [x] Masking integration verified
- [x] Compliance reports functional
