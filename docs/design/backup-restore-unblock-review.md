# Backup Restore Drill: Unblock Decision Review

Date: 2026-11-28. Status: **proposal awaiting owner decision**. Deliverable
for DEPLOY-002's named blocker (deploy-inventory-2026-11-28.md §9) and the
shape decision OPS-008 Stage C reserved ("恢复演练模式…暂缓，不盲建").
**Nothing is implemented here** — no image change, no backup created, no
drill run. The proposal narrows the owner decision to one signature.

## 1. Grounding (code + live facts, 2026-11-28)

| Fact                                                                                                                                                           | Source                                                          |
| -------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------- |
| postgres executor shells out via PATH: `run(ctx, "pg_dump", …)` with `--format=plain --no-owner --no-privileges`                                               | `internal/platform/backupexec/executor.go:261` (`dumpPostgres`) |
| Plain-SQL dump → restore needs only `psql -f`, not `pg_restore`                                                                                                | executor args above                                             |
| sqlite is the only in-process dump precedent (`VACUUM INTO`)                                                                                                   | `executor.go` `dumpSQLite`                                      |
| server runtime base `gcr.io/distroless/static:nonroot`; CGO-disabled static binary; busybox already added via `COPY --from` + `--install`; runs `USER nonroot` | `docker/Dockerfile.server`                                      |
| `distroless/static` has no dynamic loader — musl-linked pg_dump cannot be COPY'd in                                                                            | deploy-inventory §9 (container probes rc=1, twice)              |
| live postgres is `postgres:15-alpine` (major 15); pg_dump major must be ≥ server major                                                                         | deploy-inventory §3                                             |
| one compose service already uses `distroless/base-debian12:nonroot` (glibc loader present) as runtime base                                                     | `docker/docker-compose.yml:232` (sdk-demo-go)                   |
| `backups` / `db_sources` both 0 rows; scheduler not configured                                                                                                 | deploy-inventory §9                                             |
| platform-level backup path exists: no registered source → falls back to `Config.Database.Driver/DataSource` (meta DB)                                          | `internal/api/ops/backups.go:128-135,209-223`                   |
| live deploy is single-DB mode → the meta DB dump covers all tables on prod today                                                                               | deploy-inventory §1                                             |

## 2. D1 — how the server image gains pg_dump

| Option                                                       | Change                                                                                                                                                                              | Assessment                                                                                                                                                                                                                                                                                                                         |
| ------------------------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| (a) **base-debian12 + COPY pg_dump closure** (recommended)   | server-only `RUNTIME_IMAGE=gcr.io/distroless/base-debian12:nonroot`; builder stage `FROM postgres:15-bookworm` copies `pg_dump` + its `ldd` closure into `/out`, then `COPY --from` | Keeps the distroless no-package-manager posture; glibc loader present (in-repo precedent: sdk-demo-go); binary from official Debian postgres image pinned to live major 15; the lib closure is derived mechanically via `ldd` at build time — not hand-enumerated; CI smoke: run `pg_dump --version` as nonroot in the built image |
| (b) pure-Go postgres dump in backupexec                      | Go-side logical dump                                                                                                                                                                | No image change, but postgres has no single-statement equivalent of sqlite's `VACUUM INTO`; a hand-rolled dump is DR-critical fidelity risk and would run exactly when everything else is broken; library landscape NOT VERIFIED this pass. Fallback only if (a) is refused on image-policy grounds                                |
| (c) full base switch to debian-slim/alpine + package manager | apt/apk in runtime image                                                                                                                                                            | REJECTED: abandons the distroless posture; Dockerfile's own design note says "不引入包管理器"                                                                                                                                                                                                                                      |
| (d) COPY musl pg_dump into distroless/static                 | —                                                                                                                                                                                   | REJECTED: static base has no loader (deploy-inventory §9 already ruled this out)                                                                                                                                                                                                                                                   |
| (e) sidecar dump container / host cron dump                  | —                                                                                                                                                                                   | REJECTED: bypasses the backupexec → ObjectStore → sha256 verify chain and violates the acceptance clause "不新增未经设计的恢复服务" — same ruling as §9's host-manual dump rejection                                                                                                                                               |

## 3. D2 — scope of the first real backup

- (a) **Platform meta-DB path first** (recommended): the existing fallback in
  `backups.go` needs no `db_sources` row; in live single-DB mode one dump
  covers every table on prod. Unblocks A2/A3 immediately after D1.
- (b) Game-scoped sources: requires registering `db_sources` rows (ops
  action with credentials) — deliberately deferred; not a prerequisite for
  the restore drill.

## 4. D3 — restore drill shape (defines OPS-008 Stage C)

- (a) **Runbook-first drill** (recommended): one-shot `postgres:15` container
  on the deploy host; obtain the artifact via the existing download endpoint
  or file store; `Verify` (sha256 read-back) passes; restore with `psql -f`
  into a scratch database; schema + row-count spot checks against the live
  counts recorded in deploy-inventory; record RPO (backup timestamp → drill
  time) and RTO (wall-clock restore) in a dated review doc; container
  destroyed after the drill. **No new platform code.**
- (b) Recurring drill endpoint (ops API "drill mode"): only if the owner
  wants scheduled drills; separate design, not proposed here.

Side-effect boundary (from deploy-inventory §9, binding for the drill): the
restored database is never wired into the live chain; the live server is
never pointed at the scratch DB. Registry sessions, in-flight approvals,
scheduler state and SSE subscribers are memory state absent from any dump —
a restored DB must not be started as a live server without re-warm
semantics; that replay question is explicitly out of scope here.

## 5. D4 — credential verification scope

- A successful platform backup proves the configured DB credentials work for
  dump; the drill's `psql` restore proves the artifact restores.
- Credential **rotation** (deploy-inventory §4 found template-default dev
  passwords live) is an ops action outside DEPLOY-002 — named boundary, not
  covered by this proposal.
- The mysql path (`mysqldump`) shares the same image gap; live deploy is
  postgres-only, so it is recorded as a known boundary, not worked here.

## 6. Acceptance mapping

| #   | Scenario                                                                                                                          | Maps to        |
| --- | --------------------------------------------------------------------------------------------------------------------------------- | -------------- |
| A1  | Built server image runs `pg_dump --version` as nonroot, rc=0, major 15                                                            | D1(a)          |
| A2  | Platform backup via existing API → `succeeded` + sha256 + object stored; `backups` row > 0                                        | D1(a)+D2(a)    |
| A3  | `Verify` read-back recomputes sha256 against stored object → pass                                                                 | existing chain |
| A4  | One-shot postgres:15 container: `psql -f` restore of the API-produced artifact; schema/row spot checks recorded; RPO/RTO recorded | D3(a)          |
| A5  | Drill isolation: scratch DB on one-shot container, destroyed after; live chain untouched                                          | D3 boundary    |
| A6  | Memory-state boundary documented (sessions/approvals/scheduler/SSE not in dump)                                                   | D3             |

## 7. Decisions needed from the owner

- [ ] D1: approve option (a) base-debian12 + pg_dump closure COPY (or choose b/fallback).
- [ ] D2: confirm first backup scope = platform meta DB.
- [ ] D3: approve runbook-first drill (a) and authorize a deploy-host window to run it (live-host discipline), or defer drill shape again (OPS-008 Stage C stays deferred).
- [ ] D4: confirm credential rotation stays out of DEPLOY-002 scope.

## 8. Boundary

- Nothing implemented: no Dockerfile change, no image build, no backup, no
  drill — this document only. Implementation (A1/A2) follows a D1/D2
  signature; the drill itself (A4/A5) additionally needs the deploy-host
  window approval in D3.
- The pg_dump library closure list is deliberately not enumerated by hand —
  it must be derived mechanically (`ldd`) in the builder stage if D1(a) is
  approved. Pure-Go dump library landscape: NOT VERIFIED (only relevant if
  D1(b) is chosen).
