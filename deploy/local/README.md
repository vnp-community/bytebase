# Bytebase Local Deployment

## Quick Start

```bash
cd deploy/local

# Build backend + start containers
./build-and-run.sh

# Then start frontend dev server (separate terminal)
./build-and-run.sh frontend
```

## Architecture

```
Host Machine                          Docker
┌──────────────────┐     ┌──────────────────────────────────┐
│  Frontend (Vite) │────▶│  backend (debian:bookworm-slim)  │
│  localhost:3000   │     │  Binary: ./build/bytebase (ro)  │
└──────────────────┘     │  Port: 8080                      │
                         └──────────┬───────────────────────┘
                                    │
                         ┌──────────▼───────────────────────┐
                         │  postgres (postgres:16-alpine)   │
                         │  DB: bbdev  User: bbdev          │
                         │  Port: 5432                      │
                         └──────────────────────────────────┘
```

## How It Works

1. **Build**: Uses a `golang:1.24-bookworm` Docker container to cross-compile the Go backend
   - `CGO_ENABLED=1` (required by `go-sqlite3` plugin)
   - Go module & build caches stored in Docker volumes for fast rebuilds
   - Output: `./build/bytebase` (Linux binary)
2. **Mount**: Binary mounted read-only into `gcr.io/distroless/base-debian12:nonroot`
   - **No shell, no package manager** → minimal attack surface
   - Runs as **non-root** (UID 65534) by default
   - Includes glibc (needed for CGO/go-sqlite3)
3. **Metadata DB**: PostgreSQL 16 stores Bytebase metadata
4. **Frontend Dev**: `pnpm dev` on host, Vite proxies API calls to `http://localhost:8080`

> Binary NOT built inside Dockerfile → avoids Docker image bloat and Go module duplication

## Commands

| Command                    | Description                              |
|----------------------------|-----------------------------------------|
| `./build-and-run.sh`      | Build + start all                       |
| `./build-and-run.sh build`| Cross-compile backend binary only       |
| `./build-and-run.sh up`   | Start containers (binary must exist)    |
| `./build-and-run.sh down` | Stop containers                         |
| `./build-and-run.sh clean`| Stop + delete volumes + binary          |
| `./build-and-run.sh logs` | Tail backend container logs             |
| `./build-and-run.sh frontend` | Start Vite frontend dev server     |

## Endpoints

| Service     | URL                    | Credentials    |
|-------------|------------------------|----------------|
| Backend API | http://localhost:8080   | Setup wizard   |
| Frontend    | http://localhost:3000   | (proxies API)  |
| PostgreSQL  | localhost:5432         | bbdev / bbdev  |

## Customization

```bash
BB_PORT=9090 PG_PORT=5433 ./build-and-run.sh
```

## Files

```
deploy/local/
├── docker-compose.yml   # PostgreSQL + backend container
├── build-and-run.sh     # Build + orchestration script
├── bb.env               # Docker marker (isDocker() detection)
├── README.md            # This file
└── .gitignore           # Excludes build/
```
