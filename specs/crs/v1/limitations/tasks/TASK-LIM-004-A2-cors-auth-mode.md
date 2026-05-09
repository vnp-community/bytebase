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

- [ ] CORS headers set when `CORS_ALLOWED_ORIGINS` configured
- [ ] No CORS middleware when env var empty (backward compat)
- [ ] Bearer token auth works alongside cookie auth
- [ ] Login returns tokens in body when `X-Auth-Mode: token`
- [ ] Existing cookie auth unaffected
