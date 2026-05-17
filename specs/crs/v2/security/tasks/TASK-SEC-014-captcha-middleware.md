# TASK-SEC-014 — CAPTCHA Middleware

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-014                               |
| **Source**       | SOL-SEC-003 §2                             |
| **Status**       | Pending                                    |
| **Priority**     | P1                                         |
| **Complexity**   | Low                                        |
| **Sprint**       | Sprint 2                                   |

---

## Mô tả

Implement CAPTCHA validation middleware (L2) cho login endpoint khi triggered bởi lockout logic.

## Scope

1. **Middleware**: Echo middleware kiểm tra `X-Captcha-Token` header
2. **Providers**: Support reCAPTCHA v3, hCaptcha, Cloudflare Turnstile — configurable via workspace setting
3. **Server-side validation**: HTTP call to provider verify endpoint
4. **Integration**: AuthService returns `captcha_required` status → frontend hiển thị CAPTCHA widget

## Acceptance Criteria

- [ ] CAPTCHA validated server-side
- [ ] Multiple provider support
- [ ] Frontend widget renders correctly

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/server/echo_routes.go` | CAPTCHA middleware |
| `frontend/src/pages/auth/` | CAPTCHA widget |

## Definition of Done

- CAPTCHA flow verified end-to-end
