#!/usr/bin/env bash
# Start (or reuse) throwaway MySQL 8.4 / Mongo 7 (rs0) / Postgres 16 containers
# and run the CDC recovery target integration suites against them.
#
#   scripts/cdc-recovery-integration.sh            # start/reuse + run all three
#   scripts/cdc-recovery-integration.sh --down     # stop and remove the containers
#
# Containers are left running afterwards (现场保留) for inspection; re-running
# the script reuses healthy ones. Override ports via CDC_RECOVERY_MYSQL_PORT,
# CDC_RECOVERY_MONGO_PORT, CDC_RECOVERY_PG_PORT.
set -euo pipefail
cd "$(dirname "$0")/.."

MYSQL_NAME=cdc-recovery-mysql
MONGO_NAME=cdc-recovery-mongo
PG_NAME=cdc-recovery-postgres
MYSQL_PORT=${CDC_RECOVERY_MYSQL_PORT:-13317}
MONGO_PORT=${CDC_RECOVERY_MONGO_PORT:-13318}
PG_PORT=${CDC_RECOVERY_PG_PORT:-13319}

if [[ "${1:-}" == "--down" ]]; then
  docker rm -f "$MYSQL_NAME" "$MONGO_NAME" "$PG_NAME" 2>/dev/null || true
  exit 0
fi

docker_start() { # name image hostport containerport [dockerargs...] -- [cmd...]
  local name=$1 image=$2 hostport=$3 containerport=$4
  shift 4
  if [[ $(docker inspect -f '{{.State.Running}}' "$name" 2>/dev/null || true) == true ]]; then
    return 0
  fi
  docker rm -f "$name" >/dev/null 2>&1 || true
  local docker_args=() cmd=() seen=false arg
  for arg in "$@"; do
    if [[ $arg == -- ]]; then
      seen=true
      continue
    fi
    if $seen; then cmd+=("$arg"); else docker_args+=("$arg"); fi
  done
  docker run -d --name "$name" -p "$hostport:$containerport" "${docker_args[@]}" "$image" "${cmd[@]}" >/dev/null
}

wait_exec() { # name cmd... (retry until the in-container command succeeds)
  local name=$1
  shift
  local attempt
  for attempt in $(seq 1 30); do
    if docker exec "$name" "$@" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "container $name did not become ready" >&2
  return 1
}

docker_start "$MYSQL_NAME" mysql:8.4 "$MYSQL_PORT" 3306 \
  -e MYSQL_ROOT_PASSWORD=root -- \
  --gtid-mode=ON --enforce-gtid-consistency=ON --server-id=1 --binlog-format=ROW --log-bin=binlog --binlog-row-metadata=FULL
wait_exec "$MYSQL_NAME" mysqladmin ping -h127.0.0.1 -proot

docker_start "$MONGO_NAME" mongo:7 "$MONGO_PORT" 27017 -- --replSet rs0 --bind_ip_all
wait_exec "$MONGO_NAME" mongosh --quiet --eval 'db.hello().ok'
# Single-node replica set is required for multi-document transactions; the
# member advertises localhost:27017 inside the container, tests use
# directConnection so the mapped host port is never routed through discovery.
docker exec "$MONGO_NAME" mongosh --quiet \
  --eval 'try { rs.initiate({_id:"rs0",members:[{_id:0,host:"localhost:27017"}]}).ok } catch(e) { print(e.codeName) }' \
  >/dev/null
wait_exec "$MONGO_NAME" mongosh --quiet --eval 'quit(db.hello().isWritablePrimary ? 0 : 1)'

docker_start "$PG_NAME" postgres:16 "$PG_PORT" 5432 -e POSTGRES_PASSWORD=postgres
wait_exec "$PG_NAME" pg_isready -U postgres

echo "== MySQL 8.4 recovery targets (:$MYSQL_PORT) =="
TEST_MYSQL_RECOVERY_DSN="root:root@tcp(127.0.0.1:$MYSQL_PORT)/" \
  go test -tags=integration -count=1 -timeout 8m -v ./pkg/cdc/recovery/mysql/

echo "== MongoDB 7 recovery targets (:$MONGO_PORT) =="
TEST_MONGO_RECOVERY_DSN="mongodb://127.0.0.1:$MONGO_PORT/?replicaSet=rs0&directConnection=true" \
  go test -tags=integration -count=1 -timeout 8m -v ./pkg/cdc/recovery/mongo/

echo "== PostgreSQL 16 recovery targets (:$PG_PORT) =="
TEST_POSTGRES_RECOVERY_DSN="postgres://postgres:postgres@127.0.0.1:$PG_PORT/postgres?sslmode=disable" \
  go test -tags=integration -count=1 -timeout 8m -v ./pkg/cdc/recovery/postgres/

echo "Containers left running for inspection: $MYSQL_NAME (:${MYSQL_PORT}) $MONGO_NAME (:${MONGO_PORT}) $PG_NAME (:${PG_PORT})"
echo "Tear down with: scripts/cdc-recovery-integration.sh --down"
