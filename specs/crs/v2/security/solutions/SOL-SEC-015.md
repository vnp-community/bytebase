# Solution: CR-SEC-015 — Content Security Policy & HTTP Security Headers

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-SEC-015                |
| **Solution**   | SOL-SEC-015               |
| **Status**     | Proposed                  |
| **Complexity** | Medium                    |

---

## 1. Tóm tắt giải pháp

Harden existing `securityHeadersMiddleware` (L2) và extend existing CSP system (build-time hash generation via `vite-plugin-export-csp-hashes.ts`). Chuyển từ hash-based sang nonce-based CSP cho scripts. Thêm CSP violation reporting endpoint (L4). CORS strict config via workspace settings (L8).

---

## 2. Architectural Alignment

Bytebase đã có security headers foundation (TDD Section 11.2, Architecture L2 Middleware Stack):
- CSP with build-time hashes
- X-Frame-Options: SAMEORIGIN
- X-Content-Type-Options: nosniff
- HSTS
- Cross-Origin-Opener-Policy

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L2** | `echo_routes.go` → `securityHeadersMiddleware` | Enhanced headers |
| **L1** | `vite-plugin-export-csp-hashes.ts` | Nonce-based CSP build |
| **L4** | `csp_report_service.go` (new) | CSP violation reports |
| **L8** | `store/setting.go` | CORS/CSP config storage |

---

## 3. Chi tiết Implementation

### 3.1 L2 — Enhanced Security Headers Middleware

**File**: `backend/server/echo_routes.go` (extend existing `securityHeadersMiddleware`)

```go
func securityHeadersMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        h := c.Response().Header()

        // EXISTING headers (TDD Section 11.2) — enhanced:
        h.Set("X-Content-Type-Options", "nosniff")
        h.Set("X-Frame-Options", "DENY") // Tightened from SAMEORIGIN
        h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
        h.Set("Cross-Origin-Opener-Policy", "same-origin")

        // NEW headers:
        h.Set("X-XSS-Protection", "0") // Disable legacy filter, rely on CSP
        h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
        h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
        h.Set("Cross-Origin-Embedder-Policy", "require-corp")
        h.Set("Cross-Origin-Resource-Policy", "same-origin")

        // CSP with nonce
        nonce := generateCSPNonce() // 128-bit random
        c.Set("csp-nonce", nonce)
        csp := buildCSP(nonce, c)
        h.Set("Content-Security-Policy", csp)

        return next(c)
    }
}

func buildCSP(nonce string, c echo.Context) string {
    // Base CSP policy
    directives := []string{
        "default-src 'self'",
        fmt.Sprintf("script-src 'self' 'nonce-%s' 'wasm-unsafe-eval'", nonce),
        "style-src 'self' 'unsafe-inline'", // Monaco Editor requires this
        "img-src 'self' data: blob:",
        "font-src 'self'",
        "connect-src 'self' wss:",
        "frame-ancestors 'none'",
        "base-uri 'self'",
        "form-action 'self'",
        "object-src 'none'",
        "upgrade-insecure-requests",
    }

    // Add SSO redirect domains from workspace config
    ssoOrigins := getSSOrigins(c.Request().Context())
    if len(ssoOrigins) > 0 {
        directives = append(directives,
            fmt.Sprintf("connect-src 'self' wss: %s", strings.Join(ssoOrigins, " ")))
    }

    // CSP reporting
    directives = append(directives, "report-uri /v1/csp-report")

    return strings.Join(directives, "; ")
}
```

### 3.2 L1 — Nonce Injection in Frontend

**File**: `frontend/vite-plugin-export-csp-hashes.ts` (extend existing)

```typescript
// Extend existing plugin to support nonce-based loading
export default function cspNoncePlugin(): Plugin {
    return {
        name: 'csp-nonce',
        transformIndexHtml(html) {
            // Replace script tags with nonce placeholder
            // Server will inject actual nonce at runtime
            return html.replace(
                /<script/g,
                '<script nonce="{{CSP_NONCE}}"'
            );
        }
    };
}
```

**File**: `backend/server/server_frontend_embed.go` (extend existing 1.8KB)

```go
func (s *Server) serveFrontend(c echo.Context) error {
    // Read embedded HTML
    html := s.frontendHTML

    // Inject CSP nonce into script tags
    nonce := c.Get("csp-nonce").(string)
    html = strings.ReplaceAll(html, "{{CSP_NONCE}}", nonce)

    return c.HTML(200, html)
}
```

### 3.3 L4 — CSP Violation Report Endpoint

**File**: `backend/api/v1/csp_report_service.go` (new)

```go
func (s *CSPReportService) HandleCSPReport(c echo.Context) error {
    var report struct {
        CSPReport struct {
            DocumentURI    string `json:"document-uri"`
            ViolatedDirective string `json:"violated-directive"`
            OriginalPolicy string `json:"original-policy"`
            BlockedURI     string `json:"blocked-uri"`
        } `json:"csp-report"`
    }
    if err := c.Bind(&report); err != nil {
        return c.NoContent(400)
    }

    // Log violation
    slog.Warn("CSP violation",
        "uri", report.CSPReport.DocumentURI,
        "directive", report.CSPReport.ViolatedDirective,
        "blocked", report.CSPReport.BlockedURI)

    // Store for analysis
    s.store.CreateCSPViolation(ctx, &report.CSPReport)

    return c.NoContent(204)
}
```

### 3.4 CORS Hardening

```go
func corsMiddleware(allowedOrigins []string) echo.MiddlewareFunc {
    return middleware.CORSWithConfig(middleware.CORSConfig{
        AllowOrigins:     allowedOrigins, // From workspace settings, NO wildcard
        AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
        AllowHeaders:     []string{"Content-Type", "Authorization", "X-Request-ID"},
        AllowCredentials: true,
        MaxAge:           3600,
    })
}
```

---

## 4. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-ENT-008 (SSO) | SSO redirect domains in CSP connect-src |
| CR-SEC-010 (SIEM) | CSP violations forwarded as security events |

---

## 5. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | Enhanced security headers + CORS hardening | Sprint 1 |
| 2 | Nonce-based CSP (report-only mode) | Sprint 1 |
| 3 | CSP enforcement + violation reporting | Sprint 2 |
| 4 | Monaco Editor compatibility testing | Sprint 2 |
