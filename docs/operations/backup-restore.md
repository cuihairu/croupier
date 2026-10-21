---
title: Backup and Restore
icon: box
order: 30
category:
  - Operations Manual
tag:
  - Backup
  - Restore
---

# Backup and Restore

## Built-in Platform Backup (Dashboard/API)

The "Ops Center → Data Backup" page and `/api/v1/backups` provide **logical database backups**:

- Supported drivers: `mysql` (`mysqldump`), `postgres` (`pg_dump`), `sqlite` (file copy)
- Backups are executed by the Server; the corresponding tools (`mysqldump`/`pg_dump`) must exist inside the Server container/host

```bash
# Trigger a full backup
curl -X POST https://<server>:18780/api/v1/backups \
  -H "Authorization: Bearer ***" \
  -d '{"name":"weekly-meta","type":"full"}'

# List / download / delete
curl -H "Authorization: Bearer ***" https://<server>:18780/api/v1/backups
curl -O -H "Authorization: Bearer ***" https://<server>:18780/api/v1/backups/{id}/download
curl -X DELETE -H "Authorization: Bearer ***" https://<server>:18780/api/v1/backups/{id}
```

Scheduling: call the API directly from crontab (or use the platform's scheduled scheduling feature to host itself).

## Backup Scope Checklist

| Object                                                               | Method                           | Notes                                                                                                                                       |
| -------------------------------------------------------------------- | -------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------- |
| **meta database** (users/roles/audit/member tables/game registry)    | Built-in backup or external dump | The lifeline for upgrades/rollbacks                                                                                                         |
| **game databases** (one per `(game_id, env)` when `multiGame: true`) | Per-database dump                | Built-in backup only covers the database the Server connects to; in multi-database environments an external traversal script is recommended |
| `data/` directory                                                    | File backup                      | assignments, uploaded files (config version artifacts, etc.)                                                                                |
| Configuration files                                                  | Git-managed                      | Keep `server.yaml` etc. under version control; do not rely on backups                                                                       |
| Redis                                                                | Usually not needed               | Cache/member tables are all rebuildable runtime state                                                                                       |

## Restore Steps

**MySQL**:

```bash
mysql -h <host> -u root -p croupier_meta < weekly-meta.sql
```

**PostgreSQL**:

```bash
psql -h <host> -U postgres -d croupier_meta -f weekly-meta.sql
```

**SQLite**: stop the Server → replace `data/croupier.db` (or the corresponding dataSource path) → start it.

Notes for the first startup after a restore:

- Losing audit/task records created after the restore point is expected; Agents re-register and member-table leases converge automatically

## Recommended Cadence

| Object            | Frequency                      | Retention                        |
| ----------------- | ------------------------------ | -------------------------------- |
| meta database     | Daily + manual before upgrades | 30 days                          |
| game databases    | Daily                          | Per game compliance requirements |
| `data/` directory | Daily rsync/snapshot           | 14 days                          |

## Verify Backup Usability

An unverified backup is no backup at all — run a restore drill once a month:

1. Restore to an isolated instance (different port/database name)
2. Start a Server of the same version pointing at the restored database; `/healthz` passes
3. Log into the Dashboard and spot-check: admins can log in, the function catalog is complete, audit chains are queryable
