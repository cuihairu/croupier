# Coverage Exemption Ledger

Per-package statement coverage for internal/ targets is 100%. This document is the **sole exemption list**: after exhaustive review, each defensive branch judged "unreachable under the current implementation" is recorded here with a structured argument and its invalidation condition. Every exemption block ships with a pin test locking the invariants it depends on — if a future implementation change invalidates the argument (the branch becomes reachable), the corresponding test fails and demands real error-path cases; at that point coverage must be added, not the exemption renewed.

## Classification and Admission

Only **class C (defensive fallback)** exemptions are accepted: branches defending hypothetical anomalies "beyond standard-library/upstream invariants", with an exhaustive structural argument available. The following do **not** justify an exemption:

- "The call chain upstream happens not to pass a certain value" (a private function can be tested directly to construct it) — precedent: the empty-operationId branch of `createUnboundContractsForSource`; production rejects whole sources upstream, but a direct test passing a blank element makes it genuinely reachable; a direct test was added instead of an exemption.
- Branches deterministically reachable by the error-injection toolbox (gorm After callbacks injecting errors per table/SQL, sqlite RAISE triggers, read-only PRAGMAs, closing connection pools, `exporterFunc` channel confirmation, etc.).
- Coverage flakiness of the "goroutine scheduling luck" kind from fire-and-forget goroutines — precedent: the per-exporter goroutines of `TracerProvider.EndSpan`; made deterministic by waiting on a channel for the delivery signal instead of an exemption.
- Timing choreography of lock-phase alignment/probabilistic rounds — precedent: the singleflight re-check branch of `Router.GameDB`; deterministically triggered through the callback test seam `gameDBInflightHook`, replacing the retired 24-round lock parking + TryLock spin choreography (that choreography could miss entire rounds under full load — exactly the flakiness source; timing branches use injection points).

## Current Exemption List (4 items)

### 1. `internal/api/menu` — the `!check(parent)` branch of filterAccessibleTree

**Location**: `internal/api/menu/service.go` (the `filterAccessibleTree` main loop, `!check(parent) { continue }`).

**Argument**: for any entry `item`, reaching this branch in the main loop requires `check(item)=true`, `item.ParentID != nil`, and the parent existing in `byID`. The recursive structure of `check` guarantees: when `check(item)=true` and the parent exists, `check` has already executed `ok = check(parent)` internally and returned true (otherwise the final `ok` would be false). The main loop's second `check(parent)` hits the `visited` cache and returns the same final value, so `!check(parent)` is always false.

Key invariants (locked by pin tests):

- `check` computes each node's final value exactly once and caches it; all later calls hit the cache (value consistency);
- a cycle member's final value is always false (false is set before recursing as a cycle guard, and false propagates along the cycle);
- `ParentID`, `byID`, and the permission set are immutable for the whole function execution (the main loop and the recursion see the same world).

**Invalidation condition**: `check` becomes re-computable on re-entry (e.g. dynamic visibility, cache removed), or the main loop and the recursion resolve parents differently.

### 2. `internal/api/openapi` — the `"fn-"` prefix branch of unboundFunctionID

**Location**: `internal/api/openapi/service.go` (the tail `return "fn-" + out` of `unboundFunctionID`).

**Argument**: the function first normalizes the input to the charset `[a-z0-9._-]` (lowercase folding + each out-of-charset rune replaced with `-`; multi-byte runes cannot survive to the output), then trims both ends with `strings.Trim(out, ".-_")`. The cutset `".-_"` is **exactly the set of all non-alphanumeric members of the charset**, so a non-empty result's first character must be in `[a-z0-9]`, the preceding `if out[0]` alphanumeric test is always true, and execution always takes `return out`. The `"fn-"` branch is the self-evidencing fallback of the charset invariant.

**Invalidation condition**: the builder charset admits other symbols (e.g. `~`), or the first-character normalization strategy stops using Trim.

### 3. `internal/platform/monitoring/certificates` — the `len(certs)==0` branch of fetchCertificateInfo

**Location**: `internal/platform/monitoring/certificates/certificates.go` (`fetchCertificateInfo`; the argument is also commented in place in the production code).

**Argument**: a successful `tls.DialWithDialer` handshake guarantees `ConnectionState().PeerCertificates` is non-empty — the Go standard library TLS client implements no anonymous (aNULL) suites, so a server presenting no certificate fails the handshake and takes the err branch above; the Go client also supports no client-side PSK, and even in TLS 1.3 session resumption the server still sends the Certificate message. This branch is a defensive fallback beyond standard-library behavior, and both dialer and config are constructed inside the function with no injection seam, so a fake server cannot reach it (Go's `tls.Server` likewise requires the client to present a certificate to complete the handshake).

**Invalidation condition**: the Go standard library introduces client-side PSK/anonymous suites, or the function becomes injectable with `tls.Config`/dialer.

### 4. `internal/security/identity` — the WeChat Exchange openid fallback and the double-absence fallback

**Location**: `internal/security/identity/wechat.go` (at the end of `Exchange`: `if openID == "" { openID = ...user.OpenID }` and `if openID == "" { return ... "identity has no openid" }`, registered in round 27 on 2026-09-29).

**Argument**: the guard above — `if strings.TrimSpace(token.AccessToken) == "" || strings.TrimSpace(token.OpenID) == ""` — already rejects an empty `access_token`/`openid` and returns an error; after it passes, `openID := strings.TrimSpace(token.OpenID)` is necessarily non-empty, so both `if openID == ""` tests are always false. Self-evidencing double insurance (the same construction as item 2's `"fn-"` prefix): openid semantically comes from the token endpoint, and the userinfo openid field is mere redundancy.

**Pin**: the first arm of `TestWeChat_ExchangeWings` in `wechat_generic_wings_r27_test.go` locks "token response missing access_token/openid → the guard at line 122 fires before the fallback"; if the guard is removed or loosened, that arm fails and demands real fallback-path cases.

**Invalidation condition**: if the line-122 guard is removed/loosened (e.g. allowing tokens without openid, preferring userinfo), the branches become reachable — coverage must then be added, not the exemption renewed.

---

## cmd/ Coverage Scope and Exemption List (extended 2026-09-22)

cmd/'s coverage target differs from internal/'s: the binary assembly layer legitimately contains process boundaries and system-mutation surfaces, while **all policy logic has sunk into internal/** (already 100%). Current readings (`go test -cover`, after round 26 on 2026-09-29): `cmd/server` ≈98%, `cmd/sidecar-agent` ≈99% (named **`sidecar-agent`**; plan A landed 2026-10-11: only the source directory renamed, the artifact name unchanged — see the [Agent inventory](../design/agents-inventory.md) §9), `cmd/analytics-export` 87.7%, `cmd/schema-validator` ≈91%, `cmd/ingest/cmd` 99.5%. Beyond the exemptions below, all other unreachable cmd/ branches have been closed under "construct first; exempt only when construction is impossible" (including the full-semantics fixture REST, the full startCluster assembly matrix, full interconnect routing, the service manager state machine, the service mutation command's five-body branch matrix, the schema-validator archive dissection edges, and direct tests of the dashboard fixture assembly wings). `pkg/protocol` is 100%; `pkg/pb/**` is protoc-generated (`make proto` output, updated with the generation chain) and outside the hand-written test scope.

### cmd-1. (process boundary) `main` / `Execute` of all binaries

**Location**: the `root.go` / `main.go` entry functions of `cmd/{server,agent,ingest,schema-validator,analytics-export,analytics-worker,check-db}`.

**Argument**: `main` and `Execute` are process assembly boundaries (global flag binding, cobra execution, `os.Exit`, signal and lifecycle ownership by the init process). Executing them in-test conflicts with the tested process's own lifecycle. Each command's `run*` strategy function is directly tested — the entry only forwards and holds no branch logic. Same family: the HTTP `ListenAndServe` error branch inside runServer in `cmd/server/root.go` (registered in round 24 on 2026-09-29) — the branch body is `os.Exit(1)`; any in-process construction (a taken port triggers it) kills the test binary outright.

**Invalidation condition**: none (structural boundary). If `Execute` grows unit-testable branch logic, extract it into a directly testable function rather than testing at the entry.

### cmd-2. (system mutation) the service mutation command bodies of server/agent [no exemption block after the 2026-09-29 narrowing]

**Location (historical)**: in `cmd/server/service.go` and `cmd/sidecar-agent/service.go`, the `svc.Install()/Uninstall()/Start()/Stop()` calls inside `run*ServiceInstall/Uninstall/Start/Stop/Restart` and the printing after them.

**Original argument (invalidated)**: install/uninstall/start/stop/restart trigger **real system-level mutations** (writing systemd units, starting/stopping system services), which a unit-test process cannot execute.

**2026-09-29 narrowing (round 25)**: `newKardianosService` is a package-level seam variable in both packages; `createService`/`createServerService` call it, so with a fake injected the five bodies **no longer touch real systemd** — the "system-level mutation" premise only holds for the un-injected path, which is exactly the line replaced. Each package gained a `service_wings_r25_test.go` (controllable fake: status/statusErr + per-method errors + call counters) whose branch matrix is fully directly tested: install (creation failure / already-exists early exit / Install error / success), uninstall (status query failure / not found / Stop error while running / Stopped-uninstall error / success while running with a 2s wait wing), start (status query failure / not-found hint to install / already-running early exit / Start error / success), stop (status query failure / not found / already-stopped early exit / Stop error / success), restart (status query failure / not found / Stop error while running / Stop success then Start error while running with 2s wait wing / direct start when stopped). cmd/server additionally gained the Start background goroutine's panic-recovery wing (a runServerFunc stand-in panicking → recover → svc.Stop) and the `wd()` Getwd-failure wing (chdir into a deleted directory). From this round on the item **covers no blocks** and is kept as the narrowing record; all remaining uncovered blocks in service.go belong to cmd-6.

**Invalidation condition**: not applicable (no exemption block). New command branches should be tested via the seam-injection pattern.

### cmd-3. (main shell packages) cmd/check-db, cmd/analytics-worker, cmd/ingest

**Location**: the three packages in full (lines 7–108).

**Argument**: each package is main assembly (connection-string parsing + calls into internal diagnostic/export logic) with no policy branches; the called logic in internal/ is already 100%. check-db is an ops diagnostic tool — an assembly error exits the process with a message.

**Invalidation condition**: re-evaluate when if/err branch logic appears in the packages (then argue block by block under class C admission).

### cmd-4. (class C defense) schema-validator — extractTarGz's `invalid path` double insurance and the `out.Close` failure branch

**Location**: `cmd/schema-validator/main.go` (`extractTarGz`: the `!strings.HasPrefix(target, destAbs+sep) && target != destAbs` branch; the `out.Close()` err branch).

**Argument**:

- The `invalid path` branch: `target = filepath.Join(destAbs, cleanName)`, and `cleanName = filepath.Clean(hdr.Name)` has already blocked `..` prefixes and embedded `../` in the previous step. Go's `Join`/`Clean` semantics guarantee: a non-empty `cleanName` through `Join` yields either **exactly** `destAbs` (when `cleanName` normalizes to `"."`) or a path prefixed with `destAbs+PathSeparator` — the two disjuncts of the binary test cover every possible `Join` output, so the condition is always false. It is the self-evidencing double insurance of the normalization invariant (the same construction as internal item 2's `"fn-"` prefix).
- The `out.Close()` branch: after a successful `OpenFile`, a `Close` failure is limited to ENOSPC-style delayed write errors (flush at close), which the test environment cannot construct deterministically and which does not affect extraction semantics (the data has already been `io.Copy`-ed).

**Invalidation condition**: normalization chain changes (dropping the `Clean`+`Join` pair, loosening the `..` check) making `invalid path` reachable; or a write-path abstraction (e.g. an injectable fs) making Close failure injectable.

### cmd-5. (class C defense) cmd/server — startCluster's NormalizeConfig dead branch

**Location**: `cmd/server/cluster.go` (the `NormalizeConfig` err → standalone branch).

**Argument**:

- `NormalizeConfig` returns a nil error for any input (a default-filling normalizer with no failure path); the err branch is dead-code defense.

> **2026-09-29 narrowing (round 23)**: the formerly parallel exemption `DBOwnerResolver.EnsureTable` err → standalone has landed a real case (`TestStartCluster_OwnerEnsureTableFailure` in `cmd/server/cluster_ddl_wings_test.go`: pre-create the member table on a writable connection, then reopen with `mode=ro`; the member table's `EnsureTable` passes idempotently while the owner table's CREATE is refused) — the old "unconstructible over one connection" argument fails; two connections suffice to fork the two tables' DDL fates. The same test file also covers the reconcile Touch-failure warn wing and the interconnect Serve error wing.

**Invalidation condition**: `NormalizeConfig` gains real validation — then add tests via the error-injection toolbox.

### cmd-6. (class C defense + cross-platform surface) the environment-always-succeeds guards and windows/darwin branches of server/agent service.go

**Location**: `cmd/server/service.go` and `cmd/sidecar-agent/service.go` (the three guards and windows branch of `createServerService`/`createService`, the windows/darwin cases of `defaultServerConfigDir`/`defaultConfigDir`, and the `os.Executable` failure fallback to "unknown" in the tail `exePath()` of cmd/server); `cmd/sidecar-agent/root.go` (the `os.Hostname` empty-value fallback of `resolveAgentID`).

**Argument**:

- `os.Executable` always succeeds for one's own process (no arguments, no environment dependencies); the err branch is defensive fallback.
- The `filepath.Abs(execPath)` re-conversion: `execPath` was already confirmed absolute in the previous step, and `Abs` on absolute input just `Clean`s and returns without calling `Getwd`, so it always succeeds. (The relative-path branch is itself testable: deleting the cwd makes `Getwd` fail and `Abs` error; `TestCreateService_AbsFailureFallback`/`TestCreateServerService_AbsFailureFallback` cover the fallback chain.)
- The `os.Hostname()` empty fallback of `resolveAgentID`: `Hostname` returns empty only on pathological system-call failure, unconstructible in a test process.
- The windows branch (`StartType: auto`) and the windows/darwin cases of the `defaultConfigDir` family: real cross-platform branches, reachable on the corresponding OS, unreachable in a linux test environment. These are environmental blind spots, not dead code — the normalization semantics of `platformFamily` ("linux-systemd"→linux, "windows-service"→windows, "darwin-launchd"→darwin) are locked by `TestPlatformFamily` (one per package).

**Invalidation condition**: when the above system calls become injectable (e.g. hostname/executable variables), add tests via the error-injection toolbox; when CI gains windows/darwin runners, the windows branch becomes coverable and this item should be removed.

**2026-09-22 same-round fix record** (not an exemption, archived here for reference): `service.Platform()` returns `"linux-systemd"` on linux+systemd, while the historical code compared `== "linux"` exactly — always false — so the systemd unit's `network-online` dependency, the `croupier` run user, and the status command's Linux management hints never took effect (the same disease in both server/agent). A `platformFamily` normalization was introduced with three-state/per-platform branch cases. The same round also deleted the canonical TLS/OutboundTLS redundant views in cmd/sidecar-agent's AgentConfig parsing (key sets fully overlapping the inline fields; the branches were always false) and fixed a defect in the Abs failure fallback chain where the receiving variable was zeroed by the err return, silently dropping relative config directories.

### cmd/ residual partially covered surfaces (not exemptions, recorded honestly)

`cmd/server/dashboard_fixture.go` (after the round-26 closure on 2026-09-29 only 14 blocks remain in the file, all unreachable wings registered in the test files' header comments): the E2E fixture assembly surface was closed by direct tests in `fixture_wings_r26_test.go` — the pure helper matrix (addr normalization/candidate fallback), StartDashboardFixture's precondition error wings (five addr parse failures, MkdirTemp failure, the empty-BaseDir own-directory branch), the five-step failure wings (Sscanf/serve listen taken/provider/agent directory/SDK cmd.Start/fixture API port, including the `CROUPIER_E2E_PUBLISH_REVIEW` env override wing), the start* manual-construction error wings, the ensureUIScope nil/missing-table/trigger-rejects-write four-rung ladder (including the Router non-nil wing), the fixture API endpoint all-method arms (health dual-state/SDK tri-state/calls five arms/audit tri-state/provider both endpoints), ready()'s empty-store/missing-function/missing-contract-table three wings, CleanupScope's six-rung error ladder (missing tables ×3 + BEFORE UPDATE/DELETE triggers ×3, each first laying real rows so UPDATE/DELETE hits), and Close's full runtime-handle walk (control/dual HTTP/telemetry/Router/own BaseDir deletion chain). Registered unreachable: the `fixtureFreePort` loopback random-port Listen error wing and its passthrough (fd exhaustion unconstructible), the `filepath.Abs` failure wing inside the candidate loop (the window where Getwd has succeeded yet fails again in-process cannot be constructed stably), startServer's telemetry init wing (falsified by the round-24 probe), the ensureUIScope error passthrough wing inside startServer (boot's own DB migrates fresh every time; no injection seam; its deeper ladder is directly tested), the four Serve goroutines' non-ErrServerClosed error log wings (the Shutdown path is filtered; sudden listener failure has no deterministic injection surface), the json.Marshal wings of `Game.SetEnvs`/`startSDKLocked` (pure string fields, no failure path), and `WaitReady`'s 60s timeout wing (hardcoded deadline; cost out of proportion to value). The real end-to-end orchestration is carried by the `real-dashboard` E2E suite.

`cmd/server/root.go` runServer's runtime wing family (round 24, 2026-09-29, registered in `root_wings_r24_test.go` comments): the telemetry init failure wing (the construction path falsified by six-shaped probes — the OTLP client does not pre-parse the endpoint, EAGER always returns nil error) and the Shutdown error wing, the error sub-wings of the listener/HTTP/Router Close success paths inside graceful shutdown and the 30s timeout wing, the session prune ticker body (hardcoded 30s + 5min staleness threshold, no injection point), and tcpListener.Serve's non-Canceled error wing (Close→nil and cancel→Canceled are filtered). runServer's main chain and the full mode/debug/logLevel/gin matrix are genuinely covered by three complete boots + SIGINT graceful shutdowns.

## tools/ and scripts/ Coverage Scope and Exemption List (extended 2026-09-23)

The tools/ adapters and the protoc plugin are small standalone deployables with the same coverage target as cmd/: assembly/registration/invocation chains are fully directly tested (the fake agent runs over tcptr; main error paths run through stdin/stdout fd injection and the fatalExit seam), while process boundaries and construction-always-succeeds branches are exempt. Current readings (`go test -cover`): `tools/adapters/prom` 93.8%, `tools/adapters/http` 97.0%, `tools/protoc-gen-croupier` 99.3%, `scripts` (gen_bcrypt) 80.0%.

### tools-1. (class C defense) two Marshals in the prom adapter

**Location**: the `proto.Marshal(regReq)` in `run()` and the `proto.Marshal(hbReq)` in `heartbeatLoop()` error branches of `tools/adapters/prom/main.go`.

**Argument**: both messages' fields are string/slice/nested-message only (`buildProviderConnectRequest` constructs one; `ProviderHeartbeatRequest` has two string fields); protobuf Marshal of purely typed constructed messages always succeeds in-process, with no constructible trigger.

**Pin**: `TestBuildProviderConnectRequest` in `adapter_run_test.go` asserts Marshal always succeeds for both regReq and hbReq.

**Invalidation condition**: when a message gains a non-serialization-safe field type (e.g. a custom Marshaler with an error path), add tests via the error-injection toolbox and remove this item.

### tools-2. (class C defense) four spots in the http adapter

**Location**: the two `http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)` error branches in `tools/adapters/http/main.go` (the `q.Encode()`-constructed URLs of the alertmanager alerts and grafana search calls) and the two Marshal error branches (regReq in `run()`, hbReq in `keepAlive()`).

**Argument**: `u.String()` yields a URL already validated by `url.Parse`, and `NewRequestWithContext` always succeeds for a valid method+URL+nil body (the generic invoke's in.Method path already covers the error side via `TestInvokeGenericBadMethodFailsNewRequest`); the Marshal branches argue as in tools-1.

**Pin**: `TestMarshalProviderMessagesAlwaysSucceeds` in `adapter_run_test.go`.

**Invalidation condition**: as in tools-1; on the NewRequestWithContext side, if the path changes to concatenating user-controlled raw URLs, that branch becomes coverable.

### tools-3. (class C defense) protoc-gen-croupier main's Marshal(resp)

**Location**: the `proto.Marshal(resp)` error branch of `main()` in `tools/protoc-gen-croupier/main.go`.

**Argument**: `CodeGeneratorResponse` contains only file names and byte data (string fields); Marshal of a constructed instance always succeeds, with no constructible trigger.

**Pin**: `TestCodeGeneratorResponseMarshalPin` in `main_plugin_test.go`.

**Invalidation condition**: as in tools-1.

### tools-4. (process boundary, same shape as cmd-1) protoc-gen-croupier's fatalf itself

**Location**: `fatalf()` in `tools/protoc-gen-croupier/main.go` (containing `os.Exit(1)`).

**Argument**: `os.Exit` belongs to the tested process; an in-process assertion would kill the test process. A package-level seam `fatalExit` was added, and main()'s four error paths (read stdin / unmarshal / marshal / write stdout) are asserted via panic-type stand-ins in three cases of `main_plugin_test.go` (directory fd EISDIR / garbage bytes / read-only `/dev/null` EBADF); the body itself never goes through in-process assertion.

**Invalidation condition**: none (structural boundary, same reasoning as cmd-1).

### tools-5. (class C defense) the panic branch of scripts/gen_bcrypt

**Location**: the `panic(err)` of `main()` in `scripts/gen_bcrypt.go` (the `bcrypt.GenerateFromPassword` error branch).

**Argument**: a one-shot ops utility with fixed input `"admin"` + `bcrypt.DefaultCost`; in-process GenerateFromPassword always succeeds, so the panic branch has no constructible trigger. main's body is directly tested in the same package by `TestGenBcryptMain` in `gen_bcrypt_test.go`.

**Pin**: `TestGenBcryptMainPin` (always succeeds + hash round-trip verifying back to "admin").

**Invalidation condition**: when the input becomes externally injectable (flag/env), add tests via the error-injection toolbox and remove this item.

## Review Process

1. For each exemption candidate, enumerate every reachable path exhaustively (including cache consistency, cycles, byte/multi-byte runes, Unicode folding and other corners), falsifying every attempt at "constructible trigger";
2. Try the error-injection toolbox and direct-test constructions first; only on failure does an exemption enter;
3. An exemption must land in all three places: an entry in this document (with its invalidation condition), an in-place comment (in the test file or production code), and a pin test locking the invariants;
4. When the implementation changes, run the pin tests first — a failure means the argument is dead, and coverage must be added, not the exemption renewed.
