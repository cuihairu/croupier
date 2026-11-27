#!/usr/bin/env bash
# AUD-095: local, deploy-free verification of the self-hosted deploy .env merge.
# Extracts the shell block between the AUD-095 markers in the workflow, replaces
# GitHub expression inputs with plain shell values, and runs it twice in a temp
# dir. No docker, no network, no real workflow run.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
workflow="${repo_root}/.github/workflows/deploy-self-hosted.yml"
start_marker='# BEGIN deploy env merge (AUD-095)'
end_marker='# END deploy env merge (AUD-095)'

fail() { echo "FAIL: $*" >&2; exit 1; }

[[ -f "${workflow}" ]] || fail "workflow not found: ${workflow}"
command -v python3 >/dev/null || fail "python3 required"

merge_script="$(mktemp)"
work_dir="$(mktemp -d)"
trap 'rm -rf -- "${merge_script}" "${work_dir}"' EXIT

python3 - "${workflow}" "${start_marker}" "${end_marker}" > "${merge_script}" <<'PY'
import re
import sys

path, start, end = sys.argv[1:4]
lines = open(path, encoding="utf-8").read().splitlines()
begin = next((i for i, line in enumerate(lines) if start in line), None)
finish = next((i for i, line in enumerate(lines) if end in line), None)
if begin is None or finish is None or finish <= begin:
    sys.exit("merge markers missing or misordered")

block = lines[begin + 1:finish]
if not block:
    sys.exit("empty merge block")
indent = min(len(line) - len(line.lstrip()) for line in block if line.strip())
body = "\n".join(line[indent:] if line.strip() else "" for line in block)
# Workflow inputs become local simulation variables. Keep the first 8 lines
# (comments / DEPLOY_DIR / key arrays) untouched so the test runs real logic.
body = re.sub(r"\$\{\{\s*inputs\.([a-z_]+)\s*\}\}", r'"${\1}"', body)
print(body)
PY
[[ -s "${merge_script}" ]] || fail "empty extracted merge script"

# Atomicity is a structural property: the merged file must be built in a
# sibling temp file and published with one rename. A runtime temp-dir test
# cannot observe a torn write, so assert the workflow block itself.
grep -q 'mktemp "${DEPLOY_DIR}/.env.XXXXXX"' "${merge_script}" \
  || fail "workflow does not build env in a sibling temp file"
grep -q 'mv -f "${TMP_ENV}" "${ENV_FILE}"' "${merge_script}" \
  || fail "workflow does not publish env with a single rename"
if grep -qE '(cat|printf|tee)[^#]*>>?[[:space:]]*"\$\{ENV_FILE\}"' "${merge_script}"; then
  fail "workflow writes directly to the live .env (non-atomic)"
fi

# Simulated workflow input values.
ghcr_owner="example-owner"
server_tag="v1"
agent_tag="v1"
dashboard_tag="v1"
sdk_example_tag="v1"
enable_sdk_examples="true"
analytics_bridge="true"
deploy_dir="${work_dir}/deploy"
mkdir -p "${deploy_dir}"
export ghcr_owner server_tag agent_tag dashboard_tag sdk_example_tag
export enable_sdk_examples analytics_bridge deploy_dir

write_seed_env() {
  cat > "${deploy_dir}/.env" <<'EOF'
GHCR_OWNER=stale-owner
CROUPIER_SERVER_TAG=old-tag
CROUPIER_AGENT_TAG=old-tag
CROUPIER_DASHBOARD_TAG=old-tag
CROUPIER_SDK_EXAMPLE_TAG=old-tag
ENABLE_SDK_EXAMPLES=false
ANALYTICS_BRIDGE_ENABLED=false
DATABASE_URL=postgres://custom:secret@postgres:5432/custom
POSTGRES_PORT=15432
REDIS_PORT=16379
CLICKHOUSE_HTTP_PORT=18123
CLICKHOUSE_NATIVE_PORT=19010
CLICKHOUSE_USER=custom-user
CLICKHOUSE_PASSWORD=custom-password
CLICKHOUSE_DB=custom_analytics
CLICKHOUSE_DSN=clickhouse://custom:secret@clickhouse:9000/custom_analytics
CLICKHOUSE_IMAGE=custom/clickhouse:v23
ANALYTICS_INGEST_SECRET=custom-ingest-secret
ANALYTICS_MQ_TYPE=redis
ANALYTICS_REDIS_ADDR=redis:6379/3
CROUPIER_SDK_EXAMPLE_GAME_ID=demo-game
CROUPIER_SDK_EXAMPLE_ENV=production
DASHBOARD_PORT=18000
EOF
}

assert_env() {
  local key="$1" expected="$2" actual
  actual="$(sed -n "s/^${key}=//p" "${deploy_dir}/.env")"
  [[ "${actual}" == "${expected}" ]] || fail "${key}: expected '${expected}', got '${actual}'"
}

assert_no_duplicates() {
  local key="$1" count
  count="$(grep -c "^${key}=" "${deploy_dir}/.env" || true)"
  count="${count:-0}"
  [[ "${count}" -eq 1 ]] || fail "${key}: expected 1 line, got ${count}"
}

assert_no_orphan_temp() {
  local orphans
  orphans="$(find "${deploy_dir}" -maxdepth 1 -name '.env.*' -print)"
  [[ -z "${orphans}" ]] || fail "temporary env files left behind: ${orphans}"
}

write_seed_env
echo "=== deploy 1 (seed: custom instance config, stale image tags) ==="
bash "${merge_script}"
assert_env DATABASE_URL "postgres://custom:secret@postgres:5432/custom"
assert_env CROUPIER_SERVER_TAG v1

server_tag="v2"
agent_tag="v2"
dashboard_tag="v2"
sdk_example_tag="v2"
export server_tag agent_tag dashboard_tag sdk_example_tag
echo "=== deploy 2 (new image tags, preserve instance config) ==="
bash "${merge_script}"

echo "=== assertions ==="
# Target image tag and managed switches must update normally.
assert_env GHCR_OWNER "${ghcr_owner}"
assert_env CROUPIER_SERVER_TAG v2
assert_env CROUPIER_AGENT_TAG v2
assert_env CROUPIER_DASHBOARD_TAG v2
assert_env CROUPIER_SDK_EXAMPLE_TAG v2
assert_env ENABLE_SDK_EXAMPLES true
assert_env ANALYTICS_BRIDGE_ENABLED true

# Instance-owned keys survive two consecutive deploys unchanged.
for pair in \
  "DATABASE_URL:postgres://custom:secret@postgres:5432/custom" \
  "POSTGRES_PORT:15432" \
  "REDIS_PORT:16379" \
  "CLICKHOUSE_HTTP_PORT:18123" \
  "CLICKHOUSE_NATIVE_PORT:19010" \
  "CLICKHOUSE_USER:custom-user" \
  "CLICKHOUSE_PASSWORD:custom-password" \
  "CLICKHOUSE_DB:custom_analytics" \
  "CLICKHOUSE_DSN:clickhouse://custom:secret@clickhouse:9000/custom_analytics" \
  "CLICKHOUSE_IMAGE:custom/clickhouse:v23" \
  "ANALYTICS_INGEST_SECRET:custom-ingest-secret" \
  "ANALYTICS_MQ_TYPE:redis" \
  "ANALYTICS_REDIS_ADDR:redis:6379/3" \
  "CROUPIER_SDK_EXAMPLE_GAME_ID:demo-game" \
  "CROUPIER_SDK_EXAMPLE_ENV:production" \
  "DASHBOARD_PORT:18000"
do
  assert_env "${pair%%:*}" "${pair#*:}"
  assert_no_duplicates "${pair%%:*}"
done
assert_no_orphan_temp

# Unknown-but-instance-owned suffix keys (future ports/collection settings)
# must also survive; a managed key never appears twice.
[[ "$(grep -c '^CROUPIER_SERVER_TAG=' "${deploy_dir}/.env")" -eq 1 ]] || fail "managed key duplicated"
printf 'MY_CUSTOM_PORT=19999\n' > "${deploy_dir}/.env.extra"
extra="$(cat "${deploy_dir}/.env.extra")"
cp "${deploy_dir}/.env" "${deploy_dir}/.env.preextra"
printf '%s\n' "${extra}" >> "${deploy_dir}/.env"
rm -f "${deploy_dir}/.env.extra"
echo "=== deploy 3 (future *_PORT key) ==="
bash "${merge_script}"
assert_env MY_CUSTOM_PORT 19999
assert_no_duplicates MY_CUSTOM_PORT

echo "=== final .env ==="
cat "${deploy_dir}/.env"

# Fresh install (no prior .env): managed keys land, and the legacy runner-level
# demo scope fallback still seeds the game/env.
fresh_dir="${work_dir}/fresh"
mkdir -p "${fresh_dir}"
echo "=== deploy fresh (no prior .env, legacy scope fallback) ==="
deploy_dir="${fresh_dir}" \
  CROUPIER_SDK_EXAMPLE_GAME_ID_PERSIST=fresh-game \
  CROUPIER_SDK_EXAMPLE_ENV_PERSIST=staging \
  bash "${merge_script}"
for expected in \
  "CROUPIER_SERVER_TAG=v2" \
  "CROUPIER_SDK_EXAMPLE_GAME_ID=fresh-game" \
  "CROUPIER_SDK_EXAMPLE_ENV=staging"
do
  grep -qx "${expected}" "${fresh_dir}/.env" || fail "fresh install missing ${expected}"
done

echo "PASS: AUD-095 env preservation verified across three simulated deploys plus fresh install"
