# TASK-LIM-004-A2: Backend CORS + Auth Mode Extension

| Field | Value |
|-------|-------|
| Solution | SOL-LIM-004 |
| Phase | A — Backend Support |
| Priority | P0 |
| Depends On | — |
| Est. | M (~150 LoC) |

## Objective

Add CORS middleware for cross-origin frontend and extend auth interceptor to support Bearer token alongside cookies.

## Files

| Action | Path |
|--------|------|
| MODIFY | `backend/server/echo_routes.go` — add CORS middleware |
| MODIFY | `backend/api/auth/auth.go` — dual auth mode |
| MODIFY | `backend/api/v1/auth_service.go` — token response mode |

## Specification

### CORS middleware

Config from env `CORS_ALLOWED_ORIGINS` (comma-separated). If empty → no CORS (embedded mode).

```go
e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
    AllowOrigins:     strings.Split(os.Getenv("CORS_ALLOWED_ORIGINS"), ","),
    AllowMethods:     []string{GET, POST, PUT, DELETE, PATCH, OPTIONS},
    AllowHeaders:     []string{"Authorization", "Content-Type", "Connect-Protocol-Version"},
    AllowCredentials: true,
    MaxAge:           86400,
}))
```

### Auth interceptor — dual mode

```go
func extractToken(req) (string, error) {
    // Priority 1: Authorization Bearer header
    if auth := req.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
        return strings.TrimPrefix(auth, "Bearer "), nil
    }
    // Priority 2: Cookie (existing behavior)
    return extractCookieToken(req, "access-token"), nil
}
```

### Login response — token mode

When `X-Auth-Mode: token` header present:
- Return `accessToken` + `refreshToken` in response body
- When absent: set HttpOnly cookies (existing behavior)

## Acceptance Criteria

- [x] CORS headers set when `CORS_ALLOWED_ORIGINS` configured → **DONE**: New `else if` branch in `configureEchoRouters()` parses env var
- [x] No CORS middleware when env var empty (backward compat) → **DONE**: Only activates when `os.Getenv("CORS_ALLOWED_ORIGINS") != ""`
- [x] Bearer token auth works alongside cookie auth → **DONE**: `GetTokenFromHeaders()` already checks Authorization header first (Priority 1: Bearer, Priority 2: Cookie) — no changes needed
- [x] Login returns tokens in body when `X-Auth-Mode: token` → **DONE**: New token-mode branch in `finalizeLogin()` sets `response.Token` + `X-Refresh-Token` header
- [x] Existing cookie auth unaffected → **DONE**: Token mode is `else if` before existing `req.Msg.Web` branch

## Implementation Notes

- Modified `backend/server/echo_routes.go`:
  - Added `os` import
  - Production CORS with `CORS_ALLOWED_ORIGINS` env var (comma-separated origins)
  - Allows `Authorization`, `X-Auth-Mode` headers; exposes `X-Refresh-Token`
  - `MaxAge: 86400` (24h preflight cache)
- Modified `backend/api/v1/auth_service_login.go`:
  - Token mode: reads `X-Auth-Mode: token` header
  - Returns access token in response body + refresh token via `X-Refresh-Token` header
  - No protobuf changes needed — uses existing `Token` field + custom header
- **No changes to auth.go** — dual auth already implemented in `GetTokenFromHeaders()`

**Status: ✅ DONE**
