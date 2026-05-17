# Solution: CR-SEC-011 — Tamper-Proof Audit Log

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-SEC-011                |
| **Solution**   | SOL-SEC-011               |
| **Status**     | Proposed                  |
| **Complexity** | High                      |

---

## 1. Tóm tắt giải pháp

Implement hash chain trên `audit_log` table (L8) với digital signature per entry. Extend Audit Interceptor (L3) để compute hash chain. Integrity verification runner (L6). Optional external immutable storage (S3 Object Lock). Sử dụng existing DataCleaner runner (L6) cho retention — nhưng chỉ purge entries older than configurable period.

---

## 2. Architectural Alignment

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L3** | `audit.go` (25KB) | Hash chain computation, digital signature |
| **L4** | `audit_log_service.go` (7KB) | Integrity verification API |
| **L5** | `component/integrity/` (new) | Hash chain engine, signature service |
| **L6** | `runner/audit_integrity/` (new) | Scheduled verification |
| **L8** | `store/audit_log.go` | Append-only enforcement, hash columns |

---

## 3. Chi tiết Implementation

### 3.1 L5 — Hash Chain Engine

**File**: `backend/component/integrity/hashchain.go` (new)

```go
type HashChainEngine struct {
    signer    *SigningService
    store     *store.Store
    lastHash  atomic.Value  // string: last computed hash
    sequence  atomic.Int64  // monotonic sequence counter
}

func (h *HashChainEngine) ComputeEntryHash(entry *store.AuditLogMessage) string {
    previousHash := h.lastHash.Load().(string)
    seq := h.sequence.Add(1)

    payload := fmt.Sprintf("%s|%d|%d|%s|%s|%s|%d",
        previousHash,
        seq,
        entry.CreatedTS.UnixNano(),
        entry.UserUID,
        entry.Method,
        entry.Resource,
        entry.Status,
    )
    hash := sha256.Sum256([]byte(payload))
    hashStr := hex.EncodeToString(hash[:])

    h.lastHash.Store(hashStr)
    return hashStr
}

func (h *HashChainEngine) SignEntry(hash string) (string, error) {
    return h.signer.Sign([]byte(hash)) // ECDSA P-256
}
```

### 3.2 L5 — Digital Signature Service

```go
type SigningService struct {
    privateKey *ecdsa.PrivateKey
    publicKey  *ecdsa.PublicKey
    keyID      string
}

func (s *SigningService) Sign(data []byte) (string, error) {
    hash := sha256.Sum256(data)
    r, ss, err := ecdsa.Sign(rand.Reader, s.privateKey, hash[:])
    if err != nil { return "", err }
    sig := append(r.Bytes(), ss.Bytes()...)
    return base64.StdEncoding.EncodeToString(sig), nil
}

func (s *SigningService) Verify(data []byte, sigStr string) bool {
    sig, _ := base64.StdEncoding.DecodeString(sigStr)
    hash := sha256.Sum256(data)
    r := new(big.Int).SetBytes(sig[:len(sig)/2])
    ss := new(big.Int).SetBytes(sig[len(sig)/2:])
    return ecdsa.Verify(s.publicKey, hash[:], r, ss)
}
```

### 3.3 L3 — Audit Interceptor Extension

**File**: `backend/api/v1/audit.go` (extend existing 25KB)

```go
func (a *AuditInterceptor) createAuditLog(ctx context.Context, entry *store.AuditLogMessage) error {
    // Compute hash chain
    entry.ChainHash = a.hashChain.ComputeEntryHash(entry)
    entry.Sequence = a.hashChain.GetSequence()

    // Digital signature
    sig, _ := a.hashChain.SignEntry(entry.ChainHash)
    entry.Signature = sig
    entry.SigningKeyID = a.hashChain.GetKeyID()

    // Store (append-only)
    return a.store.CreateAuditLog(ctx, entry)
}
```

### 3.4 L8 — Database Enforcement

```sql
-- Extend audit_log table
ALTER TABLE audit_log ADD COLUMN chain_hash TEXT NOT NULL;
ALTER TABLE audit_log ADD COLUMN sequence BIGINT NOT NULL;
ALTER TABLE audit_log ADD COLUMN signature TEXT NOT NULL;
ALTER TABLE audit_log ADD COLUMN signing_key_id TEXT NOT NULL;
ALTER TABLE audit_log ADD COLUMN previous_hash TEXT;

-- Append-only trigger: prevent UPDATE and DELETE
CREATE OR REPLACE FUNCTION prevent_audit_modification()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'UPDATE' OR TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'audit_log is immutable: % operation not allowed', TG_OP;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER audit_log_immutable
    BEFORE UPDATE OR DELETE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION prevent_audit_modification();

-- Exception: system retention purge bypasses trigger via session variable
-- SET LOCAL bytebase.audit_purge = 'true' in DataCleaner
```

### 3.5 L6 — Integrity Verification Runner

```go
type AuditIntegrityRunner struct {
    store      *store.Store
    hashChain  *integrity.HashChainEngine
    webhook    *webhook.Manager
    interval   time.Duration // Daily
}

func (r *AuditIntegrityRunner) Run(ctx context.Context) {
    ticker := time.NewTicker(r.interval)
    for {
        select {
        case <-ticker.C:
            result := r.verifyChain(ctx)
            if !result.Valid {
                r.webhook.NotifyIntegrityViolation(ctx, result)
            }
        case <-ctx.Done():
            return
        }
    }
}

func (r *AuditIntegrityRunner) verifyChain(ctx context.Context) *VerificationResult {
    entries, _ := r.store.ListAuditLogsOrdered(ctx) // ORDER BY sequence ASC
    previousHash := ""
    for _, entry := range entries {
        // Recompute hash
        expectedHash := computeHash(previousHash, entry)
        if entry.ChainHash != expectedHash {
            return &VerificationResult{Valid: false, BrokenAt: entry.Sequence}
        }
        // Verify signature
        if !r.hashChain.VerifySignature(entry.ChainHash, entry.Signature) {
            return &VerificationResult{Valid: false, BrokenAt: entry.Sequence}
        }
        previousHash = entry.ChainHash
    }
    return &VerificationResult{Valid: true, EntriesVerified: len(entries)}
}
```

---

## 4. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-ENT-003 (Audit Log) | Extends existing audit log with tamper protection |
| CR-SEC-010 (SIEM) | Integrity violations forwarded to SIEM |
| CR-ENT-015 (Secret Manager) | Signing keys stored in secret manager |

---

## 5. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | Hash chain + audit_log schema extension | Sprint 1 |
| 2 | Digital signature + append-only trigger | Sprint 2 |
| 3 | Integrity verification runner | Sprint 3 |
| 4 | External immutable storage (optional S3 Object Lock) | Sprint 4 |
