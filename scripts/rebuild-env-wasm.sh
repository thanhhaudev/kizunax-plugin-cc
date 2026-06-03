#!/usr/bin/env bash
# Rebuild env.wasm + got_mem.wasm from the WAT sources committed alongside.
#
# Run after editing WAT sources. Output is committed; not needed at runtime.
#
# Prerequisites: wabt (https://github.com/WebAssembly/wabt).
#   macOS: brew install wabt
#   Linux: apt-get install wabt   (or build from source)
#
# Usage: ./scripts/rebuild-env-wasm.sh

set -euo pipefail
cd "$(dirname "$0")/.."

SRC_DIR="internal/symbols/treesitter/wat"
OUT_DIR="internal/symbols/treesitter/embed_assets"
mkdir -p "$OUT_DIR"

if ! command -v wat2wasm >/dev/null; then
  echo "wat2wasm not found. Install wabt: brew install wabt (macOS) / apt-get install wabt (Linux)"
  exit 1
fi

for name in env got_mem; do
  wat2wasm "$SRC_DIR/$name.wat" -o "$OUT_DIR/$name.wasm"
  size=$(wc -c < "$OUT_DIR/$name.wasm")
  echo "  ✓ $name.wasm ($size bytes)"
done
