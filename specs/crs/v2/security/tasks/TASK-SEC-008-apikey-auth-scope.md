# TASK-SEC-008 — Auth Interceptor API Key Validation + Scope Check

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-008                               |
| **Source**       | SOL-SEC-002 §3.2, §3.3                    |
| **Status**       | Pending                                    |
| **Priority**     | P0                                         |
| **Complexity**   | High                                       |
| **Sprint**       | Sprint 1                                   |

---

## Mô tả

Extend Auth Interceptor (L3) để validate API keys với prefix `bb_`, và ACL Interceptor để enforce scope restrictions.

## Scope

1. **Auth Interceptor**: `authenticateAPIKey()` — parse `bb_live_`/`bb_test_` prefix, SHA-256 hash lookup, expiry check, IP restriction (CIDR matching)
2. **Backward compat**: Non-`bb_` prefix → `authenticateLegacyKey()`
3. **UserContext**: Extend với `Scopes []string` field
4. **ACL Interceptor**: `acl.go` — add scope check: `methodToScope(method)` → verify scope in UserContext.Scopes
5. **Usage tracking**: Async `go apiKeyUsageTracker.Record(keyID, method, IP)`

## Acceptance Criteria

- [ ] API key authentication qua `Authorization: Bearer bb_live_xxx`
- [ ] Expired key → 401
- [ ] IP not in allowlist → 403
- [ ] Missing scope → 403 with "API key lacks scope: X"
- [ ] Legacy keys still work
- [ ] Unit tests cho authenticateAPIKey, methodToScope

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/api/auth/` | authenticateAPIKey |
| `backend/api/v1/acl.go` | Scope check extension |

## Definition of Done

- Auth + ACL interceptor chain verified end-to-end
