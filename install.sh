#!/usr/bin/env bash
# hlw installer — installs the latest release binary for this platform.
#
# Usage:
#   curl -fsSL https://hlw.first-it-consulting.com/install.sh | bash
#   curl -fsSL https://hlw.first-it-consulting.com/install.sh | bash --dry-run
#   curl -fsSL https://hlw.first-it-consulting.com/install.sh | bash --help
#
# Environment:
#   HLW_PREFIX    Install prefix. Defaults to ~/.local/bin if unset and that
#                 directory exists, otherwise /usr/local/bin. Set to a path you
#                 can write to (no sudo) for a user-only install.
#   HLW_VERSION   Pin a specific version (default: latest release tag).
#   HLW_BIN       Destination binary name (default: hlw).

set -euo pipefail

show_help() {
  cat <<'EOF'
hlw installer — fetch and verify the latest hlw release binary.

Usage:
  curl -fsSL https://hlw.first-it-consulting.com/install.sh | bash [OPTIONS]

Options (piped to the installer):
  --dry-run        Resolve version + platform, download and verify the asset,
                   but do not copy anything into place.
  --prefix=DIR     Install to DIR/bin (default: $HLW_PREFIX or ~/.local/bin).
  --version=VER    Install a specific version (default: latest release tag).

Set HLW_VERSION to pin, or export HLW_PREFIX=/usr/local for a system install.
EOF
}

# --- resolve script location + args -----------------------------------------
SELF="${BASH_SOURCE[0]}"
while [ $# -gt 0 ]; do
  case "$1" in
    --dry-run) DRY_RUN=1; shift ;;
    -h|--help) show_help; exit 0 ;;
    --prefix=*) HLW_PREFIX="${1#*=}"; shift ;;
    --version=*) HLW_VERSION="${1#*=}"; shift ;;
    *) echo "hlw: unknown argument '$1' (see --help)" >&2; exit 2 ;;
  esac
done

log()  { printf 'hlw: %s\n' "$*" >&2; }
warn() { printf 'hlw: warning: %s\n' "$*" >&2; }

require() {
  local cmd="$1"
  command -v "$cmd" >/dev/null 2>&1 || { echo "hlw: required tool '$cmd' not found on PATH" >&2; exit 1; }
}

# --- preflight --------------------------------------------------------------
for t in curl sha256sum tar; do require "$t"; done

case "$(uname -s)" in
  Darwin) OS=darwin ;;
  Linux)  OS=linux ;;
  *) echo "hlw: unsupported platform '$(uname -s)' (expected darwin or linux)" >&2; exit 1 ;;
esac

case "$(uname -m)" in
  arm64)   ARCH=arm64 ;;
  x86_64)  ARCH=amd64 ;;
  *) echo "hlw: unsupported architecture '$(uname -m)' (expected arm64 or x86_64)" >&2; exit 1 ;;
esac

# --- resolve version --------------------------------------------------------
if [ -z "${HLW_VERSION:-}" ]; then
  HLW_VERSION="$(curl -fsSL --proto '=https' --tlsv1.2 \
    "https://api.github.com/repos/first-it-consulting/hlw/releases/latest" \
    | grep -oE '"tag_name":[[:space:]]*"[^"]+"' \
    | head -1 \
    | sed 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/' | sed 's/^v//')"
fi

if [ -z "${HLW_VERSION:-}" ]; then
  echo "hlw: could not determine latest release version" >&2; exit 1
fi

ASSET="hlw-${HLW_VERSION}-${OS}-${ARCH}.tar.gz"
BASE_URL="https://github.com/first-it-consulting/hlw/releases/download/v${HLW_VERSION}"
CHECKSUM_URL="${BASE_URL}/checksums.txt"

# --- dry run (download + verify only) --------------------------------------
if [ "${DRY_RUN:-0}" = "1" ]; then
  log "dry-run: hlw ${HLW_VERSION} for ${OS}/${ARCH}"
  TMP="$(mktemp -d)"
  trap 'rm -rf "$TMP"' EXIT
  curl -fsSL --proto '=https' --tlsv1.2 "$CHECKSUM_URL" -o "$TMP/checksums.txt"
  curl -fsSL --proto '=https' --tlsv1.2 "${BASE_URL}/${ASSET}" -o "$TMP/${ASSET}"
  expected="$(grep " ${ASSET}\$" "$TMP/checksums.txt" | awk '{print $1}')"
  actual="$(sha256sum "$TMP/${ASSET}" | awk '{print $1}')"
  if [ "${expected:-}" != "$actual" ]; then
    echo "hlw: checksum mismatch (got $actual, expected $expected)" >&2; exit 1
  fi
  log "verified checksum for ${ASSET}"
  exit 0
fi

# --- destination ------------------------------------------------------------
HLW_PREFIX="${HLW_PREFIX:-}"
if [ -z "$HLW_PREFIX" ]; then
  if [ -d "${HOME:-}/.local/bin" ] && [ -w "${HOME:-}/.local/bin" ]; then
    HLW_PREFIX="${HOME:-}/.local/bin"
  else
    HLW_PREFIX="/usr/local/bin"
  fi
fi

# --- install ----------------------------------------------------------------
log "installing hlw ${HLW_VERSION} for ${OS}/${ARCH}"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

curl -fsSL --proto '=https' --tlsv1.2 "${BASE_URL}/${ASSET}" -o "$TMP/${ASSET}"
curl -fsSL --proto '=https' --tlsv1.2 "$CHECKSUM_URL" -o "$TMP/checksums.txt"

expected="$(grep " ${ASSET}\$" "$TMP/checksums.txt" | awk '{print $1}')"
actual="$(sha256sum "$TMP/${ASSET}" | awk '{print $1}')"
if [ "${expected:-}" != "$actual" ]; then
  echo "hlw: checksum mismatch (got $actual, expected $expected)" >&2; exit 1
fi
log "verified checksum for ${ASSET}"

DEST="${HLW_PREFIX}/hlw"
if [ -d "$HLW_PREFIX" ] && [ -w "$HLW_PREFIX" ]; then
  install_dir="$HLW_PREFIX"
elif mkdir -p "$HLW_PREFIX" >/dev/null 2>&1 && [ -w "$HLW_PREFIX" ]; then
  install_dir="$HLW_PREFIX"
else
  echo "hlw: cannot write to $install_dir without sudo; retry with:" >&2
  echo "    HLW_PREFIX=\"\$(mktemp -d)\" curl ... | bash   # or export a writable prefix" >&2
  exit 1
fi

tar -xzf "$TMP/${ASSET}" -C "$TMP"
install -m 0755 "$TMP/hlw" "$DEST"
chmod +x "$DEST"

log "installed hlw to $DEST"
if ! command -v hlw >/dev/null 2>&1; then
  warn "hlw not on PATH yet. Add $HLW_PREFIX to your PATH, e.g.:"
  warn "    export PATH=\"\$PATH:$HLW_PREFIX\""
fi
