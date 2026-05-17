# TASK-SEC-009 — API Key Service CRUD + Proto

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-009                               |
| **Source**       | SOL-SEC-002 §3.5                           |
| **Status**       | Pending                                    |
| **Priority**     | P0                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 2                                   |

---

## Mô tả

Implement APIKeyService gRPC service với CRUD operations. Full key chỉ hiển thị 1 lần tại creation.

## Scope

1. **Proto**: `api_key_service.proto` — CreateAPIKey, ListAPIKeys, RevokeAPIKey, RotateAPIKey, GetAPIKeyUsage
2. **CreateAPIKey**: Generate `bb_live_<random32>`, return full key ONCE, store SHA-256 hash + last 4 chars hint
3. **ListAPIKeys**: Return metadata only (name, hint, scopes, created_ts, last_used) — NEVER return key
4. **RevokeAPIKey**: Set is_active=false, blacklist if needed
5. **RotateAPIKey**: Create new key same rotation_id, deprecate old with 24h grace period
6. **GetAPIKeyUsage**: Usage stats aggregated by endpoint, time range

## Acceptance Criteria

- [ ] Create returns full key exactly once
- [ ] List never exposes key value
- [ ] Rotate creates new key, old key works during grace period
- [ ] Permission check: only key owner or workspace admin

## Files cần thay đổi

| File | Action |
|------|--------|
| `proto/v1/api_key_service.proto` | New proto |
| `backend/api/v1/api_key_service.go` | New service |

## Definition of Done

- gRPC API functional with ConnectRPC
