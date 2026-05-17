# TASK-WEAK-001-3: Frontend Nonce Injection

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-001 |
| Priority | P0 |
| Depends On | TASK-WEAK-001-1 |
| Est. | M (~120 LoC) |
| Status | ✅ Done |
| Completed | 2026-05-10 |

## Objective

Inject CSP nonce into served HTML (style/link tags) and configure Naive UI to use nonce for runtime style injection.

## Files

| Action | Path |
|--------|------|
| MODIFY | `backend/server/server_frontend_embed.go` — `injectNonceIntoHTML()` |
| MODIFY | `frontend/index.html` — add `<meta name="csp-nonce">` placeholder |
| MODIFY | `frontend/src/App.vue` — Naive UI `n-config-provider :csp` |

## Specification

### Backend HTML injection

```go
func injectNonceIntoHTML(html []byte, nonce string) []byte {
    result := string(html)
    result = strings.ReplaceAll(result, "<style", fmt.Sprintf(`<style nonce="%s"`, nonce))
    result = strings.ReplaceAll(result, `<link rel="stylesheet"`,
        fmt.Sprintf(`<link nonce="%s" rel="stylesheet"`, nonce))
    result = strings.ReplaceAll(result, `content="CSP_NONCE_PLACEHOLDER"`,
        fmt.Sprintf(`content="%s"`, nonce))
    return []byte(result)
}
```

### Frontend Naive UI config

```typescript
const cspNonce = document.querySelector('meta[name="csp-nonce"]')?.getAttribute('content') || '';
// In n-config-provider: :csp="{ nonce: cspNonce }"
```

## Acceptance Criteria

- [x] Served `index.html` has nonce on all `<style>` and `<link rel="stylesheet">` tags
- [x] Meta tag contains current request nonce
- [x] Naive UI reads nonce and applies to dynamically injected styles
- [x] No CSP violations on dashboard page load (Playwright test)

## Implementation Notes

- Added `injectNonceIntoHTML()` to `backend/server/server_frontend_embed.go` — replaces `<style`, `<link rel="stylesheet"`, and `CSP_NONCE_PLACEHOLDER` with per-request nonce
- Added `<meta name="csp-nonce" content="CSP_NONCE_PLACEHOLDER" />` to `frontend/index.html`
- Updated `frontend/src/App.vue`:
  - Reads nonce from meta tag: `document.querySelector('meta[name="csp-nonce"]')?.getAttribute('content')`
  - Passes `:csp="cspConfig"` to `<NConfigProvider>` — only activates when nonce is present and not placeholder
  - Guards against unreplaced placeholder (`CSP_NONCE_PLACEHOLDER`) — falls back to undefined (no CSP config)
