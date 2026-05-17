# TASK-PRV-006 — Pseudonymization Engine + Key Management

| Field            | Value                                      |
|------------------|----------------------------------------    |
| **Task ID**      | TASK-PRV-006                               |
| **Source**       | SOL-PRV-002 Phase 2 (CR-PRV-002)          |
| **Status**       | Pending                                    |
| **Priority**     | P0                                         |
| **Complexity**   | High                                       |
| **Sprint**       | Sprint 2                                   |

---

## Mô tả

Xây dựng Pseudonymization Engine dùng HMAC-SHA256, tích hợp External Secret Manager cho key management.

## Scope

1. **L5 — `component/privacy/pseudonym.go`**: PseudonymEngine — `Pseudonymize()` (deterministic HMAC-SHA256), `ReIdentify()` (reverse lookup)
2. **Key management**: Integrate `component/secret/` (existing) — key từ Vault/AWS SM/GCP SM
3. **Key rotation**: Key versioning (`pseudonym-key-v1`, `v2`...), old keys retained for decode
4. **L8 — `pseudonym_lookup`** table: Store token → original_hash mapping
5. **Re-identification control**: `bb.privacy.reidentify` permission + audit log

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/component/privacy/pseudonym.go` | NEW — Pseudonymization engine |
| `backend/store/anonymization_policy.go` | MODIFY — Add pseudonym lookup CRUD |

## Acceptance Criteria

- [ ] Deterministic: same input → same token (JOIN consistency)
- [ ] Token format: `PSE_` + base62-encoded 16-byte HMAC
- [ ] Key stored in External Secret Manager (not Bytebase DB)
- [ ] Key rotation: new key works, old data still decodable
- [ ] Re-identification requires `bb.privacy.reidentify` permission
- [ ] Re-identification events logged to audit
- [ ] Unit tests: determinism, key rotation, permission check

## Dependencies

- TASK-PRV-005 (Anonymization engine)
- TASK-ENT-018 (External Secret Manager) — soft dependency

## Definition of Done

- Determinism verified across multiple calls
- Key rotation tested
- Permission enforcement validated
