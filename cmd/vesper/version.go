package main

// version.go — single source of truth for the release version.
//
// Kept out of config on purpose: `vesper --version` must never touch
// disk, so the fast path in main.go can answer with zero side effects.

// version is the current release tag (without the leading "v").
const version = "1.0.0"
