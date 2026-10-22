---
title: Go SDK Guide
---

# Go SDK Guide

## Build Patterns

The Go SDK currently has two build concerns: local development and CI/production. Both documentation and implementation should follow the protocol definitions in the monorepo; syncing from an external SDK repository is no longer required.

## Integration Advice

- Split handlers into separate packages by business domain
- Review protocol changes and SDK upgrades in the same PR
- Build a unified wrapper for timeouts, retries, and error mapping

## Concurrency Model

The Go SDK suits concurrent service scenarios, but handlers still need explicit boundaries to avoid uncontrolled goroutine leaks.

## Keep Reading
