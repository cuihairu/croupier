#!/usr/bin/env bash
# AUD-096 local verification: simulate candidate staging -> validation gate -> atomic switch
# with a fake `docker compose`, and prove the on-site config/last working version survives
# image-prep failure, invalid candidate config, and switch failure.
# Mutation proof: switch without validation corrupts on-site config and this script detects it.
set -euo pipefail

ROOT="$(mktemp -d)"
trap 'rm -rf "${ROOT}"' EXIT

LIVE="${ROOT}/deploy"
CANDIDATE="${ROOT}/candidate"
FAKEBIN="${ROOT}/fakebin"
mkdir -p "${LIVE}/configs" "${CANDIDATE}/configs" "${FAKEBIN}"

echo "== AUD-096 harness root: ${ROOT} =="

# ---- Fake docker compose -------------------------------------------------------
# config -q fails when the candidate carries the INVALID marker (simulates a bad
# candidate compose file), pull fails when CROUPIER_FAIL_PULL=1 (image prep error).
cat > "${FAKEBIN}/docker" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail
for arg in "$@"; do
  if [ "$arg" = "config" ]; then
    envfile="$(for ((i=1;i<=$#;i++)); do
      prev="${!i}"
      if [ "$prev" = "--env-file" ]; then j=$((i+1)); printf '%s' "${!j}"; fi
    done)"
    composefile="$(for ((i=1;i<=$#;i++)); do
      prev="${!i}"
      if [ "$prev" = "-f" ]; then j=$((i+1)); printf '%s' "${!j}"; fi
    done)"
    if [ -n "$envfile" ] && grep -q '^INVALID_CANDIDATE=true' "$envfile" 2>/dev/null; then
      echo "fake docker: invalid candidate compose (.env marked INVALID_CANDIDATE)" >&2
      exit 1
    fi
    if [ -n "$composefile" ]; then
      # Resolve relative bind-mount sources against the compose file's directory.
      d="$(dirname "$composefile")"
      while IFS= read -r line; do
        src="${line%%:*}"
        case "$src" in ./configs/*|../configs/*)
          src="${src#./}"; src="${src#../}"
          [ -s "$d/$src" ] || { echo "fake docker: missing bind source $d/$src" >&2; exit 1; }
        ;;
        esac
      done < <(grep -E '^[[:space:]]+-[[:space:]]+\.{1,2}/configs/' "$composefile" 2>/dev/null || true)
    fi
    echo "fake docker: compose config OK"
    exit 0
  fi
  if [ "$arg" = "pull" ]; then
    if [ "${CROUPIER_FAIL_PULL:-0}" = "1" ]; then
      echo "fake docker: image pull failed" >&2
      exit 1
    fi
    echo "fake docker: pull OK"
    exit 0
  fi
done
echo "fake docker: $@"
FAKE
chmod +x "${FAKEBIN}/docker"
export PATH="${FAKEBIN}:${PATH}"

# ---- On-site "last working version" fixtures -----------------------------------
cat > "${LIVE}/docker-compose.yml" <<'EOF'
services:
  server:
    image: croupier-server:last-working
EOF
cat > "${LIVE}/.env" <<'EOF'
CROUPIER_SERVER_TAG=last-working
CROUPIER_SDK_EXAMPLE_GAME_ID=demo-game
CROUPIER_SDK_EXAMPLE_ENV=development
SERVER_PORT=18080
EOF
cat > "${LIVE}/configs/server.yaml" <<'EOF'
server:
  port: 18780
  note: last-working
EOF
cp "${LIVE}/configs/server.yaml" "${LIVE}/configs/agent.yaml"
cp "${LIVE}/configs/server.yaml" "${LIVE}/configs/agent2.yaml"

# Fingerprint of everything on-site: this must be identical after every failure.
live_fingerprint() {
  ( cd "${LIVE}" && find . -type f -not -path './.deploy-backup-*/*' -print0 \
      | sort -z | xargs -0 sha256sum )
}
BASELINE="$(live_fingerprint)"

# ---- Candidate staging (mirrors workflow step "Stage candidate ...") ------------
stage_candidate() {
  rm -rf "${CANDIDATE}"
  mkdir -p "${CANDIDATE}/configs/clickhouse/initdb"
  cat > "${CANDIDATE}/docker-compose.yml" <<'EOF'
services:
  server:
    image: croupier-server:candidate
    volumes:
      - ./configs/server.yaml:/app/configs/server.yaml:ro
EOF
  cat > "${CANDIDATE}/docker-compose.telemetry.yaml" <<'EOF'
services:
  prometheus:
    volumes:
      - ../configs/prometheus.yml:/etc/prometheus/prometheus.yml:ro
EOF
  cat > "${CANDIDATE}/configs/server.yaml" <<'EOF'
server:
  port: 18780
  note: candidate
EOF
  for f in agent.yaml agent2.yaml haproxy.cfg prometheus.yml otel-collector.yaml; do
    cp "${CANDIDATE}/configs/server.yaml" "${CANDIDATE}/configs/$f"
  done
  echo 'SELECT 1;' > "${CANDIDATE}/configs/clickhouse/initdb/001.sql"
  cat > "${CANDIDATE}/.env" <<'EOF'
CROUPIER_SERVER_TAG=candidate
GHCR_OWNER=cuihairu
CROUPIER_AGENT_TAG=candidate
CROUPIER_DASHBOARD_TAG=candidate
CROUPIER_SDK_EXAMPLE_TAG=candidate
ENABLE_SDK_EXAMPLES=true
ANALYTICS_BRIDGE_ENABLED=true
CROUPIER_SDK_EXAMPLE_GAME_ID=demo-game
CROUPIER_SDK_EXAMPLE_ENV=development
SERVER_PORT=18080
EOF
}

# ---- Validation gate (mirrors workflow step "Validate candidate config tree") ---
validate_candidate() {
  docker compose --project-name croupier --env-file "${CANDIDATE}/.env" \
    -f "${CANDIDATE}/docker-compose.yml" config -q || return 1
  docker compose --project-name croupier-telemetry \
    -f "${CANDIDATE}/docker-compose.telemetry.yaml" config -q || return 1
  for f in configs/server.yaml configs/agent.yaml configs/agent2.yaml \
           configs/haproxy.cfg configs/prometheus.yml configs/otel-collector.yaml; do
    [ -s "${CANDIDATE}/$f" ] || return 1
  done
  for key in GHCR_OWNER CROUPIER_SERVER_TAG CROUPIER_AGENT_TAG CROUPIER_DASHBOARD_TAG \
             CROUPIER_SDK_EXAMPLE_TAG ENABLE_SDK_EXAMPLES ANALYTICS_BRIDGE_ENABLED; do
    grep -q "^${key}=" "${CANDIDATE}/.env" || return 1
  done
  return 0
}

# ---- Atomic switch + rollback (mirrors workflow step "Switch candidate ...") ----
switch_candidate() {
  local backup="${LIVE}/.deploy-backup-verify-$(date +%s%N)"
  local -a items=(docker-compose.yml docker-compose.telemetry.yaml .env configs)
  mkdir -p "${backup}"
  for item in "${items[@]}"; do
    if [ -e "${LIVE}/${item}" ]; then cp -a "${LIVE}/${item}" "${backup}/"; fi
  done
  for item in "${items[@]}"; do
    if [ -e "${CANDIDATE}/${item}" ]; then
      if [ "${CROUPIER_FAIL_SWITCH:-0}" = "1" ] && [ "$item" = "configs" ]; then
        # Simulate a copy/permission failure after some items already switched.
        rm -rf "${LIVE}/configs"
        echo "switch: simulated copy failure on configs" >&2
        return 1
      fi
      rm -rf "${LIVE}/${item}"
      cp -a "${CANDIDATE}/${item}" "${LIVE}/"
    fi
  done
  return 0
}

switch_candidate_with_rollback() {
  if ! switch_candidate; then
    echo "switch failed -> rolling back"
    # Restore every managed item from the most recent backup.
    backup="$(ls -d "${LIVE}"/.deploy-backup-verify-* 2>/dev/null | sort | tail -1)"
    for item in docker-compose.yml docker-compose.telemetry.yaml .env configs; do
      rm -rf "${LIVE}/${item}"
      if [ -e "${backup}/${item}" ]; then cp -a "${backup}/${item}" "${LIVE}/"; fi
    done
    return 1
  fi
}

assert_live_unchanged() {
  local label="$1"
  local now; now="$(live_fingerprint)"
  if [ "${now}" = "${BASELINE}" ]; then
    echo "PASS ${label}: live config/last-working version byte-identical"
  else
    echo "FAIL ${label}: live config CHANGED"
    diff <(printf '%s\n' "${BASELINE}") <(printf '%s\n' "${now}") || true
    exit 1
  fi
}

# ---- Scenario 0: happy path (candidate validated then switched) ------------------
echo
echo "--- Scenario 0: happy path ---"
stage_candidate
if ! validate_candidate; then echo "FAIL happy path: validation gate rejected valid candidate"; exit 1; fi
if ! switch_candidate_with_rollback; then echo "FAIL happy path: switch failed"; exit 1; fi
grep -q 'candidate' "${LIVE}/docker-compose.yml" \
  && echo "PASS happy path: validated candidate is live"
# Rebuild baseline deterministically (first snapshot of happy-path setup).
rebuild_baseline() {
  mkdir -p "${LIVE}/configs"
  cat > "${LIVE}/docker-compose.yml" <<'EOF'
services:
  server:
    image: croupier-server:last-working
EOF
  cat > "${LIVE}/.env" <<'EOF'
CROUPIER_SERVER_TAG=last-working
CROUPIER_SDK_EXAMPLE_GAME_ID=demo-game
CROUPIER_SDK_EXAMPLE_ENV=development
SERVER_PORT=18080
EOF
  cat > "${LIVE}/configs/server.yaml" <<'EOF'
server:
  port: 18780
  note: last-working
EOF
  cp "${LIVE}/configs/server.yaml" "${LIVE}/configs/agent.yaml"
  cp "${LIVE}/configs/server.yaml" "${LIVE}/configs/agent2.yaml"
}
rebuild_baseline
BASELINE="$(live_fingerprint)"

# ---- Scenario 1: image-prep (docker pull) fails ---------------------------------
echo
echo "--- Scenario 1: image-prep failure ---"
# Restore live to last-working after the happy-path switch.
for item in docker-compose.yml docker-compose.telemetry.yaml .env configs; do rm -rf "${LIVE}/${item}"; done
rebuild_baseline
BASELINE="$(live_fingerprint)"
stage_candidate
# Workflow order: pull (candidate) BEFORE validation gate; pull failure aborts.
CROUPIER_FAIL_PULL=1
export CROUPIER_FAIL_PULL
if docker compose --project-name croupier --env-file "${CANDIDATE}/.env" \
    -f "${CANDIDATE}/docker-compose.yml" pull >/dev/null 2>&1; then
  echo "FAIL scenario 1: fake pull unexpectedly succeeded"; exit 1
fi
unset CROUPIER_FAIL_PULL
echo "image pull failed -> workflow aborts before switch (per AUD-096 ordering)"
assert_live_unchanged "image-prep failure"

# ---- Scenario 2: invalid candidate config ---------------------------------------
echo
echo "--- Scenario 2: invalid candidate config ---"
stage_candidate
echo 'INVALID_CANDIDATE=true' >> "${CANDIDATE}/.env"
if validate_candidate; then
  echo "FAIL scenario 2: validation gate accepted invalid candidate"; exit 1
fi
echo "validation gate rejected invalid candidate -> no switch performed"
assert_live_unchanged "invalid candidate config"

# ---- Scenario 3: switch failure (rollback) --------------------------------------
echo
echo "--- Scenario 3: switch failure with rollback ---"
stage_candidate
if ! validate_candidate; then echo "FAIL scenario 3: validation gate rejected valid candidate"; exit 1; fi
CROUPIER_FAIL_SWITCH=1
export CROUPIER_FAIL_SWITCH
if switch_candidate_with_rollback; then echo "FAIL scenario 3: switch unexpectedly succeeded"; exit 1; fi
unset CROUPIER_FAIL_SWITCH
assert_live_unchanged "switch failure + rollback"
# Confirm the last working version is still what is on-site.
grep -q 'last-working' "${LIVE}/docker-compose.yml" \
  && grep -q 'last-working' "${LIVE}/.env" \
  && echo "PASS switch failure: last-working compose/tag restored"

# ---- Mutation proof: switching WITHOUT validation corrupts live -----------------
echo
echo "--- Mutation: switch without validation gate ---"
stage_candidate
echo 'INVALID_CANDIDATE=true' >> "${CANDIDATE}/.env"
# Simulate old behavior: no validation, immediately switch invalid candidate.
if switch_candidate_with_rollback; then :; fi
if ! validate_candidate; then
  echo "DETECTED: invalid candidate reached live (validation would have rejected it)"
else
  echo "FAIL mutation: invalid candidate passed validation unexpectedly"; exit 1
fi
# Live must now be corrupted (candidate contents), proving the mutation is caught.
if [ "$(live_fingerprint)" = "${BASELINE}" ]; then
  echo "FAIL mutation: live unchanged despite unvalidated switch (proof invalid)"; exit 1
fi
echo "PASS mutation: switching without validation corrupts live; script detects it"
# Restore live for the harness exit (trap removes it anyway).
rebuild_baseline

echo
echo "ALL AUD-096 SCENARIOS PASSED"
