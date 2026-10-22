---
title: SDK Documentation
---

# SDK Documentation

This directory is the documentation entry point for the official SDKs. It focuses on cross-language contracts, the capability matrix, onboarding, and per-language usage guides. The SDK source directories `sdks/<lang>/README.md` serve only as the development, build, and release entry points for each language package.

## Documentation Boundaries

| Location                 | Responsibility                                                                                        |
| ------------------------ | ----------------------------------------------------------------------------------------------------- |
| `docs/sdks/`             | User-facing documentation, capability matrix, cross-language behavior rules                           |
| `docs/sdks/<lang>/`      | Per-language onboarding guides, API overviews, example walkthroughs                                   |
| `sdks/<lang>/README.md`  | Source directory entry point: builds, tests, releases, package management                             |
| `sdks/<lang>/*_GUIDE.md` | Legacy migration-era documents; content should gradually merge into `docs/sdks/<lang>/` or the README |

## Current Unified Baseline

- L1/L2 Providers use a TCP session on the Agent's local gateway; the L3 Invoker is a standalone caller that only talks to the Server HTTP API and does not reuse Provider sessions.
- SDKs must no longer expose `rpc_addr`, `local_listen`, or local server callback semantics.

## Language Entry Points

| Language   | Documentation               | Source                  |
| ---------- | --------------------------- | ----------------------- |
| C++        | [C++ SDK](/sdks/cpp/)       | `sdks/cpp/README.md`    |
| Go         | [Go SDK](/sdks/go/)         | `sdks/go/README.md`     |
| Java       | [Java SDK](/sdks/java/)     | `sdks/java/README.md`   |
| JavaScript | [JavaScript SDK](/sdks/js/) | `sdks/js/README.md`     |
| Python     | [Python SDK](/sdks/python/) | `sdks/python/README.md` |
| C#         | [C# SDK](/sdks/csharp/)     | `sdks/csharp/README.md` |

## Migration Requirements

- New user-facing documentation must go into `docs/sdks/<lang>/`.
- New build, test, and release notes must go into `sdks/<lang>/README.md` or a development document in the same directory.
- C++ currently has multiple configuration, plugin, and resource documents. When migrating, pick the canonical content first, then turn duplicate pages into redirects or archive them.
