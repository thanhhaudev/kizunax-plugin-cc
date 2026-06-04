#!/usr/bin/env bash
# Kizunax setup wrapper — runs the configuration wizard outside Claude Code.
set -euo pipefail

REPO_DIR="$(cd "$(dirname "$0")" && pwd)"
BINARY="$REPO_DIR/plugins/kizunax/bin/kizunax"

if [ ! -x "$BINARY" ]; then
  echo "ERROR: $BINARY not found. Run ./install.sh first." >&2
  exit 1
fi

if [ "$#" -eq 0 ]; then
  exec "$BINARY" setup --web
else
  exec "$BINARY" setup "$@"
fi
