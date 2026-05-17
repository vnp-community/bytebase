# TASK-SEC-005 — Refresh Token Rotation + Reuse Detection

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-005                               |
| **Source**       | SOL-SEC-001 §3.5, §4                      |
| **Status**       | Pending                                    |
| **Priority**     | P0                                         |
| **Complexity**   | High                                       |
| **Sprint**       | Sprint 3                                   |

---

## Mô tả

Implement refresh token rotation với family-based reuse detection. Nếu old refresh token bị reuse → revoke toàn bộ family.

## Scope

1. **Family tracking**: `family_id` trên `web_refresh_token` — group related tokens
2. **Rotation logic**: Trên refresh request → invalidate old token, issue new token cùng family_id, increment rotation_count
3. **Reuse detection**: Nếu token đã used (rotation_count mismatch) → revoke ALL tokens in family
4. **Security alert**: Emit reuse event (potential session hijacking)

## Acceptance Criteria

- [ ] Token rotation: old token invalidated, new token issued
- [ ] Reuse detection: family revoked on reuse attempt
- [ ] Grace period: 10s window cho concurrent requests
- [ ] Unit tests cho rotation logic, reuse detection

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/api/v1/auth_service.go` | Rotation logic |
| `backend/store/web_refresh_token.go` | Family operations |

## Definition of Done

- Rotation verified end-to-end
- Reuse triggers family revocation
