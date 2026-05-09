# Solution: CSP Security Hardening — CR-WEAK-001

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **Solution ID**    | SOL-WEAK-001                                             |
| **CR Reference**   | CR-WEAK-001                                              |
| **Title**          | CSP Nonce Middleware + Inline Style Extraction            |
| **Affected Layers**| L2 (API Gateway), L1 (Presentation)                      |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-09                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Architecture Context

Per [architecture.md](../../architecture.md) §3 (L2 — API Gateway):
- Echo v5 HTTP server handles security headers via `securityHeadersMiddleware` in `echo_routes.go`
- CSP hashes are generated at build time by `vite-plugin-export-csp-hashes.ts` and loaded via `loadCSPHashes()`
- Middleware stack order: `recoverMiddleware` → `securityHeadersMiddleware` → CORS → RequestLogger → Prometheus

Per [TDD.md](../../TDD.md) §11.2 (HTTP Security Headers):
- CSP is a **static string** computed once at middleware registration — not per-request
- Frontend uses Vue 3 + React 19 hybrid with Monaco Editor requiring WASM runtime

---

## 2. Current Implementation Analysis

### 2.1 CSP Header (echo_routes.go:126-162)

```go
// Current — STATIC CSP string, not per-request
func securityHeadersMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
    scriptHashes := loadCSPHashes()
    csp := "default-src 'self'; " +
        "script-src 'self' " + strings.Join(scriptHashes, " ") + " 'wasm-unsafe-eval'; " +
        "style-src 'self' 'unsafe-inline'; " +           // ← TARGET
        "img-src 'self' data: blob: discordapp.com; " +
        "connect-src 'self' data: ws: wss: https://api.github.com https://hub.bytebase.com; " +
        // ...
    return func(c *echo.Context) error {
        c.Response().Header().Set("Content-Security-Policy", csp)
        return next(c)
    }
}
```

**Key problem**: CSP is computed once and reused — no per-request nonce injection.

### 2.2 Root Cause Analysis

| Directive              | Issue                        | Root Cause                              |
|------------------------|-----------------------------|-----------------------------------------|
| `style-src 'unsafe-inline'` | XSS via style injection | Vue components use `:style="..."` bindings; Naive UI injects runtime styles |
| `'wasm-unsafe-eval'`   | WASM module injection        | Monaco Editor's TextMate grammar uses WASM compilation |
| `connect-src ws:`      | Unencrypted WebSocket        | LSP server (`/lsp`) supports both `ws:` and `wss:` |
| `connect-src data:`    | Data URI exfiltration        | Monaco language definitions use `data:` scheme |

---

## 3. Solution Design

### 3.1 Phase 1 — Per-Request CSP Nonce Middleware

**New file**: `backend/server/csp.go`

```go
package server

import (
    "crypto/rand"
    "encoding/base64"
    "fmt"
    "strings"

    "github.com/labstack/echo/v5"
)

// cspNonceKey is the context key for the CSP nonce.
type cspNonceKeyType struct{}
var cspNonceKey = cspNonceKeyType{}

// generateNonce creates a cryptographically secure 128-bit nonce.
func generateNonce() (string, error) {
    b := make([]byte, 16) // 128-bit
    if _, err := rand.Read(b); err != nil {
        return "", err
    }
    return base64.StdEncoding.EncodeToString(b), nil
}

// GetCSPNonce retrieves the CSP nonce from the request context.
func GetCSPNonce(c *echo.Context) string {
    if v := c.Request().Context().Value(cspNonceKey); v != nil {
        return v.(string)
    }
    return ""
}

// buildCSP constructs the CSP header with the given nonce.
// scriptHashes are precomputed at startup from dist/csp-hashes.json.
func buildCSP(nonce string, scriptHashes []string) string {
    parts := []string{
        "default-src 'self'",
        fmt.Sprintf("script-src 'self' %s 'wasm-unsafe-eval'", strings.Join(scriptHashes, " ")),
        fmt.Sprintf("style-src 'self' 'nonce-%s'", nonce), // NONCE replaces unsafe-inline
        "img-src 'self' data: blob: discordapp.com",
        "connect-src 'self' wss: https://api.github.com https://hub.bytebase.com", // ws: → wss: only
        "font-src 'self'",
        "object-src 'none'",
        "base-uri 'self'",
        "form-action 'self'",
        "frame-ancestors 'self'",
        "report-uri /api/csp-report", // violation reporting
    }
    return strings.Join(parts, "; ")
}
```

**Modified file**: `backend/server/echo_routes.go`

```go
func securityHeadersMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
    scriptHashes := loadCSPHashes()

    return func(c *echo.Context) error {
        // Generate per-request nonce
        nonce, err := generateNonce()
        if err != nil {
            slog.Error("failed to generate CSP nonce", log.BBError(err))
            // Fallback: use unsafe-inline if nonce generation fails
            nonce = ""
        }

        // Store nonce in context for template/HTML injection
        ctx := context.WithValue(c.Request().Context(), cspNonceKey, nonce)
        c.SetRequest(c.Request().WithContext(ctx))

        // Build CSP with nonce
        var csp string
        if nonce != "" {
            csp = buildCSP(nonce, scriptHashes)
        } else {
            // Graceful degradation — keep unsafe-inline
            csp = buildCSPLegacy(scriptHashes)
        }

        c.Response().Header().Set("Cross-Origin-Opener-Policy", "same-origin-allow-popups")
        c.Response().Header().Set("X-Frame-Options", "SAMEORIGIN")
        c.Response().Header().Set("X-Content-Type-Options", "nosniff")
        if c.Request().TLS != nil || c.Request().Header.Get("X-Forwarded-Proto") == "https" {
            c.Response().Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
        }
        c.Response().Header().Set("Content-Security-Policy", csp)
        return next(c)
    }
}
```

### 3.2 Phase 2 — CSP Violation Report Endpoint

**New file**: `backend/server/csp_report.go`

```go
package server

import (
    "encoding/json"
    "io"
    "log/slog"
    "net/http"

    "github.com/labstack/echo/v5"
    "github.com/prometheus/client_golang/prometheus"
)

var cspViolationCounter = prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "bytebase_csp_violations_total",
        Help: "Total number of CSP violation reports",
    },
    []string{"directive", "blocked_uri"},
)

func init() {
    prometheus.MustRegister(cspViolationCounter)
}

type cspReport struct {
    Body struct {
        DocumentURI     string `json:"document-uri"`
        BlockedURI      string `json:"blocked-uri"`
        ViolatedDirective string `json:"violated-directive"`
        EffectiveDirective string `json:"effective-directive"`
        OriginalPolicy  string `json:"original-policy"`
        StatusCode      int    `json:"status-code"`
    } `json:"csp-report"`
}

func cspReportHandler(c *echo.Context) error {
    body, err := io.ReadAll(c.Request().Body)
    if err != nil {
        return c.NoContent(http.StatusBadRequest)
    }

    var report cspReport
    if err := json.Unmarshal(body, &report); err != nil {
        return c.NoContent(http.StatusBadRequest)
    }

    slog.Warn("CSP violation",
        slog.String("document_uri", report.Body.DocumentURI),
        slog.String("blocked_uri", report.Body.BlockedURI),
        slog.String("violated_directive", report.Body.ViolatedDirective),
    )

    cspViolationCounter.WithLabelValues(
        report.Body.EffectiveDirective,
        report.Body.BlockedURI,
    ).Inc()

    return c.NoContent(http.StatusNoContent)
}
```

**Registration** in `configureEchoRouters`:
```go
e.POST("/api/csp-report", cspReportHandler)
```

### 3.3 Phase 3 — Frontend Nonce Injection

**Modified file**: `backend/server/server_frontend_embed.go`

The SPA `index.html` is served by the embedded frontend handler. We need to inject the nonce into `<style>` and `<link>` tags at serve time:

```go
func embedFrontend(e *echo.Echo) {
    // ...existing code...
    e.GET("/*", func(c *echo.Context) error {
        // For index.html, inject CSP nonce
        if isIndexHTML(c.Request().URL.Path) {
            nonce := GetCSPNonce(c)
            html := injectNonceIntoHTML(indexHTMLContent, nonce)
            return c.HTMLBlob(http.StatusOK, html)
        }
        return serveStaticFile(c)
    })
}

// injectNonceIntoHTML adds nonce="..." to all <style> and <link rel="stylesheet"> tags.
func injectNonceIntoHTML(html []byte, nonce string) []byte {
    if nonce == "" {
        return html
    }
    result := string(html)
    // Add nonce to <style> tags
    result = strings.ReplaceAll(result, "<style", fmt.Sprintf(`<style nonce="%s"`, nonce))
    // Add nonce to <link rel="stylesheet"> tags
    result = strings.ReplaceAll(result, `<link rel="stylesheet"`,
        fmt.Sprintf(`<link nonce="%s" rel="stylesheet"`, nonce))
    return []byte(result)
}
```

### 3.4 Phase 4 — Naive UI Style Injection Nonce

Naive UI dynamically injects `<style>` elements at runtime. We need to configure it to use the CSP nonce:

**Modified file**: `frontend/src/App.vue` (or equivalent config)

```typescript
// Naive UI v2.38+ supports CSP nonce via n-config-provider
// Inject nonce from server via <meta> tag
const cspNonce = document.querySelector('meta[name="csp-nonce"]')?.getAttribute('content') || '';

// In n-config-provider
<n-config-provider :csp="{ nonce: cspNonce }">
  <router-view />
</n-config-provider>
```

**Server-side injection**: Add `<meta name="csp-nonce" content="{nonce}">` to `index.html` during serve.

### 3.5 Phase 5 — connect-src Tightening

```go
// In buildCSP():
// BEFORE: "connect-src 'self' data: ws: wss: https://api.github.com https://hub.bytebase.com"
// AFTER:  "connect-src 'self' wss: https://api.github.com https://hub.bytebase.com"
//
// Removed: data: (Monaco can use import() instead)
//          ws: (force encrypted WebSocket in production)
```

For development mode, keep the broader policy:
```go
func buildCSPDev(nonce string, scriptHashes []string) string {
    // Dev mode allows ws: for localhost development
    // ... same as buildCSP but with ws: in connect-src
}
```

---

## 4. File Change Manifest

| File | Layer | Action | Impact |
|------|-------|--------|--------|
| `backend/server/csp.go` | L2 | **NEW** | CSP nonce generation + builder |
| `backend/server/csp_report.go` | L2 | **NEW** | CSP violation reporting endpoint |
| `backend/server/echo_routes.go` | L2 | **MODIFY** | Per-request nonce in `securityHeadersMiddleware` |
| `backend/server/server_frontend_embed.go` | L2 | **MODIFY** | Inject nonce into served HTML |
| `frontend/src/App.vue` | L1 | **MODIFY** | Naive UI csp nonce config |
| `frontend/index.html` | L1 | **MODIFY** | Add `<meta name="csp-nonce">` placeholder |

---

## 5. Dependency Direction Validation

```
L2 (echo_routes.go) → crypto/rand (stdlib)
L2 (csp_report.go)  → prometheus (existing dep)
L2 (embed.go)       → strings (stdlib)
L1 (App.vue)        → naive-ui n-config-provider (existing dep)
```

**No new external dependencies.** All changes use stdlib `crypto/rand` and existing Naive UI CSP API.

---

## 6. Migration Strategy

### Incremental Rollout via Feature Flag

```go
// backend/component/config/profile.go
type Profile struct {
    // ...existing fields...
    CSPNonceEnabled bool `env:"CSP_NONCE_ENABLED" default:"false"`
}
```

```go
// In securityHeadersMiddleware:
if profile.CSPNonceEnabled {
    csp = buildCSP(nonce, scriptHashes)    // New nonce-based CSP
} else {
    csp = buildCSPLegacy(scriptHashes)     // Current unsafe-inline CSP
}
```

This allows gradual rollout:
1. Deploy with `CSP_NONCE_ENABLED=false` — same behavior
2. Enable in staging — monitor CSP violation reports
3. Fix violations iteratively
4. Enable in production
5. Remove feature flag after stabilization

---

## 7. Test Strategy

### Unit Tests

```go
// backend/server/csp_test.go
func TestGenerateNonce(t *testing.T) {
    n1, err := generateNonce()
    require.NoError(t, err)
    require.Len(t, n1, 24) // base64(16 bytes) = 24 chars

    n2, _ := generateNonce()
    require.NotEqual(t, n1, n2) // each nonce unique
}

func TestBuildCSP(t *testing.T) {
    csp := buildCSP("abc123", []string{"'sha256-xxx'"})
    assert.Contains(t, csp, "'nonce-abc123'")
    assert.NotContains(t, csp, "'unsafe-inline'")
    assert.NotContains(t, csp, "ws:")  // only wss:
    assert.Contains(t, csp, "wss:")
    assert.Contains(t, csp, "report-uri")
}

func TestCSPReportHandler(t *testing.T) {
    // POST /api/csp-report with valid CSP report JSON
    // Verify 204 response + metric incremented
}
```

### Integration Tests (Playwright)

```typescript
// Visual regression: verify all pages render without CSP violations
test('no CSP violations on dashboard', async ({ page }) => {
    const violations: string[] = [];
    page.on('console', msg => {
        if (msg.text().includes('[Report Only]') || msg.text().includes('CSP')) {
            violations.push(msg.text());
        }
    });
    await page.goto('/');
    await page.waitForLoadState('networkidle');
    expect(violations).toHaveLength(0);
});
```

---

## 8. Rollback Plan

1. Set `CSP_NONCE_ENABLED=false` in environment → immediate fallback to `unsafe-inline`
2. Feature flag change requires no restart if using runtime config reload
3. No database migration → no rollback needed for data layer
