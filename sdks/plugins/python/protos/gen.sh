#!/usr/bin/env bash
# Regenerate the checked-in Python gRPC stubs for the plugin contracts.
#
# The plugin IPC contracts version separately from the main proto/ chain
# (see scripts/check-no-grpc.sh), so these stubs are generated from the
# local copies in this directory and committed, never wired into
# `make proto`. funcplugin.proto / probeplugin.proto are the canonical
# sources (agents/sidecar/funcplugin/protocol, agents/synthetics/
# pluginhost/protocol); plugin_controller.proto / plugin_stdio.proto are
# mirrored from the pinned go-plugin version's internal/plugin.
#
# Usage (from sdks/plugins/python):
#   protos/gen.sh <python-with-grpcio-tools>   # e.g. a venv python
#
# The generated *_grpc.py files import siblings as `import X_pb2`; the
# rewrite at the end flattens them into package-relative imports so the
# stubs can live inside croupier_plugin/ directly.
set -euo pipefail
cd "$(dirname "$0")"

PY="${1:?usage: protos/gen.sh <python-with-grpcio-tools>}"

for proto in funcplugin probeplugin plugin_controller plugin_stdio; do
  "$PY" -m grpc_tools.protoc -I. --python_out=../croupier_plugin --grpc_python_out=../croupier_plugin "${proto}.proto"
done

cd ../croupier_plugin
for f in *_grpc.py; do
  python3 - "$f" <<'EOF'
import re, sys
path = sys.argv[1]
body = open(path).read()
fixed = re.sub(r'^import (\w+_pb2) as (\w+)$', r'from . import \1 as \2', body, flags=re.M)
if fixed != body:
    open(path, "w").write(fixed)
    print(f"fixed relative imports: {path}")
EOF
done

echo "stubs regenerated into croupier_plugin/"
