# Solution: CR-SEC-007 — Encryption at Rest (Application-Level)

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-SEC-007                |
| **Solution**   | SOL-SEC-007               |
| **Status**     | Proposed                  |
| **Complexity** | High                      |

---

## 1. Tóm tắt giải pháp

Triển khai envelope encryption component (L5) tích hợp vào Store layer (L8) via transparent encrypt/decrypt hooks. DEK per data category (AES-256-GCM), KEK managed bởi existing Secret component (`component/secret/`). Key rotation via new Runner (L6). Migration tool cho existing plaintext data.

---

## 2. Architectural Alignment

```
L5 Component ──► component/encryption/ (new)
                      │
                      ├── EnvelopeEncryptor: DEK generate + AES-256-GCM
                      ├── KeyManager: KEK from component/secret/ (Vault/AWS/GCP)
                      └── KeyRotator: Background re-encryption
                      │
L8 Store ──► Transparent hooks in store methods
              │
              ├── store/instance.go: Encrypt DataSource.Password
              ├── store/setting.go: Encrypt sensitive settings
              └── store/policy.go: Encrypt secret fields in JSONB
                      │
L6 Runner ──► runner/keyrotation/ (new): Background DEK rotation
```

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L5** | `component/encryption/` (new) | Envelope encryption engine |
| **L5** | `component/secret/` (existing) | KEK provider (Vault/AWS/GCP) |
| **L8** | `store/` (multiple files) | Transparent encrypt/decrypt hooks |
| **L6** | `runner/keyrotation/` (new) | DEK rotation worker |

---

## 3. Chi tiết Implementation

### 3.1 L5 — Encryption Component

**File**: `backend/component/encryption/envelope.go` (new)

```go
type EnvelopeEncryptor struct {
    keyManager  *KeyManager
    activeDEKs  map[string]*DEK  // category → active DEK
    mu          sync.RWMutex
}

type DEK struct {
    ID           string    // Unique key identifier
    Key          []byte    // 32 bytes for AES-256
    EncryptedKey []byte    // DEK encrypted with KEK
    Category     string    // "db_credentials", "api_keys", "sso_secrets"
    Version      int
    CreatedAt    time.Time
}

func (e *EnvelopeEncryptor) Encrypt(category string, plaintext []byte) (*EncryptedData, error) {
    dek := e.getActiveDEK(category)

    // AES-256-GCM encryption
    block, _ := aes.NewCipher(dek.Key)
    gcm, _ := cipher.NewGCM(block)
    nonce := make([]byte, gcm.NonceSize())
    io.ReadFull(rand.Reader, nonce)

    ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

    return &EncryptedData{
        Ciphertext: ciphertext,
        KeyID:      dek.ID,
        KeyVersion: dek.Version,
    }, nil
}

func (e *EnvelopeEncryptor) Decrypt(data *EncryptedData) ([]byte, error) {
    dek := e.getDEKByID(data.KeyID, data.KeyVersion)
    // ... AES-256-GCM decryption ...
}
```

### 3.2 L5 — Key Manager (using existing Secret component)

**File**: `backend/component/encryption/key_manager.go` (new)

```go
type KeyManager struct {
    secretManager *secret.Manager // Existing component/secret/
    fallbackKEK   []byte          // From env var for non-Vault deployments
}

func (km *KeyManager) WrapDEK(dek []byte) ([]byte, error) {
    // Use existing Secret Manager for KEK operations
    // Vault: transit/encrypt endpoint
    // AWS KMS: Encrypt API
    // GCP KMS: Encrypt API
    // Fallback: AES-256-GCM with env-var KEK
    return km.secretManager.EncryptData(context.Background(), dek)
}

func (km *KeyManager) UnwrapDEK(encryptedDEK []byte) ([]byte, error) {
    return km.secretManager.DecryptData(context.Background(), encryptedDEK)
}
```

### 3.3 L8 — Store Layer Integration

**File**: `backend/store/instance.go` (extend existing)

Transparent encryption for DataSource passwords:

```go
func (s *Store) CreateDataSource(ctx context.Context, ds *DataSourceMessage) error {
    // Encrypt sensitive fields before storage
    if ds.Password != "" {
        encrypted, err := s.encryptor.Encrypt("db_credentials", []byte(ds.Password))
        if err != nil { return err }
        ds.EncryptedPassword = encoded(encrypted) // base64-encoded EncryptedData
        ds.Password = ""                           // Clear plaintext
    }
    // ... existing SQL INSERT ...
}

func (s *Store) GetDataSource(ctx context.Context, id int) (*DataSourceMessage, error) {
    ds, err := s.queryDataSource(ctx, id)
    if err != nil { return nil, err }

    // Decrypt on read
    if ds.EncryptedPassword != "" {
        decrypted, err := s.encryptor.Decrypt(decode(ds.EncryptedPassword))
        if err != nil { return nil, err }
        ds.Password = string(decrypted)
    }
    return ds, nil
}
```

### 3.4 Database Schema Changes

```sql
-- Add encrypted columns alongside existing ones
ALTER TABLE data_source ADD COLUMN encrypted_password TEXT;
ALTER TABLE setting ADD COLUMN encrypted_value TEXT;

-- DEK registry
CREATE TABLE encryption_key (
    id          TEXT PRIMARY KEY,
    category    TEXT NOT NULL,
    version     INT NOT NULL,
    encrypted_key BYTEA NOT NULL,  -- DEK encrypted with KEK
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_ts  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_encryption_key_category ON encryption_key (category, is_active);
```

### 3.5 L6 — Key Rotation Runner

**File**: `backend/runner/keyrotation/rotation_runner.go` (new)

```go
func (r *KeyRotationRunner) Run(ctx context.Context) {
    ticker := time.NewTicker(1 * time.Hour)
    for {
        select {
        case <-ticker.C:
            r.checkKeyAge(ctx)          // Alert if DEK older than policy
            r.executeScheduledRotation(ctx) // Re-encrypt with new DEK
        case <-ctx.Done():
            return
        }
    }
}

func (r *KeyRotationRunner) executeScheduledRotation(ctx context.Context) {
    // 1. Generate new DEK
    // 2. Wrap with KEK
    // 3. Mark new DEK as active
    // 4. Background re-encrypt existing data with new DEK (batched)
    // 5. Mark old DEK as inactive after all data migrated
}
```

---

## 4. Sensitive Data Map

| Store File | Field | Category |
|-----------|-------|----------|
| `store/instance.go` | `DataSource.Password` | db_credentials |
| `store/instance.go` | `DataSource.SSLKey` | db_credentials |
| `store/setting.go` | SSO client secrets | sso_secrets |
| `store/setting.go` | Webhook signing keys | webhook_secrets |
| `store/api_key.go` | API key hashes | api_keys |

---

## 5. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-ENT-015 (Secret Manager) | KEK storage via Vault/AWS/GCP |
| CR-SEC-008 (Credential Rotation) | Uses encryption layer for rotated credentials |

---

## 6. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | Encryption component + Key Manager | Sprint 1 |
| 2 | Store layer hooks (instance passwords) | Sprint 2 |
| 3 | Migration tool (plaintext → encrypted) | Sprint 3 |
| 4 | Key rotation runner | Sprint 4 |
