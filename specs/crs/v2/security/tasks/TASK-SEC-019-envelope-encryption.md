# TASK-SEC-019 — Envelope Encryption Component + Key Manager

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-019                               |
| **Source**       | SOL-SEC-007 §3.1, §3.2                    |
| **Status**       | Pending                                    |
| **Priority**     | P0                                         |
| **Complexity**   | High                                       |
| **Sprint**       | Sprint 1                                   |

---

## Mô tả

Implement envelope encryption engine (L5): AES-256-GCM DEK per category, KEK managed bởi existing Secret component.

## Scope

1. **EnvelopeEncryptor**: `component/encryption/envelope.go` — DEK management, `Encrypt(category, plaintext) → EncryptedData`, `Decrypt(EncryptedData) → plaintext`
2. **AES-256-GCM**: 32-byte key, random nonce, authenticated encryption
3. **KeyManager**: `component/encryption/key_manager.go` — WrapDEK/UnwrapDEK via existing `component/secret/` (Vault/AWS/GCP/fallback env var)
4. **DEK registry**: Migration `encryption_key` table (id, category, version, encrypted_key BYTEA, is_active, created_ts)
5. **DEK categories**: "db_credentials", "api_keys", "sso_secrets", "webhook_secrets"

## Acceptance Criteria

- [ ] Encrypt → Decrypt roundtrip preserves plaintext
- [ ] DEK wrapped with KEK via Secret Manager
- [ ] Fallback KEK from env var works
- [ ] Multiple DEK categories supported
- [ ] Unit tests + encryption correctness tests

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/component/encryption/envelope.go` | New file |
| `backend/component/encryption/key_manager.go` | New file |
| `backend/migrator/migration/` | encryption_key table |

## Definition of Done

- Encryption verified with test vectors
