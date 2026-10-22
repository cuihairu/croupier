---
title: Python SDK Guide
---

# Python SDK Guide

## Connection Model

The Python SDK communicates with the Agent over a single bidirectional connection; no extra local listening port is needed.

## Integration Advice

- Keep business handlers as pure functions or thin wrappers
- Use a background task system for long-running work; do not block the main invocation chain for long
- Review protocol upgrades together with `proto/**` changes

## Threads and Concurrency

For concurrent processing, prefer Python's existing async or task-queue mechanisms; do not share the SDK client instance across processes.

## Keep Reading

- [API Reference](../api/)
