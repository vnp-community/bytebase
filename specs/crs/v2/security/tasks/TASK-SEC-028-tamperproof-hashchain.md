# TASK-SEC-028 — Tamper-Proof Audit Log Hash Chain

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-028                               |
| **Source**       | SOL-SEC-011 §3.1-§3.4                     |
| **Status**       | Pending                                    |
| **Priority**     | P0                                         |
| **Complexity**   | High                                       |
| **Sprint**       | Sprint 2                                   |

---

## Mô tả

Implement hash chain + digital signature trên audit_log. Extend Audit Interceptor. Immutability trigger trên database.

## Scope

1. **HashChainEngine**: `component/integrity/hashchain.go` — `ComputeEntryHash()` (SHA-256 of previousHash + sequence + entry fields), atomic lastHash/sequence
2. **SigningService**: `component/integrity/signing.go` — ECDSA P-256, `Sign(hash)`, `Verify(hash, sig)`
3. **Audit Interceptor**: `audit.go` — compute hash chain + sign before `CreateAuditLog()`
4. **Schema**: ALTER `audit_log` ADD `chain_hash TEXT`, `sequence BIGINT`, `signature TEXT`, `signing_key_id TEXT`, `previous_hash TEXT`
5. **Immutability trigger**: PostgreSQL trigger `prevent_audit_modification()` — RAISE EXCEPTION on UPDATE/DELETE
6. **Purge bypass**: DataCleaner uses `SET LOCAL bytebase.audit_purge = 'true'`

## Acceptance Criteria

- [ ] Hash chain sequential, verifiable
- [ ] ECDSA signature valid
- [ ] UPDATE/DELETE blocked by trigger
- [ ] DataCleaner purge still works (bypass)
- [ ] Performance: hash + sign < 1ms per entry

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/component/integrity/hashchain.go` | New file |
| `backend/component/integrity/signing.go` | New file |
| `backend/api/v1/audit.go` | Hash chain + sign |
| `backend/migrator/migration/` | audit_log columns + trigger |

## Definition of Done

- Hash chain verification end-to-end
