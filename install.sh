#!/usr/bin/env bash
# Kizunax installer
set -euo pipefail

REPO_DIR="$(cd "$(dirname "$0")" && pwd)"
VERSION_FILE="$REPO_DIR/internal/cli/cli.go"
SETTINGS="$HOME/.claude/settings.json"
BINARY="$REPO_DIR/plugins/kizunax/bin/kizunax"
BACKUP_DIR="$HOME/.claude/.kizunax-backups"
RELEASE_BASE="https://github.com/thanhhaudev/kizunax-plugin-cc/releases/download"

BACKED_UP_SETTINGS=""
BACKED_UP_BINARY=""
NEW_BINARY_WRITTEN=0
SETTINGS_PREEXISTED=0
JSON_TOOL=""
PLATFORM=""
VERSION=""
RUN_SETUP=1

for arg in "$@"; do
  case "$arg" in
    --no-setup) RUN_SETUP=0 ;;
    -h|--help)
      echo "Usage: $0 [--no-setup]"
      echo "  --no-setup  Skip the post-install setup wizard prompt."
      exit 0
      ;;
    *)
      echo "Unknown argument: $arg" >&2
      echo "Usage: $0 [--no-setup]" >&2
      exit 2
      ;;
  esac
done

err()  { echo "ERROR: $*" >&2; }
info() { echo "==> $*"; }

detect_platform() {
  local os arch
  case "$(uname -s)" in
    Darwin) os="darwin" ;;
    Linux)  os="linux" ;;
    *) err "Unsupported OS: $(uname -s). Use the Windows manual install (see README)."; exit 1 ;;
  esac
  case "$(uname -m)" in
    arm64|aarch64) arch="arm64" ;;
    x86_64|amd64)  arch="amd64" ;;
    *) err "Unsupported architecture: $(uname -m)."; exit 1 ;;
  esac
  echo "${os}-${arch}"
}

detect_json_tool() {
  if command -v jq >/dev/null 2>&1; then
    echo "jq"
  elif command -v python3 >/dev/null 2>&1; then
    echo "python3"
  else
    err "Need jq or python3 to patch settings.json. Install one and re-run."
    exit 1
  fi
}

read_version() {
  if [ ! -f "$VERSION_FILE" ]; then
    err "Cannot find $VERSION_FILE. Run install.sh from inside the kizunax-plugin-cc repo."
    exit 1
  fi
  grep -E '^const Version = "[^"]+"' "$VERSION_FILE" | sed -E 's/.*"([^"]+)".*/\1/'
}

preflight() {
  PLATFORM="$(detect_platform)"
  VERSION="$(read_version)"
  [ -n "$VERSION" ] || { err "Failed to parse Version from $VERSION_FILE"; exit 1; }
  JSON_TOOL="$(detect_json_tool)"

  if [ -f "$SETTINGS" ]; then
    if [ "$JSON_TOOL" = "jq" ]; then
      jq empty "$SETTINGS" >/dev/null 2>&1 || { err "$SETTINGS is not valid JSON. Fix it manually, then re-run."; exit 1; }
    else
      python3 -c "import json,sys; json.load(open(sys.argv[1]))" "$SETTINGS" 2>/dev/null \
        || { err "$SETTINGS is not valid JSON. Fix it manually, then re-run."; exit 1; }
    fi
  fi

  [ -f "$SETTINGS" ] && SETTINGS_PREEXISTED=1

  mkdir -p "$BACKUP_DIR"
  mkdir -p "$(dirname "$BINARY")"

  info "Platform: $PLATFORM"
  info "Version:  $VERSION"
  info "JSON tool: $JSON_TOOL"
  info "Settings: $SETTINGS"
}

preflight

TIMESTAMP="$(date +%Y%m%d-%H%M%S)"

backup() {
  # Set BACKED_UP_* only AFTER cp succeeds - otherwise a failed cp would leave
  # the var pointing at a partial backup file and rollback would restore from
  # that corrupt copy.
  local dest
  if [ -f "$SETTINGS" ]; then
    dest="$BACKUP_DIR/settings.json.$TIMESTAMP"
    cp "$SETTINGS" "$dest"
    BACKED_UP_SETTINGS="$dest"
    info "Backed up settings → $BACKED_UP_SETTINGS"
  fi
  if [ -f "$BINARY" ]; then
    dest="$BACKUP_DIR/kizunax.bin.$TIMESTAMP"
    cp "$BINARY" "$dest"
    BACKED_UP_BINARY="$dest"
    info "Backed up binary  → $BACKED_UP_BINARY"
  fi
}

rollback() {
  local line="${1:-?}"
  err "Aborted at line $line - reverting changes..."

  if [ -n "$BACKED_UP_SETTINGS" ] && [ -f "$BACKED_UP_SETTINGS" ]; then
    cp "$BACKED_UP_SETTINGS" "$SETTINGS" \
      && info "Restored $SETTINGS" \
      || err "FAILED to restore settings - original at $BACKED_UP_SETTINGS, current at $SETTINGS"
  fi

  if [ "$NEW_BINARY_WRITTEN" = "1" ]; then
    if [ -n "$BACKED_UP_BINARY" ] && [ -f "$BACKED_UP_BINARY" ]; then
      cp "$BACKED_UP_BINARY" "$BINARY" \
        && info "Restored $BINARY" \
        || err "FAILED to restore binary - original at $BACKED_UP_BINARY"
    else
      rm -f "$BINARY"
      info "Removed partially-written binary"
    fi
  fi

  # Clean up tmp files (best-effort) so a failed mv doesn't orphan them.
  rm -f "$BINARY.download.tmp" "$BINARY.sha.tmp" "$SETTINGS.tmp"

  # If settings.json did not exist before install, remove the empty {} we created.
  if [ "$SETTINGS_PREEXISTED" = "0" ] && [ -f "$SETTINGS" ]; then
    rm -f "$SETTINGS"
    info "Removed $SETTINGS (did not exist before install)"
  fi

  err "Rollback complete. Backups retained at $BACKUP_DIR/"
  exit 1
}

trap 'rollback $LINENO' ERR INT TERM
backup

download_binary() {
  local url="$RELEASE_BASE/v$VERSION/kizunax-$PLATFORM"
  local sha_url="$url.sha256"
  local tmp="$BINARY.download.tmp"
  local tmp_sha="$BINARY.sha.tmp"

  info "Trying $url"
  if ! curl -fsL --retry 2 -o "$tmp" "$url" 2>/dev/null; then
    rm -f "$tmp"
    return 1
  fi

  if curl -fsL --retry 2 -o "$tmp_sha" "$sha_url" 2>/dev/null; then
    local expected actual
    expected="$(awk '{print $1}' "$tmp_sha")"
    if command -v sha256sum >/dev/null 2>&1; then
      actual="$(sha256sum "$tmp" | awk '{print $1}')"
    else
      actual="$(shasum -a 256 "$tmp" | awk '{print $1}')"
    fi
    rm -f "$tmp_sha"
    if [ "$expected" != "$actual" ]; then
      rm -f "$tmp"
      err "SHA256 mismatch on downloaded binary (expected $expected, got $actual)"
      err "Aborting install - possible corrupted download or tampering. Not falling back to local build."
      exit 1
    fi
    info "SHA256 verified"
  else
    rm -f "$tmp_sha"
    info "No .sha256 published - skipping checksum check"
  fi

  mv "$tmp" "$BINARY"
  chmod +x "$BINARY"
  NEW_BINARY_WRITTEN=1
  return 0
}

go_version_ok() {
  command -v go >/dev/null 2>&1 || return 1
  local v
  v="$(go version | awk '{print $3}' | sed 's/^go//')"
  # accept 1.21.x and above (1.22, 1.23, …) - simple major.minor check
  local major minor
  major="$(echo "$v" | cut -d. -f1 | sed 's/[^0-9].*//')"
  minor="$(echo "$v" | cut -d. -f2 | sed 's/[^0-9].*//')"
  if [ "$major" -gt 1 ] || { [ "$major" -eq 1 ] && [ "$minor" -ge 21 ]; }; then
    return 0
  fi
  return 1
}

build_binary_local() {
  if ! go_version_ok; then
    err "No release binary for v$VERSION on $PLATFORM, and Go ≥1.21 not found."
    err "Either install Go from https://go.dev/dl/ or wait for the v$VERSION release."
    return 1
  fi
  info "Building locally with $(go version)"
  bash "$REPO_DIR/scripts/build.sh"
  NEW_BINARY_WRITTEN=1
}

acquire_binary() {
  if download_binary; then
    info "Downloaded pre-built binary"
  else
    info "Falling back to local go build"
    build_binary_local
  fi
}

acquire_binary

smoke_test() {
  local out
  out="$("$BINARY" --version 2>&1 || true)"
  if [ "$out" != "kizunax $VERSION" ]; then
    err "Smoke test failed: expected 'kizunax $VERSION', got '$out'"
    return 1
  fi
  info "Smoke test passed: $out"
}

smoke_test

patch_settings_jq() {
  local tmp="$SETTINGS.tmp"
  if [ ! -f "$SETTINGS" ]; then
    echo "{}" > "$SETTINGS"
  fi
  jq --arg path "$REPO_DIR" '
    .enabledPlugins //= {}
    | .extraKnownMarketplaces //= {}
    | .enabledPlugins["kizunax@kizunax-local"] = true
    | .extraKnownMarketplaces["kizunax-local"] = {
        "source": { "source": "directory", "path": $path }
      }
  ' "$SETTINGS" > "$tmp"
  jq empty "$tmp" >/dev/null
  mv "$tmp" "$SETTINGS"
}

patch_settings_python() {
  local tmp="$SETTINGS.tmp"
  if [ ! -f "$SETTINGS" ]; then
    echo "{}" > "$SETTINGS"
  fi
  python3 - "$SETTINGS" "$REPO_DIR" "$tmp" <<'PY'
import json, sys
src, repo_dir, dst = sys.argv[1], sys.argv[2], sys.argv[3]
with open(src) as f:
    data = json.load(f)
data.setdefault("enabledPlugins", {})["kizunax@kizunax-local"] = True
data.setdefault("extraKnownMarketplaces", {})["kizunax-local"] = {
    "source": {"source": "directory", "path": repo_dir}
}
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
  if [ "$JSON_TOOL" = "jq" ]; then
    patch_settings_jq
  else
    patch_settings_python
  fi
  info "Patched $SETTINGS"
}

patch_settings

trap - ERR INT TERM

echo
echo "Kizunax v$VERSION installed."
echo "  Binary:   $BINARY"
echo "  Settings: $SETTINGS"
if [ -n "$BACKED_UP_SETTINGS" ] || [ -n "$BACKED_UP_BINARY" ]; then
  echo "  Backups:  $BACKUP_DIR/   (delete after verifying things work)"
fi
echo
echo "Next steps:"
echo "  1. Restart Claude Code if it is running."
echo "  2. Run /kizunax:setup to configure provider, model, and API key."
