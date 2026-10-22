---
home: true
title: User Guide
heroImage: /logo.png
heroText: Croupier Guide
tagline: Getting started and core concepts — for deployment, configuration, and operations see the "Operations" section
actions:
  - text: Quick Start →
    link: /guide/quick-start.html
    type: primary
  - text: Configuration Management
    link: /operations/
    type: secondary
---

## Maintenance Notes

This section only keeps usage documentation that matches the current repository structure:

- `quick-start.md`: the shortest path to a local startup
- `installation.md`: dependency installation

Operations content such as deployment, full configuration reference, load balancing, monitoring/security/troubleshooting, and backup/restore has been split into the [Operations section](/operations/).

The following legacy content has been removed:

- Entry-point instructions based on `services/*`
- go-zero migration process documentation
- The outdated legacy FAQ, tutorial, and runbooks

## Recommended Reading Order

1. [Quick Start](./quick-start.md)
2. [Installation Guide](./installation.md)
3. [Configuration Management](../operations/config-server.md)
