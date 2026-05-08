# Bytebase Dev Deployment

## Network Topology

```
                    Internet
                       │
                       ▼
              b10.openledger.vn
                       │
        ┌──────────────▼──────────────┐
        │  Nginx Reverse Proxy         │
        │  103.67.184.32 (ext)         │
        │  172.20.2.16   (int)         │
        │  Port 443 (SSL) → proxy_pass│
        └──────────────┬──────────────┘
                       │ :8080
        ┌──────────────▼──────────────┐
        │  Bytebase App Server         │
        │  172.20.2.41                 │
        │  ┌────────────────────────┐  │
        │  │ bb-dev-backend  :8080  │  │
        │  │ (distroless/nonroot)   │  │
        │  │  ↕                     │  │
        │  │ bb-dev-postgres :5432  │  │
        │  │ (postgres:16-alpine)   │  │
        │  └────────────────────────┘  │
        └──────────────────────────────┘
```

## Quick Start

```bash
cd deploy/dev

# Full deploy: build → push → start
./deploy.sh

# Install nginx config on proxy server (first time only)
./deploy.sh nginx-install
```

## Commands

| Command                    | Description                                    |
|----------------------------|------------------------------------------------|
| `./deploy.sh`              | Build + push + start (full deploy)             |
| `./deploy.sh build`        | Cross-compile backend binary only              |
| `./deploy.sh push`         | rsync binary + configs to 172.20.2.41          |
| `./deploy.sh start`        | Start containers on remote server              |
| `./deploy.sh stop`         | Stop containers on remote server               |
| `./deploy.sh restart`      | Restart containers on remote server             |
| `./deploy.sh logs`         | Tail backend logs (remote)                     |
| `./deploy.sh status`       | Show container status (remote)                 |
| `./deploy.sh nginx-install`| Install nginx config on proxy server           |

## Endpoints

| Service     | URL                           | From            |
|-------------|-------------------------------|-----------------|
| Bytebase    | https://b10.openledger.vn     | Internet        |
| Backend API | http://172.20.2.41:18081      | Internal only   |
| PostgreSQL  | 172.20.2.41:15432 (localhost) | Server only     |

## Prerequisites

1. **SSH access** to both servers:
   ```bash
   ssh root@172.20.2.41    # Bytebase app server
   ssh root@172.20.2.16    # Nginx proxy server
   ```

2. **Docker + Docker Compose** on 172.20.2.41

3. **Nginx** on 103.67.184.32 with:
   - SSL certificates at `/etc/nginx/ssl/b10.openledger.vn/`
   - `sites-available/sites-enabled` directory structure

4. **DNS**: `b10.openledger.vn` → `103.67.184.32`

## Configuration

Override defaults via environment variables:

```bash
BB_SERVER=172.20.2.41 BB_USER=deploy ./deploy.sh
```

| Variable       | Default         | Description                |
|----------------|-----------------|----------------------------|
| `BB_SERVER`    | 172.20.2.41     | Bytebase server IP         |
| `BB_USER`      | root            | SSH user for app server    |
| `BB_DEPLOY_DIR`| /opt/bytebase   | Remote deploy directory    |
| `NGINX_SERVER` | 172.20.2.16     | Nginx proxy server IP      |
| `NGINX_USER`   | root            | SSH user for proxy server  |

## Files

```
deploy/dev/
├── docker-compose.yml                  # Backend + PostgreSQL
├── deploy.sh                           # Build + deploy orchestration
├── bb.env                              # Docker marker file
├── nginx/
│   └── b10.openledger.vn.conf          # Nginx reverse proxy config
├── .gitignore
└── README.md
```

## Nginx Special Routes

The nginx config handles these Bytebase-specific protocols:

| Path               | Protocol       | Notes                          |
|--------------------|----------------|--------------------------------|
| `/`                | HTTP           | Main web UI + REST/ConnectRPC  |
| `/v1:adminExecute` | **WebSocket**  | Streaming SQL execution        |
| `/lsp`             | **WebSocket**  | Language Server Protocol       |
| `/mcp/`            | **SSE**        | Model Context Protocol (AI)    |
| `/healthz`         | HTTP           | Health check (no access log)   |
