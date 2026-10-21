---
title: Cluster Topology and HA Operations
icon: cluster
order: 32
category:
  - Operations Manual
tag:
  - High Availability
  - Cluster
---

# Cluster Topology and HA Operations

"Ops Center → Cluster Topology" is the operations observability window for multi-instance Server HA (requires `cluster.enabled`). With a single-instance deployment the page shows a single `standalone` record.

## Page Structure

`GET /api/v1/ops/cluster` returns:

| Field                           | Description                                                                     |
| ------------------------------- | ------------------------------------------------------------------------------- |
| `enabled`                       | Whether the cluster is enabled                                                  |
| `self` / `aliveCount` / `total` | Current instance / online count / total                                         |
| `items[].instanceId`            | Instance identifier                                                             |
| `items[].advertiseAddr`         | Instance interconnect address                                                   |
| `items[].epoch`                 | Fencing generation (prevents split brain, see HA Architecture (Chinese only))   |
| `items[].startedAt`             | Start time                                                                      |
| `items[].agentCount`            | Number of Agent connections owned by this instance (ownership table statistics) |
| `items[].alive`                 | Online = lease not expired                                                      |

## Member State Machine

```
Instance starts → self-registers in the member table (advertiseAddr + lease)
        → heartbeat renewal (cluster.heartbeatInterval, default 5s)
        → no heartbeat within the lease TTL (cluster.leaseTtl, default 15s ≈ 3 heartbeats) → shown as offline
        → comes back → re-registers (epoch + 1; requests from the old generation are rejected by fencing)
```

Agent ownership: within `cluster.ownerTtl` (default 3m) Agents renew ownership via heartbeats; when an Agent disconnects ownership is released immediately, and whichever instance it reconnects to becomes the owner — **the agent distribution on the page is real-time ownership, not static configuration**.

## Common Operational Scenarios

### What to Do When an Instance Goes Offline

1. Confirm the offline instance on the topology page (`alive=false` + lease-expired tooltip)
2. The Agents under that instance have automatically reconnected to surviving instances (compare the agentCount distribution before and after)
3. After fixing the instance (check logs: migration/ports/network), restarting it makes it rejoin automatically — no manual "member removal" needed; the lease mechanism cleans up naturally
4. In-flight tasks: when an instance dies, its running tasks are marked failed/timed out by reconciliation; compensate and retry according to business semantics

### Rolling Upgrades

See Version Upgrades and Rollbacks — drain traffic per instance → replace → healthy → return traffic; the topology page lets you watch members come and go throughout.

### Split-Brain Concerns

Epoch fencing guarantees that a pre-partition old primary no longer issues calls (the peer rejects low-epoch requests). If the topology page shows two instances mutually reporting online while calls fail with `ErrNotOwner`/epoch errors, check the connectivity of the shared storage (Redis/DB) and system clock skew.

### Uneven Distribution

Agent spreading is decided by the L4 LB (leastconn); unevenness is normal (long-lived connection inventory). Only when "one instance has zero agents but the Agent total is normal" do you need to investigate LB health checks.

## Related Configuration

| Key                         | Default        | Description                                             |
| --------------------------- | -------------- | ------------------------------------------------------- |
| `cluster.enabled`           | `false`        | HA master switch (zero overhead for single instance)    |
| `cluster.advertiseAddr`     | Required*      | Peer-reachable interconnect address (use POD_IP on K8s) |
| `cluster.instanceId`        | Auto-generated | Explicit naming recommended for multi-instance          |
| `cluster.heartbeatInterval` | `5s`           | Member heartbeat                                        |
| `cluster.leaseTtl`          | `15s`          | Lease TTL (3x heartbeat recommended)                    |
| `cluster.ownerTtl`          | `3m`           | Agent ownership survival window                         |
| `cluster.peerPollInterval`  | `10s`          | Peer discovery polling                                  |
