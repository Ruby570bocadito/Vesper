#!/bin/bash
echo "=== Vesper Obfuscated Build ==="
SEED=$(date +%s)
echo "Seed: $SEED"
if command -v garble &>/dev/null; then
    garble -literals -tiny -seed=random build -ldflags="-s -w" -o vesper_obfuscated ./cmd/vesper/ 2>&1 && echo "OK: vesper_obfuscated"
else
    go build -ldflags="-s -w" -o vesper_obfuscated ./cmd/vesper/ && echo "OK (go build fallback): vesper_obfuscated"
fi
ls -la vesper_obfuscated 2>/dev/null
