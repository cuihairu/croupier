---
title: Demo Game (Persistent Simulated Integrator)
---

# Demo Game (Persistent Simulated Integrator)

> **This is a simulated integrator, not a real game.** `sdks/go/cmd/demo-game` exists to unblock
> game-side acceptance of the game-operations surface (todo GAME-002/004/005/006, UI-001) without a
> real integration partner. It only imports the public Go SDK (`sdks/go`) — never `internal/` — so
> it dogfoods the same integration path a real game would take. It does **not** close GAME-001
> (real-integrator contract survey) and does not substitute for real-environment acceptance
> records.

## 1. What It Is

A standalone game process that:

- dials the sidecar-agent SDK gateway (default `127.0.0.1:19091`) through the public Go SDK;
- registers the full game-ops contract surface — **34 functions** (`player.*`, `mail_task.*`,
  `ban.*`, `order.*`, `activity.*`, plus the task-mode `report.generate`), identical IDs and
  schemas to `sdks/go/examples/gameops-demo`;
- serves them against a **durable exactly-once store** (postgres or sqlite via DSN, in-memory
  fallback for tests) instead of browser/process memory;
- seeds a deterministic dataset from a fixed PRNG seed and a fixed anchor date.

Predecessors and why they do not satisfy this role:

| Predecessor                       | Gap that demo-game closes                              |
| --------------------------------- | ------------------------------------------------------ |
| `web` browser-memory demo runtime | No process, no provider registration, no persistence   |
| `sdks/go/examples/gameops-demo`   | In-memory / state-file store; no durable replay proof  |
| `sdks/go/examples/gameops-rpg`    | Different 17-function surface; snapshot store, not SQL |

## 2. Contract Surface

34 descriptors are copied verbatim from `examples/gameops-demo/contracts.go`. A three-way drift
lock (`contracts_drift_test.go`) asserts:

```
gameops-demo set == demo-game set == template set (33 binding IDs) + report.generate
```

- `report.generate` is the only `Execution: "task"` contract; everything else is `sync`.
- List contracts return the `{items, total, page, pageSize}` envelope; idempotency markers
  `already_credited` (order.recredit) and `no_failed_targets` (mail_task.retry_failed) are kept.
- The wire-facing JSON keys are lowerCamelCase; the drift test is the review gate for schema edits.

## 3. Exactly-Once Store

Tables (all prefixed `demo_`, gorm AutoMigrate — this is a demo-owned database, not
`internal/model`, so the numbered-migration contract does not apply):

| Table              | Purpose                                                                  |
| ------------------ | ------------------------------------------------------------------------ |
| `demo_idempotency` | `UNIQUE(game_id, env, op, operation_id)` + `fingerprint` + receipt JSON  |
| `demo_players`     | Players                                                                  |
| `demo_mail_tasks`  | Mail / compensation tasks                                                |
| `demo_bans`        | Ban records                                                              |
| `demo_orders`      | Orders with credit states                                                |
| `demo_activities`  | Activities and `demo_activity_versions` (`UNIQUE(activity_id, version)`) |
| `demo_audit_log`   | Append-only operation audit                                              |
| `demo_meta`        | Seed marker and report run sequence                                      |

Protocol (every mutation handler, one transaction):

1. `Begin(gameID, env, op, operationId, fingerprint)` arbitrates:
   - committed row exists → **replay the stored receipt verbatim**, no side effects;
   - same `operationId`, different fingerprint → `operation_fingerprint_mismatch` error, client
     must pick a new id;
   - stale `pending` row → released, retry proceeds.
2. Business mutation + audit row are written in the same transaction.
3. `Commit` stores the receipt; `Fail` releases the pending row (crash-before-commit is retryable).

`operationId` is supplied explicitly by the caller. Platform-side issuance/propagation of
operation IDs remains GAME-009/GAME-010; "unknown outcome" convergence beyond Fail-release is also
GAME-010 scope.

The store refuses DSNs whose database name is `croupier` — demo tables must never land in a
platform database.

## 4. Deterministic Seed

- `-seed N` drives `mulberry32(N)`; all timestamps derive from the fixed anchor
  `-anchor` (default `2026-11-01T00:00:00Z`), never wall-clock. Golden counts are recomputable
  for any N.
- Fixture IDs that executors depend on (e.g. `task-mail-1002`, order `2026102308`, `act-4002`)
  are identical for every N; only counts and derived attributes vary.
- Same N re-run is a no-op; a different N is rejected unless `-reseed` wipes and re-seeds.
- Seed 7: 26 players, 30 orders, 12 bans, mail/activity sets — the E2E and screenshots rely on it.

## 5. Running It

Flags: `-agent`, `-game-id` (default `demo-game`), `-env` (default `demo`), `-dsn`
(`postgres://…` or sqlite path; empty = in-memory), `-seed`, `-reseed`, `-http-addr`
(default `127.0.0.1:8090`, serves `/healthz` + `/readyz`), `-register-only`, `-anchor`,
`-insecure`.

Local single-process stack (what `scripts/e2e/demo-game.sh` automates):

```bash
./scripts/dev-certs.sh && make build
./bin/croupier-server --config configs/test-sqlite.yaml
./bin/croupier-agent agent --config configs/agent.demo.yaml   # gateway 19091, scope demo-game/demo
go run ./cmd/demo-game -agent 127.0.0.1:19091 -seed 7 -dsn /tmp/demo.db
```

The console switches scope to `demo-game` / `demo` (scope baked into `configs/games.json`). On an
existing database the baked list does not hot-apply — bootstrap only runs when the games table is
empty — and the games REST API cannot mint a hyphenated gameId (it derives the id from the name and
collapses `-` to `_`; tracked as GAME-013). Until GAME-013 lands, an existing deployment needs a
one-time DB insert matching the seed semantics (games row + game_envs binding), as done on
192.168.5.5. Then it opens the player-manage template page and executes bindings against live
functions.

## 6. Deployment (192.168.5.5 demo host)

`docker/docker-compose.demo-game.yml` joins the existing `croupier` compose network:

- `demo-postgres` (postgres:15-alpine, `demo_game` database);
- `demo-agent` (croupier-agent image + `configs/agent.demo.yaml`, dials `croupier-haproxy:19090`);
- `demo-game` (`docker/Dockerfile.demo-game`, CGO off; health `8090` published as `18090`);
- demo-internal network isolates db + game from the platform network.

```bash
docker compose -f docker/docker-compose.demo-game.yml up -d
docker restart croupier-demo-game   # replay-proof drill: balances must not move
```

## 7. E2E

`bash scripts/e2e/demo-game.sh` boots the real three-process stack (server on sqlite, agent,
demo-game) and asserts the chain end to end:

1. provider registration surfaces `player.list` in the scoped registry (34 functions);
2. publish closure: draft (fixture exported from the player-manage TSX template) → publish →
   published console snapshot (11 bindings, localized title);
3. binding execution: list / create / detail with authoritative re-reads via direct invoke;
4. exactly-once across restart: kill demo-game, restart on the same DSN, replay the same
   recharge payload + operationId → identical receipt, balance unchanged; same operationId with a
   different payload → fingerprint conflict rejected.

CI: `.github/workflows/ci-demo-game.yml` runs the same script on `sdks/go/**`, `internal/**`,
server/agent cmd paths. The pagespec fixture is drift-locked to the TSX constant by
`web/src/components/PageRenderer/templates/__tests__/playerManagePagespecFixture.test.ts`.

## 8. Known Boundaries

1. Simulated integrator: GAME-001 stays BLOCKED; real-environment acceptance records are separate.
2. GAME-002/004/005/006 game-side halves are acceptable against demo-game; closing the items still
   waits for the platform halves (GAME-010 durable credentials, GAME-009 sign-off, ARCH-024).
3. GAME-003 additionally waits for GAME-012 (batch ledger); UI-001 waits for GAME-002 + GAME-010 +
   ARCH-024.
4. The E2E fixture exports only the player-manage template; the other four templates are covered
   by the same contract binary but not by a dedicated published-page E2E.
5. Analytics event emission is a compile-time stub (no-op unless `ANALYTICS_INGEST_SECRET` is set;
   HMAC contract already verified). Probe/perf hooks are commented placeholders.
6. Seed anchor is fixed at 2026-11-01 by default; changing `-anchor` changes derived timestamps
   but not fixture IDs.
7. On an existing database the games.json seed does not hot-apply (bootstrap skips a non-empty
   games table), and dashboard manual creation derives an underscored gameId (`demo-game` becomes
   `demo_game`), so a hyphenated scope currently requires a direct DB insert (tracked as GAME-013).
