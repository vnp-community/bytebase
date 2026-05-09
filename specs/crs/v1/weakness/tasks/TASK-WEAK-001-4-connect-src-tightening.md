# TASK-WEAK-001-4: connect-src Tightening

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-001 |
| Priority | P1 |
| Depends On | TASK-WEAK-001-1 |
| Est. | S (~30 LoC) |

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

- [ ] Production CSP: no `ws:`, no `data:` in connect-src
- [ ] Dev CSP: `ws:` allowed for HMR/localhost
- [ ] LSP WebSocket still works via `wss:` in production
- [ ] Monaco Editor loads without data: URI (uses import() instead)
