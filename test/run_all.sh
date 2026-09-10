#!/bin/bash
# Vesper — Master Test Suite Runner
# Ejecuta todas las fases de testing en orden
set -e
cd "$(dirname "$0")"
BOLD='\033[1m'; RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'

echo -e "${CYAN}${BOLD}"
echo "  ╔═══════════════════════════════════════╗"
echo "  ║       Vesper Complete Test Suite       ║"
echo "  ╚═══════════════════════════════════════╝"
echo -e "${NC}"

START=$(date +%s%N)
TOTAL=0; PASS=0; FAIL=0

run_phase() {
  local phase=$1; local name=$2; local script=$3
  TOTAL=$((TOTAL+1))
  echo -e "\n${CYAN}${BOLD}[${phase}]${NC} ${BOLD}${name}${NC}"
  echo -e "${CYAN}──────────────────────────────────────────${NC}"
  if bash "$script"; then
    echo -e "${GREEN}  [${phase}] PASS${NC}"
    PASS=$((PASS+1))
  else
    echo -e "${RED}  [${phase}] FAIL${NC}"
    FAIL=$((FAIL+1))
  fi
}

# ── F1: Go Core ──
run_phase "F1" "Go Tests (whole module)" "go/run_core.sh"

# ── F3: Python Bridge (sandboxed) ──
run_phase "F3" "Python Bridge" "python/run_bridge.sh"

# ── Summary ──
ELAPSED=$((($(date +%s%N) - START)/1000000))
echo -e "\n${BOLD}${CYAN}═══════════════════════════════════════════${NC}"
echo -e "${BOLD}  RESULTS: ${GREEN}${PASS} PASS${NC} / ${RED}${FAIL} FAIL${NC} / ${TOTAL} TOTAL"
echo -e "${BOLD}  ELAPSED: ${ELAPSED}ms${NC}"
echo -e "${BOLD}${CYAN}═══════════════════════════════════════════${NC}"

[ $FAIL -eq 0 ]
