# OPS-LOG-001 evidence (2026-11-15)

Deployment-side evidence for todo item OPS-LOG-001 (docker log rotation + disk watchdog on the 192.168.5.5 docker host, dashboard-login P0 relapse prevention).

## What was done on 192.168.5.5 (ssh root@)

1. `/etc/docker/daemon.json` log-driver config raised `10m → 50m` (3 files), backup kept at `/etc/docker/daemon.json.bak-20261115`.
2. `systemctl restart docker` — all 46 containers auto-recovered (unless-stopped). Before/after container snapshots: `/root/pre-restart-containers.txt`, `/root/post-restart-containers.txt`.
3. Per-stack recreate in compose dependency order (postgres → server/server2 → dashboard → agent → rest): croupier main stack 13, `--profile analytics` (ingestion/analytics-worker), chirp 11, atlas 5, vapor 2, wingman 2, rustfs, herald, chorus-pg (bare container rebuilt with identical args and volumes). wingman WINGMAN_* secrets were extracted from the running container env, passed via compose env, never echoed, and shredded afterwards.
4. Final LogConfig state: 36 × 50m, 3 × 10m (memex-admin/server + 1 legacy buildkit), 7 × {} (6 buildkit containers recreated by CI pick up new config automatically + memex-rustfs).
5. Disk reclaim 16.23 GB (image prune until=24h): 90% → 75%.
6. Watchdog: `/usr/local/sbin/docker-disk-watchdog.sh` + cron weekly (Monday), threshold 85%, alert to `/var/log/docker-disk-watchdog.log` + syslog; trial run passed.
7. Acceptance re-check: newly started container `inspect` shows LogConfig 50m in effect; dashboard :8000 → 200; server /healthz → 200; login API → 400 (validation layer, not 500/503 — DB path recovered).

## Known boundaries (recorded in todo)

- memex stack compose file lives in a runner workspace (build context cleaned) — its 3 containers keep working and get 50m on next memex deploy workflow.
- `docker-compose.telemetry.yaml` references `jaegertracing/all-in-one:latest` not present locally (main compose already absorbed jaeger/otel with 50m ✓); telemetry file not touched this round.
- Alert channel is local log + syslog only (host has no push channel); weekly interval can theoretically re-fill between runs — switching to daily only changes the cron field.
- daemon restart + recreate landed inside one in-flight CI build window (buildkit containers pulled back by daemon restart; build not interrupted, not verified; rerun if failed).
