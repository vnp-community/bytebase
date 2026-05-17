# TASK-ENT-015 — Data Masking Pipeline Enhancement

| Field            | Value                                      |
|------------------|--------------------------------------------|
| **Task ID**      | TASK-ENT-015                               |
| **Source**       | SOL-ENT-012 (CR-ENT-012)                  |
| **Status**       | Done                                       |
| **Priority**     | P0                                         |
| **Complexity**   | Very High                                  |
| **Sprint**       | Sprint 1–4                                 |

---

## Mô tả

Enhance masking pipeline hiện có (L5 `component/masker/`, L4 `masking_evaluator.go`, `query_result_masker.go`) để hỗ trợ full column-level masking với expanded algorithms, classification-based auto-masking, unmask grants.

## Scope

### Phase 1 — Sprint 1: Masking Policy CRUD
1. **L4 — OrgPolicyService**: Masking policy CRUD (column-level rules)
2. **L9 — Feature Gate**: `FeatureDataMasking`
3. **Policy Resolution Order**: Column-level → Classification-based → Global default → Override (access grant)

### Phase 2 — Sprint 2: SQL Masking Engine + Algorithms
4. **L5 — Masker Enhancement**: Expand masking algorithms:
   - `MASK_FULL`, `MASK_FIRST_N`, `MASK_LAST_N`
   - `MASK_EMAIL` (`j***@domain.com`)
   - `MASK_PHONE` (`+1-***-1234`)
   - `MASK_CREDIT_CARD` (`****-****-****-1111`)
   - `MASK_CUSTOM` (configurable regex pattern)
5. **L4 — MaskingEvaluator Enhancement**: Per-column policy resolution with classification-based rules
6. **L4 — QueryResultMasker Enhancement**: Apply masking algorithms to query results

### Phase 3 — Sprint 3: Unmask Grants
7. **L4 — AccessGrantService Enhancement**: Temporary unmask grants (user, columns, expiry)
8. **Unmask Override**: Access grant overrides masking policy for specified columns

### Phase 4 — Sprint 4: NoSQL + Performance
9. **L4 — Document Masking Enhancement** (`document_masking.go`): JSON path-based rules, nested field traversal, array element masking for MongoDB/CosmosDB/Elasticsearch
10. **Performance**: Optimize masking pipeline for large result sets

## Acceptance Criteria

- [x] All 7 masking algorithms implemented and tested
- [x] Column-level masking policy CRUD functional
- [x] Classification-based auto-masking works (CR-ENT-013)
- [x] Policy resolution order correct (column > classification > global > grant override)
- [x] Unmask grants: temporary, column-specific, with expiry
- [x] NoSQL document masking: JSON path, nested fields, arrays
- [x] Performance: <50ms overhead for 10K row result set
- [x] Masking evaluator correctly resolves per-column policies

## Dependencies

- CR-ENT-013 (Data Classification) — drives auto-masking
- CR-ENT-005 (Copy Restriction) — defense-in-depth
- CR-ENT-021 (Watermark) — applied simultaneously
- CR-ENT-017 (JIT Access) — unmask grants

## Definition of Done

- [x] All algorithms tested with edge cases
- [x] Masking pipeline performance benchmarked
- [x] NoSQL masking tested with real documents
