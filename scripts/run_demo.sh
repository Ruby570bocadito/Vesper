#!/usr/bin/env bash
# Vesper — DEMO MODE (dry-run + simulation de punta a punta)
# Alternativa cross-platform al run_simulation.bat de Windows
# Ejecuta toda la cadena ofensiva sin acciones reales.
# Uso: bash scripts/run_demo.sh [--full]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$(dirname "$SCRIPT_DIR")")"

echo "============================================================"
echo " Vesper — FULL DEMO (DRY-RUN + SIMULATION)"
echo "============================================================"
echo ""
echo " Seguridad:"
echo "  - DRY RUN: No se ejecutan exploits reales"
echo "  - SIMULATION: Solo markers, sin cifrado"
echo "  - Kill switch: EMERGENCY_STOP_SIMULATION"
echo "============================================================"
echo ""

FULL_MODE="${1:-}"

# ── 1. Compilar agentes ──────────────────────────────────────────
echo "[1/5] Compilando agentes..."
(cd "$PROJECT_ROOT" && go build -o /dev/null ./cmd/vesper/ 2>/dev/null) || echo "  [!] Go no disponible — saltando compilación"

# ── 2. Iniciar bridge Python ─────────────────────────────────────
echo "[2/5] Iniciando bridge Python (dry-run)..."
(cd "$PROJECT_ROOT/modules/bridge" && python3 -c "
import sys
sys.path.insert(0, '.')
from handlers.ransomware import handle_scan
print('[Bridge] Simulation bridge online')
print(handle_scan({'target': '127.0.0.1', 'ports': [22, 80, 443], 'simulation': True}))
" 2>&1 | head -5)

echo ""

# ── 3. Demo de escaneo ───────────────────────────────────────────
echo "[3/5] Escaneando red (simulation)..."
(cd "$PROJECT_ROOT" && python3 -c "
import sys
sys.path.insert(0, 'modules/bridge')
try:
    from handlers.ransomware import handle_scan
    result = handle_scan({'target': '192.168.1.0/24', 'ports': [22, 80, 443, 445, 3389], 'simulation': True})
    print(f'  Hosts encontrados: {result.get(\"hosts_found\", 0)}')
    print(f'  Servicios: {result.get(\"services\", {})}')
except Exception as e:
    print(f'  [!] Escaneo: {e}')
")

echo ""

# ── 4. Worm simulation ───────────────────────────────────────────
echo "[4/5] Ejecutando worm en modo simulación..."
if [ -f "$PROJECT_ROOT/plugins/worm/worm_core.py" ]; then
    (cd "$PROJECT_ROOT/plugins/worm" && python3 worm_core.py --config configs/config_simulation.yaml 2>&1 | head -10) || echo "  [!] Worm simulación terminado (esperado con dry-run)"
else
    echo "  [!] worm_core.py no encontrado"
fi

echo ""

# ── 5. Generar informe ───────────────────────────────────────────
echo "[5/5] Generando informe de misión..."
(cd "$PROJECT_ROOT" && python3 -c "
from pathlib import Path
import datetime, json
mission_dir = Path('missions/demo_$(date +%Y%m%d_%H%M%S)')
mission_dir.mkdir(parents=True, exist_ok=True)
(mission_dir / 'events.jsonl').write_text(json.dumps({
    'timestamp': datetime.datetime.now().isoformat(),
    'action': 'demo_started',
    'target': 'simulation_network'
}) + '\n')
print(f'  Misión creada: {mission_dir}')
")

echo ""
echo "============================================================"
echo " DEMO COMPLETO"
echo "============================================================"
echo ""
echo " Todos los módulos ejecutados en modo seguro."
echo " No se realizaron cambios reales en el sistema."
echo ""
