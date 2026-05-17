# TASK-SEC-020 — Store Layer Encryption Hooks

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-020                               |
| **Source**       | SOL-SEC-007 §3.3, §3.4                    |
| **Status**       | Pending                                    |
| **Priority**     | P0                                         |
| **Complexity**   | High                                       |
| **Sprint**       | Sprint 2                                   |

---

## Mô tả

Integrate encryption vào Store layer (L8) cho transparent encrypt/decrypt of sensitive fields. Migration từ plaintext sang encrypted.

## Scope

1. **Schema**: ALTER `data_source` ADD `encrypted_password TEXT`, `setting` ADD `encrypted_value TEXT`
2. **store/instance.go**: `CreateDataSource()` — encrypt password before INSERT, clear plaintext; `GetDataSource()` — decrypt on read
3. **store/setting.go**: Encrypt SSO client secrets, webhook signing keys
4. **Sensitive data map**: DataSource.Password, DataSource.SSLKey, SSO client secrets, webhook keys
5. **Migration tool**: `cmd/encrypt-migrate/` — scan existing plaintext → encrypt → update

## Acceptance Criteria

- [ ] Passwords never stored in plaintext after migration
- [ ] Decrypt-on-read transparent to callers
- [ ] Migration tool handles all existing data
- [ ] Rollback capability (keep plaintext during migration window)

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/migrator/migration/` | encrypted columns |
| `backend/store/instance.go` | Encrypt/decrypt hooks |
| `backend/store/setting.go` | Encrypt sensitive settings |

## Definition of Done

- Zero plaintext sensitive data in DB after migration
