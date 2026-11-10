# Roadmap

This page describes where Croupier is heading, in plain terms. It lists
themes, not dated promises: there are no ETAs here, and items can move between
sections as feedback arrives. Day-to-day engineering detail lives in the
repository's internal work ledger and is not mirrored on this page.

## What is in place today

- Control plane and distributed agents with a single shared session transport
  (single TCP connection, bidirectional requests, reconnect, backpressure)
- Function registration driving invocation, jobs, and descriptor-driven UI
- Permission control: RBAC/ABAC policies, approval workflows, audited
  operations with hash-chain integrity, two-person rule for high-risk actions
- Multi-game / multi-environment scoping on one company boundary
- Web console (dashboard) covering functions, pages, ops, analytics, and
  support workflows; Flutter mobile client for on-call operations
- SDKs for Go, JavaScript/TypeScript, Python, Java, C#, and C++

## Working on now

- **Fresh-install credential hygiene** — bootstrap passwords generated at
  install time and shown once; the demo credentials stay demo-only
- **Nightly test depth** — full-suite runs with race detection and shuffle,
  flaky-test detection, short fuzz runs, plus concurrency performance checks
  with latency thresholds
- **Mobile client quality** — higher test coverage, golden UI tests, and a
  mobile threat model
- **SDK capability transparency** — a published capability-by-version matrix
  so integrators can see exactly what each SDK package supports
- **Accessibility** — lint gates and axe-based scans (including theme
  contrast sweeps) already gate the dashboard; continuing to clear findings
- **Documentation polish** — shared terminology, screenshot regeneration,
  style-system and versioning strategy notes

## Next up

- **Stricter internal layering** — enforced import-direction rules between
  transport, core, platform, and API layers
- **Cross-SDK wire-contract tests** — shared golden message frames exercised
  by every SDK to catch protocol drift early
- **Tech-debt burn-down** — request-scoped contexts replacing bare
  backgrounds, narrower panic usage, and TODO cleanup by area

## Not planned

- **SaaS multi-tenancy** — Croupier is deliberately a single-company platform
  with per-game isolation, not a multi-tenant service
- **Replacing the session transport with gRPC** — evaluated and declined; see
  the [transport ADR](docs/architecture/adr/0001-transport.md)

## Feedback

Open a GitHub issue with your use case — roadmap input from real game
operations is the strongest signal for prioritization. Contribution mechanics
are in [CONTRIBUTING.md](CONTRIBUTING.md).
