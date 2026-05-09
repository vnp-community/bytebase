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

- [ ] SPA routing works (deep links serve index.html)
- [ ] Static assets cached with immutable headers
- [ ] API proxy routes to backend correctly
- [ ] WebSocket upgrade works for LSP
- [ ] Docker image builds and serves frontend
