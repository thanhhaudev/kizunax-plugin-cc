#!/bin/sh
# Slash-command wrapper for kizunax. Keeps the slash-command body to a single
# shell command (no inline if/then or && chains) so it passes Claude Code's
# safety heuristic for multi-operation commands.
#
# Usage: run.sh <missing-binary-msg> <subcommand> [args...]
# The first positional arg is the user-facing message to print when the
# kizunax binary is missing. Subsequent args are forwarded to the binary.

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BIN="$SCRIPT_DIR/../bin/kizunax"

MSG="$1"
shift

if [ ! -f "$BIN" ]; then
  echo "$MSG"
  exit 1
fi

exec "$BIN" "$@"
