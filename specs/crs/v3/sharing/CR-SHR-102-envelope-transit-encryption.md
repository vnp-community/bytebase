# Change Request: Sensitive Data Envelope & Transit Encryption

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-SHR-102                                               |
| **Gap ID**         | G-SHR-2                                                  |
| **Title**          | Sensitive Data Envelope & Transit Encryption             |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-17                                               |
| **Author**         | VNP AI Ops Team                                          |
| **Extends**        | CR-SHR-001 (Encryption module), CR-SHR-006 (Vault Transit) |

---

## 1. Tổng quan

### 1.1 Mô tả
Xây dựng **Envelope Encryption Layer** cho mọi sensitive data trước khi rời Bytebase — đảm bảo defense-in-depth. CR-SHR-001 đã định nghĩa AES-256-GCM encryption cơ bản. CR này mở rộng thành **full key hierarchy**:

- **3-tier key model**: Master KEK → DEK (per-secret) → Ciphertext
- **Multiple KEK providers**: Local, Vault Transit, AWS KMS, GCP KMS
- **Key rotation** without re-encrypting data
- **Bytebase Encrypted Envelope (BEE)** format — self-describing, portable

### 1.2 Bối cảnh
Khi chia sẻ secrets qua nền tảng trung gian:
```
Bytebase → [Vaultwarden/Vault/CI/CD] → Consumer
```
Data đi qua ít nhất 2 hops. CR-SHR-001 encrypt tại application level, nhưng:
- Dùng single encryption key cho tất cả secrets
- Không hỗ trợ key rotation
- Không hỗ trợ external KMS

---

## 2. Yêu cầu chức năng

### FR-001: Bytebase Encrypted Envelope (BEE) Format
```json
{
  "version": "BEE/1.0",
  "key_id": "kek-2026-05-17-001",
  "algorithm": "AES-256-GCM",
  "encrypted_dek": "<base64-encoded encrypted DEK>",
  "iv": "<base64-encoded 12-byte nonce>",
  "ciphertext": "<base64-encoded encrypted data>",
  "auth_tag": "<base64-encoded 16-byte GCM auth tag>",
  "metadata": {
    "source": "bytebase",
    "workspace_id": "ws-xxx",
    "created_at": "2026-05-17T12:00:00Z",
    "expires_at": "2026-05-18T12:00:00Z",
    "content_type": "database_credential"
  }
}
```

### FR-002: Key Hierarchy Management

| Level | Key | Purpose | Rotation |
|---|---|---|---|
| Tier 1 | Master KEK | Encrypt/decrypt DEKs | Annual or on-demand |
| Tier 2 | DEK (per-secret) | Encrypt actual secret data | Per-secret, unique |
| Tier 3 | Derived keys | Per-share session keys | Per-session |

KEK Storage Options:
- **Local** — Derived from Bytebase auth secret via HKDF-SHA256
- **HashiCorp Vault Transit** — KEK never leaves Vault
- **AWS KMS / GCP Cloud KMS** — Cloud-managed KEK

### FR-003: Encrypt/Decrypt Engine
```go
type EnvelopeEncryptor interface {
    Seal(ctx context.Context, plaintext []byte, metadata EnvelopeMetadata) (*EncryptedEnvelope, error)
    Open(ctx context.Context, envelope *EncryptedEnvelope) ([]byte, error)
    Rekey(ctx context.Context, envelope *EncryptedEnvelope, newKeyID string) (*EncryptedEnvelope, error)
    CurrentKeyID(ctx context.Context) (string, error)
}
```

### FR-004: Key Rotation Support
- KEK rotation: Tạo KEK mới → DEKs re-wrapped (not re-encrypt data) — fast operation
- Grace period: Old KEK valid 30 ngày sau rotation
- Zero-downtime: Background rotation, không block operations

---

## 3. Yêu cầu kỹ thuật

| Component | File/Package | Thay đổi |
|---|---|---|
| Envelope Core | `backend/component/sharing/envelope/envelope.go` | Encrypt/decrypt |
| Key Manager | `backend/component/sharing/envelope/keymanager.go` | KEK lifecycle |
| Local KEK Provider | `backend/component/sharing/envelope/local_kek.go` | HKDF-based derivation |
| Vault Transit Provider | `backend/component/sharing/envelope/vault_transit.go` | Vault Transit backend |
| AWS KMS Provider | `backend/component/sharing/envelope/aws_kms.go` | AWS KMS integration |
| BEE Format | `backend/component/sharing/envelope/format.go` | Serialization |
| Key Rotation Runner | `backend/runner/keyrotation/rotation.go` | Background rotation |
| Database Migration | `backend/migrator/migration/*/` | `encryption_key`, `key_rotation_log` |

### 3.1 Database Schema

```sql
CREATE TABLE encryption_key (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    provider TEXT NOT NULL,
    provider_key_ref TEXT,
    algorithm TEXT NOT NULL DEFAULT 'AES-256-GCM',
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    rotated_at TIMESTAMPTZ,
    retire_after TIMESTAMPTZ,
    usage_count BIGINT DEFAULT 0,
    creator_id INT REFERENCES principal(id)
);

CREATE TABLE key_rotation_log (
    id SERIAL PRIMARY KEY,
    old_key_id TEXT REFERENCES encryption_key(id),
    new_key_id TEXT REFERENCES encryption_key(id),
    total_items INT,
    rewrapped_items INT,
    failed_items INT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    status TEXT DEFAULT 'in_progress',
    error_message TEXT
);
```

### 3.2 Encryption Flow

```
Seal: DEK(random) → encrypt(data) → wrap(DEK, KEK) → BEE envelope
Open: unwrap(DEK, KEK) → decrypt(data, DEK) → plaintext
Rekey: unwrap(DEK, old_KEK) → wrap(DEK, new_KEK) → updated envelope
```

---

## 4. Security Considerations

| Concern | Mitigation |
|---|---|
| DEK reuse | Unique DEK per secret, never reused |
| IV/nonce collision | Random 12-byte nonce; collision probability negligible |
| KEK compromise | Re-key operation replaces KEK without touching ciphertext |
| Side-channel | `crypto/subtle.ConstantTimeCompare` for auth tag |
| Memory exposure | Zero DEK from memory after use |
| Local KEK derivation | HKDF-SHA256 with workspace-specific salt |

---

## 5. Test Cases

| Test ID | Mô tả | Expected Result |
|---|---|---|
| TC-001 | Seal + Open roundtrip (Local KEK) | Plaintext matches |
| TC-002 | Seal with Vault Transit KEK | Decrypt succeeds |
| TC-003 | Key rotation (old → new KEK) | All envelopes re-wrapped |
| TC-004 | Tampered ciphertext → Open fails | GCM auth tag fails |
| TC-005 | Expired KEK → Open fails | Decryption refused |
| TC-006 | Concurrent Seal operations | Thread-safe, no IV collision |

---

## 6. Rollout Plan

| Phase | Mô tả | Timeline |
|---|---|---|
| Phase 1 | Envelope core + Local KEK + BEE format | Sprint 3-4 |
| Phase 2 | Vault Transit + AWS KMS | Sprint 5 |
| Phase 3 | Key rotation runner + management UI | Sprint 6 |
| Phase 4 | Integration with CR-SHR-001/002/003 | Sprint 7 |
