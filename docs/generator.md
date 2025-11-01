# protoc-gen-croupier (skeleton)

This plugin turns your .proto into Croupier "packs": descriptors, UI schema, a manifest and an fds.pb. It can also bundle them into `pack.tgz`.

Status: initial skeleton. It derives defaults when no custom options are present. Custom option parsing will be added next.
Update: basic custom options parsing implemented via UninterpretedOption aggregate parsing.

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
- transport: protobuf (pb-json UI, Server encodes to pb-bin)
- semantics: mode=query, route=lb, timeout=30s, idempotency_key=false
- auth: permission=function_id, two_person_rule=false
- placement: agent
- outputs: a default `json.view`

## Next steps
- Parse map-style options (labels/enum_map) – basic support added; improve nested parsing
- UI annotations enrich generated UI Schema – widget/label/placeholder/sensitive/show_if/required_if supported
- Enum detection in JSON Schema – supported (string names + enum list)
- Map fields in JSON Schema – supported (additionalProperties)
- Per-method route/approval/placement/timeout – supported
- Pack signature and validation

## Supported custom options (current)
- Method option `(croupier.options.function)` fields parsed:
  - `function_id`, `version`, `category`, `risk`, `route`, `timeout`, `two_person_rule`, `placement`, `mode`, `idempotency_key`
- Field option `(croupier.options.ui)` fields parsed:
  - `widget`, `label`, `placeholder`, `sensitive`, `show_if`, `required_if`, `enum_map`

Example:
```
rpc Ban(BanRequest) returns (BanResponse) {
  option (croupier.options.function) = {
    function_id: "player.ban" version: "1.2.0" risk: "high"
    route: "lb" timeout: "30s" two_person_rule: true placement: "agent"
    mode: "command" idempotency_key: true
  };
}

message BanRequest {
  string player_id = 1 [(croupier.options.ui) = { label: "玩家ID", widget: "input" }];
  string reason    = 2 [(croupier.options.ui) = { widget: "textarea", placeholder: "原因" }];
}
```
- Pack signature and validation
