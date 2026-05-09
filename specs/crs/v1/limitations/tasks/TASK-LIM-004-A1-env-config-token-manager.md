# TASK-LIM-004-A1: Runtime Env Config + Token Manager

| Field | Value |
|-------|-------|
| Solution | SOL-LIM-004 |
| Phase | A — Architecture Decoupling |
| Priority | P0 |
| Depends On | — |
| Est. | M (~180 LoC) |

## Objective

Create 3-tier environment config (Runtime > Build > Default) and JWT Bearer token manager for standalone CSR frontend deployment.

## Files

| Action | Path |
|--------|------|
| CREATE | `frontend/public/env-config.js` — runtime overrides |
| CREATE | `frontend/src/config/env.ts` — 3-tier config reader |
| CREATE | `frontend/src/auth/token-manager.ts` — JWT token lifecycle |
| MODIFY | `frontend/index.html` — add `<script src="/env-config.js">` |

## Specification

### `env-config.js` — deploy-time overridable

```javascript
window.__ENV__ = { API_URL: '', AUTH_MODE: 'cookie' };
```

### `env.ts` — config reader

```typescript
export function getEnvConfig(): { apiUrl: string; authMode: 'token' | 'cookie' } {
    const r = window.__ENV__ || {};
    return {
        apiUrl: r.API_URL || import.meta.env.VITE_API_URL || '',
        authMode: r.AUTH_MODE || 'cookie',
    };
}
```

### `token-manager.ts`

- Store access token in memory (not localStorage)
- Store refresh token in localStorage
- `getAccessToken()`: return current access token
- `createTransport(baseUrl)`: ConnectRPC transport with Bearer header interceptor
- `refreshAccessToken()`: POST to `/v1/auth/refresh`
- Auto-schedule refresh 1min before expiry (parse JWT `exp`)
- On refresh failure: clear tokens, redirect to `/auth/login`

## Acceptance Criteria

- [ ] `env-config.js` loaded before app (in index.html head)
- [ ] Config fallback chain: runtime → build-time → default
- [ ] Token stored in memory only (not localStorage for access token)
- [ ] Auto-refresh before expiry
- [ ] ConnectRPC transport injects Bearer header
