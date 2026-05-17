# TASK-SEC-002 — JWT RS256 Migration + Cookie Hardening

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-002                               |
| **Source**       | SOL-SEC-001 §3.1, §3.6                    |
| **Status**       | Pending                                    |
| **Priority**     | P0                                         |
| **Complexity**   | High                                       |
| **Sprint**       | Sprint 1                                   |

---

## Mô tả

Chuyển JWT signing từ HMAC-SHA256 sang RS256 asymmetric. Harden cookie attributes. Dual-validation period 30 ngày.

## Scope

1. **TokenSigner**: `backend/api/auth/` — RS256 key pair generation, `privateKey`/`publicKey`/`keyID`
2. **SessionClaims**: Extend JWT claims với `Fingerprint` (fp), `KeyID` (kid)
3. **Dual validation**: Accept cả HMAC và RS256 trong 30 ngày migration period
4. **Cookie**: `__Host-bb-session` prefix, `HttpOnly`, `Secure`, `SameSite=Strict`, `Path=/`
5. **Key storage**: RS256 keys lưu trong setting hoặc external secret (nếu có CR-ENT-015)

## Acceptance Criteria

- [ ] RS256 signing/verification hoạt động
- [ ] Legacy HMAC tokens vẫn valid trong 30 ngày
- [ ] Cookie attributes verified qua browser DevTools
- [ ] Unit tests cho TokenSigner, SessionClaims
- [ ] Key rotation mechanism documented

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/api/auth/` | TokenSigner RS256, SessionClaims |
| `backend/server/echo_routes.go` | Cookie hardening |

## Definition of Done

- Zero downtime migration (dual-validate)
- Cookie security verified
