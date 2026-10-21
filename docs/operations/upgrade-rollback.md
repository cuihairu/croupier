---
title: Version Upgrade and Rollback
icon: refresh
order: 6
category:
  - Operations Manual
tag:
  - Deployment
  - Upgrade
---

# Version Upgrade and Rollback

## Migration Mechanism Recap (Required Reading Before Upgrading)

- Schema changes go through **versioned migrations** (goose numbered migrations, 0001–0014+, embedded in the binary): automatic catch-up at startup, no manual SQL execution needed (see [Database Migration Strategy (Chinese-only section)](/zh/architecture/database-migration-strategy))
- The entire migration runs under a **cross-process lock** (MySQL `GET_LOCK` / Postgres `pg_advisory_lock` / SQL Server `sp_getapplock`) — rolling restarts of multiple instances at the same time are safe; the migration is executed by only one process
- Each binary has a `MinimumRequiredVersion`: when the version already applied on the database is **lower** than required, startup automatically catches up; once caught up, if the version is higher than what an old binary requires, **the old binary refuses to start** (preventing old code from writing to the new schema)

## Pre-Upgrade Checklist

1. Read the Release Notes: watch for `MinimumRequiredVersion` bumps and breaking configuration changes
2. Back up (see [Backup and Restore](./backup-restore)) — especially the meta database (user/role/audit/member tables)
3. Confirm disk space and DB quota (migrations may add tables/columns)

## Single-Instance Upgrade

```bash
# Compose
docker compose pull && docker compose up -d

# Binary
systemctl stop croupier-server
cp bin/croupier-server /opt/croupier/bin/croupier-server.new && mv /opt/croupier/bin/croupier-server.new /opt/croupier/bin/croupier-server
systemctl start croupier-server
journalctl -u croupier-server -f | grep -i migrate   # Watch for catch-up completion
```

Verify: `/healthz` returns 200, `/api/v1` reports the expected version, and the Dashboard home page allows login.

## HA Rolling Upgrade (Zero Stream Interruption)

Execute one node at a time; if any node fails verification, stop immediately and investigate:

```
Drain traffic (LB set state drain / scale down replicas) → Replace binary → Start (automatic migration or dry run)
→ /healthz passes → Restore traffic → Next node
```

- No action needed on the Agent side: disconnect/reconnect + re-registration automatically migrates ownership (owner updates with the connection)
- The first node to start holds the lock and runs the migration; subsequent instances start with the migration already in place and skip it directly (idempotent catch-up)
- K8s: `maxUnavailable: 0` + readinessProbe guarantee that live instances exist throughout the rollout

## Rollback Semantics (Important)

**Migrations only go forward; there is no down path.** When rolling back the binary version in production:

| Scenario                                                                                                    | Resolution                                                                                                                                              |
| ----------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------- |
| New binary **introduced no** new migration (`MinimumRequiredVersion` not bumped)                            | Rolling back the binary/image directly is fine                                                                                                          |
| New version **has applied** new migrations, and the old binary's `MinimumRequiredVersion` ≤ applied version | Roll back directly (the version check only rejects "database too old", not "database newer"); but confirm the old code does not write new columns       |
| Old binary refuses to start due to the version check                                                        | Two options: **roll forward with a fix** (preferred, publish a patched release) or **restore a DB backup** (accepting data loss after the backup point) |

Hence the discipline during an upgrade window: **back up first, then upgrade; if verification fails, drain traffic first, then decide**.

## Data-Plane Components

`agent`, `analytics-worker`, and `ingest` are version-decoupled from the Server (the session protocol compatibility window is governed by the Release Notes). Agent rolling upgrades are just disconnect/reconnect; they are stateless and can be restarted at any time.

## Managed-Program Updates and Fingerprint Auditing (Design finalized 2026-10-14, shipping with R2-R4)

> This section covers binary updates of managed processes on agent nodes (game assist programs / external probe adapters / **the game process itself**); the sections above are about upgrading Croupier's own components. For the design and authoritative source, see [Agent Supervisor Design (Chinese-only section)](/zh/design/agent-supervisor-design) §10.4.

- **Every update must record a fingerprint** (binary sha256): fields include the target identifier, execution location (agentID), old fingerprint → new fingerprint, timestamp, operator/source, verification result (hash/signature/compatibility), and outcome (success/failed/**rolled_back** — rollbacks are audited too).
- **Query model**: search the execution audit (execlog) by "target + time + fingerprint"; one log entry carries fingerprint + version string, letting you trace a problem back to the update that introduced it. Aligning the alert/probe timeline with update records by (target, time) locates "after which update the alerts started".
- **Additional preconditions for the game process itself**: a maintenance window or online player count below a threshold + pausing scheduling and new-connection intake (**probes must not stop**, otherwise update failures become invisible) + rollback on failure.
- Agent self-upgrades likewise leave fingerprints (exposed via registration/heartbeat version info); for missing-agent investigations, the server-side last-known version + fingerprint serves as the anchor.
- Current status: **not implemented** — today the managed surface only has monitoring/auto-restart/event logging (S1-S3); the update capability and fingerprints land with §10.
