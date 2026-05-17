# Solution: CR-ENT-012 — Data Masking

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-ENT-012                |
| **Solution**   | SOL-ENT-012               |
| **Status**     | Proposed                  |
| **Complexity** | Very High                 |

---

## 1. Tóm tắt giải pháp

Enhance masking pipeline hiện có (L5 `component/masker/`, L4 `masking_evaluator.go`, `query_result_masker.go`) để hỗ trợ full column-level masking. Codebase đã có masking infrastructure — giải pháp tập trung vào expanding masking algorithms, classification-based auto-masking, unmask grants, và NoSQL document masking.

---

## 2. Architectural Alignment

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L5 — Component** | `component/masker/` | Core masking algorithms (enhance existing) |
| **L4 — Service** | `masking_evaluator.go` (12KB) | Column masking policy evaluation |
| **L4 — Service** | `query_result_masker.go` (18KB) | Apply masking to query results |
| **L4 — Service** | `document_masking.go` (44KB) | NoSQL document masking (MongoDB, CosmosDB) |
| **L4 — Service** | `org_policy_service.go` | Masking policy CRUD |
| **L4 — Service** | `access_grant_service.go` | Unmask grants (JIT-style) |
| **L9 — Enterprise** | `feature.go` | `FeatureDataMasking` gate |

---

## 3. Chi tiết Implementation

### 3.1 Masking Pipeline (from TDD §9)

```
SQL Query → Parse AST (L7 SQL Parser)
  → Identify columns in result set
  → MaskingEvaluator: per-column policy resolution
    → Check column-level rules
    → Check classification-based rules (CR-ENT-013)
    → Check access grants (unmask overrides)
  → QueryResultMasker: apply masking algorithms
  → Return masked results
```

### 3.2 Masking Algorithms

| Algorithm | Implementation |
|-----------|---------------|
| `MASK_FULL` | Replace all chars with `*` |
| `MASK_FIRST_N` | Mask first N chars |
| `MASK_LAST_N` | Mask last N chars |
| `MASK_EMAIL` | `j***@domain.com` |
| `MASK_PHONE` | `+1-***-1234` |
| `MASK_CREDIT_CARD` | `****-****-****-1111` |
| `MASK_CUSTOM` | Configurable regex pattern |

### 3.3 Masking Policy Resolution Order

1. Column-level rule (`table.column` → masking level)
2. Classification-based rule (`classification=PII` → `PARTIAL`)
3. Global default policy
4. **Override**: Access grant (`unmask=true`) overrides all above

### 3.4 Unmask Grants

```go
// access_grant_service.go (already exists)
// Enhance to support temporary unmask grants
type AccessGrant struct {
    User       string
    Columns    []string  // specific columns or wildcard
    UnmaskLevel MaskingLevel
    ExpiresAt  time.Time  // temporary grant
}
```

### 3.5 Document Masking (NoSQL)

`document_masking.go` (44KB) already handles MongoDB, CosmosDB, Elasticsearch. Enhance:
- JSON path-based masking rules
- Nested field traversal
- Array element masking

---

## 4. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-ENT-013 | Data Classification drives auto-masking |
| CR-ENT-005 | Copy restriction + masking = defense-in-depth |
| CR-ENT-021 | Watermark + masking applied simultaneously |
| CR-ENT-017 | JIT Access can include unmask grants |

---

## 5. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | Masking policy CRUD | Sprint 1 |
| 2 | SQL masking engine enhancement | Sprint 2 |
| 3 | Masking algorithms | Sprint 2 |
| 4 | Unmask grants | Sprint 3 |
| 5 | Document masking (NoSQL) | Sprint 4 |
| 6 | Performance optimization | Sprint 4 |
