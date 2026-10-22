#!/usr/bin/env bash
# Regenerate the function adapter plugin gRPC contract (todo:35 slice 1).
# Pinned tooling: protoc-gen-go v1.36.x, protoc-gen-go-grpc v1.6.x.
# Run: bash agents/sidecar/funcplugin/protocol/gen.sh
set -euo pipefail
cd "$(dirname "$0")"
PATH="$PATH:$HOME/go/bin:$HOME/.local/bin" protoc \
  --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
  funcplugin.proto
echo "regenerated: funcplugin.pb.go funcplugin_grpc.pb.go"
