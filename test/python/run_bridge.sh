#!/bin/bash
# Vesper — Python bridge test runner (sandboxed)
set -e
cd "$(dirname "$0")/../.."
BOLD='\033[1m'; RED='\033[38;5;203m'; GREEN='\033[38;5;42m'; YELLOW='\033[38;5;214m'; NC='\033[0m'
echo -e "${BOLD}========================================${NC}"
echo -e "${BOLD}  Vesper — Python Bridge Tests${NC}"
echo -e "${BOLD}========================================${NC}"

if ! command -v python3 &>/dev/null; then
  echo -e "${RED}python3 not found${NC}"
  exit 1
fi

cd modules/bridge
python3 -m pytest tests/ -q --tb=short
STATUS=$?
[ $STATUS -eq 0 ] && echo -e "${GREEN}  BRIDGE TESTS PASSED${NC}" || echo -e "${RED}  BRIDGE TESTS FAILED${NC}"
exit $STATUS
