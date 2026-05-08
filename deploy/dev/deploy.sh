#!/usr/bin/env bash
#
# deploy.sh — Build, transfer, and deploy Bytebase to dev server
#
# Usage:
#   ./deploy.sh              # Build + deploy + start
#   ./deploy.sh build        # Build Linux binary only
#   ./deploy.sh push         # Transfer files to server only
#   ./deploy.sh start        # Start containers on server (remote)
#   ./deploy.sh stop         # Stop containers on server (remote)
#   ./deploy.sh restart      # Restart containers on server (remote)
#   ./deploy.sh logs         # Tail backend logs on server (remote)
#   ./deploy.sh status       # Show container status (remote)
#   ./deploy.sh nginx-install # Install nginx config on proxy server
#
set -euo pipefail

# ─────────────────────────────────────────────────────────────
# Configuration
# ─────────────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
BUILD_DIR="$SCRIPT_DIR/build"
BINARY="$BUILD_DIR/bytebase"

# Remote servers
BB_SERVER="${BB_SERVER:-172.20.2.41}"
BB_USER="${BB_USER:-ubuntu}"
BB_DEPLOY_DIR="${BB_DEPLOY_DIR:-/opt/bytebase}"

NGINX_SERVER="${NGINX_SERVER:-172.20.2.16}"
NGINX_USER="${NGINX_USER:-ubuntu}"

# ─────────────────────────────────────────────────────────────
# Build frontend (Vite → backend/server/dist/)
# ─────────────────────────────────────────────────────────────
build_frontend() {
    echo "══════════════════════════════════════════════════════════"
    echo "  Building Bytebase frontend ..."
    echo "══════════════════════════════════════════════════════════"

    cd "$PROJECT_ROOT/frontend"

    # Install deps if needed
    if [ ! -d "node_modules" ]; then
        echo "  → Installing dependencies ..."
        pnpm install
    fi

    # Build for release — output goes to ../backend/server/dist/
    pnpm release

    echo "✅ Frontend built → backend/server/dist/"
}

# ─────────────────────────────────────────────────────────────
# Build backend binary for Linux (with embedded frontend)
# CGO_ENABLED=0 → static binary, no C cross-compiler needed
# -tags embed_frontend → embeds frontend/dist into the binary
# ─────────────────────────────────────────────────────────────
build_backend() {
    echo "══════════════════════════════════════════════════════════"
    echo "  Building Bytebase backend for linux/amd64 ..."
    echo "  (with embedded frontend)"
    echo "══════════════════════════════════════════════════════════"

    # Ensure frontend was built
    if [ ! -d "$PROJECT_ROOT/backend/server/dist" ]; then
        echo "❌ Frontend dist not found. Run: $0 build-frontend"
        exit 1
    fi

    mkdir -p "$BUILD_DIR"

    cd "$PROJECT_ROOT"
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
        go build \
        -tags "embed_frontend" \
        -ldflags "-w -s" \
        -o "$BINARY" \
        ./backend/bin/server/main.go

    chmod +x "$BINARY"

    local size
    size=$(du -h "$BINARY" | cut -f1)
    echo "✅ Binary built: $BINARY ($size)"
}

# ─────────────────────────────────────────────────────────────
# Push files to dev server
# ─────────────────────────────────────────────────────────────
push_to_server() {
    if [ ! -f "$BINARY" ]; then
        echo "❌ Binary not found. Run: $0 build"
        exit 1
    fi

    echo "══════════════════════════════════════════════════════════"
    echo "  Transferring to $BB_USER@$BB_SERVER:$BB_DEPLOY_DIR ..."
    echo "══════════════════════════════════════════════════════════"

    # Create deploy dir on remote (needs sudo for /opt)
    ssh "$BB_USER@$BB_SERVER" "sudo mkdir -p $BB_DEPLOY_DIR/build && sudo chown -R \$(id -u):\$(id -g) $BB_DEPLOY_DIR"

    # Transfer binary
    echo "  → Uploading binary ..."
    rsync -avz --progress "$BINARY" "$BB_USER@$BB_SERVER:$BB_DEPLOY_DIR/build/bytebase"

    # Transfer compose + config files
    echo "  → Uploading config files ..."
    rsync -avz \
        "$SCRIPT_DIR/docker-compose.yml" \
        "$SCRIPT_DIR/bb.env" \
        "$BB_USER@$BB_SERVER:$BB_DEPLOY_DIR/"

    # Ensure binary is executable
    ssh "$BB_USER@$BB_SERVER" "chmod +x $BB_DEPLOY_DIR/build/bytebase"

    echo "✅ Files transferred."
}

# ─────────────────────────────────────────────────────────────
# Remote Docker Compose commands
# ─────────────────────────────────────────────────────────────
remote_exec() {
    ssh "$BB_USER@$BB_SERVER" "cd $BB_DEPLOY_DIR && $*"
}

cmd_start() {
    echo "Starting containers on $BB_SERVER ..."
    remote_exec "docker compose up -d"
    echo "✅ Bytebase started: https://b10.openledger.vn"
}

cmd_stop() {
    echo "Stopping containers on $BB_SERVER ..."
    remote_exec "docker compose down"
    echo "✅ Stopped."
}

cmd_restart() {
    echo "Restarting containers on $BB_SERVER ..."
    remote_exec "docker compose restart"
    echo "✅ Restarted."
}

cmd_logs() {
    ssh -t "$BB_USER@$BB_SERVER" "cd $BB_DEPLOY_DIR && docker compose logs -f backend"
}

cmd_status() {
    remote_exec "docker compose ps"
}

# ─────────────────────────────────────────────────────────────
# Install nginx config on proxy server
# The nginx runs inside container "ms-nginx-proxy"
# Config dir: /home/ubuntu/vnp-qa-platform/proxy/conf.d/
# ─────────────────────────────────────────────────────────────
nginx_install() {
    echo "══════════════════════════════════════════════════════════"
    echo "  Installing nginx config on $NGINX_SERVER ..."
    echo "══════════════════════════════════════════════════════════"

    local NGINX_CONF="$SCRIPT_DIR/nginx/b10.openledger.vn.conf"
    local REMOTE_CONF_DIR="/home/ubuntu/vnp-qa-platform/proxy/conf.d"

    if [ ! -f "$NGINX_CONF" ]; then
        echo "❌ Nginx config not found: $NGINX_CONF"
        exit 1
    fi

    # Upload config to mounted conf.d directory
    scp "$NGINX_CONF" "$NGINX_USER@$NGINX_SERVER:$REMOTE_CONF_DIR/b10-openledger-nginx.conf"

    # Test + reload nginx inside container
    ssh "$NGINX_USER@$NGINX_SERVER" '
        sudo docker exec ms-nginx-proxy nginx -t && \
        sudo docker exec ms-nginx-proxy nginx -s reload && \
        echo "✅ Nginx reloaded." || \
        echo "❌ Nginx config test failed!"
    '
}

# ─────────────────────────────────────────────────────────────
# Main
# ─────────────────────────────────────────────────────────────
CMD="${1:-all}"

case "$CMD" in
    build-frontend) build_frontend ;;
    build)          build_frontend; build_backend ;;
    build-backend)  build_backend ;;
    push)           push_to_server ;;
    start)          cmd_start ;;
    stop)           cmd_stop ;;
    restart)        cmd_restart ;;
    logs)           cmd_logs ;;
    status)         cmd_status ;;
    nginx-install)  nginx_install ;;
    all|"")
        build_frontend
        build_backend
        push_to_server
        cmd_start
        ;;
    *)
        echo "Usage: $0 {build|build-frontend|build-backend|push|start|stop|restart|logs|status|nginx-install|all}"
        exit 1
        ;;
esac
