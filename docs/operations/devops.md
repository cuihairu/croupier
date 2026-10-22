---
title: DevOps Notes (CI/CD, Build, Deploy and Rollback)
---

# DevOps Notes

For operations and release roles: an inventory of this repo's CI/CD pipeline, build artifacts, deployment options, and rollback means, each with file:line evidence; gaps are labeled honestly. Facts verified as of 2026-10-20.

> Path note: some deploy/rollback docs under `docs/operations/` in this repo (deploy-overview, deploy-binary, deploy-kubernetes, upgrade-rollback) live on the English side but hold Chinese content, and are excluded from the site build (the `srcExclude` at `docs/.vitepress/config.mjs:13-25` includes `operations/**`) — these files are readable in the repo but invisible on the docs site. This page folds the DevOps-relevant facts into a zh-side accessible page; the original files remain the detailed source.
>
> Term disambiguation: "DevOps" in this page means CI/CD and the delivery surface; `agents/devops` (devops-agent, an intranet remote-access proxy) is a different thing — see the current state of its bastion/jump capabilities in the Permissions notes.

## CI/CD Pipeline

`.github/workflows/` holds 26 workflows, grouped by responsibility:

| Group       | Files                                                                                                                   | Triggers                                                                        |
| ----------- | ----------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------- |
| Core CI     | `ci.yml`                                                                                                                | push main + PR (paths-ignore excludes docs-only, `:4-25`)                       |
| Targeted CI | `ci-dashboard.yml`, `ci-analytics.yml`, `ci-mobile.yml`, six `ci-sdk-*.yml`                                             | push main + PR (path-filtered); dashboard also responds to `v*` tags (`:16-18`) |
| Security    | `codeql.yml` (push + Monday cron `:24`)                                                                                 | SAST                                                                            |
| Images      | `docker.yml` (push main + `v*` tag + manual, `:4-26`), `docker-go-sdk-examples.yml` (push main)                         | GHCR image builds                                                               |
| Release     | `nightly.yml` (cron + manual, `:4-6`), `release.yml` (`v*` tag + manual, `:4-7`), six `release-sdk-*.yml` (manual only) | Release surface                                                                 |
| Deploy      | `deploy-self-hosted.yml` (**manual only**, `:4`)                                                                        | Self-hosted environment deployment                                              |
| Docs        | `docs-check.yml` (link/term guards), `docs-pages.yml` (Pages publish)                                                   | push/PR/manual                                                                  |

**CI - Core job topology**: `lint`/`security`/`test` run as three parallel tracks; `e2e`, `migrate-matrix`, and `capture-matrix` all depend only on `test` (`ci.yml:232`/`:331`/`:432`). Each job's content and guard-script call sites are in the Development lifecycle and release notes.

**Self-hosted deploy workflow** (`deploy-self-hosted.yml`): after manual trigger it runs on a `self-hosted` runner (`:51-52`) — copies the compose stack and telemetry stack (`:66-67`), rebuilds `configs/` (`:74-110`, including converging the agent address to `haproxy:19090`), writes the `.env` image tags from inputs (`:125-132`, default `main`), GHCR login and full `docker compose pull` (`:155-188`), compose ownership checks against mistakes (`:213`), `docker compose up -d --wait --remove-orphans` (`:240`) + force-recreate of core containers (`:249`), and collects server panic, agent registration, and dashboard failure evidence (`:220`/`:265`/`:268`).

## Build Artifacts

- **Binaries**: `make build` produces 8 binaries (`Makefile:132` aggregates `server agents worker ingest tools`) — croupier-server (`:89`), croupier-agent (`:109`), devops-agent (`:117`), capture-agent (`:122`), synthetics-agent (`:128`, already in the `make agents` aggregate `:130`), schema-validator (`:144`), analytics-worker (`:150`), ingest (`:157`), all landing in `bin/` (`Makefile:1`).
- **Archives**: nightly produces a per-platform bundle `croupier-bin-<os>-<arch>.tar.gz` (`nightly.yml:156`; `.zip` on Windows `:167`; ARM normalized to armv7 `:152`, that surface is sidecar/devops only) + single-agent packages via `scripts/package-agents.py` (binary, example configs, AGENTS.md/INSTALL.md, an `agents-<os>-<arch>.json` manifest, and `.sha256`; extension rules at `:66`).
- **Upload and release**: build artifacts are first uploaded as artifacts kept 7 days (`nightly.yml:197-202`); the release job collects binaries/SDK/dashboard into `release-assets/` (`:692-719`) and generates release notes (`:724` onward, the `## Contents` list at `:734`), then rolls the `nightly` tag (`:814` force push) + prerelease (`:819-829`). Stable releases are created by `release.yml:201-205` as a GitHub Release (`generate_release_notes: true`).
- **Docker images** (ghcr.io): the five images server/agent/ingest/analytics-worker/openapi-demo are built by `docker.yml` (`:33`/`:129`/`:212`/`:268`/`:322`; Dockerfiles in `docker/`, tag triggers re-stamp `VERSION="${GITHUB_REF_NAME#v}"` at `:47`), with cosign signing and SARIF upload (`supply-chain` job `:393-421`); the dashboard image is in `ci-dashboard.yml:355-417` (explicit `:main` tag `:409`); the six SDK example images are in `docker-go-sdk-examples.yml:31-48`.

## Deployment Options

| Form                            | Doc source                                                                                                             | Key points                                                                                                                      |
| ------------------------------- | ---------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------- |
| Docker Compose (single node/HA) | `docs/zh/operations/deploy-docker.md:34-56` (quick-start five-container healthy check), `:149` (HA dual instance)      | Orchestration in `docker/docker-compose.yml` and `docker-compose.deploy.yml`; image tags overridden via `.env` `CROUPIER_*_TAG` |
| Binary + systemd                | `docs/operations/deploy-binary.md:24-71` (`/opt/croupier` layout, config validate, systemd unit `:63-71`)              | Scripts `scripts/install-systemd.sh`, `install-launchd.sh`, `install-windows-service.ps1`                                       |
| Kubernetes                      | `docs/operations/deploy-kubernetes.md:60-70` (rolling `maxUnavailable: 0`), `:142-168` (Agent Deployment outbound TCP) | No helm/kustomize manifests; YAML embedded in docs (gap)                                                                        |
| One-line installer              | `scripts/install.sh:11-33` (curl usage and `--version latest\|nightly\|vX.Y.Z`)                                        | Downloads the bundle from GitHub Releases (`:440-448`), installs to `/usr/local/bin` or the user directory (`:452-483`)         |
| CI self-hosted                  | `deploy-self-hosted.yml` (manual only)                                                                                 | Full compose-stack rebuild on the deploy machine, see the section above                                                         |

Agents always dial the Server outbound (control entry `:19090`); the Server never dials back — see the [Operations manual](/operations/) port quick reference.

## Rollback Means

- **The only rollback doc**: `docs/operations/upgrade-rollback.md` (Chinese content; note it is invisible on the site — see the path note at the top) — single-instance upgrade (`:26-37`), HA rolling upgrade (`:41-52`), rollback semantics (`:54-63`).
- **Core discipline: database migrations are forward-only, there is no down path** (`upgrade-rollback.md:56`). Code-side confirmation: `internal/db/migrate/migrate.go` exports only `EnsureUpToDate` (`:184`) and `CurrentVersion` (`:259`), with no Down/Rollback entry point at all; `MinimumRequiredVersion` (`:48-54`, currently 45) rejects startup on check failure (`:251-252`; an old binary refuses to start against a newer database, `:18`).
- **Image rollback**: point `.env`'s `CROUPIER_*_TAG` at an old tag, then `docker compose pull && up -d` (`upgrade-rollback.md:29-31`; the tag surface comes from `docker.yml:68`'s metadata-action main+sha semantics and the tag-trigger semver re-stamp at `:47`).
- **Binary rollback**: replace the fixed-path file `/opt/croupier/bin/croupier-server` (`deploy-binary.md:63-64`; `install.sh:473-483` overwrites in place without backing up the old version).
- **Data-plane fallback**: the platform ships built-in backup/restore (`docs/operations/backup-restore.md:46` restore steps, `:75` backup availability verification).
- **Three rollback scenarios** (`upgrade-rollback.md:57-61`): ① the new version introduced no new migrations → roll back the binary/image directly; ② the old binary passes the version check but needs manual confirmation that it writes no new columns; ③ the old binary refuses to start → roll forward with a fix (preferred) or restore a DB backup (data loss after the backup point is accepted).

### Gaps (rollback)

- `docs/zh/operations/` has no upgrade/rollback page at all (`find docs/zh -iname "*rollback*" -o -iname "*upgrade*"` hits nothing) — this page is the zh-side entry added to fill that.
- No helm/kustomize manifests; K8s rollback relies only on the rolling semantics of the Deployment embedded in docs, with no in-repo automatic rollback mechanism.
- The content map at `zh/operations/index.md:21-22` claims coverage of "upgrade rollback" and other entries, but the corresponding files are not in the site build — navigation and actually visible pages disagree (a known discrepancy; this page does not fix targets beyond its own scope).

## Related pages
