# Solution: CR-PRV-002 — Data Anonymization & Pseudonymization

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-PRV-002                |
| **Solution**   | SOL-PRV-002               |
| **Status**     | Proposed                  |
| **Complexity** | Very High                 |

---

## 1. Tóm tắt giải pháp

Mở rộng Masking pipeline hiện có (L5 `component/masker/`) bằng cách thêm **Anonymization Engine** mới tại L5 (`component/privacy/anonymizer.go`). Engine hoạt động ở data-transformation level (thay đổi actual data), khác với display-level masking hiện tại. Pseudonymization sử dụng HMAC-SHA256 với key từ External Secret Manager (L5 `component/secret/`). FPE dùng FF1 algorithm. Policy engine tích hợp vào Export pipeline (L5 `component/export/`) và TaskRun executor (L6).

---

## 2. Architectural Alignment

```
L4 Service (SQLService / ExportService)
  │  export request or env-sync trigger
  ▼
L5 Component (privacy/anonymizer.go)
  │  ├─ Anonymization Strategy Resolver
  │  │     reads policy from L8 Store
  │  ├─ Technique Executors:
  │  │     ├─ Suppression
  │  │     ├─ Generalization
  │  │     ├─ Noise Addition
  │  │     ├─ Synthetic Data Generator
  │  │     └─ Data Swapping
  │  ├─ Pseudonymization Engine
  │  │     └─ HMAC-SHA256 via L5 Secret Manager
  │  └─ FPE Engine (FF1/FF3-1)
  │
  ▼
L5 Component (export/) — Apply on data export
L6 Runner (taskrun/) — Apply on database clone/sync
```

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L5 — Component** | `component/privacy/anonymizer.go` | Core anonymization pipeline + strategy resolution |
| **L5 — Component** | `component/privacy/pseudonym.go` | HMAC-based deterministic pseudonymization |
| **L5 — Component** | `component/privacy/fpe.go` | Format-preserving encryption (FF1) |
| **L5 — Component** | `component/privacy/synthetic.go` | Synthetic data generation |
| **L5 — Component** | `component/secret/` | Key management for pseudonymization (existing) |
| **L5 — Component** | `component/masker/` | Extend with anonymization mode (existing) |
| **L5 — Component** | `component/export/` | Apply anonymization on data export (existing) |
| **L8 — Store** | `store/anonymization_policy.go` | Policy CRUD |
| **L9 — Enterprise** | `feature.go` | `FeatureDataAnonymization` gate |

---

## 3. Chi tiết Implementation

### 3.1 L5 — Anonymization Engine

**File**: `backend/component/privacy/anonymizer.go`

```go
type AnonymizationEngine struct {
    store     *store.Store
    secret    *secret.Manager
    pseudonym *PseudonymEngine
    fpe       *FPEEngine
    synthetic *SyntheticGenerator
}

type AnonymizationTechnique int
const (
    TechniqueSuppression AnonymizationTechnique = iota
    TechniqueGeneralization
    TechniqueNoiseAddition
    TechniqueSyntheticData
    TechniqueDataSwapping
    TechniquePseudonymization
    TechniqueFormatPreserving
)

type AnonymizationRule struct {
    ColumnPattern   string                 // glob: "*.email", "users.ssn"
    Classification  string                 // or match by classification level
    Technique       AnonymizationTechnique
    Params          map[string]any         // technique-specific params
}

func (e *AnonymizationEngine) Anonymize(ctx context.Context, 
    rows []*v1pb.QueryRow, columns []ColumnMeta, policy *AnonymizationPolicy) ([]*v1pb.QueryRow, error) {
    
    for colIdx, col := range columns {
        rule := policy.ResolveRule(col) // match column → rule
        if rule == nil {
            continue
        }
        transformer := e.getTransformer(rule.Technique)
        for _, row := range rows {
            row.Values[colIdx] = transformer.Transform(ctx, row.Values[colIdx], rule.Params)
        }
    }
    return rows, nil
}
```

### 3.2 L5 — Pseudonymization Engine

**File**: `backend/component/privacy/pseudonym.go`

```go
type PseudonymEngine struct {
    secretManager *secret.Manager
    keyID         string // current active key
}

func (p *PseudonymEngine) Pseudonymize(ctx context.Context, value string) (string, error) {
    // Deterministic: same input → same token (for JOIN consistency)
    key, _ := p.secretManager.GetSecret(ctx, p.keyID)
    mac := hmac.New(sha256.New, key)
    mac.Write([]byte(value))
    token := base62.Encode(mac.Sum(nil)[:16]) // 16-byte token → ~22 chars
    return "PSE_" + token, nil
}

func (p *PseudonymEngine) ReIdentify(ctx context.Context, token string, lookupTable string) (string, error) {
    // Requires bb.privacy.reidentify permission + audit log
    // Lookup from reverse mapping table (encrypted at rest)
    return p.store.LookupPseudonym(ctx, token, lookupTable)
}
```

**Key Management** (via existing `component/secret/`):
- Key stored in External Secret Manager (Vault/AWS SM/GCP SM)
- Key rotation: new key for new pseudonymization, old key retained for decode
- Key ID versioning: `pseudonym-key-v1`, `pseudonym-key-v2`, ...

### 3.3 L5 — Format-Preserving Encryption

**File**: `backend/component/privacy/fpe.go`

```go
// Using FF1 algorithm (NIST SP 800-38G)
// Go library: github.com/capitalone/fpe (or equivalent)
type FPEEngine struct {
    secretManager *secret.Manager
}

func (f *FPEEngine) EncryptEmail(ctx context.Context, email string) (string, error) {
    parts := strings.SplitN(email, "@", 2)
    encLocal := f.ff1Encrypt(ctx, parts[0], charsetAlphaNumeric)
    return encLocal + "@" + parts[1], nil // domain preserved
}

func (f *FPEEngine) EncryptPhone(ctx context.Context, phone string) (string, error) {
    digits := extractDigits(phone)
    encDigits := f.ff1Encrypt(ctx, digits, charsetDigits)
    return reformatPhone(phone, encDigits), nil // format preserved
}
```

### 3.4 L5 — Export Integration

**File**: `backend/component/export/privacy.go` (new, extends existing export/)

```go
// Hook into existing DataExportExecutor (L6 runner/taskrun/)
func (e *ExportExecutor) applyAnonymization(ctx context.Context, 
    rows []*v1pb.QueryRow, columns []ColumnMeta, envTier EnvironmentTier) ([]*v1pb.QueryRow, error) {
    
    if envTier == EnvironmentProduction {
        return rows, nil // no anonymization for prod-to-prod
    }
    
    policy, _ := e.store.GetAnonymizationPolicy(ctx, targetEnv)
    if policy == nil {
        return nil, errors.New("anonymization policy required for non-prod export")
    }
    
    return e.anonymizer.Anonymize(ctx, rows, columns, policy)
}
```

### 3.5 L8 — Database Schema

```sql
CREATE TABLE anonymization_policy (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    workspace_uid BIGINT NOT NULL,
    target_environments TEXT[] NOT NULL, -- ['dev', 'staging']
    rules JSONB NOT NULL, -- AnonymizationRule[] as protobuf JSON
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_by BIGINT NOT NULL,
    created_ts TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_ts TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_uid, name)
);

CREATE TABLE pseudonym_lookup (
    id BIGSERIAL PRIMARY KEY,
    policy_id BIGINT NOT NULL REFERENCES anonymization_policy(id),
    key_version TEXT NOT NULL,
    token TEXT NOT NULL,
    original_hash TEXT NOT NULL, -- SHA-256 of original (for verification only)
    created_ts TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (policy_id, token)
);
```

---

## 4. Security Considerations

| Concern | Mitigation |
|---------|-----------|
| Key exposure | Stored in External Secret Manager (not in Bytebase DB) |
| Re-identification abuse | `bb.privacy.reidentify` permission + audit + approval workflow |
| Pseudonym collision | HMAC-SHA256 with 16-byte output → collision probability negligible |
| FPE weak for short strings | Minimum input length enforcement (≥ 6 chars for FF1) |
| Anonymized data re-linking | K-anonymity check for quasi-identifiers post-anonymization |

---

## 5. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-ENT-012 (Data Masking) | Anonymization extends masking pipeline |
| CR-ENT-015 (External Secret) | Key storage for pseudonymization |
| CR-PRV-001 (PII Discovery) | Discovery results drive anonymization targets |
| CR-PRV-007 (Env Isolation) | Anonymization auto-applied on cross-env sync |
| CR-PRV-006 (Export Control) | Anonymization mode in export pipeline |

---

## 6. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | Suppression + Generalization techniques | Sprint 1 |
| 2 | Pseudonymization engine + Secret Manager integration | Sprint 2 |
| 3 | FPE engine (FF1) + format-specific handlers | Sprint 3 |
| 4 | Policy engine + environment integration | Sprint 3 |
| 5 | Synthetic data generator | Sprint 4 |
| 6 | Export pipeline integration + dry-run mode | Sprint 4 |
