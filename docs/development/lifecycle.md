---
title: Development Lifecycle and Release Notes
---

# Development Lifecycle and Release Notes

This page inventories four aspects of this repo's development lifecycle (branching model, release cadence, versioning policy, testing and release gates), each with file:line evidence; items where the repo has no corresponding mechanism are labeled as gaps honestly. Facts verified as of 2026-10-20; line numbers may drift with later evolution, so treat the verification date as authoritative.

## Branching Model

- **Single-main trunk development**: every workflow trigger points at main only — `ci.yml:5`/`:14` (both push and pull_request use `branches: [main]`), `ci-analytics.yml:4`/`:13`, `ci-mobile.yml:12`/`:17`, and the six `ci-sdk-*.yml` follow the same pattern; no develop/release branches exist in the repo.
- **PR as a supplement, direct push as the norm**: `git log --merges` measures 88 merges on main, of which only 4 are PR merges (`#72`–`#76`, including dependabot); the rest are merges of temporary worktree branches. The current day-to-day shape is direct pushes to main (CI gates fully at push time).
- **Merge discipline**: PRs must state what changed, why, and how it was verified, and `make test` must pass before merging — `docs/zh/development/repository-guidelines.md:93-96`.
- **Gap (branch protection)**: no `CODEOWNERS` and no branch-protection config files under `.github/`; branch protection is a GitHub repository setting with no citable evidence inside repo files, honestly labeled "no in-repo mechanism".

## Release Cadence

- **Nightly daily build**: `schedule` triggers `cron: "0 2 * * *"` (daily 02:00 UTC, `.github/workflows/nightly.yml:5`), with `workflow_dispatch` manual trigger (`:6`); `concurrency.group: nightly-release` without canceling in-flight runs (`:14-16`). Each build job checks out main (`:53` etc.); the release job collects artifacts, rolls the `nightly` tag (`:814` force push), and publishes a prerelease (`:817-829`, `tag_name: nightly`, `prerelease: true`).
- **Gap (change detection)**: nightly builds unconditionally every day; there is no "only release if something changed in 48 hours" change-detection logic (no hits across `nightly.yml` and full history greps); that claim is process convention only.
- **Stable releases**: the only effective entry is a manually pushed `v*` tag — `release.yml:4-6` (`push: tags: ["v*"]`); `workflow_dispatch` exists (`:7`) but all three jobs guard with `if: startsWith(github.ref, 'refs/tags/v')` (`:22`/`:151`/`:201`), so a manual run is a no-op. `docker.yml:4-6` responds to `v*` tags too (plus push main).
- **SDK releases**: one standalone manual workflow per language, `workflow_dispatch` only, never auto-triggered (`:3` comment in each `release-sdk-*.yml`; in `release-sdk-go.yml:5-11` an empty `version` input falls back to the latest release tag, logic at `:24-34`).
- **Security scan cadence**: CodeQL every Monday 03:00 UTC (`codeql.yml:24`).

## Versioning Policy

- **The root `VERSION` file is the single version source of truth** (currently `0.1.6`): `Makefile:2` `VERSION := $(shell cat VERSION ...)`; `:10` assembles `LDFLAGS := -X main.Version=$(FULL_VERSION) ...`.
- **Injection points**: the server uses the `Version` variable at `cmd/server/root.go:34` and `ServerVersion` at `internal/svc/version.go:6` (full import-path injection at `Makefile:89`); devops/capture/synthetics inject `agents/*/runner.Version` (`Makefile:117`/`:122`/`:128`).
- **Re-stamping at release time**: the VERSION file value can lag behind the release tag (`release.yml:58-60` records the v0.1.5 evidence staying 0.1.4), so the release surface uniformly re-stamps via `git describe` (`release.yml:61`); nightly uniformly re-stamps as `nightly-YYYYMMDD` (`nightly.yml:81`, the assertion that binary `--version` contains `nightly-` is at `:190`).
- **SDK version sync**: after bumping the root VERSION, run `make version-sync` (`scripts/sync-sdk-versions.sh:2-16`).
- **Version drift observed (recorded honestly)**: root `VERSION` (0.1.6) > latest tag (`v0.1.5`, measured via `git ls-remote --tags`) > SDK manifests (`sdks/go/version.go:5`, `sdks/js/package.json:3`, `sdks/python/pyproject.toml:7` all 0.1.4) — three layers coexisting, direct evidence of "VERSION moves first, releases lag".
- **Tag naming**: Server/Agent use `v*`; SDKs use submodule tags (`docs/operations/sdk-release.md:96`: `sdks/go/vX.Y.Z`; actually executed at `release-sdk-go.yml:40-46`). The `sdk-go-v*` wording in `repository-guidelines.md:104-110` has drifted from the current implementation — submodule tags are authoritative.
- **Convention boundary (not a mechanism)**: "version numbers are only user-given and manually triggered" is currently process convention — mechanically true (the only release entry is a manually pushed `v*` tag), but no workflow or code in the repo enforces it; anyone with push access could in theory tag and trigger a release. Honestly labeled a convention, not an in-repo enforcement mechanism.

## Testing and Release Gates

### CI - Core (`ci.yml`)

- Six jobs: `lint` (`:27`), `security` (`:119`), `test` (`:161`) run as three parallel tracks; `e2e` (`:225`), `migrate-matrix` (`:321`), `capture-matrix` (`:418`) with `needs: [test]` (`:232`/`:331`/`:432`).
- **Lint guard chain**: architecture guard `ci.yml:45`, contracts `:48`, API response contracts `:51`, dashboard vnext guard and self-test `:54`/`:57`, SDK matrix `:60`, demo contract baseline `:63`, `buf lint proto` `:89`, gofmt `:106`, `go vet` `:114`, transport dependency boundary (the `transport-no-grpc` guard) `:117`.
- **security**: govulncheck (wrapped with an exemption list, `ci.yml:159`).
- **test**: full `go test ./internal/... ./core/... ./agents/...` without `-race` (`ci.yml:208`) + targeted `-race` (`:212`, register/devops/capture/sidecar) + Codecov upload (`:214-223`).
- **e2e**: starts the server on SQLite and runs happy-path and task lifecycle scripts (`:311-312`, SQLite config at `:282`).
- **migrate-matrix**: MySQL 8.4 / Postgres 16 / MSSQL 2022 containers run the migration matrix (`:332-367`) + `atlas migrate validate` (`:416`).
- **capture-matrix**: container integration tests for the four sources MySQL/MariaDB/Postgres/MongoDB (`:460-528`).

### CI - Dashboard (`ci-dashboard.yml`)

- Four jobs in parallel: quality gates (`:41`, including dashboard guard `:94`, localization text guard `:99`, `tsc --noEmit` `:122`, sharded jest `:136`), mock E2E (`:223`), real E2E (`:245`), Docker image and nginx smoke (`:355`, `:402-417`).
- **Note**: `:16-18` makes v* tag pushes run the full dashboard quality gate automatically, but it runs in parallel with `release.yml` and does not block releases.

### The Rest

- Analytics E2E (`ci-analytics.yml:39`), Mobile (`ci-mobile.yml:24`, release APK manual only `:48-49`), docs checks (`docs-check.yml:44`/`:48` link and term guards).
- **Local gates**: pre-commit hooks run BOM and gofmt checks (`scripts/git-hooks/pre-commit:29-43`, one section each); `make test-changed` is a local selective-testing tool, while CI gates stay full (`repository-guidelines.md:71`).

### Gaps (pre-release gates)

- Neither `release.yml` nor `nightly.yml` has any test/gate job (`release.yml`'s `build-go` `:20`, `build-sdk-sources` `:150`, `release` `:189` only build and package); the six SDK release workflows publish directly with no tests either. "Gates must pass before release" currently relies on human discipline (confirming CI green before tagging); no in-repo enforcement — honestly labeled.

## Known Gaps Summary

| Aspect                         | Status                    | Notes                                                                                                  |
| ------------------------------ | ------------------------- | ------------------------------------------------------------------------------------------------------ |
| Branch protection              | Not in repo               | A GitHub repository setting; no citable evidence inside files                                          |
| Nightly change detection       | None                      | Unconditional daily build; "release only on change" is process convention                              |
| Pre-release gates              | Not enforced              | release/nightly/SDK workflows have no test jobs; human confirmation of green CI                        |
| Manually given version numbers | Convention, not mechanism | Mechanically the only entry is a manual `v*` tag; no in-repo enforcement                               |
| SDK tag convention             | Drifted                   | Current submodule tags (`sdks/go/vX.Y.Z`); `repository-guidelines.md`'s `sdk-go-v*` is the old wording |
