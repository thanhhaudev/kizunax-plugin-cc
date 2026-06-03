#!/usr/bin/env bash
# Rebuild tree-sitter WASM grammars for kizunax v0.12+.
#
# Approach: install official tree-sitter language packages from npm,
# copy their pre-built .wasm files into internal/symbols/grammars/.
# This avoids needing emscripten locally.
#
# Usage:
#   ./scripts/rebuild-grammars.sh
#
# Prerequisites:
#   - Node.js 18+ (for npm)
#
# Output:
#   internal/symbols/grammars/<name>.wasm (15 files)
#
# Run this script once initially, and again when:
#   - A bundled language ships a major version (e.g., Go 1.23 syntax)
#   - A tree-sitter grammar fixes a relevant bug
#   - Adding a new language to the bundle

set -euo pipefail

cd "$(dirname "$0")/.."
OUT="internal/symbols/grammars"
mkdir -p "$OUT"

GRAMMARS=(
  "tree-sitter-go:go"
  "tree-sitter-typescript:typescript"
  "tree-sitter-python:python"
  "tree-sitter-rust:rust"
  "tree-sitter-java:java"
  "tree-sitter-c-sharp:csharp"
  "tree-sitter-ruby:ruby"
  "tree-sitter-php:php"
  "tree-sitter-kotlin:kotlin"
  "tree-sitter-swift:swift"
  "tree-sitter-scala:scala"
  "tree-sitter-cpp:cpp"
  "tree-sitter-c:c"
  "tree-sitter-javascript:javascript"
)

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

cd "$TMPDIR"
npm init -y > /dev/null

echo "[rebuild-grammars] installing ${#GRAMMARS[@]} tree-sitter packages..."
for entry in "${GRAMMARS[@]}"; do
  pkg="${entry%%:*}"
  npm install --no-save "$pkg" >/dev/null 2>&1 || echo "  ! failed to install $pkg (skipping)"
done

cd - > /dev/null

count=0
for entry in "${GRAMMARS[@]}"; do
  pkg="${entry%%:*}"
  name="${entry##*:}"
  src=$(find "$TMPDIR/node_modules/$pkg" -name "*.wasm" -type f 2>/dev/null | head -n 1)
  if [[ -z "$src" ]]; then
    echo "  ! no .wasm found in $pkg (skipping)"
    continue
  fi
  cp "$src" "$OUT/$name.wasm"
  size=$(wc -c < "$OUT/$name.wasm")
  echo "  ✓ $name.wasm ($size bytes)"
  count=$((count + 1))
done

echo "[rebuild-grammars] wrote $count grammar files to $OUT/"
echo "[rebuild-grammars] next steps:"
echo "  git add internal/symbols/grammars/*.wasm"
echo "  git commit -m 'chore(grammars): refresh tree-sitter WASM bundles'"
