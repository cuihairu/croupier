---
title: Sidecar External Function Adapter Plugin Design
---

# Sidecar External Function Adapter Plugin

## Status and Conclusion

- Status: the first slice (the minimal loop) is implemented — the shared resident host engine was lifted into `core/pluginhost`, and the function plugin contract, host, declaration validation, and reference plugin landed in `agents/sidecar/funcplugin`, with the sidecar assembly and dispatch hooks wired (todo.md:35, 2026-10-20).
- Conclusion: function plugins and probe plugins share the same go-plugin resident host mechanism (lazy start, resident reuse, per-call deadline, crash classification, bounded shutdown), but the **contracts, permission surfaces, and error closed-sets are fully separate**; when a game has no management interface the platform cannot invent an entry point — developers still provide HTTP/TCP management interfaces or SDK providers, and the plugin only adapts interfaces that already exist.

## Background and Goals

todo.md:35 (raised by the user on 2026-10-17, reusing the go-plugin pattern): developer plugins declare functions/schemas and map invocations onto the game's existing HTTP/TCP management interfaces; the sidecar reuses function registration, permission approval, audit, and routing chains; the plugin host shares base mechanics with probing but keeps interfaces/permissions separate, and both are governed by the optional build tag (`no_go_plugin`) and runtime switches.

Design basis: `docs/zh/research/synthetics-agent-go-plugin.md` ("the sidecar can provide an optional function adapter plugin with the same host mechanism… probe plugins and function plugins define their contracts and permissions separately"), `docs/zh/development/agent-runtime-layout.md` ("extract the shared process supervision/plugin host capabilities into an independent shared package").

## Shared Host Mechanism (core/pluginhost)

The generic resident engine originally in the probe host (`agents/synthetics/pluginhost`) — concurrency-slot backpressure, crash exponential backoff, consecutive-failure circuit breaking, lazy start, the Describe handshake, bounded shutdown draining — was lifted verbatim into `core/pluginhost` (a generic `Controller[Impl]` + `Contract[Impl]`); the probe controller and the function host both consume the same implementation, with no duplicated second copy of the lifecycle code. The gRPC dependency boundary expands from "the synthetics plugin host subtree only" to "the plugin host subtree (synthetics + sidecar funcplugin + core/pluginhost)", with `scripts/check-no-grpc.sh` closed accordingly; the game SDK and the Agent↔Server TCP links remain zero-gRPC.

The probe host keeps its probing semantics (`CheckInput/CheckOutput`, `probe.Result` mapping, native HTTP fallback); the function host takes only the subset it needs with different semantics: probing is a self-driven tick loop that must own its backoff/circuit breaking; function invocation is synchronous request-response whose retry cadence is held by the task dispatch layer (TaskRunner/approval) — the host never retries silently.

## Function Plugin Contract (funcplugin.v1)

`agents/sidecar/funcplugin/protocol/funcplugin.proto`, versioned on the same track as `probeplugin.proto` (local go-plugin gRPC IPC, not promoted into the top-level proto/):

- `Describe`: the name/version handshake (same as probing; sent right after host startup, metadata reported with registration).
- `ListFunctions`: the plugin's declared function set — each entry carries `id`, `summary`, `param_schema`/`result_schema`/`error_schema` (JSON Schema Draft 2020-12; request/response are always JSON objects), and `risk_level` (the closed set low|medium|high|danger, empty = low).
- `Invoke(function_id, params_json, timeout_ms)`: a single invocation. **Business failures travel in the response envelope (ok=false + error_code/error_message), never as gRPC errors**; gRPC errors are reserved for plugin-process-level failures (crash/disconnect), classified by the host into `plugin_*` reasons.

### Declaration Validation (the declare subpackage, pure Go, compiled under both build modes)

- Namespacing: function ids must look like `plugin.<plugin name>.<function name>` — isolated from the SDK provider function and platform function spaces, so a plugin can never shadow existing routes.
- Schemas must be valid JSON with a top-level `type: object`, each ≤64KiB; error_schema is optional and validated the same way when present.
- Plugin names match `[a-z0-9][a-z0-9-]*`; function ids are globally unique — a duplicate declaration (including cross-plugin id collisions) is rejected at registration and the whole registration is discarded, never a silent upsert.
- Validated declarations are built into `functionv1.FunctionMetadata` via `declare.Metadata` (resource=`plugin.<name>`, tags carrying `plugin`/the plugin name, execution=sync, audit_log=true, risk mapping) for the existing registration/approval/audit chains to consume.

## Invocation Mapping and Error Translation

The mapping logic lives on the **plugin side**: the host stays protocol-agnostic (it knows nothing of HTTP/TCP); the plugin is the protocol adapter that translates `Invoke(params_json)` into calls to the game's existing interfaces. Target configuration (URL/address, per-function method and path, target timeout) is held by the plugin's own startup arguments/environment, never distributed through sidecar configuration, and the plugin never sees agent credentials or the server address. The reference plugin `httpget` maps invocations to local HTTP GETs (`GET <base><path>`); `tcp` (the fourth slice) maps invocations to the game's existing TCP management/text interfaces — one connection per invocation, with the request frame `<path>[ <compact JSON body>]` + newline and the response read as one line (newline or EOF ends it, capped at 1 MiB). The TCP protocol has no status codes: when the response is not an `{"ok":...}` object, it is returned as data verbatim (no success semantics are invented for a status-code-less protocol); only `{"ok":false}` translates to `target_rejected`.

The error closed-set (design constraint: the caller always receives a finite, discriminable, auditable set of shapes):

| error_code                                                          | Meaning                                                                            | Typical source      |
| ------------------------------------------------------------------- | ---------------------------------------------------------------------------------- | ------------------- |
| (ok=true)                                                           | Success; result_json carries the game interface's response                         | —                   |
| `target_unavailable`                                                | Game interface unreachable/timed out/5xx                                           | Plugin translation  |
| `target_rejected`                                                   | The game interface explicitly refused (4xx/business error code)                    | Plugin translation  |
| `bad_response`                                                      | The interface response violates the declared contract (unparseable/missing fields) | Plugin translation  |
| `plugin_unavailable` / `plugin_timeout` / `plugin_error`            | Plugin process crash, no answer within the deadline, internal plugin error         | Host classification |
| `plugin_not_configured` / `plugin_disabled` / `plugin_not_compiled` | Undeclared, runtime/per-plugin switch off, trimmed build                           | Host classification |

Self-owned target timeouts: the adapter's socket wait takes the earliest of ctx cancellation, the host's per-call deadline, and the plugin's own `-target-timeout` (the same as httpget's client timeout). In adapters without a self-owned timeout, a hung target always hits the host deadline first and reports `plugin_timeout` (host-level degradation), and the adapter cannot report the business fact "the game interface timed out without answering" — that is why the two timeouts have separate duties.

Translation rules: target 2xx and parseable → success; target-side connection failure/timeout → `target_unavailable`; target 4xx/business rejection → `target_rejected` (carrying the status code and a response digest); target 5xx → `target_unavailable` (a target failure, not the caller's fault). The host's per-call deadline = min(caller budget, plugin-configured timeout); timeouts propagate to the plugin via ctx cancellation, and the plugin should abort the target call.

## Permission and Scope Boundaries (never bypassing the 8508540 defense line)

- **The sole source of scope ownership is the agent configuration**. Function plugins neither declare nor carry their own game/env; at host startup it takes the sidecar's already-validated `gameId/env` and refuses to start on blanks (fail-closed, same standard as `internal/app/agent.ValidateAgentScope`) — there is no path for "plugin-provided scope" or "cross-scope functions".
- **Functions are reported and routed under the agent session's scope**. Plugin functions ride the same registration/dispatch chain as SDK functions: the server binds dispatch by the session's game/env, and the sidecar's local dispatch hook only honors function ids registered by this agent; the registry matches ids exactly and rejects cross-plugin/unregistered ids — no wildcard routing.
- **Namespace isolation is half the permission boundary**. The `plugin.<name>.<fn>` prefix is enforced by the host, so SDK/platform functions can never be shadowed by a plugin declaration; declaration conflicts (duplicate ids) are rejected at registration, never silently upserted.
- **Permissions/approval/audit reuse the existing function metadata surface**. The declared risk_level maps into `FunctionSecurity.risk_level`, with `audit_log=true` always set; high-risk functions go through the server's existing approval chain, and the plugin offers no approval bypass (the host implements Invoke only and makes no authorization decisions).
- **Plugins are minimal-trust processes**: launched only by explicit sidecar configuration (no auto-discovery), with a single manager (the host), holding no croupier agent credentials, server address, or TLS material; mapping targets come from the plugin's local config, by policy restricted to local-machine/intranet game interfaces. The gRPC IPC is only the host↔plugin local channel and never leaves the machine.

## Failure and Degradation Semantics

- Plugin process crash: the host observes `client.Exited()`, the invocation reports `plugin_unavailable`, and the process is rebuilt by the host (the same applies when the function side reuses the core engine's backoff/circuit-break state machine; the minimal implementation classifies per invocation, and retries are decided by the caller/task layer).
- Timeout: the per-call deadline reports `plugin_timeout` on expiry; caller context cancellation (task cancellation, disconnect) propagates to the plugin, with the same semantics as ctx cancellation against a directly connected SDK — the extra overhead is one local gRPC round trip.
- Registration drift: the host re-runs `ListFunctions` every 30s and replaces that plugin's registered set as a whole (`Start` is the strict variant); the registered set only changes upon a fully valid response — an unreachable process or an invalid declaration keeps the previous set, and degradation is visible at the next dispatch; the server-side registration snapshot is rebuilt wholesale, with absence meaning revocation (third slice).
- Shutdown: `Stop()` drains with a bound (5s window) before the sole manager kills all resident plugins, leaving no orphan child processes.
- Optional build: with `no_go_plugin`, the function host compiles to a stub, and declaring function plugins in config fails at startup (fail-closed, the same trimming standard as the probe host); the runtime master switch `functions.enabled` (default true, integrated by default; no `functions:` section = no plugins hosted) + per-plugin `enabled` — when off, no child processes are launched and dispatch reports `plugin_disabled` explicitly.

## Comparison with the Direct SDK

Functionally the plugin path is equivalent to the direct SDK (the same registration/dispatch/approval/audit chain). The differences: cancellation/error semantics (the plugin side uses the error closed-set envelope + ctx propagation; the SDK side uses proto responses/errors) and extra IPC latency (one extra local gRPC hop per invocation; go-plugin's measured order of magnitude is sub-millisecond — precise numbers await benchmarks against real game interfaces, and this slice fabricates no measurement conclusions). When a game has no management interface, developers must still provide the entry point (a game-side HTTP/TCP management interface or an SDK provider function); the plugin only adapts existing interfaces and never proxies ones that do not exist.

## Minimal Implementation Scope and This Slice's Deliverables

Delivered (2026-10-20):

1. `core/pluginhost`: the generic resident host engine lifted, with the probe controller switched to consume it (all existing probe tests still passing is the evidence of no behavioral drift).
2. `agents/sidecar/funcplugin`: the contract proto, the host (including the no_go_plugin stub), declare validation/metadata construction.
3. Sidecar assembly and dispatch: `internal/app/agent` assembly/validation hooks, the `internal/agent.LocalHandler` plugin-function dispatch branch, and the `cmd/sidecar-agent` config section; `App.Stop` as the host's sole lifecycle exit.
4. The reference plugin `agents/sidecar/funcplugin/examples/httpget` (HTTP GET mapping).
5. Config validation, the gRPC dependency boundary guard extension, and integration tests for the generic engine and the function host.
6. Second slice: plugin functions merged into the registration report snapshot and real wiring into the server-side registry/approval/audit (`UpstreamClient.SetPluginFunctionsSource` late-bound snapshot source + `Registered.Descriptor()` risk normalization; the store gives same-named ids to SDK/provider permanently; the server registration boundary locks the plugin namespace and rejects presentation — the loop closes with zero server changes).
7. Third slice: registration reconcile (`Host.Reconcile` + a 30s host-owned loop, whose ticks carry no deadline of their own; the registered set only changes on a fully valid response, and revocation closes via the server snapshot rebuild).
8. Fourth slice: the TCP reference plugin `examples/tcp` (the line-protocol frame convention of one connection per invocation, error closed-set translation, and both the self-owned `-target-timeout` and the host deadline bounding each other; mapping success, the five failure translations, and the dual timeouts all have real subprocess tests).
9. Fifth slice: the Python plugin SDK (sharing `sdks/plugins/python/croupier-plugin-sdk` with the probe side; for protocol surface/size measurements/boundaries see docs/zh/research/synthetics-agent-go-plugin.md "Python plugin SDK delivery"; the function-side contract is `croupier_plugin.funcplugin`, the reference plugin is `examples/func_echo.py`; cross-language evidence via `hostdriver` driving the real Go host passes all seven scenarios, with CI `ci-sdk-plugins-python.yml`). Zero host-side changes to adopt — under the same host contract, a Python plugin process is indistinguishable from a Go plugin.

Remaining (later slices): the multi-language (JS) plugin SDK, measured IPC latency vs the direct SDK, and end-to-end acceptance against real game interfaces.
