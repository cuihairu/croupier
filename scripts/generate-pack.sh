#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BIN_DIR="$ROOT_DIR/bin"
OUT_DIR="$ROOT_DIR/gen/croupier"

echo "[pack] generating croupier pack via protoc-gen-croupier"

if ! command -v protoc >/dev/null 2>&1; then
  echo "error: protoc not found on PATH" >&2
  echo "hint: install protoc (https://grpc.io/docs/protoc-installation/) and ensure it's on PATH" >&2
  exit 127
fi

if [ ! -x "$BIN_DIR/protoc-gen-croupier" ]; then
  echo "[build] protoc-gen-croupier"
  mkdir -p "$BIN_DIR"
  (cd "$ROOT_DIR" && GOCACHE=$PWD/.gocache GOMODCACHE=$PWD/.gomodcache go build -o "$BIN_DIR/protoc-gen-croupier" ./tools/protoc-gen-croupier)
fi

mkdir -p "$OUT_DIR"
PATH="$BIN_DIR:$PATH" \
protoc -I "$ROOT_DIR/proto" \
  --croupier_out=emit_pack=true:"$OUT_DIR" \
  $(
    cd "$ROOT_DIR" && rg -n --files proto | rg -v "(^|/)buf\\.yaml$" | tr '\n' ' '
  )

echo "done: artifacts in $OUT_DIR (manifest/descriptors/ui/fds.pb/pack.tgz if enabled)"

