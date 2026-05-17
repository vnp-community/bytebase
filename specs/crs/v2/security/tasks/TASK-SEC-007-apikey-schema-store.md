# TASK-SEC-007 — API Key Schema + Store Layer

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-007                               |
| **Source**       | SOL-SEC-002 §3.1                           |
| **Status**       | Pending                                    |
| **Priority**     | P0                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 1                                   |

---

## Mô tả

Tạo DB migration cho `api_key` và `api_key_usage` tables. Implement Store layer cho API key CRUD.

## Scope

1. **Migration**: `api_key` table (id, name, prefix, key_hash UNIQUE, key_hint, principal_uid FK, scopes JSONB, allowed_ips TEXT[], env_restrict TEXT[], expires_at, rotation_id, is_active, created_ts, last_used)
2. **Migration**: `api_key_usage` table (id, key_id FK, endpoint, ip_address, status_code, created_ts)
3. **Indexes**: `idx_api_key_principal`, `idx_api_key_hash` (partial WHERE is_active), `idx_api_key_usage_ts`
4. **Store**: `store/api_key.go` — CreateAPIKey, GetAPIKeyByHash, ListAPIKeys, RevokeAPIKey, UpdateLastUsed
5. **Store**: `store/api_key_usage.go` — RecordUsage, GetUsageStats

## Acceptance Criteria

- [ ] Migration idempotent
- [ ] key_hash stored as SHA-256, full key never persisted
- [ ] Store CRUD unit tests pass

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/migrator/migration/` | New migration |
| `backend/store/api_key.go` | New file |
| `backend/store/api_key_usage.go` | New file |

## Definition of Done

- Store layer 100% tested
