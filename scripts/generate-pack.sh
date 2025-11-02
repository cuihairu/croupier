#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BIN_DIR="$ROOT_DIR/bin"
OUT_DIR="$ROOT_DIR/gen/croupier"

echo "[pack] generating croupier pack via buf (protoc-gen-croupier plugin)"

if ! command -v buf >/dev/null 2>&1; then
  echo "error: buf not found on PATH" >&2
  echo "hint: install buf (https://docs.buf.build/installation) and ensure it's on PATH" >&2
  exit 127
fi

if [ ! -x "$BIN_DIR/protoc-gen-croupier" ]; then
  echo "[build] protoc-gen-croupier"
  mkdir -p "$BIN_DIR"
  # Use repo-local caches to avoid polluting global env; make cache writable to prevent permission issues.
  (cd "$ROOT_DIR" && GOCACHE=$PWD/.gocache GOMODCACHE=$PWD/.gomodcache go build -modcacherw -o "$BIN_DIR/protoc-gen-croupier" ./tools/protoc-gen-croupier)
fi

mkdir -p "$OUT_DIR"

# Run buf generate with a local template that only uses the local croupier plugin.
# This avoids remote plugin fetch and works offline.
export PATH="$BIN_DIR:$PATH"
export BUF_CACHE_DIR="$ROOT_DIR/.bufcache"
export XDG_CACHE_HOME="$ROOT_DIR/.bufcache"

(cd "$ROOT_DIR" && buf generate --template "$ROOT_DIR/buf.gen.local.yaml")

echo "done: artifacts in $OUT_DIR (manifest/descriptors/ui/fds.pb/pack.tgz if enabled)"
