# TASK-SEC-035 — CSP Nonce-Based + Security Headers Enhancement

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-035                               |
| **Source**       | SOL-SEC-015 §3.1-§3.4                     |
| **Status**       | Pending                                    |
| **Priority**     | P0                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 1                                   |

---

## Mô tả

Enhance existing `securityHeadersMiddleware` (L2) với full security headers và nonce-based CSP. CSP violation reporting endpoint.

## Scope

1. **Enhanced headers**: X-Frame-Options → DENY, Referrer-Policy: strict-origin-when-cross-origin, Permissions-Policy (disable camera/mic/geo/payment), COEP: require-corp, CORP: same-origin
2. **Nonce-based CSP**: Generate 128-bit random nonce per request, set in CSP `script-src 'nonce-xxx'`, pass via context
3. **Frontend injection**: `server_frontend_embed.go` — replace `{{CSP_NONCE}}` in HTML with actual nonce
4. **Vite plugin**: Extend `vite-plugin-export-csp-hashes.ts` — add nonce attribute to script tags
5. **CSP reporting**: `POST /v1/csp-report` endpoint — parse violation report, log, store for analysis
6. **CORS hardening**: Explicit origins from workspace setting, NO wildcard `*`, credentials=true
7. **SSO integration**: Add SSO redirect domains to CSP connect-src

## Acceptance Criteria

- [ ] All enhanced headers present in responses
- [ ] CSP nonce injected correctly
- [ ] Monaco Editor still works (style-src unsafe-inline)
- [ ] CSP violations logged
- [ ] CORS only allows configured origins

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/server/echo_routes.go` | Enhanced middleware |
| `backend/server/server_frontend_embed.go` | Nonce injection |
| `frontend/vite-plugin-export-csp-hashes.ts` | Nonce support |
| `backend/api/v1/csp_report_service.go` | New endpoint |

## Definition of Done

- Headers verified via browser DevTools
- Monaco Editor regression tested
