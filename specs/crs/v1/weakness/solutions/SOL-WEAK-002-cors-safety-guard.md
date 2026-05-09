# Solution: CORS Safety Guard — CR-WEAK-002

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **Solution ID**    | SOL-WEAK-002                                             |
| **CR Reference**   | CR-WEAK-002                                              |
| **Title**          | CORS Safety Guard — Dev Mode Protection & Configurable Origins |
| **Affected Layers**| L2 (API Gateway), L10 (Infrastructure)                   |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-09                                               |

---

## 1. Architecture Context

Per architecture.md §3 (L2): CORS middleware is conditionally applied only in dev mode. Production has no CORS middleware (same-origin only). Per TDD.md §2: `profile.Mode` set from `BB_MODE` env var.

---

## 2. Current Implementation (echo_routes.go:39-49)

```go
if profile.Mode == common.ReleaseModeDev {
    e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
        UnsafeAllowOriginFunc: func(_ *echo.Context, origin string) (string, bool, error) {
            return origin, true, nil  // WILDCARD
        },
        AllowCredentials: true,       // Combined with wildcard = CORS misconfig
    }))
}
```

**Risk**: If `BB_MODE=dev` set in production → full CORS bypass with credentials.

---

## 3. Solution Design

### 3.1 Dev Mode Warning (server.go)

```go
if profile.Mode == common.ReleaseModeDev {
    slog.Warn("⚠️  SERVER RUNNING IN DEVELOPMENT MODE",
        slog.Bool("cors_wildcard", true),
        slog.String("action", "Set BB_MODE=release for production"))
    devModeGauge.Set(1)
}
```

### 3.2 Configurable CORS Origins (profile.go)

```go
type Profile struct {
    CORSAllowedOrigins []string // CORS_ALLOWED_ORIGINS env var
    CORSMaxAge         int      // CORS_MAX_AGE, default 3600
}
```

### 3.3 Refactored CORS (echo_routes.go)

```go
func configureCORS(e *echo.Echo, profile *config.Profile) {
    if profile.Mode == common.ReleaseModeDev {
        // Existing wildcard behavior (dev only)
        e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
            UnsafeAllowOriginFunc: func(_ *echo.Context, origin string) (string, bool, error) {
                return origin, true, nil
            },
            AllowCredentials: true,
        }))
        return
    }
    // Production: specific origins only, reject "*"
    if len(profile.CORSAllowedOrigins) == 0 { return } // same-origin
    for _, o := range profile.CORSAllowedOrigins {
        if o == "*" {
            slog.Error("CORS wildcard '*' not allowed in production — ignoring")
            return
        }
    }
    e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
        AllowOrigins:     profile.CORSAllowedOrigins,
        AllowCredentials: true,
        MaxAge:           profile.CORSMaxAge,
    }))
}
```

### 3.4 CORS Audit Middleware (cors_audit.go — NEW)

Log rejected CORS requests + Prometheus counter `bytebase_cors_rejected_total{origin}`.

---

## 4. File Change Manifest

| File | Action | Impact |
|------|--------|--------|
| `backend/server/echo_routes.go` | MODIFY | Extract CORS to `configureCORS()` |
| `backend/server/cors_audit.go` | NEW | Rejection logging + metrics |
| `backend/server/server.go` | MODIFY | Dev mode startup warning |
| `backend/component/config/profile.go` | MODIFY | Add CORS config fields |

## 5. Test Strategy

- Verify dev mode → wildcard CORS + warning log
- Verify release mode → no CORS by default
- Verify `CORS_ALLOWED_ORIGINS=https://app.example.com` → only that origin
- Verify `CORS_ALLOWED_ORIGINS=*` → rejected, no CORS installed

## 6. Rollback

Remove `CORS_ALLOWED_ORIGINS` env → same-origin. No DB changes.
