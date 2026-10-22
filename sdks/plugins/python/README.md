# Croupier Plugin SDK (Python)

Python plugin SDK for the Croupier go-plugin hosts: **sidecar function
adapter plugins** (map function calls onto a game's existing
HTTP/TCP/admin interface) and **synthetics probe plugins** (check one
configured target). Both speak HashiCorp go-plugin's local gRPC mode as
pinned in the server's `go.mod`; this package has no relationship to the
game-facing SDK in `sdks/python/` and adds no dependencies to it.

## Install

```bash
pip install -e .        # runtime: grpcio, grpcio-health-checking, protobuf
```

Python >= 3.12.

## Function adapter plugin

```python
import croupier_plugin as cp

class MyAdapter(cp.FunctionPlugin):
    def describe(self):
        return "myadapter", "1.0.0"          # must match the sidecar's plugin name

    def list_functions(self):
        return [
            cp.FunctionDescriptor(
                id="plugin.myadapter.ban_player",   # plugin.<name>.<fn>, lowercase
                summary="ban a player via the game admin HTTP API",
                param_schema='{"type":"object"}',
                result_schema='{"type":"object"}',
                risk_level=cp.RISK_HIGH,
            ),
        ]

    def invoke(self, call):
        # call.function_id / call.params_json / call.timeout_ms
        # call.context.is_active() is False once the host deadline fires
        ...
        return cp.ok_result('{"banned": true}')
        # business failures ride the closed-set envelope, never raise:
        return cp.error_result(cp.TARGET_REJECTED, "player not found")

cp.serve_function_plugin(MyAdapter())
```

Sidecar config (`cmd/sidecar-agent`):

```yaml
functions:
  enabled: true
  plugins:
    - name: myadapter
      command: python3
      args: ["/opt/game-plugins/my_adapter.py"]
      timeout: 10s
```

## Probe plugin

```python
import croupier_plugin as cp

class MyProbe(cp.ProbePlugin):
    def describe(self):
        return "myprobe", "1.0.0"            # must match the controller's plugin name

    def check(self, call):
        # call.target / call.timeout_ms / call.payload_json
        ...
        return cp.PassResult(status=200, latency_ms=3, detail="login ok")
        return cp.FailResult("login rejected", status=403)

cp.serve_probe_plugin(MyProbe())
```

Synthetics-agent config routes a target to the plugin by name
(`plugin: myprobe`).

Non-HTTP destinations pass their parameters through the target's
`payloadJson` field. The Python reference TCP probe accepts
`{"host":"127.0.0.1","port":5432}`; a TCP connect remains only a network
reachability assertion, not a game-protocol success.

## Contract rules

- Business failures ride the closed-set envelope
  (`target_unavailable | target_rejected | bad_response`) and must be
  returned, never raised — a raised exception means the plugin process
  itself is broken and the host classifies the call as a `plugin_*`
  degradation cause.
- The magic cookie gate refuses direct execution: a plugin process only
  does work when the host launched it.
- stdout carries only the handshake line; plugin logs go to stderr.
- Long waits must poll `call.context.is_active()`; the host's per-call
  deadline cancels the RPC.

## Tests

```bash
pip install -e '.[dev]'
pytest                      # needs a Go toolchain for the hostdriver integration suite
```

`hostdriver/` builds against the real Go hosts (`funcplugin.NewHost`,
`pluginhost.NewController`) and drives the reference plugins in
`examples/` through the real handshake; every scenario is a full
host⇄plugin round trip.

Generated gRPC stubs are committed under `croupier_plugin/`; regenerate
with `protos/gen.sh <python-with-grpcio-tools>`.
