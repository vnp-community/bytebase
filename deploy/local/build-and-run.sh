#!/usr/bin/env bash
#
# build-and-run.sh — Compile Bytebase for Linux, then start docker-compose
#
# Usage:
#   ./build-and-run.sh          # Build + start all
#   ./build-and-run.sh build    # Build binary only
#   ./build-and-run.sh up       # Start containers only (binary must exist)
#   ./build-and-run.sh down     # Stop and remove containers
#   ./build-and-run.sh clean    # Stop containers + delete volumes + binary
#   ./build-and-run.sh frontend # Start frontend dev server (connects to backend)
#   ./build-and-run.sh logs     # Tail backend logs
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
BUILD_DIR="$SCRIPT_DIR/build"
BINARY="$BUILD_DIR/bytebase"

# ─────────────────────────────────────────────────────────────
# Build backend binary for Linux using Docker builder
# (Needed because go-sqlite3 requires CGO and C cross-compiler)
# ─────────────────────────────────────────────────────────────
build_backend() {
    echo "══════════════════════════════════════════════════════════"
    echo "  Building Bytebase backend for Linux (via Docker) ..."
    echo "══════════════════════════════════════════════════════════"

    mkdir -p "$BUILD_DIR"

    # Use a Go builder container to compile natively for Linux
    # This handles CGO cross-compilation (needed for go-sqlite3)
    docker run --rm \
        -v "$PROJECT_ROOT":/src \
        -v "$BUILD_DIR":/out \
        -v bytebase-gomod-cache:/go/pkg/mod \
        -v bytebase-gobuild-cache:/root/.cache/go-build \
        -w /src \
        -e CGO_ENABLED=1 \
        -e GOOS=linux \
        golang:1.24-bookworm \
        bash -c '
            set -e
            echo "Installing C dependencies ..."
            apt-get update -qq && apt-get install -y -qq gcc > /dev/null 2>&1
            echo "Compiling ..."
            go build -ldflags "-w -s" -p=16 -o /out/bytebase ./backend/bin/server/main.go
            chmod +x /out/bytebase
            echo "Done!"
        '

    local size
    size=$(du -h "$BINARY" | cut -f1)
    echo "✅ Binary built: $BINARY ($size)"
}

# ─────────────────────────────────────────────────────────────
# Docker Compose helpers
# ─────────────────────────────────────────────────────────────
compose_up() {
    if [ ! -f "$BINARY" ]; then
        echo "❌ Binary not found at $BINARY"
        echo "   Run: $0 build"
        exit 1
    fi

    echo "══════════════════════════════════════════════════════════"
    echo "  Starting Bytebase stack ..."
    echo "══════════════════════════════════════════════════════════"

    cd "$SCRIPT_DIR"
    docker compose up -d

    echo ""
    echo "✅ Bytebase backend:  http://localhost:${BB_PORT:-8080}"
    echo "   PostgreSQL:        localhost:${PG_PORT:-5432} (bbdev/bbdev)"
    echo ""
    echo "To start frontend dev server:"
    echo "   $0 frontend"
    echo ""
    echo "Logs: $0 logs"
}

compose_down() {
    cd "$SCRIPT_DIR"
    docker compose down
    echo "✅ Containers stopped."
}

compose_clean() {
    cd "$SCRIPT_DIR"
    docker compose down -v
    rm -rf "$BUILD_DIR"
    # Optionally remove build caches too
    # docker volume rm bytebase-gomod-cache bytebase-gobuild-cache 2>/dev/null || true
    echo "✅ Containers stopped, volumes and binary deleted."
}

compose_logs() {
    cd "$SCRIPT_DIR"
    docker compose logs -f backend
}

# ─────────────────────────────────────────────────────────────
# Frontend dev server
# ─────────────────────────────────────────────────────────────
start_frontend() {
    echo "══════════════════════════════════════════════════════════"
    echo "  Starting frontend dev server ..."
    echo "  Backend API:  http://localhost:${BB_PORT:-8080}"
    echo "══════════════════════════════════════════════════════════"

    cd "$PROJECT_ROOT/frontend"

    # Install deps if needed
    if [ ! -d "node_modules" ]; then
        echo "Installing frontend dependencies ..."
        pnpm install
    fi

    # Start Vite dev server (uses .env.dev-local → BB_GRPC_LOCAL=http://localhost:8080)
    pnpm dev
}

# ─────────────────────────────────────────────────────────────
# Main
# ─────────────────────────────────────────────────────────────
CMD="${1:-all}"

case "$CMD" in
    build)
        build_backend
        ;;
    up)
        compose_up
        ;;
    down)
        compose_down
        ;;
    clean)
        compose_clean
        ;;
    logs)
        compose_logs
        ;;
    frontend)
        start_frontend
        ;;
    all|"")
        build_backend
        compose_up
        ;;
    *)
        echo "Usage: $0 {build|up|down|clean|logs|frontend|all}"
        exit 1
        ;;
esac
