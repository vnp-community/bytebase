# TASK-LIM-004-A3: Nginx Config + Docker

| Field | Value |
|-------|-------|
| Solution | SOL-LIM-004 |
| Phase | A — Deployment |
| Priority | P0 |
| Depends On | TASK-LIM-004-A1, TASK-LIM-004-A2 |
| Est. | S (~80 LoC) |

## Objective

Create Nginx config for SPA routing and Docker setup for standalone frontend deployment.

## Files

| Action | Path |
|--------|------|
| CREATE | `deploy/nginx/bytebase-frontend.conf` |
| CREATE | `deploy/docker/Dockerfile.frontend` |

## Specification

### Nginx config

- `location /` → `try_files $uri $uri/ /index.html` (SPA routing)
- `location /assets/` → `expires 1y; Cache-Control: public, immutable`
- `location /bytebase.v1.` → proxy to backend (ConnectRPC)
- `location /v1/` → proxy to backend (REST gateway)
- `location /lsp` → WebSocket proxy (Upgrade headers)
- `location /mcp/` → SSE proxy (no buffering)

### Dockerfile

```dockerfile
FROM node:20-alpine AS build
WORKDIR /app
COPY frontend/ .
RUN npm ci && npm run build

FROM nginx:alpine
COPY --from=build /app/dist /usr/share/nginx/html
COPY deploy/nginx/bytebase-frontend.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
```

## Acceptance Criteria

- [x] SPA routing works (deep links serve index.html) → **DONE**: `try_files $uri $uri/ /index.html` in `location /`
- [x] Static assets cached with immutable headers → **DONE**: `location /assets/` with `expires 1y; Cache-Control: public, immutable`
- [x] API proxy routes to backend correctly → **DONE**: 5 proxy locations: ConnectRPC, REST, LSP, MCP, webhook
- [x] WebSocket upgrade works for LSP → **DONE**: `proxy_set_header Upgrade $http_upgrade; Connection "upgrade"` in `/lsp`
- [x] Docker image builds and serves frontend → **DONE**: Multi-stage Dockerfile (node:20-alpine build → nginx:1.27-alpine serve)

## Implementation Notes

- Created `deploy/nginx/bytebase-frontend.conf`:
  - SPA fallback: `try_files $uri $uri/ /index.html`
  - 5 reverse proxy locations: `/bytebase.v1.`, `/v1/`, `/lsp`, `/mcp/`, `/hook/`
  - LSP WebSocket upgrade with 86400s read timeout
  - MCP SSE with buffering disabled
  - Health check endpoint at `/nginx-health`
- Created `deploy/docker/Dockerfile.frontend`:
  - Stage 1: `node:20-alpine` + pnpm install + build
  - Stage 2: `nginx:1.27-alpine` serving built assets
  - HEALTHCHECK via wget to `/nginx-health`
  - Runtime env-config.js override via volume mount

**Status: ✅ DONE**
