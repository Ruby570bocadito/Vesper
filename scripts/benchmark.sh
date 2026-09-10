#!/usr/bin/env bash
# Vesper — Benchmark Script
# ========================
# Runs a suite of benchmarks against the running API server.
# Usage: bash scripts/benchmark.sh [host] [port]

set -euo pipefail

HOST="${1:-localhost}"
PORT="${2:-8445}"
BASE="http://${HOST}:${PORT}"

GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

bench() {
    local name="$1"
    local url="$2"
    local iterations="${3:-50}"

    printf "${BLUE}[BENCH]${NC} %-40s " "$name"
    total=0
    for i in $(seq 1 $iterations); do
        t=$(curl -o /dev/null -s -w '%{time_total}' "$url" 2>/dev/null || echo "0.5")
        total=$(echo "$total + $t" | bc 2>/dev/null || echo "$total")
    done
    avg=$(echo "scale=4; $total / $iterations * 1000" | bc 2>/dev/null || echo "0")
    printf "${GREEN}%sms avg (%d reqs)${NC}\n" "$avg" "$iterations"
}

echo ""
echo "  ╔══════════════════════════════════════════════════╗"
echo "  ║     Vesper — Benchmark Suite                     ║"
echo "  ╚══════════════════════════════════════════════════╝"
echo ""

echo "API Server: $BASE"
echo ""

# API Benchmarks
bench "Health Check"              "$BASE/api/health"          100
bench "List Agents"               "$BASE/api/agents"          50
bench "List Hosts"                "$BASE/api/hosts"           50
bench "Get Metrics"               "$BASE/api/metrics"         50
bench "List Vulnerabilities"      "$BASE/api/vulnerabilities" 50
bench "AI Chat"                   "$BASE/api/ai/chat"         10
bench "Get Decisions"             "$BASE/api/decisions"       20

echo ""
echo "Benchmarks complete. Run './vesper dashboard' first to start the API."
