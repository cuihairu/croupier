# protoc-gen-croupier (skeleton)

This plugin turns your .proto into Croupier "packs": descriptors, UI schema, a manifest and an fds.pb. It can also bundle them into `pack.tgz`.

Status: initial skeleton. It derives defaults when no custom options are present. Custom option parsing will be added next.

## Install/Build

```
make croupier-plugin
# binary at bin/protoc-gen-croupier
```

## Generate with protoc

Requires `protoc` on PATH.

```
PATH="$PWD/bin:$PATH" \
protoc -I proto \
  --croupier_out=emit_pack=true:gen/croupier \
  proto/your/package/*.proto
```

Artifacts go to `gen/croupier/`:
- `manifest.json`: function list
- `descriptors/*.json`: function descriptors (transport/auth/semantics)
- `ui/*.schema.json` and `ui/*.uischema.json`: JSON Schema and UI Schema for requests
- `fds.pb`: FileDescriptorSet (types)
- `pack.tgz`: all the above bundled (if `emit_pack=true`)

## Generate with buf (optional)

Buf will look for `protoc-gen-croupier` on PATH.

```
PATH="$PWD/bin:$PATH" buf generate
```

Note: remote plugins in `buf.gen.yaml` may require network. You can remove them if offline.

## Defaults
- function_id: `<package>.<Service>.<Method>` lowercased
- version: `1.0.0`
- category: second-to-last segment of package (e.g., `games.player.v1` → `player`)
- transport: protobuf (pb-json UI, Core encodes to pb-bin)
- semantics: mode=query, route=lb, timeout=30s, idempotency_key=false
- auth: permission=function_id, two_person_rule=false
- placement: agent
- outputs: a default `json.view`

## Next steps
- Parse custom options from `proto/croupier/options/*.proto`
- UI annotations to enrich generated UI Schema
- Per-field sensitive/enum/show_if, and per-method route/approval config
- Pack signature and validation
