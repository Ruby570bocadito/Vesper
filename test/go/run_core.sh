#!/usr/bin/env bash
# Vesper — Go core test runner (single module, whole tree)
set -e
cd "$(dirname "$0")/../.."
BOLD='\033[1m'; RED='\033[38;5;203m'; GREEN='\033[38;5;42m'; YELLOW='\033[38;5;214m'; NC='\033[0m'
echo -e "${BOLD}========================================${NC}"
echo -e "${BOLD}  Vesper — Go Core Tests${NC}"
echo -e "${BOLD}========================================${NC}"

go test ./cmd/... ./internal/... ./pkg/... ./plugins/... -cover -timeout 180s
STATUS=$?
[ $STATUS -eq 0 ] && echo -e "${GREEN}  ALL GO TESTS PASSED${NC}" || echo -e "${RED}  GO TESTS FAILED${NC}"
exit $STATUS
