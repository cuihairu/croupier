# End-to-End Example (Proto → Pack → Import → UI)

This walkthrough shows how to go from .proto with Croupier options to a function pack, import it into the Server, and invoke from the Web UI.

Prerequisites
- `protoc` installed and on PATH (https://grpc.io/docs/protoc-installation/)
- Croupier repo built (`make build`)

1) Generate a pack from examples
```
# Build the protoc plugin if needed and generate pack artifacts for all protos
./scripts/generate-pack.sh

# Inspect the generated pack (if pack.tgz is emitted by the plugin)
./bin/croupier packs inspect gen/croupier/pack.tgz
```

2) Start Server and Agent
```
# Server (+sqlite approvals support optional)
make server-sqlite
./bin/croupier-server --config configs/server.example.yaml

# Agent
make agent
./bin/croupier-agent --config configs/agent.example.yaml
```

3) Import the pack into the Server
```
# Either import the generated pack.tgz …
./bin/croupier packs import gen/croupier/pack.tgz
# …or import example packs
make packs-build
./bin/croupier packs import packs/dist/prom.pack.tgz
```

4) Open the Web UI and invoke
- Navigate to GM Functions page
- Select `prom.query_range`
- Fill in `expr` and optional time range
- Submit, and see JSON + line chart (grid layout)

Notes
- The generator uses method-level and field-level options under `proto/croupier/options/*`.
- If `protoc` is not available, you can still use example packs under `packs/*` to try the flow.

