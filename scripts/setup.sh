#!/usr/bin/env bash
# Vesper — Automated Setup Script
# =================================
# Installs all dependencies and prepares the environment.
# Usage: bash scripts/setup.sh [--dev] [--docker]

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

info()  { echo -e "${BLUE}[*]${NC} $1"; }
ok()    { echo -e "${GREEN}[+]${NC} $1"; }
warn()  { echo -e "${YELLOW}[!]${NC} $1"; }
err()   { echo -e "${RED}[x]${NC} $1"; exit 1; }

DEV_MODE=false
DOCKER_MODE=false

for arg in "$@"; do
    case $arg in
        --dev) DEV_MODE=true ;;
        --docker) DOCKER_MODE=true ;;
    esac
done

echo ""
  echo "  ╔══════════════════════════════════════════════╗"
  echo "  ║     Vesper — Setup                          ║"
  echo "  ╚══════════════════════════════════════════════╝"
echo ""

# --- Go ---
if command -v go &> /dev/null; then
    info "Go version: $(go version)"
    info "Installing Go dependencies..."
    for mod in core/crypto core/agent core/orchestrator shared/config shared/logger shared/types; do
        if [ -f "$mod/go.mod" ]; then
            (cd "$mod" && go mod tidy) || warn "Failed to tidy $mod"
        fi
    done
    ok "Go setup complete"
else
    warn "Go not found — install from https://go.dev/dl/"
fi

# --- Python ---
if command -v python3 &> /dev/null; then
    info "Python version: $(python3 --version)"

    if [ ! -d ".venv" ]; then
        info "Creating virtual environment..."
        python3 -m venv .venv
    fi

    source .venv/bin/activate 2>/dev/null || true
    pip install --upgrade pip -q

    if [ -f "modules/requirements.txt" ]; then
        pip install -r modules/requirements.txt -q
    fi
    ok "Python setup complete"
else
    warn "Python3 not found"
fi

# --- Node.js ---
if command -v node &> /dev/null; then
    info "Node.js version: $(node --version)"
    if [ -d "core/c2/web" ]; then
        (cd core/c2/web && npm install) || warn "Failed to install Node deps"
    fi
    ok "Node.js setup complete"
else
    warn "Node.js not found — install from https://nodejs.org/"
fi

# --- Protocol Buffers ---
if command -v buf &> /dev/null; then
    info "Buf version: $(buf --version)"
elif command -v protoc &> /dev/null; then
    info "Protoc version: $(protoc --version)"
else
    warn "Protocol buffer compiler not found — install buf or protoc"
fi

# --- Docker ---
if command -v docker &> /dev/null; then
    info "Docker version: $(docker --version)"
    if $DOCKER_MODE; then
        info "Building Docker lab images..."
        docker compose -f lab/docker-compose.yml build
    fi
    ok "Docker ready"
else
    warn "Docker not found — install from https://docker.com/"
fi

# --- Config ---
if [ ! -f "config.yaml" ]; then
    cp config.yaml config.yaml.bak 2>/dev/null || true
    info "config.yaml created from defaults"
fi

echo ""
ok "Vesper setup complete!"
echo ""
echo "  Next steps:"
echo "    make lab-up    → Start the Docker lab environment"
echo "    make build     → Build all components"
echo "    make test      → Run tests"
echo ""
