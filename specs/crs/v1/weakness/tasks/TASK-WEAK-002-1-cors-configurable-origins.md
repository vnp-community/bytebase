# TASK-WEAK-002-1: CORS Refactor + Configurable Origins

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-002 |
| Priority | P0 |
| Depends On | — |
| Est. | S (~80 LoC) |

## Objective

Extract CORS logic into `configureCORS()`. Add production mode with configurable origins. Reject wildcard `*` in production.

## Files

| Action | Path |
|--------|------|
| MODIFY | `backend/server/echo_routes.go` — extract `configureCORS()` |
| MODIFY | `backend/component/config/profile.go` — add `CORSAllowedOrigins`, `CORSMaxAge` |
| MODIFY | `backend/server/server.go` — dev mode startup warning |

## Specification

### `echo_routes.go`

```go
func configureCORS(e *echo.Echo, profile *config.Profile) {
    if profile.Mode == common.ReleaseModeDev {
        // Existing wildcard (dev only)
        return
    }
    if len(profile.CORSAllowedOrigins) == 0 { return } // same-origin
    for _, o := range profile.CORSAllowedOrigins {
        if o == "*" { slog.Error("CORS wildcard not allowed in production"); return }
    }
    e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
        AllowOrigins: profile.CORSAllowedOrigins,
        AllowCredentials: true, MaxAge: profile.CORSMaxAge,
    }))
}
```

### `profile.go` — new fields

- `CORSAllowedOrigins []string` from `CORS_ALLOWED_ORIGINS` (comma-separated)
- `CORSMaxAge int` from `CORS_MAX_AGE` (default 3600)

### Dev mode warning

```go
slog.Warn("⚠️ SERVER RUNNING IN DEVELOPMENT MODE", slog.Bool("cors_wildcard", true))
```

## Acceptance Criteria

- [ ] Dev mode: wildcard CORS (unchanged) + warning log
- [ ] Production + no env: no CORS (same-origin)
- [ ] Production + `CORS_ALLOWED_ORIGINS=https://app.example.com`: only that origin
- [ ] Production + `CORS_ALLOWED_ORIGINS=*`: rejected, no CORS installed
