# TASK-WEAK-001-1: CSP Nonce Middleware

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-001 |
| Priority | P0 |
| Depends On | — |
| Est. | M (~150 LoC) |

## Objective

Replace static CSP string with per-request nonce-based CSP. Eliminates `style-src 'unsafe-inline'` XSS vector.

## Files

| Action | Path |
|--------|------|
| CREATE | `backend/server/csp.go` |
| MODIFY | `backend/server/echo_routes.go` — `securityHeadersMiddleware` |
| MODIFY | `backend/component/config/profile.go` — add `CSPNonceEnabled` flag |

## Specification

### `csp.go`

- `generateNonce() (string, error)` — `crypto/rand` 128-bit → base64
- `GetCSPNonce(c *echo.Context) string` — retrieve nonce from request context
- `buildCSP(nonce, scriptHashes) string` — CSP with `'nonce-{nonce}'` in style-src
- `buildCSPLegacy(scriptHashes) string` — current unsafe-inline CSP (fallback)

### `echo_routes.go` modification

```go
func securityHeadersMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
    scriptHashes := loadCSPHashes()
    return func(c *echo.Context) error {
        nonce, _ := generateNonce()
        ctx := context.WithValue(c.Request().Context(), cspNonceKey, nonce)
        c.SetRequest(c.Request().WithContext(ctx))
        csp := buildCSP(nonce, scriptHashes)  // or buildCSPLegacy if flag off
        c.Response().Header().Set("Content-Security-Policy", csp)
        return next(c)
    }
}
```

### Feature flag

`CSP_NONCE_ENABLED` env var (default false) → gradual rollout.

## Acceptance Criteria

- [ ] Each request gets unique 128-bit nonce
- [ ] CSP header contains `'nonce-{nonce}'` instead of `'unsafe-inline'` in style-src
- [ ] Feature flag `CSP_NONCE_ENABLED=false` → legacy CSP (no behavior change)
- [ ] Unit test: `TestGenerateNonce` unique, `TestBuildCSP` no unsafe-inline
