# External Probe Agent and HashiCorp go-plugin Feasibility Survey

Date: 2026-10-17. Status: the user confirmed the fourth agent-type requirement and designated a survey of HashiCorp go-plugin; the technical route is feasible, but it is not yet implemented nor validated by a cross-language prototype.

## Requirement and Current Gap

The fourth agent type reuses the existing name `synthetics-agent`, runs independently at a network position representing players or service consumers, and actively probes games and third-party services from outside the game process; it is responsible for launching, reusing, and supervising probe plugins implemented by developers. Plugins may use any language that supports the agreed IPC contract; game addresses, environments, credential references, frequency, and timeouts are configured by the platform.

Prior records are not entirely absent: the external-adapter review already adopted separate processes, multi-language support, and resident-process reuse, and the [supervisor design](../design/agent-supervisor-design.md) §10 added `kind=probe-adapter` metadata — but the independent agent classification, the plugin-invocation runtime, and the full backlog have not landed; `core/healthprobe`'s built-in Checkers, declarative protocol steps, and Minecraft SLP are not equivalent to external program plugins.

## go-plugin Survey Facts

- [Official repository](https://github.com/hashicorp/go-plugin): the host launches a subprocess and calls it over local RPC, providing handshake, protocol version, logging, and process-management capabilities; the official positioning is explicitly reliable local connections — it must not be used as a cross-machine Agent→Server communication framework.
- [Official non-Go plugin guide](https://github.com/hashicorp/go-plugin/blob/main/docs/guide-plugin-write-non-go.md): cross-language integration uses gRPC; developers implement the agreed protobuf service, the gRPC health service, and the stdout startup handshake, listening on local TCP or a Unix socket. "Multi-language" means implementing a common protocol — an arbitrary existing executable cannot simply be used as a plugin.
- [Official Python example](https://github.com/hashicorp/go-plugin/blob/main/examples/grpc/plugin-python/plugin.py): beyond the business interface and health, it demonstrates the Controller Shutdown and Stdio services; an implementation must verify each control service the host actually uses against the pinned version — copying only the business RPCs and claiming compatibility is not acceptable.
- Plugin health only means the plugin's communication is usable; the health of the target game or third-party service must come from business probe results. Process isolation by itself provides no permission sandbox, and request cancellation also requires plugin cooperation.

## Recommended Architecture

```text
Server ← shared TCP/TLS session → synthetics-agent
                                  ├─ core/healthprobe: thresholds, windows, statistics
                                  ├─ built-in HTTP/TCP/DB/declarative Checkers
                                  └─ plugin host ← local go-plugin gRPC → resident probe program
                                                                        ├─ game handshake/login/business probes
                                                                        └─ third-party service checks
```

All agents keep sharing the existing foundation for remote network communication; the local subprocess IPC is layered independently. External plugins only need to provide probing capability; platform scheduling, permissions, failure windows, and reporting stay host-side. Third-party services are deployed from the actual consumer's perspective; existing agents can still reuse the common probe kernel, with new independent instances for scenarios needing an independent network perspective or a plugin host.

## This Repo's Constraints and Selection Boundary

2026-10-17 user clarification: the original intent of banning gRPC was to keep game SDKs from carrying oversized runtime and build dependencies — the historical debug integration scenario was ~1.7GB; independent external probe plugins are outside that ban's scope. The transport decision has already been corrected accordingly, so local plugin IPC no longer needs to be treated as a pending-approval exception.

Within this boundary, adopt go-plugin's local gRPC multi-language mode, concentrating dependencies in the plugin host and plugins, keeping `core/healthprobe`'s Checker abstraction and the Agent↔Server↔SDK TCP contract intact. At implementation time, uniformly replace the external-adapter JSONL candidate instead of maintaining two equivalent protocols in parallel, and guarantee the game SDK's dependency closure and artifacts introduce no gRPC. `scripts/check-no-grpc.sh` is still the old global rule; it must be updated in sync with the implementation into a dependency-boundary guard — simply deleting the check is not acceptable.

## Probe Contracts to Define

- `Describe`: plugin identity, version, supported probe capabilities, and contract version.
- `Check`: request ID, target, explicit target category `targetCategory=game/third_party`, probe type, deadline, parameters, and minimally-scoped credentials; returns status, phase, latency, error reason, and bounded details. The category is fixed by the platform's target configuration and must not be rewritten by the plugin; both categories bind to gameId/env, with display and statistics rules in the [dashboard page design](../design/agent-dashboard-design.md) §4.4.
- Results distinguish success, target failure, plugin error, not-connected, timeout, and cancellation; a plugin error must not masquerade as target unavailability, and TCP reachability must not be auto-promoted to a game login success.
- The host sets startup timeouts, concurrency caps, request quotas, crash backoff and circuit breaking, log/output limits, graceful shutdown, and forced cleanup on timeout; plugin processes are resident and reused.
- One plugin process has exactly one lifecycle manager; the existing supervision capability must be reused or extracted — the go-plugin host and the supervisor must not both start/restart the same process.
- Plugins only probe; they get no platform control. Credentials are passed minimally scoped per target and never written into handshakes, logs, or result details.

## Minimal Validation and Implementation Order

1. Refine the plugin protocol and sync the guard per the user's clarified dependency boundary; pin the go-plugin version and the plugin contract version.
2. Implement the host plus Go and Python reference plugins; validate handshake, reuse, timeout cancellation, crash recovery, shutdown, and protocol-incompatible behavior; then provide C#/Java/C++ templates as actually needed.
3. Validate business results against one real game-protocol target and one real third-party HTTP service, covering three distinct situations: network failure, business error, and plugin failure.
4. Assemble the independent synthetics-agent's registration, config, probe scheduling, result uplink, and alert window; connect the panel filtered by game/env and probe point.

## Default Integration and Disable Options (confirmed by the user, 2026-10-17)

The user decided go-plugin and OTel are both integrated by default with disable options, superseding the earlier default-lightweight recommendation. The default build contains the plugin host and OTel capabilities; runtime master switches control each, with per-plugin start/stop and tracing/metrics sub-switches preserved. Default inclusion does not mean launching arbitrary plugins without config or sending telemetry to remote ends: plugins launch only per declarations, and telemetry exports only to the configured collector.

Compile-time trimming uses inverse build tags: `no_go_plugin` removes the plugin host, `no_otel` trims the OTel SDK/exporter assembly; both compose, and normal builds need no extra tag. The retained scope of the lightweight API and trace-context under `no_otel` must be stated explicitly — turning off only the exporter cannot be claimed as removing all OTel dependencies, and business metrics and agent heartbeats must not be switched off together with external telemetry.

| Build option (to implement) | Plugin host                  | OTel SDK/exporter            |
| --------------------------- | ---------------------------- | ---------------------------- |
| default, no tag             | Included, runtime-disablable | Included, runtime-disablable |
| `no_go_plugin`              | Trimmed                      | Included                     |
| `no_otel`                   | Included                     | Trimmed                      |
| `no_go_plugin,no_otel`      | Trimmed                      | Trimmed                      |

The runtime enabled master-off must take precedence over sub-switches, so tracing/metrics sub-switches cannot re-enable a disabled OTel; when off, no exporter or periodic export task may initialize. Today `internal/telemetry/config.go` lets sub-switches set Enabled back to true, and the sidecar CLI does not yet assemble full telemetry config — both must close out with this item; the status quo must not be treated as satisfied.

Runtime shutoff avoids launching subprocesses and holding connection resources, but it cannot eliminate the linked-in implementation size in the binary. Compile-time trimming must place plugin implementations, registration assembly, and generated gRPC code inside `!no_go_plugin` branches, with the off-path using lightweight implementations; unconditional blank imports or shared-type references that pull the dependency back are forbidden. OTel likewise isolates the SDK/exporter assembly from the common call interface.

Enabling plugin config without the compiled capability must fail loudly at startup; disabling at runtime first stops accepting new calls, then drains/cancels in-flight calls per the defined timeout policy and shuts the plugin down — flipping a switch off must not be treated as an unconditional process kill.

The sidecar can use the same host mechanism to offer optional function-adapter plugins, mapping function declarations/invocations onto the game's existing HTTP/TCP/management interfaces; probe plugins and function plugins define their contracts and permissions separately, and neither requires the game SDK to bring in go-plugin/gRPC. The extra IPC latency has been measured (see "Plugin-path IPC latency measurement").

Sizes should be recorded separately for the agent binary, the developer's plugin program, and its Python/.NET/JVM runtime environments; the historical 1.7GB debug scenario is not an estimate of the Go host increment, and cross-platform and final full implementations require re-measurement.

### Host Size Comparison Experiment (2026-10-17)

Environment: linux/amd64, Go 1.27.2, CGO_ENABLED=1, go-plugin v1.8.0, with project dependency resolution yielding gRPC v1.83.2; both groups built with `-trimpath -ldflags='-s -w'` — a build keeping default optimizations while stripping symbol info; debug builds were not measured.

| Subject                                         | Without the go-plugin host | With the host connection path |                     Increment |
| ----------------------------------------------- | -------------------------: | ----------------------------: | ----------------------------: |
| Current sidecar (same source, same temp module) |  32,829,703 B / 31.309 MiB |     33,685,767 B / 32.125 MiB | 856,064 B / 0.816 MiB (2.61%) |
| Clean minimal main program                      |    1,507,488 B / 1.438 MiB |     13,758,727 B / 13.121 MiB |     12,251,239 B / 11.684 MiB |

The experiment used a `/tmp/croupier-plugin-size` temporary modfile and overlay to add the sidecar an `init` gated by the `go_plugin` tag, keeping the `plugin.NewClient`, `Client()`, and `Kill()` paths, executed only when the experiment env var is 1; neither artifact launched a plugin, and project sources and go.mod/go.sum were untouched. The minimal program used the same dependency versions and set `-buildvcs=false`.

The current sidecar baseline's static import chain already contains gRPC: `cmd/sidecar-agent → internal/app/agent → internal/telemetry → otlpmetrichttp → internal/oconf → google.golang.org/grpc`; so 0.816 MiB is the increment on top of the current project's existing dependencies and must not be treated as the cost of introducing go-plugin/gRPC for the first time; the baseline does not include go-plugin packages. That import chain also does not mean the current OTel actually reports over gRPC, nor does it prove the game SDK's dependency closure contains it.

This round measured only the link size of host connection code; Describe/Check and other business services were not implemented, and plugin programs, language runtimes, deployment-package compression, runtime memory, and call latency were not counted; the final default build must be re-measured after the full implementation. The forward `go_plugin` tag used in the experiment served only this comparison and is not a user-decided product option; the official implementation uses default integration, inverse-tag trimming, and runtime-disabling — nothing here guarantees a fixed ~12 MiB addition on all platforms.

## Batch-2 Implementation Record (2026-10-20, todo:36 partially complete)

The trimming switch, concurrency backpressure, and crash backoff/circuit breaking have landed. Deliverables and registrations:

| Deliverable                                                             | Location                                                                                                    |
| ----------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------- |
| `no_go_plugin` trimming: host behind tag                                | `pluginhost/{plugin,probe,controller}.go` `//go:build !no_go_plugin`; `pluginhost/stub.go` (inside the tag) |
| Trimmed fail-closed gate                                                | `config/plugingate_trim.go` (default build's `config/plugingate_default.go` is a no-op)                     |
| Runtime switch (master + per-plugin)                                    | `config/config.go` (`pluginsEnabled` defaults true, `plugins[].enabled`), `runner/runner.go`                |
| Concurrency backpressure (bounded slots + bounded queueing + rejection) | `pluginhost/controller.go` (`gate` semaphore + `pluginQueueWindow`)                                         |
| Crash backoff + consecutive-failure circuit breaker                     | `pluginhost/controller.go` (`failLocked`/`succeed`/`pluginHealth`)                                          |
| Fault-injection knobs                                                   | `pluginhost/examples/echoprobe/main.go` (HOLD/CONC/EXIT_AFTER_CHECKS)                                       |
| Example config notes                                                    | `configs/synthetics-agent.example.yaml`                                                                     |

Semantics and defaults (all overridable via yaml):

- **Master switch** `synthetics.pluginsEnabled`: defaults true (default-integration stance unchanged); false creates no controller and launches no subprocess — plugin-routed targets probe natively over HTTP directly and record `plugin_disabled` in event details. Post-disable in-flight finish policy: `Stop()` first stops accepting new calls, then waits within a bounded 5s window for in-flight calls to finish on their own per-check deadlines, after which the single lifecycle manager kills the subprocess — no traditional "unconditional process kill".
- **Trimming**: under a `no_go_plugin` build, `config.Validate` fails at startup on any `synthetics.plugins` declaration (fail-closed); `pluginhost` compiles only the stub (`Check` always degrades to `plugin_not_compiled`), with the API shape-equivalent to the real controller so the runner compiles in both states. Measured: the trimmed artifact's `go list -deps` closure has no `hashicorp/go-plugin` and no `google.golang.org/grpc` (the default build links both).
- **Backpressure**: `Check` first takes a bounded slot (default 8); the queueing window defaults to 2s; beyond the window the call is rejected with `plugin_backpressure`. The call path spawns no goroutines at all — the queue-waiter is the caller's own function, so unbounded goroutines are impossible. Callers that queue successfully must obtain a freed slot within the window (bounded queueing, not necessarily rejected).
- **Backoff and circuit breaker**: after a failure (launch failure/call error/timeout/crash) the next restart waits `backoff * 2^(consecutive failures-1)`, capped at `backoffMax`. Exponential over fixed stepping was chosen because: for a one-off crash the first retry has the smallest window (default 1s), a thoroughly broken plugin is spaced out geometrically to avoid a restart storm, whereas fixed stepping would restart a dead plugin forever at a constant rhythm; the cap keeps worst-case recovery latency bounded. At `breakAfter` (default 5) consecutive failures the circuit opens: within the 30s window calls degrade immediately with `plugin_circuit_open` and never touch the subprocess; when the window expires a single probe path is allowed — success closes the circuit and zeroes the counter, failure reopens with the original count.
- New degrade reasons `plugin_disabled` / `plugin_not_compiled` / `plugin_backpressure` / `plugin_backoff` / `plugin_circuit_open` share the prefix shape of the existing four, so `runner.errorClass` classifies them without change.

Tests (`-race` all green; trimmed-build cases below run with `-tags no_go_plugin`):

| Case                                                          | Surface                            |
| ------------------------------------------------------------- | ---------------------------------- |
| `TestControllerCrashBackoffAndCircuitBreaker`                 | Backoff + circuit breaker          |
| `TestControllerBackpressureSlotsBoundedRejectionAndQueueing`  | Backpressure rejection/boundedness |
| `TestControllerBackpressureQueuedCallerGetsFreedSlot`         | Bounded queueing success           |
| `TestRunProbePluginRuntimeSwitchTwoStates`                    | Runtime switch, both states        |
| `TestControllerPerPluginSwitchDegradesWithoutLaunch`          | Per-plugin start/stop              |
| `TestTrimmedRuntimeAlwaysDegrades` (-tags no_go_plugin)       | Trimmed stub                       |
| `TestTrimBuildRejectsPluginDeclarations` (-tags no_go_plugin) | Trimmed gate                       |
| `TestSyntheticsPluginKnobsYamlTwoStatesAndValidation`         | Knob yaml/validation               |

Explicitly not done in this batch (listed as before): Python/JS plugin SDKs, ICMP/TCP probe plugins, targetCategory=game/third_party category binding, credential-boundary refinement, and real-game-protocol cross-language acceptance — all remaining scope of todo:36.

## Same-Parameter Trimmed-Build Measurement (2026-10-20, todo:32 close-out)

The batch-2 delivery's real inverse-tag trimming got the deferred same-parameter comparison (honoring the 2026-10-17 experiment's leftover "the final default build must be re-measured after the full implementation"): same source (8508540), same platform linux/amd64, go1.27.2, same `-trimpath` and fixed `-ldflags "-s -w -X …Version=…GitCommit=…BuildTime=…"`; the only variable is `-tags no_go_plugin`:

| Subject                            | Size                                                 |
| ---------------------------------- | ---------------------------------------------------- |
| Default build (with plugin host)   | 15,360,263 B / 14.652 MiB                            |
| Trimmed build (`no_go_plugin`)     | 9,466,119 B / 9.029 MiB                              |
| Trim increment (default − trimmed) | 5,894,144 B / 5.622 MiB (38.4% of the default build) |

- **Trimmed dependency closure**: `go list -deps -tags no_go_plugin ./cmd/synthetics-agent` has no `hashicorp/go-plugin` and no `google.golang.org/grpc` (the default build links both; 75 gRPC-related packages); the trimmed binary's strings show 0 hits for `go-plugin` / `google.golang.org/grpc` (default: 325 / 2319 hits), and the trimmed build still starts normally (`--help` rc=0) — proving the 5.622 MiB increment is exactly the plugin host + gRPC transport stack.
- **Plugin program and language-runtime sizes recorded separately**: the Go reference plugin `echoprobe` (`-trimpath -ldflags "-s -w"`) is 13,984,007 B / 13.336 MiB — a Go plugin statically embeds the language runtime, so no separate interpreter/VM needs deploying; a Python plugin program is a KiB-scale script with the interpreter provided by the deployment environment; runtime dependency size and resident memory were measured in "Python plugin SDK delivery" below; the JS plugin SDK's probe and function surfaces are both delivered (sizes and resident memory measured in "Node.js plugin SDK delivery").
- The 2026-10-17 forward-tag comparison table is kept as a historical experiment; this section is the delivery record for the official implementation (inverse tag + runtime switch).

## Batch-1 Implementation Record (2026-10-20, todo:36 partially complete)

The minimal usable plugin chain has landed: the synthetics-agent plugin host + a Go reference plugin, with Describe/Check working over go-plugin local gRPC and results entering the existing AlertEvent/probe_events chain. Deliverables and registrations:

| Deliverable                                                                                 | Location                                                                                                    |
| ------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------- |
| Plugin contract (proto + generated code)                                                    | `agents/synthetics/pluginhost/protocol/` (separate from the top-level `proto/`, versioned on its own track) |
| Host: discovery/loading/invocation/timeout/degradation                                      | `agents/synthetics/pluginhost/controller.go`, `probe.go`, `plugin.go`                                       |
| Plugin config and validation (explicit declarations, no executable auto-discovery)          | `agents/synthetics/config/config.go`                                                                        |
| Runner routing and explicit fallback (plugin failure → native HTTP probe, reason on record) | `agents/synthetics/runner/runner.go`                                                                        |
| Go reference plugin (echo/fixed result, env-injected faults)                                | `agents/synthetics/pluginhost/examples/echoprobe/`                                                          |
| Global-ban guard → dependency-boundary guard                                                | `scripts/check-no-grpc.sh` (gRPC imports allowed only under `agents/synthetics/pluginhost/`)                |

Pinned versions and licenses (registered in `go.mod`):

| Dependency                       | Version | License    | Purpose                                               |
| -------------------------------- | ------- | ---------- | ----------------------------------------------------- |
| `github.com/hashicorp/go-plugin` | v1.8.0  | MPL-2.0    | Plugin host runtime (handshake, subprocess lifecycle) |
| `github.com/hashicorp/go-hclog`  | v1.6.3  | MIT        | go-plugin internal logging (indirect)                 |
| `github.com/hashicorp/yamux`     | v0.1.2  | MPL-2.0    | go-plugin internal multiplexing (indirect)            |
| `google.golang.org/grpc`         | v1.83.2 | Apache-2.0 | Plugin IPC transport (only within the host boundary)  |
| `github.com/oklog/run`           | v1.1.0  | Apache-2.0 | go-plugin internal (indirect)                         |

This batch's semantic decisions: plugins are declared explicitly via `synthetics.plugins[]`; targets route via the `plugin:` field; on plugin unavailability/timeout/call error, a degrade reason is returned (`plugin_not_configured` / `plugin_unavailable` / `plugin_timeout` / `plugin_error`), and the same target automatically falls back to the native HTTP probe with `pluginFallback`/`pluginCause` recorded in event details; plugin processes are resident and reused, restarted after failure by the host's single manager. Tests in `pluginhost_test.go` (load success/missing degrade/timeout degrade/result return) and `runner_plugin_test.go` (plugin results into the event chain/missing fallback).

Explicitly not done in this batch (listed as before): Python/JS plugin SDKs, ICMP/TCP probe plugins, `no_go_plugin` build-tag trimming and runtime switch, concurrency backpressure and crash backoff/circuit breaking, targetCategory=game/third_party category binding, real-game-protocol acceptance — all remaining scope of todo:36 (the trimming/switch/backpressure/backoff items were delivered by the batch-2 record above).

## Python Plugin SDK Delivery (2026-10-20, shared tail of todo:38/40)

The probe and function surfaces share one Python package, `croupier-plugin-sdk` (`sdks/plugins/python/`, separate from the game SDK directory `sdks/python/`, neither carrying the other's dependencies). First complete measured validation of the cross-language plugin chain:

- **Protocol surface** (`croupier_plugin/serve.py`): the subprocess half-protocol implemented item by item against pinned go-plugin v1.8.0 — magic-cookie exact-value gate (missing/mismatched cookie refuses direct execution with exit 1), `PLUGIN_PROTOCOL_VERSIONS` negotiation (rejects startup if outside the host's speakable version set), single-line stdout handshake `1|1|unix|<sock>|grpc|` (appending `|true` when `PLUGIN_MULTIPLEX_GRPC` is set), the three host-required control services GRPCStdio parking stream / GRPCController Shutdown / grpc.health.v1, and SIGTERM/SIGINT bounded draining (2s grace). stdout carries only the handshake line; plugin logs go to stderr and are collected by the host.
- **Contract surface** (`funcplugin.py` / `probeplugin.py`): the two ABCs `FunctionPlugin` (describe/list_functions/invoke) and `ProbePlugin` (describe/check); error-code closed-set constants with `error_result` local validation (out-of-set values throw before ever reaching the host's rejection); `InvokeInput.context` / `CheckInput.context` expose RPC cancellation (long waits must poll `is_active()`, same semantics as the Go adapter's own timeout).
- **Cross-language validation** (`hostdriver/`): a Go program drives the **real** hosts (`funcplugin.NewHost` and `pluginhost.NewController`, not stubs or mocks) through seven scenarios — func-register / func-invoke / func-envelope (all five business-failure classes kept inside the closed-set envelope) / func-timeout (a short host deadline reports `plugin_timeout`) / probe-check-ok / probe-check-fail / probe-version — all PASS. pytest totals 24 cases (handshake subprocess 7 + contract mapping 17 including 7 integration).
- **CI**: `.github/workflows/ci-sdk-plugins-python.yml` (matrix 3.12/3.13, `pip install -e '.[dev]'` + setup-go compiling hostdriver for integration).
- **Runtime size measured** (honoring the todo:32 addendum promise above; linux/amd64, CPython 3.14): runtime dependencies `grpcio==1.84.0 + grpcio-health-checking + protobuf==7.36.2` total 34 MiB of site-packages (including per-platform native .so wheels; actual deployments can trim); the plugin process's handshake-ready resident RSS ≈ 2.1 MiB / VSZ ≈ 7.7 MiB (grpc native libraries page in on demand and grow under traffic; this is the idle baseline). Comparison: the Go reference plugin is 13.336 MiB statically self-contained with zero deployment dependencies; the Python surface trades "per-plugin deployment size" for "one interpreter + dependency environment per machine".
- **Known boundaries**: protocol negotiation speaks v1 only (the host contract is v1-only too); the GRPCStdio parking stream does no forwarding (plugin logs go straight to stderr, collected by the host as well); the multiplex flag is echoed per protocol but the Croupier host does not enable multiplexing; the stub is committed and regenerated by `protos/gen.sh` (not in the `make proto` main chain); the JS plugin SDK has delivered both probe and function surfaces (see next section).

## Node.js Plugin SDK Delivery (probe + function surfaces) (2026-10-20, remaining sub-item of todo:40)

`sdks/plugins/js/` (`croupier-plugin-sdk-node`, separate from the game SDK directory `sdks/js/`, neither carrying the other's dependencies) mirrors the Python surface's two contracts, probe and function adapter, with jest 57 cases + real-host seven scenarios all green:

- **Protocol surface** (`src/serve.ts`): the same go-plugin subprocess half-protocol — magic-cookie exact-value gate (direct execution refused, exit 1), `PLUGIN_PROTOCOL_VERSIONS` negotiation (v1), single-line stdout handshake, GRPCStdio parking stream / GRPCController.Shutdown (replying before teardown, 50ms gap guarding against cutting off in-flight replies) / grpc.health.v1 SERVING, SIGTERM/SIGINT bounded draining (2s grace). **Binding over TCP**: grpc-js has no unix socket support, so the default is `127.0.0.1:0` (overridable via `CROUPIER_PLUGIN_BIND`), and the handshake line declares `tcp|<addr>` — go-plugin dials per the handshake line, so host-side plugin config is identical to Go/Python plugins.
- **Contract surface**: probing (`src/probeplugin.ts`) offers `ProbePlugin` (describe/check) and `ProbeCall` (target/timeoutMs/payloadJson + an isCancelled/remainingMs cancellation view) with `passResult`/`failResult`; function adaptation (`src/funcplugin.ts`) offers `FunctionPlugin` (describe/listFunctions/invoke) and `InvokeCall` (functionId/paramsJson/timeoutMs + the same cancellation view) with `okResult`/`errorResult`, the error-code closed set `target_unavailable|target_rejected|bad_response`, the risk-level closed set `low|medium|high|danger` (default low), and the id namespace `plugin.<plugin-name>.<function>`. The two surfaces' cookies are independent (`CROUPIER_SYNTHETICS_PLUGIN` / `CROUPIER_SIDECAR_FUNC_PLUGIN`), with the public export aliasing the function surface's cookies as `FUNC_COOKIE_KEY`/`FUNC_COOKIE_VALUE`. Business failures must stay inside the closed-set envelope; throwing exceptions is forbidden (consistent with Python/Go semantics); `check`/`invoke` may be async — grpc-js has no thread pool, so long waits must `await` chunked timers rather than block the event loop (semantics aligned with Python; implementation shape differs).
- **Runtime loading and anti-drift**: contracts load dynamically at runtime from `protos/` via `@grpc/proto-loader` (no code-generation step); `scripts/verify-proto-drift.js` asserts byte equality of `probeplugin.proto` (authoritative source `agents/synthetics/pluginhost/protocol/`), `funcplugin.proto` (authoritative source `agents/sidecar/funcplugin/protocol/`), and `plugin_stdio.proto`/`plugin_controller.proto` (authoritative source: the Python SDK mirror), with `health.proto` (a grpc upstream copy, since grpc-js ships no health service) shape-asserted.
- **Cross-language validation** (hostdriver promoted to the shared `sdks/plugins/hostdriver/`, with Python integration tests re-pathed in the same PR): jest's 57 cases = handshake/negotiation/handshake-line unit tests + both surfaces' contract-mapping unit tests + an in-process full-protocol e2e (cookie rejection → handshake line → health/Describe/Check/Invoke success and failure both ways → bounded Shutdown termination) + real-host seven scenarios: probe-check-ok / probe-check-fail / probe-version (real `pluginhost.NewController`) and func-register / func-invoke / func-envelope (all five business-failure classes kept inside the closed-set envelope) / func-timeout (a short host deadline reports `plugin_timeout`) (real `funcplugin.NewHost`), all driving the compiled node reference plugins; plus a no-cookie direct-execution rejection case. The Python surface's 24 cases regressed all green as usual.
- **CI**: `.github/workflows/ci-sdk-plugins-js.yml` (node 24, `pnpm/action-setup` + `--frozen-lockfile`, four gates build/typecheck/verify:proto/test, setup-go compiling hostdriver for integration; paths cover js and the shared hostdriver).
- **Runtime size measured** (linux/amd64, Node 24.21.0): runtime dependencies `@grpc/grpc-js + @grpc/proto-loader + protobufjs(+long)` total ≈2.7 MiB of node_modules (vs Python's grpcio+protobuf native wheels at 34 MiB; the typescript/jest dev toolchain adds ~52 MiB that does not deploy); handshake-ready resident RSS ≈ 74 MiB / VSZ ≈ 1.49 GB (V8 virtual address space reservation, part of the node process baseline). Compared with the Python surface's RSS ≈ 2.1 MiB — the JS surface trades higher resident memory and a heavier runtime for a smaller dependency footprint on disk.
- **Known boundaries**: protocol negotiation speaks v1 only (the host contract is v1-only too); the multiplex flag is echoed per protocol but the Croupier host does not enable it; TCP binding only (see above); the GRPCStdio parking stream does no forwarding (plugin logs go straight to stderr, collected by the host too); the host-side sidecar function-plugin config consumption path (config surface/distribution chain) is out of this SDK's scope.

## Non-HTTP Target Parameters and the TCP Reference Plugin (2026-10-21, remaining sub-item of todo:40)

`synthetics.targets[].payloadJson` is now the explicit parameter surface for plugin targets: parsed from YAML, passed by the real `pluginhost.NewController` into `CheckRequest.payload_json`, and validated as JSON syntax (`agents/synthetics/config/config.go`, `agents/synthetics/pluginhost/controller.go`). The field serves plugins only; the native HTTP fallback does not interpret it, and degraded targets still use the existing `url/method/expectedStatus/bodyContains` fallback contract. Target parameters are not credentials: connection strings, tokens, and keys must continue to be injected into the plugin process by the deployment environment, and must never be written into committed config or event details.

The Python SDK adds `examples/probe_tcp.py`, a TCP reference plugin: it accepts `{"host":"127.0.0.1","port":<1-65535>}`, returns reachable/failed with a single TCP connect inside the configured deadline, and polls RPC cancellation during long waits. Its semantics deliberately stop at network reachability — port connectivity is not promoted to game login or business success; the `sleep_ms` knob exists solely for cross-language cancellation-injection cases.

Cross-language validation extends the shared `hostdriver -mode probe -scenario probe-tcp`: first listen on a temporary local port and assert the success result and detail; then release the port and assert connection-refused is a target failure producing no `plugin_*` degradation; finally, with an 80ms deadline plus a 1s plugin wait, assert `plugin_timeout`. The scenario is driven jointly by the Python 26-case and JS 60-case real-host integration suites (host in Go; plugins in Python and Node respectively).

## ICMP Probe Type and Capability Degradation (2026-10-21, remaining sub-item of todo:40)

`core/healthprobe` adds the `icmp` target closed set: `address` accepts hostname/IP, `timeout` defaults 2s, `count` defaults 1; the shared kernel performs a single ICMP echo assertion and never promotes network reachability to business success. Linux raw sockets require `CAP_NET_RAW`; a permission failure maps to the explicit `capability: CAP_NET_RAW is required for ICMP`, surfacing via `lastReason` / probe detail into the health view — never a panic and never masquerading as target unreachability.

The synthetics-agent's native targets add `type: icmp` (`address`/`count`); the default empty value still means `http`; a `plugin` target may carry `type: icmp` to annotate plugin semantics. Business and capability failures reported by external plugins stay as-is and no longer trigger HTTP fallback — fallback now means only host-side trouble such as the plugin runtime being unavailable/disabled/timed out. The Python/Node SDK reference plugin `probe_icmp` is driven by the shared `hostdriver -mode probe -scenario probe-icmp-capability`, deterministically asserting the capability error is a plugin-target failure producing no `plugin_*` degradation.

## Plugin-Path IPC Latency Measurement (2026-10-21, remaining sub-item of todo:38)

Measurement tool: `hostdriver -mode funcbench -scenario bench-invoke` (`sdks/plugins/hostdriver/`, shared by the three-language SDK suites; jest asserts output shape only, no timing). Within one process it first runs the **inline** baseline (in-process JSON parse/validate/marshal with the reference plugin's echo semantics — the invocation work itself with zero IPC), then launches the **real** `funcplugin.NewHost` to sequentially drive the reference plugin as the **plugin** path (host dispatch + a local go-plugin gRPC round trip + the plugin's echo work); n=300 sequential calls, 20 warmups, reporting per-call mean/p50/p95/max. Both paths share the same upstream chain (registration/approval/audit/routing — validated in todo:38 batch 2), so the plugin path's extra latency is exactly this hop difference.

Measurements (shared dev machine, linux/amd64, sequential load, two independent runs showing jitter; Python is venv CPython 3.14.4 + grpcio 1.84 + protobuf 7.36, JS is Node 24.21.0):

| Path                                                                | run1 mean / p50 / p95 / max             | run2 mean / p50 / p95 / max      |
| ------------------------------------------------------------------- | --------------------------------------- | -------------------------------- |
| inline baseline (in-process echo, same magnitude in both languages) | 0.002 / 0.001 / 0.004–0.005 / ≤0.031 ms | —                                |
| plugin hop (Node 24 + grpc-js)                                      | 0.573 / 0.534 / 0.789 / 2.282 ms        | 0.773 / 0.719 / 1.107 / 3.304 ms |
| plugin hop (Python + grpcio)                                        | 0.583 / 0.560 / 0.703 / 2.033 ms        | 0.593 / 0.578 / 0.726 / 1.484 ms |

Conclusion and boundaries: **each function call through the plugin adapter adds ≈0.5–0.8ms (p50 0.53–0.72ms)**; the difference between the two languages is within the run-to-run jitter of the shared dev machine (the cost is dominated by the local gRPC hop and host dispatch, not the interpreter). Against the inline ~2µs, the plugin path is roughly 250–400× — imperceptible for management-plane/health-check low-frequency calls; high-frequency batch paths should batch/aggregate inside the plugin or switch to the direct SDK. Boundaries: shared dev machine, sequential calls, including host dispatch and framework overhead — not a throughput ceiling (concurrency and dedicated hardware to be measured separately); the full deployment chain end-to-end (SDK→agent→sidecar→plugin cross-process stacking) is not covered — that acceptance belongs to todo:38's remaining "real game interface end-to-end acceptance".

## Credential Boundary Refinement (2026-10-23, remaining sub-item of todo:36)

The survey's contract line "参数与限定范围的凭证" and the boundary rule "凭证按目标最小范围传递，不写握手、日志或结果详情" are now implemented, keeping the closed shape: config carries **references, never values** — a synthetics target declares `secretEnv: [ENV_NAME...]` (`agents/synthetics/config/config.go`), and the check-time resolution lives in the probe controller (`agents/synthetics/pluginhost/controller.go`, `resolveCredentials`). The resolved values ride only the per-check `CheckRequest.credentials` map (`probeplugin.proto` field 4); the Describe handshake and the go-plugin handshake stay value-free by construction, and a missing or empty reference degrades with a new explicit cause `plugin_credential_missing` that names the variable but never carries a value, before any plugin subprocess is touched.

Redaction closes the write-back half of the rule: before the host maps a plugin answer onto the shared result — and before a call error reaches the shared engine's log and degradation causes — every resolved credential value is scrubbed to `[redacted]` (longest value first, so a prefix value cannot leave a partial secret behind); the wrapper preserves the error chain (`Unwrap`), so deadline/unavailable classification is unchanged. Because the scrub happens inside the synthetics controller, plugin failures and details reaching `probe_events` / AlertEvents / the log sink are clean regardless of how chatty a plugin is.

Cross-language acceptance rides the shared driver: `hostdriver -mode probe -scenario probe-credentials` sets a host env var, drives a real subprocess, and asserts the three facts — the plugin received exactly the resolved credential ("N credential(s)" detail), a deliberately chatty plugin's echoed secret comes back scrubbed in both error and detail, and a missing reference degrades before launch. The reference plugins (Go `echoprobe`, Python and Node `probe_echo`) mirror the `cred...`/`leak...` target semantics; both SDK `CheckInput` views expose `credentials` with the doc warning "use in-process, never echo into error/detail" (the host scrub is a safety net, not a license to leak). Evidence: `go test -race` on `agents/synthetics/pluginhost` + `config` (delivery, missing-reference degrade, leak scrub, Describe cleanliness, redactor unit cases), pytest 27 passed (Python `probe_echo` through the real host), jest 61 passed (Node `probe_echo` through the real host), `scripts/check-no-grpc.sh` PASSED, `-tags no_go_plugin` build/tests green. Remaining on todo:36: real-game-protocol target acceptance.

Related: [Agent inventory](../design/agents-inventory.md), the health review, and the root `todo.md`.
