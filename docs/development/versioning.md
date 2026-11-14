---
title: Versioning and Dependency Policy
---

# Versioning and Dependency Policy

How versions and dependency bumps are managed across the Go modules, the pnpm
workspaces, and the six SDK manifests. Facts verified 2026-11-10; the
`scripts/check-version-sync.sh` and `Makefile` targets referenced here are the
executable form of this page.

## Version Sources

The root `VERSION` file is the single source of truth for the platform version
(currently `0.1.6`). All six SDK manifests must equal it exactly:

| Surface    | File                                               | Form                 |
| ---------- | -------------------------------------------------- | -------------------- |
| Platform   | `VERSION`                                          | `MAJOR.MINOR.PATCH`  |
| Go SDK     | `sdks/go/version.go` + `sdks/go` module            | `const Version`      |
| JS/TS SDK  | `sdks/js/package.json` + `sdks/js/src/version.ts`  | `version` field      |
| Python SDK | `sdks/python/pyproject.toml`                       | `project.version`    |
| Java SDK   | `sdks/java/build.gradle` + `SdkInfo.java`          | `baseVersion`        |
| C# SDK     | `sdks/csharp/src/Croupier.Sdk/Croupier.Sdk.csproj` | `<Version>`          |
| C++ SDK    | `sdks/cpp/VERSION.cmake` + `croupier_client.cpp`   | CMake version triple |

- **Guard**: `scripts/check-version-sync.sh` is a read-only consistency check
  wired into CI ("Version guard" job). In the default CI mode `VERSION` must
  be **greater than or equal to** the latest `vX.Y.Z` tag (being ahead of the
  tag is the normal between-releases state); `--require-tag-match` (release
  pipeline) demands strict equality. It never edits anything.
- **Sync**: after bumping `VERSION`, run `make version-sync`
  (`scripts/sync-sdk-versions.sh`) — do not hand-edit six manifests; the guard
  failure message says the same.
- **Who bumps**: version numbers and release tags are maintainer-manual.
  Nightly builds publish a single rolling `nightly` prerelease that keeps
  only the latest build, never a numbered version (see
  [Development lifecycle](./lifecycle) for the release cadence).
- **Java snapshot behavior**: on main the Java SDK builds as
  `X.Y.Z-SNAPSHOT`; exact versions are used only when built for a `v*` tag.

## Go Modules

Two Go modules, deliberately separate:

- Root module `github.com/cuihairu/croupier` — server, agents, CLI, shared
  packages (`internal/`, `core/`, `agents/`, `pkg/`, `cmd/`).
- SDK module `github.com/cuihairu/croupier/sdks/go` — published SDK, own
  `go.mod`/`go.sum` so its dependency surface stays independent of the
  server's.

Dependency rules:

- **Dependabot** (`gomod`, weekly) proposes minor/patch bumps; major bumps are
  ignored by config and done deliberately, per dependency, when a release line
  requires it.
- **Vulnerability gate**: the Security Scan job runs `govulncheck`. Scan red
  is fixed by bumping the affected dependency (or the `go` directive for
  stdlib findings) — never by waiving the check. Note `go get` may silently
  raise the `go` directive using the local `GOTOOLCHAIN`; verify the diff
  after dependency work.
- Root module pins `go 1.27.4`, `sdks/go` pins `1.27.3` (as of 2026-11-14);
  keep them aligned when bumping.

## pnpm Workspaces

`web/` (dashboard) and `docs/` (this site) are independent pnpm workspaces,
each with its own `pnpm-workspace.yaml` and lockfile. pnpm resolves them
separately; nothing hoists versions across the two.

Rules that come from real incidents (each `web/pnpm-workspace.yaml` override
carries a dated rationale comment):

- **Overrides live only in `pnpm-workspace.yaml`**: pnpm 11 reads the
  `overrides` block exclusively from `pnpm-workspace.yaml`; a top-level
  `overrides` key in `package.json` is silently ignored.
- **Security-driven minimum-version pins**: overrides exist to force patched
  versions of transitive dependencies (Dependabot/GHSA referenced in each
  comment). Two value-form rules keep them safe:
  - Scope ranges to a major line (`>=x.y.z <next-major`) unless you intend a
    cross-major move. An open-ended floor once let a pinned `axios` line
    re-resolve onto 1.x; it is now pinned to `>=0.30.0 <1.0.0`.
  - Exact-line pins (`semver`-style) have twice caused full-tree breakage
    (`path-to-regexp`) or broken downstream SDK CI (`semver` 7.x→6.x). If you
    must pin a moving line, re-verify every consumer package.
- **Direct-dependency specifiers must equal the override's resolved value**:
  when an override pins a direct dependency, the `package.json` specifier must
  match it. A mismatching specifier installs fine locally (override wins) but
  fails `--frozen-lockfile` in CI.
- **`minimumReleaseAgeExclude`**: newly published versions are held back by a
  release-age policy; the exclude list is for urgent security fixes that
  cannot wait out the window (e.g. the `react-router` 6.30.6 fix line).
- **`allowBuilds`**: postinstall scripts are denied by default except the
  build tools on the allow list (`esbuild`, `unrs-resolver`); everything else
  (`core-js`, `@scarf/scarf`, …) stays denied.

## Related

- [Development lifecycle and release notes](./lifecycle) — branching model,
  release cadence, nightly mechanics.
- [SDK feature matrix](https://github.com/cuihairu/croupier/blob/main/sdks/SDK_FEATURE_MATRIX.md)
  — per-language capability levels (single source of truth for SDK features;
  the version number itself follows the table above).
