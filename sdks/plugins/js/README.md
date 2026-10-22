# Croupier Plugin SDK (Node.js)

Node.js plugin SDK for the Croupier go-plugin hosts: **synthetics probe
plugins** (check one configured target) and **sidecar function adapter
plugins** (map registered functions onto a game's existing HTTP/TCP/admin
interface). It speaks HashiCorp go-plugin's local gRPC mode as pinned in
the server's `go.mod`; this package has no relationship to the
game-facing SDK in `sdks/js/` and adds no dependencies to it. Both
contracts mirror the Python SDK (`sdks/plugins/python/`), so a plugin
authored in either language is interchangeable behind the same host
config (`synthetics.plugins[]` / the sidecar function host).

## Install

```bash
pnpm install    # runtime: @grpc/grpc-js, @grpc/proto-loader, protobufjs
pnpm build      # tsc -> dist/
```

Node >= 24 (see `engines` in package.json; no experimental flags).

## Probe plugin

```ts
import { passResult, failResult, serve, type ProbePlugin } from "croupier-plugin-sdk-node";

const probe: ProbePlugin = {
  describe: () => ({ name: "probe-echo", version: "1.0.0" }), // must match the plugin name
  check: (call) =>
    call.target.startsWith("fail")
      ? failResult("synthetic probe failure", {
          status: 500,
          detail: `probe-echo checked ${call.target}`,
        })
      : passResult({ status: 200, detail: `probe-echo checked ${call.target}` }),
};

void serve(probe);
```

- `call.target` / `call.timeoutMs` / `call.payloadJson` mirror the Python
  `CheckInput`; `call.isCancelled()` is false once the host deadline
  fires (poll it in long waits), `call.remainingMs()` is the grace left.
- `check` may be async; unlike the Python SDK there is no worker pool, so
  never block the event loop — `await` timers instead of spinning.
- Run the compiled entry with `node dist/your-plugin.js`. Direct
  execution is refused: the plugin serves only behind the host's magic
  cookie env (`CROUPIER_SYNTHETICS_PLUGIN`).

A complete runnable example ships in `examples/probe_echo.ts`.

## Function adapter plugin

```ts
import {
  errorResult,
  okResult,
  serveFunctionPlugin,
  type FunctionDescriptor,
  type FunctionPlugin,
} from "croupier-plugin-sdk-node";

const plugin: FunctionPlugin = {
  describe: () => ({ name: "pyecho", version: "1.2.3" }), // must match the sidecar config
  listFunctions: () => [
    {
      id: "plugin.pyecho.echo", // namespace: "plugin.<plugin-name>.<function>"
      summary: "echo the message back",
      paramSchema: '{"type":"object","properties":{"message":{"type":"string"}}}',
      resultSchema: '{"type":"object","properties":{"message":{"type":"string"}}}',
      riskLevel: "low", // closed set: low|medium|high|danger (default low)
    },
  ],
  invoke: (call) => {
    if (call.functionId !== "plugin.pyecho.echo") {
      return errorResult("bad_response", `unknown function ${call.functionId}`);
    }
    const params = JSON.parse(call.paramsJson) as { message?: unknown };
    if (typeof params.message !== "string" || params.message === "") {
      return errorResult("bad_response", "message must be a non-empty string");
    }
    return okResult(JSON.stringify({ message: params.message }));
  },
};

void serveFunctionPlugin(plugin);
```

- Business failures ride the closed-set envelope
  (`target_unavailable | target_rejected | bad_response`) and must never
  throw — a thrown exception means the plugin process itself is broken,
  which the host classifies as a `plugin_*` degradation cause.
- `call.functionId` / `call.paramsJson` / `call.timeoutMs` mirror the
  Python `InvokeInput`; `call.isCancelled()` fires on the host deadline,
  `call.remainingMs()` is the time left. Long work must `await` timer
  slices (same event-loop rule as the probe face).

A complete runnable example ships in `examples/func_echo.ts` (the
`pyecho` reference: echo, fail_unavailable, fail_rejected, slow).

## Protocol behaviour (same as the Python SDK)

- Cookie gate, `PLUGIN_PROTOCOL_VERSIONS` negotiation (v1 only),
  single stdout handshake line, GRPCStdio parked stream,
  GRPCController.Shutdown, grpc.health.v1 SERVING, SIGTERM/SIGINT with a
  bounded 2s stop grace. Plugin logs go to plain stderr (the host
  collects them); stdout carries only the handshake line.
- **Bind is TCP, not a unix socket**: grpc-js has no unix domain socket
  support. The SDK binds `127.0.0.1:0` (override with
  `CROUPIER_PLUGIN_BIND=host:port`) and the handshake advertises
  `tcp|<addr>` — go-plugin dials whatever the line says, so the host
  config is identical for Go/Python/JS plugins.

## Contracts and drift

The contracts load at runtime from `protos/` (no codegen):
`probeplugin.proto` (canonical: `agents/synthetics/pluginhost/protocol/`),
`funcplugin.proto` (canonical:
`agents/sidecar/funcplugin/protocol/`), `plugin_stdio.proto` /
`plugin_controller.proto` (canonical: the Python SDK's mirrors), and
`health.proto` (grpc-js ships no health service). `pnpm verify:proto`
asserts the mirrors are byte-identical.

## Tests

```bash
pnpm typecheck && pnpm test && pnpm verify:proto
```

`test/integration_go_host.test.ts` builds the shared Go hostdriver
(`sdks/plugins/hostdriver`) and drives the compiled examples through the
real hosts — the synthetics controller's probe scenarios and the sidecar
function host's `func-register` / `func-invoke` / `func-envelope` /
`func-timeout` scenarios — not a mock of either side. It skips itself
when no Go toolchain is installed (CI installs one).

## Known boundaries

- Protocol negotiation speaks v1 only (the Croupier host also only has v1).
- The multiplex flag is echoed per the protocol, but the Croupier host
  does not enable multiplexed gRPC.
