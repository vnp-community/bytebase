# SOL-SHR-002 — Envelope Encryption Engine (BEE)

| Metadata | Value |
|---|---|
| Solution ID | SOL-SHR-002 |
| CRs | CR-SHR-102 (Envelope & Transit Encryption) |
| Arch Layers | L5 (Component), L10 (Infrastructure) |
| Priority | P0 — Critical |
| Sprints | 3–4 |
| Dependencies | SOL-SHR-001 (encryption key source) |

---

## 1. Phân tích kiến trúc hiện tại

### 1.1 Obfuscation hiện tại

Bytebase dùng `common.Obfuscate` — XOR với `auth_secret`:

```go
// common/obfuscation.go
func Obfuscate(plaintext string, seed string) string {
    // XOR → base64 — KHÔNG phải encryption
}
```

**Vấn đề**: Nếu DB bị compromise, `auth_secret` lộ → tất cả secrets đều reversible.

### 1.2 External Secret hiện tại

`component/secret/secret.go` — switch/case 4 providers, mỗi provider encrypt riêng. **Không có unified envelope format**.

### 1.3 Go Crypto Standard Library

Go cung cấp `crypto/aes`, `crypto/cipher` (GCM mode), `crypto/rand`, `golang.org/x/crypto/hkdf` — đầy đủ cho AES-256-GCM + HKDF.

---

## 2. Giải pháp chi tiết

### 2.1 Module Structure

```
backend/component/sharing/envelope/
├── envelope.go          ← EnvelopeEncryptor interface + implementation
├── format.go            ← BEE JSON serialization/deserialization
├── keymanager.go        ← Key hierarchy management
├── local_kek.go         ← HKDF-based local KEK provider
├── vault_transit.go     ← HashiCorp Vault Transit KEK
├── aws_kms.go           ← AWS KMS KEK (future)
└── gcp_kms.go           ← GCP Cloud KMS KEK (future)
```

### 2.2 BEE Format — Self-describing Encrypted Envelope

```go
// backend/component/sharing/envelope/format.go
package envelope

import (
    "encoding/json"
    "time"
)

// EncryptedEnvelope is the Bytebase Encrypted Envelope (BEE/1.0).
// Self-describing, portable, JSON-serializable.
type EncryptedEnvelope struct {
    Version      string            `json:"version"`       // "BEE/1.0"
    KeyID        string            `json:"key_id"`        // KEK identifier
    Algorithm    string            `json:"algorithm"`     // "AES-256-GCM"
    EncryptedDEK []byte            `json:"encrypted_dek"` // DEK wrapped by KEK
    IV           []byte            `json:"iv"`            // 12-byte nonce
    Ciphertext   []byte            `json:"ciphertext"`    // Encrypted payload
    AuthTag      []byte            `json:"auth_tag"`      // 16-byte GCM auth tag
    Metadata     EnvelopeMetadata  `json:"metadata"`
}

type EnvelopeMetadata struct {
    Source      string    `json:"source"`       // "bytebase"
    WorkspaceID string    `json:"workspace_id"`
    CreatedAt   time.Time `json:"created_at"`
    ExpiresAt   time.Time `json:"expires_at,omitempty"`
    ContentType string    `json:"content_type"` // "database_credential", etc.
}

func (e *EncryptedEnvelope) Marshal() ([]byte, error) {
    return json.Marshal(e)
}

func UnmarshalEnvelope(data []byte) (*EncryptedEnvelope, error) {
    var env EncryptedEnvelope
    if err := json.Unmarshal(data, &env); err != nil {
        return nil, err
    }
    if env.Version != "BEE/1.0" {
        return nil, fmt.Errorf("envelope: unsupported version %q", env.Version)
    }
    return &env, nil
}
```

### 2.3 Key Hierarchy — 3-Tier Model

```
┌─────────────────────────────────────────────────────────────┐
│  Tier 1: Master KEK                                          │
│  ┌────────────┐  ┌──────────────┐  ┌───────────────────┐   │
│  │ Local HKDF │  │ Vault Transit│  │ AWS KMS / GCP KMS │   │
│  │ (default)  │  │ (enterprise) │  │ (cloud-managed)   │   │
│  └──────┬─────┘  └──────┬───────┘  └─────────┬─────────┘   │
│         │               │                    │              │
│         └───────────────┼────────────────────┘              │
│                         │                                    │
│                    wrap(DEK)                                 │
│                         │                                    │
│  ┌──────────────────────▼──────────────────────────────┐    │
│  │  Tier 2: DEK (per-secret, random 256-bit)            │    │
│  │  → AES-256-GCM encrypt(plaintext)                    │    │
│  └──────────────────────┬──────────────────────────────┘    │
│                         │                                    │
│  ┌──────────────────────▼──────────────────────────────┐    │
│  │  Tier 3: Ciphertext + IV + AuthTag                    │    │
│  │  → Stored in BEE envelope                             │    │
│  └──────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

### 2.4 KEK Provider Interface

```go
// backend/component/sharing/envelope/keymanager.go
package envelope

import "context"

// KEKProvider wraps/unwraps DEKs using a Key Encryption Key.
type KEKProvider interface {
    // Name returns the provider name (e.g., "local", "vault-transit").
    Name() string
    // WrapDEK encrypts a DEK using the current KEK.
    WrapDEK(ctx context.Context, dek []byte) (wrappedDEK []byte, keyID string, err error)
    // UnwrapDEK decrypts a wrapped DEK using the KEK identified by keyID.
    UnwrapDEK(ctx context.Context, wrappedDEK []byte, keyID string) (dek []byte, err error)
    // CurrentKeyID returns the active KEK identifier.
    CurrentKeyID(ctx context.Context) (string, error)
    // Healthy checks KEK availability.
    Healthy(ctx context.Context) error
}

// KeyManager manages KEK lifecycle and DEK wrapping.
type KeyManager struct {
    provider KEKProvider
    store    *store.Store
}

// RotateKEK creates a new KEK and schedules re-wrapping of all existing DEKs.
func (km *KeyManager) RotateKEK(ctx context.Context) (newKeyID string, err error) {
    // Provider-specific: Vault Transit creates new key version,
    // Local HKDF generates new salt.
    // Old KEK remains valid during grace period (30 days).
    // Background runner re-wraps all DEKs with new KEK.
}
```

### 2.5 Local KEK Provider (Default)

```go
// backend/component/sharing/envelope/local_kek.go
package envelope

import (
    "crypto/sha256"
    "golang.org/x/crypto/hkdf"
    "io"
)

type LocalKEKProvider struct {
    masterSecret []byte // Derived from Bytebase auth_secret
    salt         []byte // Workspace-specific salt
}

// NewLocalKEKProvider creates KEK derived from auth_secret via HKDF-SHA256.
// Uses auth_secret from store.Store.Secret field.
func NewLocalKEKProvider(authSecret string, workspaceID string) *LocalKEKProvider {
    salt := sha256.Sum256([]byte("bytebase-kek-" + workspaceID))
    return &LocalKEKProvider{
        masterSecret: []byte(authSecret),
        salt:         salt[:],
    }
}

func (p *LocalKEKProvider) Name() string { return "local" }

func (p *LocalKEKProvider) WrapDEK(ctx context.Context, dek []byte) ([]byte, string, error) {
    // Derive KEK from master secret via HKDF
    kek := p.deriveKEK()
    
    // AES-256-GCM wrap the DEK
    block, _ := aes.NewCipher(kek)
    gcm, _ := cipher.NewGCM(block)
    nonce := make([]byte, gcm.NonceSize())
    crypto_rand.Read(nonce)
    
    wrapped := gcm.Seal(nonce, nonce, dek, nil)
    return wrapped, "local-v1", nil
}

func (p *LocalKEKProvider) UnwrapDEK(ctx context.Context, wrapped []byte, keyID string) ([]byte, error) {
    kek := p.deriveKEK()
    
    block, _ := aes.NewCipher(kek)
    gcm, _ := cipher.NewGCM(block)
    nonceSize := gcm.NonceSize()
    
    nonce, ciphertext := wrapped[:nonceSize], wrapped[nonceSize:]
    return gcm.Open(nil, nonce, ciphertext, nil)
}

func (p *LocalKEKProvider) deriveKEK() []byte {
    reader := hkdf.New(sha256.New, p.masterSecret, p.salt, []byte("envelope-kek"))
    kek := make([]byte, 32) // 256-bit
    io.ReadFull(reader, kek)
    return kek
}
```

### 2.6 EnvelopeEncryptor — Seal/Open/Rekey

```go
// backend/component/sharing/envelope/envelope.go
package envelope

import (
    "context"
    "crypto/aes"
    "crypto/cipher"
    crypto_rand "crypto/rand"
    "crypto/subtle"
    "fmt"
)

type Encryptor struct {
    keyManager *KeyManager
}

func NewEncryptor(keyManager *KeyManager) *Encryptor {
    return &Encryptor{keyManager: keyManager}
}

// Seal encrypts plaintext using a fresh DEK, wraps DEK with KEK.
func (e *Encryptor) Seal(ctx context.Context, plaintext []byte, meta EnvelopeMetadata) (*EncryptedEnvelope, error) {
    // 1. Generate random DEK (256-bit)
    dek := make([]byte, 32)
    if _, err := crypto_rand.Read(dek); err != nil {
        return nil, fmt.Errorf("envelope: failed to generate DEK: %w", err)
    }
    defer zeroize(dek) // Clear from memory after use
    
    // 2. Encrypt plaintext with DEK (AES-256-GCM)
    block, err := aes.NewCipher(dek)
    if err != nil {
        return nil, err
    }
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }
    
    iv := make([]byte, gcm.NonceSize()) // 12 bytes
    if _, err := crypto_rand.Read(iv); err != nil {
        return nil, err
    }
    
    ciphertext := gcm.Seal(nil, iv, plaintext, nil)
    
    // Separate auth tag (last 16 bytes of GCM output)
    tagSize := gcm.Overhead()
    authTag := ciphertext[len(ciphertext)-tagSize:]
    ciphertext = ciphertext[:len(ciphertext)-tagSize]
    
    // 3. Wrap DEK with KEK
    wrappedDEK, keyID, err := e.keyManager.provider.WrapDEK(ctx, dek)
    if err != nil {
        return nil, fmt.Errorf("envelope: failed to wrap DEK: %w", err)
    }
    
    return &EncryptedEnvelope{
        Version:      "BEE/1.0",
        KeyID:        keyID,
        Algorithm:    "AES-256-GCM",
        EncryptedDEK: wrappedDEK,
        IV:           iv,
        Ciphertext:   ciphertext,
        AuthTag:      authTag,
        Metadata:     meta,
    }, nil
}

// Open decrypts an envelope: unwrap DEK → decrypt ciphertext.
func (e *Encryptor) Open(ctx context.Context, env *EncryptedEnvelope) ([]byte, error) {
    // 1. Unwrap DEK
    dek, err := e.keyManager.provider.UnwrapDEK(ctx, env.EncryptedDEK, env.KeyID)
    if err != nil {
        return nil, fmt.Errorf("envelope: failed to unwrap DEK: %w", err)
    }
    defer zeroize(dek)
    
    // 2. Decrypt with DEK
    block, err := aes.NewCipher(dek)
    if err != nil {
        return nil, err
    }
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }
    
    // Reconstruct ciphertext with auth tag
    fullCiphertext := append(env.Ciphertext, env.AuthTag...)
    
    return gcm.Open(nil, env.IV, fullCiphertext, nil)
}

// Rekey re-wraps the DEK with a new KEK (no re-encryption of data).
func (e *Encryptor) Rekey(ctx context.Context, env *EncryptedEnvelope, newKeyID string) (*EncryptedEnvelope, error) {
    // 1. Unwrap with old KEK
    dek, err := e.keyManager.provider.UnwrapDEK(ctx, env.EncryptedDEK, env.KeyID)
    if err != nil {
        return nil, err
    }
    defer zeroize(dek)
    
    // 2. Re-wrap with new KEK
    newWrappedDEK, keyID, err := e.keyManager.provider.WrapDEK(ctx, dek)
    if err != nil {
        return nil, err
    }
    
    // 3. Return updated envelope (ciphertext unchanged)
    return &EncryptedEnvelope{
        Version:      env.Version,
        KeyID:        keyID,
        Algorithm:    env.Algorithm,
        EncryptedDEK: newWrappedDEK,
        IV:           env.IV,
        Ciphertext:   env.Ciphertext,
        AuthTag:      env.AuthTag,
        Metadata:     env.Metadata,
    }, nil
}

// zeroize clears a byte slice from memory (defense-in-depth).
func zeroize(b []byte) {
    for i := range b {
        b[i] = 0
    }
}
```

### 2.7 Key Rotation Runner (L6)

```go
// backend/runner/keyrotation/rotation.go
package keyrotation

// KeyRotationRunner re-wraps DEKs when KEK rotates.
// Pattern: same as DataCleaner runner (periodic background goroutine).
type KeyRotationRunner struct {
    store      *store.Store
    encryptor  *envelope.Encryptor
    interval   time.Duration // Check every 1 hour
}

func (r *KeyRotationRunner) Run(ctx context.Context) {
    ticker := time.NewTicker(r.interval)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            r.processRotation(ctx)
        }
    }
}

func (r *KeyRotationRunner) processRotation(ctx context.Context) {
    // 1. Find rotation jobs in progress
    logs, _ := r.store.ListKeyRotationLogs(ctx, &store.FindKeyRotationLogMessage{
        Status: "in_progress",
    })
    for _, log := range logs {
        // 2. Find envelopes using old key ID
        envelopes, _ := r.store.ListEnvelopesByKeyID(ctx, log.OldKeyID)
        for _, env := range envelopes {
            // 3. Rekey each envelope
            newEnv, err := r.encryptor.Rekey(ctx, env, log.NewKeyID)
            if err != nil {
                log.FailedItems++
                continue
            }
            // 4. Update in store
            r.store.UpdateEnvelope(ctx, newEnv)
            log.RewrappedItems++
        }
        // 5. Mark rotation complete
        if log.RewrappedItems+log.FailedItems >= log.TotalItems {
            log.Status = "completed"
            log.CompletedAt = time.Now()
            r.store.UpdateKeyRotationLog(ctx, log)
        }
    }
}
```

### 2.8 Database Migration

```sql
-- Tables for key management
CREATE TABLE encryption_key (
    id TEXT PRIMARY KEY,                    -- e.g., "local-v1", "vault-transit-v3"
    workspace_id TEXT NOT NULL,
    provider TEXT NOT NULL,                 -- "local", "vault-transit", "aws-kms"
    provider_key_ref TEXT,                  -- External key ID (Vault key name, KMS ARN)
    algorithm TEXT NOT NULL DEFAULT 'AES-256-GCM',
    status TEXT NOT NULL DEFAULT 'active',  -- active, rotating, retired
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    rotated_at TIMESTAMPTZ,
    retire_after TIMESTAMPTZ,              -- Grace period end
    usage_count BIGINT DEFAULT 0,
    creator_id INT REFERENCES principal(id)
);

CREATE INDEX idx_encryption_key_workspace ON encryption_key(workspace_id, status);

CREATE TABLE key_rotation_log (
    id SERIAL PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    old_key_id TEXT REFERENCES encryption_key(id),
    new_key_id TEXT REFERENCES encryption_key(id),
    total_items INT,
    rewrapped_items INT DEFAULT 0,
    failed_items INT DEFAULT 0,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    status TEXT DEFAULT 'in_progress',     -- in_progress, completed, failed
    error_message TEXT
);
```

---

## 3. Integration với SOL-SHR-001

SOL-SHR-001 `SharingManager.CreateShare()` sẽ sử dụng `envelope.Encryptor` thay cho `PayloadEncryptor`:

```go
// Updated flow:
// 1. SharingManager.CreateShare()
// 2.   → envelope.Seal(plaintext, metadata)  ← BEE envelope
// 3.   → provider.CreateShare(BEE envelope)  ← Send to Vaultwarden
// 4.   → store.CreateSharedCredential()       ← Persist metadata
```

---

## 4. Test Strategy

| Test | Description | Method |
|---|---|---|
| Seal/Open roundtrip | Encrypt → decrypt = original | Unit test |
| Rekey correctness | Old envelope → rekey → Open succeeds | Unit test |
| Tamper detection | Modify ciphertext → Open fails | Unit test |
| IV uniqueness | 10K Seal calls → no IV collision | Statistical test |
| Memory zeroing | DEK cleared after use | Inspection test |
| Local KEK determinism | Same auth_secret + workspace → same KEK | Unit test |
| Concurrent safety | 100 goroutines Seal/Open | Race detector |
