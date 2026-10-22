---
title: JavaScript SDK Guide
---

# JavaScript SDK Guide

## Integration Advice

- Prefer running on Node.js 24+
- Split business handlers by domain so no single entry point gets too heavy
- Upgrade in sync with `proto/**` to avoid protocol drift between the SDK and the server

## Concurrency and Threading Model

The JavaScript SDK runs on a single-process event loop. Move blocking CPU work to Workers or external services so Agent callback latency is unaffected.

## Conventions

- Use stable function IDs, e.g. `player.ban`
- Match version numbers to the level of change; never smuggle breaking changes into a patch release
- Keep explicit constraints on input/output JSON structures

## Keep Reading

- [API Reference](../api/)
