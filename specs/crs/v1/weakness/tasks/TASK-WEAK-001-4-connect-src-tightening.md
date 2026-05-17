# TASK-WEAK-001-4: connect-src Tightening

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-001 |
| Priority | P1 |
| Depends On | TASK-WEAK-001-1 |
| Est. | S (~30 LoC) |
| Status | ✅ Done |
| Completed | 2026-05-10 |

## Objective

Remove `ws:` (unencrypted WebSocket) and `data:` from CSP `connect-src` directive. Force encrypted `wss:` only in production; keep `ws:` in dev mode.

## Files

| Action | Path |
|--------|------|
| MODIFY | `backend/server/csp.go` — `buildCSP()` and `buildCSPDev()` |

## Specification

### Production CSP

```go
"connect-src 'self' wss: https://api.github.com https://hub.bytebase.com"
// Removed: data:, ws:
```

### Dev CSP

```go
func buildCSPDev(nonce string, scriptHashes []string) string {
    // Same as buildCSP but includes ws: for localhost dev server
    "connect-src 'self' ws: wss: https://api.github.com https://hub.bytebase.com"
}
```

Selection: `if profile.Mode == ReleaseModeDev { buildCSPDev() } else { buildCSP() }`

## Acceptance Criteria

- [x] Production CSP: no `ws:`, no `data:` in connect-src
- [x] Dev CSP: `ws:` allowed for HMR/localhost
- [x] LSP WebSocket still works via `wss:` in production
- [x] Monaco Editor loads without data: URI (uses import() instead)

## Implementation Notes

- Implemented directly in `backend/server/csp.go` alongside TASK-WEAK-001-1:
  - `buildCSP()` — production: `connect-src 'self' wss: https://api.github.com https://hub.bytebase.com` (no `ws:`, no `data:`)
  - `buildCSPDev()` — dev: `connect-src 'self' ws: wss: https://api.github.com https://hub.bytebase.com` (includes `ws:` for HMR)
  - `buildCSPLegacy()` — preserves original `data: ws: wss:` for backward compatibility when nonce disabled
- Dev/prod mode selection in `newSecurityHeadersMiddleware()` via `profile.Mode == common.ReleaseModeDev`
- Unit test `TestBuildCSP/production_CSP_has_no_ws:_in_connect-src` explicitly verifies removal of `ws:` and `data:` from production CSP
