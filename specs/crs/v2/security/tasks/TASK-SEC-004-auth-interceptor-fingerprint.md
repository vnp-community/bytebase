# TASK-SEC-004 — Auth Interceptor Fingerprint & Blacklist Check

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-004                               |
| **Source**       | SOL-SEC-001 §3.2                           |
| **Status**       | Pending                                    |
| **Priority**     | P0                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 2                                   |

---

## Mô tả

Extend Auth Interceptor (L3) thêm blacklist check và fingerprint validation vào authentication flow.

## Scope

1. **Blacklist check**: Sau `validateToken()`, check `blacklist.IsBlacklisted(claims.ID)` → return `codes.Unauthenticated`
2. **Fingerprint**: `computeFingerprint(headers)` — SHA-256 of (User-Agent + Accept-Language + IP subnet /24)
3. **Validation modes**: `strict` (exact match), `relaxed` (partial match), `off` — configurable via workspace setting
4. **Security event**: Emit fingerprint mismatch event to `SecurityEventChan` (nếu TASK-SEC-028 đã xong)
5. **Concurrent session**: Extend AuthService.Login — count active sessions, enforce limit, terminate_oldest hoặc deny_new

## Acceptance Criteria

- [ ] Blacklisted token → 401 Unauthenticated
- [ ] Fingerprint mismatch → 401 (strict mode)
- [ ] Concurrent session limit enforced (default: 5)
- [ ] Overhead < 1ms per request
- [ ] Unit tests cho computeFingerprint, validateFingerprint

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/api/auth/` | Blacklist + fingerprint checks |
| `backend/api/v1/auth_service.go` | Concurrent session logic |

## Definition of Done

- Interceptor chain verified end-to-end
- Performance benchmarked
