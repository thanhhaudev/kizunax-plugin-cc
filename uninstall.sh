#!/usr/bin/env bash
# Kizunax uninstaller
set -euo pipefail

REPO_DIR="$(cd "$(dirname "$0")" && pwd)"
SETTINGS="$HOME/.claude/settings.json"
BINARY="$REPO_DIR/plugins/kizunax/bin/kizunax"
BACKUP_DIR="$HOME/.claude/.kizunax-backups"

BACKED_UP_SETTINGS=""
JSON_TOOL=""

err()  { echo "ERROR: $*" >&2; }
info() { echo "==> $*"; }

detect_json_tool() {
  if command -v jq >/dev/null 2>&1; then echo "jq"
  elif command -v python3 >/dev/null 2>&1; then echo "python3"
  else err "Need jq or python3."; exit 1
  fi
}

preflight() {
  JSON_TOOL="$(detect_json_tool)"
  if [ -f "$SETTINGS" ]; then
    if [ "$JSON_TOOL" = "jq" ]; then
      jq empty "$SETTINGS" >/dev/null 2>&1 || { err "$SETTINGS is not valid JSON."; exit 1; }
    else
      python3 -c "import json,sys; json.load(open(sys.argv[1]))" "$SETTINGS" 2>/dev/null \
        || { err "$SETTINGS is not valid JSON."; exit 1; }
    fi
  fi
  mkdir -p "$BACKUP_DIR"
}

TIMESTAMP="$(date +%Y%m%d-%H%M%S)"

backup() {
  if [ -f "$SETTINGS" ]; then
    local dest="$BACKUP_DIR/settings.json.$TIMESTAMP"
    cp "$SETTINGS" "$dest"
    BACKED_UP_SETTINGS="$dest"
    info "Backed up settings → $BACKED_UP_SETTINGS"
  fi
}

rollback() {
  local line="${1:-?}"
  err "Aborted at line $line - reverting settings..."
  if [ -n "$BACKED_UP_SETTINGS" ] && [ -f "$BACKED_UP_SETTINGS" ]; then
    cp "$BACKED_UP_SETTINGS" "$SETTINGS" \
      && info "Restored $SETTINGS" \
      || err "FAILED to restore - original at $BACKED_UP_SETTINGS"
  fi
  rm -f "$SETTINGS.tmp"
  exit 1
}

patch_settings_jq() {
  local tmp="$SETTINGS.tmp"
  jq '
    if .enabledPlugins then .enabledPlugins |= del(.["kizunax@kizunax-local"]) else . end
    | if .extraKnownMarketplaces then .extraKnownMarketplaces |= del(.["kizunax-local"]) else . end
  ' "$SETTINGS" > "$tmp"
  jq empty "$tmp" >/dev/null
  mv "$tmp" "$SETTINGS"
}

patch_settings_python() {
  local tmp="$SETTINGS.tmp"
  python3 - "$SETTINGS" "$tmp" <<'PY'
import json, sys
src, dst = sys.argv[1], sys.argv[2]
with open(src) as f:
    data = json.load(f)
if "enabledPlugins" in data and isinstance(data["enabledPlugins"], dict):
    data["enabledPlugins"].pop("kizunax@kizunax-local", None)
if "extraKnownMarketplaces" in data and isinstance(data["extraKnownMarketplaces"], dict):
    data["extraKnownMarketplaces"].pop("kizunax-local", None)
with open(dst, "w") as f:
    json.dump(data, f, indent=2)
PY
  python3 - "$tmp" <<'PY'
import json, sys
json.load(open(sys.argv[1]))
PY
  mv "$tmp" "$SETTINGS"
}

patch_settings() {
  [ -f "$SETTINGS" ] || { info "No settings.json - nothing to clean up."; return; }
  if [ "$JSON_TOOL" = "jq" ]; then patch_settings_jq; else patch_settings_python; fi
  info "Removed kizunax keys from $SETTINGS"
}

preflight
backup
trap 'rollback $LINENO' ERR INT TERM
patch_settings

# Binary removal is best-effort - not part of the rollback contract.
trap - ERR INT TERM
if [ -f "$BINARY" ]; then
  rm -f "$BINARY" && info "Removed $BINARY" || err "Could not remove $BINARY (continuing)"
fi

echo
echo "Kizunax uninstalled."
[ -n "$BACKED_UP_SETTINGS" ] && echo "  Settings backup: $BACKED_UP_SETTINGS"
echo "  Config + state at ~/.kizunax/ preserved."
echo "  Run 'rm -rf ~/.kizunax' to delete those too."
echo "  Restart Claude Code if it is running."
