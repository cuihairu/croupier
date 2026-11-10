# Changelog

All notable changes to Croupier are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

Version tags are cut by the maintainer only; see `VERSION` for the version
the `main` branch currently carries. Daily/nightly builds are rolling
binaries and are not listed here.

## [Unreleased]

Working line toward `0.1.6` (see `VERSION`):

### Added

- Dual-lane (control/business) bounded dispatch across Go agent transport and
  all six SDK transports — heartbeats survive business saturation; busy
  responses fast-fail with an empty-body frame.
- Provider handshake auth: SDKs carry `authToken` in the provider connect
  metadata; agents can enforce verification (`agent.providerAuth.require`),
  fail-closed on empty token, with the key stripped from session metadata.
- Six-registry SDK release matrix (`release-sdk-matrix.yml`): npm / PyPI /
  Maven / NuGet / Go proxy / vcpkg-prebuilt orchestrated in one reusable
  workflow; the GitHub Release stays draft until all lanes are green, with
  draft conversion and alerting on any lane failure.
- Delivery semantics matrix documented (per message class: at-most-once /
  at-least-once, idempotency key propagation, saturation behavior) with
  regression tests pinning that busy responses never trigger cross-instance
  failover.

### Changed

- CI test scope now includes `./pkg/...` and `./cmd/...`; Codecov project
  checks are enforced (SDKs at an absolute 85% floor).
- SQLite legacy-column drop in the agent-session registry uses native
  `ALTER TABLE ... DROP COLUMN` with logged (never swallowed) failures.

## [0.1.5] - 2026-09-28

Initial public release line: server (HTTP 18780 / agent control 19090),
sidecar provider dialing (19091), agents for sidecar/capture/synthetics/devops
roles, RBAC + approvals + audit chain, multi-game scoping, HA multi-instance
with owner forwarding, descriptor-driven dashboard, and Go / Python / JS /
Java / C++ / C# SDKs.

[Unreleased]: https://github.com/cuihairu/croupier/compare/v0.1.5...HEAD
[0.1.5]: https://github.com/cuihairu/croupier/releases/tag/v0.1.5
