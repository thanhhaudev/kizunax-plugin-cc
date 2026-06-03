#!/usr/bin/env bash
# Fetch tree-sitter grammar .wasm fixtures used by treesitter package tests.
# Not embedded in the binary; not committed to git (large + license question).
set -euo pipefail
cd "$(dirname "$0")"

PHP_VER="0.24.2"
PHP_PKG="tree-sitter-php"

if [[ ! -f "$PHP_PKG.wasm" ]]; then
  echo "fetching $PHP_PKG@$PHP_VER..."
  TARBALL_URL=$(curl -sSL "https://registry.npmjs.org/$PHP_PKG/$PHP_VER" | python3 -c "import json,sys; print(json.load(sys.stdin)['dist']['tarball'])")
  curl -sSL "$TARBALL_URL" | tar -xz -C . package/tree-sitter-php.wasm
  mv package/tree-sitter-php.wasm "$PHP_PKG.wasm"
  rm -rf package
  echo "wrote $PHP_PKG.wasm ($(wc -c < "$PHP_PKG.wasm") bytes)"
fi

TS_VER="0.23.2"
TS_PKG="tree-sitter-typescript"
if [[ ! -f "$TS_PKG.wasm" ]]; then
  echo "fetching $TS_PKG@$TS_VER..."
  curl -sSL -o "$TS_PKG.wasm" "https://unpkg.com/$TS_PKG@$TS_VER/tree-sitter-typescript.wasm"
  echo "wrote $TS_PKG.wasm ($(wc -c < "$TS_PKG.wasm") bytes)"
fi
