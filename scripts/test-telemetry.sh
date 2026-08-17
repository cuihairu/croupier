#!/usr/bin/env bash
# Validate the real Console execution audit -> OTLP -> Jaeger path and the
# Prometheus scrape path for the Collector.
#
# This script owns only its Compose project and a temporary dashboard fixture.
# It never removes repository data, shared Docker resources, or external
# services. Docker is required: absence is a failed prerequisite, not a skip.

set -euo pipefail

readonly REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly COMPOSE_FILE="${REPO_ROOT}/docker/docker-compose.telemetry.yaml"
readonly COMPOSE_PROJECT="croupier-telemetry-e2e-${RANDOM}-${RANDOM}"
readonly FIXTURE_DIR="$(mktemp -d "${TMPDIR:-/tmp}/croupier-telemetry-fixture.XXXXXX")"
readonly FIXTURE_LOG="${FIXTURE_DIR}/fixture.log"
readonly FIXTURE_STATE="${FIXTURE_DIR}/state.json"
readonly FIXTURE_BIN="${FIXTURE_DIR}/croupier-server"

fixture_pid=""
cleanup_started=0

log() {
    printf '[telemetry-e2e] %s\n' "$*"
}

fail() {
    printf '[telemetry-e2e] ERROR: %s\n' "$*" >&2
    exit 1
}

compose() {
    docker compose --project-name "${COMPOSE_PROJECT}" -f "${COMPOSE_FILE}" "$@"
}

require_command() {
    command -v "$1" >/dev/null 2>&1 || fail "required command is unavailable: $1"
}

wait_http() {
    local url="$1"
    local description="$2"
    local attempts="${3:-60}"
    local index
    for ((index = 1; index <= attempts; index++)); do
        if curl --fail --silent --show-error "${url}" >/dev/null; then
            log "ready: ${description}"
            return 0
        fi
        sleep 1
    done
    fail "timed out waiting for ${description}: ${url}"
}

cleanup() {
    local status="$?"
    if [[ "${cleanup_started}" -eq 1 ]]; then
        exit "${status}"
    fi
    cleanup_started=1

    if [[ -n "${fixture_pid}" ]] && kill -0 "${fixture_pid}" 2>/dev/null; then
        kill "${fixture_pid}" 2>/dev/null || true
        wait "${fixture_pid}" 2>/dev/null || true
    fi
    if command -v docker >/dev/null 2>&1; then
        compose down --volumes --remove-orphans >/dev/null 2>&1 || true
    fi
    rm -rf "${FIXTURE_DIR}"
    exit "${status}"
}

trap cleanup EXIT INT TERM

start_telemetry_stack() {
    log "validating Compose configuration"
    compose config >/dev/null

    log "starting telemetry stack with project ${COMPOSE_PROJECT}"
    compose up --detach --wait

    wait_http "http://127.0.0.1:113133/" "OTel Collector health"
    wait_http "http://127.0.0.1:17686/" "Jaeger UI"
    wait_http "http://127.0.0.1:19092/-/ready" "Prometheus"
    wait_http "http://127.0.0.1:13000/api/health" "Grafana"
}

start_fixture() {
    log "building isolated real-dashboard fixture"
    (
        cd "${REPO_ROOT}"
        go build -o "${FIXTURE_BIN}" ./cmd/server
    )

    log "starting fixture with OTLP export enabled"
    (
        cd "${REPO_ROOT}"
        OTEL_ENABLED=true \
        OTEL_ENABLE_TRACING=true \
        OTEL_ENABLE_METRICS=true \
        OTEL_EXPORTER_OTLP_ENDPOINT="http://127.0.0.1:14318" \
        OTEL_SERVICE_NAME="croupier-telemetry-e2e" \
        OTEL_SERVICE_VERSION="test" \
        OTEL_ENVIRONMENT="test" \
        OTEL_SAMPLING_RATIO="1" \
        "${FIXTURE_BIN}" dev-fixture \
            --dir "${FIXTURE_DIR}/runtime" \
            --http-addr "127.0.0.1:0" \
            --bootstrap-dir "${REPO_ROOT}/configs"
    ) >"${FIXTURE_LOG}" 2>&1 &
    fixture_pid="$!"

    local index
    for ((index = 1; index <= 90; index++)); do
        if rg -q '^FIXTURE_READY ' "${FIXTURE_LOG}"; then
            rg '^FIXTURE_READY ' "${FIXTURE_LOG}" | tail -n 1 | sed 's/^FIXTURE_READY //' >"${FIXTURE_STATE}"
            break
        fi
        if ! kill -0 "${fixture_pid}" 2>/dev/null; then
            cat "${FIXTURE_LOG}" >&2 || true
            fail "real-dashboard fixture exited before it became ready"
        fi
        sleep 1
    done
    [[ -s "${FIXTURE_STATE}" ]] || {
        cat "${FIXTURE_LOG}" >&2 || true
        fail "real-dashboard fixture did not become ready"
    }

    local http_addr fixture_addr
    http_addr="$(jq -r '.httpAddr' "${FIXTURE_STATE}")"
    fixture_addr="$(jq -r '.fixtureAddr' "${FIXTURE_STATE}")"
    wait_http "http://${http_addr}/healthz" "fixture server"
    wait_http "http://${fixture_addr}/__fixture__/health" "fixture control API"
}

login_and_publish() {
    local http_addr token current_status publish_status
    http_addr="$(jq -r '.httpAddr' "${FIXTURE_STATE}")"
    token="$(curl --fail --silent --show-error \
        --header 'Content-Type: application/json' \
        --data '{"username":"admin","password":"admin123"}' \
        "http://${http_addr}/api/v1/auth/login" | jq -er '.token')"

    local -a headers=(
        --header "Authorization: Bearer ${token}"
        --header 'X-Game-ID: e2e-game'
        --header 'X-Env: e2e'
    )
    current_status="$(curl --silent --output /dev/null --write-out '%{http_code}' \
        "${headers[@]}" \
        "http://${http_addr}/api/v1/console/pages/operation--mail.send")"
    if [[ "${current_status}" == "404" ]]; then
        publish_status="$(curl --silent --output /dev/null --write-out '%{http_code}' \
            --request POST "${headers[@]}" \
            "http://${http_addr}/api/v1/proposals/operation%3Amail.send/accept-and-publish")"
        [[ "${publish_status}" == "200" ]] || fail "publish operation proposal returned HTTP ${publish_status}"
    elif [[ "${current_status}" != "200" ]]; then
        fail "read published operation returned HTTP ${current_status}"
    fi

    local execute_response request_id trace_id
    execute_response="$(curl --fail --silent --show-error \
        --request POST "${headers[@]}" \
        --header 'Content-Type: application/json' \
        --data '{"context":{"form":{"player_id":"p-001","title":"Telemetry verification","content":"audit trace correlation"}}}' \
        "http://${http_addr}/api/v1/console/pages/operation--mail.send/bindings/mail.send.main/execute")"
    request_id="$(jq -er '.result.requestId' <<<"${execute_response}")"
    trace_id="$(jq -er '.result.traceId' <<<"${execute_response}")"
    [[ -n "${request_id}" && -n "${trace_id}" ]] || fail "execution response omitted requestId or traceId"

    jq -n --arg request_id "${request_id}" --arg trace_id "${trace_id}" \
        '{requestId: $request_id, traceId: $trace_id}' >"${FIXTURE_DIR}/execution.json"
    log "executed published binding: request_id=${request_id} trace_id=${trace_id}"
}

verify_audit_trace_and_metric() {
    local fixture_addr request_id trace_id audit_trace_id http_addr
    fixture_addr="$(jq -r '.fixtureAddr' "${FIXTURE_STATE}")"
    http_addr="$(jq -r '.httpAddr' "${FIXTURE_STATE}")"
    request_id="$(jq -r '.requestId' "${FIXTURE_DIR}/execution.json")"
    trace_id="$(jq -r '.traceId' "${FIXTURE_DIR}/execution.json")"

    local audit
    audit="$(curl --fail --silent --show-error "http://${fixture_addr}/__fixture__/audit/page-execute")"
    audit_trace_id="$(jq -er '.details.trace_id' <<<"${audit}")"
    [[ "${audit_trace_id}" == "${trace_id}" ]] || fail "audit trace_id does not match execution trace_id"
    jq -e --arg request_id "${request_id}" '
        .eventType == "page.execute" and
        .outcome == "success" and
        .details.request_id == $request_id and
        .details.page_key == "operation--mail.send" and
        .details.binding_id == "mail.send.main" and
        .details.function_id == "mail.send"
    ' >/dev/null <<<"${audit}" || fail "page execution audit lacks published binding context"

    local jaeger_query jaeger_trace_found=0 index
    for ((index = 1; index <= 60; index++)); do
        jaeger_query="$(curl --silent --show-error \
            "http://127.0.0.1:17686/api/traces/${trace_id}" || true)"
        if jq -e --arg trace_id "${trace_id}" '
            any((.data // [])[] | .spans[]?; .traceID == $trace_id and .operationName == "page.binding.execute")
        ' >/dev/null <<<"${jaeger_query}"; then
            jaeger_trace_found=1
            break
        fi
        sleep 1
    done
    [[ "${jaeger_trace_found}" == "1" ]] || fail "Jaeger did not receive page.binding.execute trace ${trace_id}"

    local scrape_found=0 query_result
    for ((index = 1; index <= 60; index++)); do
        query_result="$(curl --silent --show-error \
            --get --data-urlencode 'query=up{job="otel-collector"}' \
            'http://127.0.0.1:19092/api/v1/query' || true)"
        if jq -e '(.status == "success") and ((.data.result // []) | length > 0)' >/dev/null <<<"${query_result}"; then
            scrape_found=1
            break
        fi
        sleep 1
    done
    [[ "${scrape_found}" == "1" ]] || fail "Prometheus is not scraping the Collector exporter"

    log "verified audit -> trace correlation and Collector Prometheus scrape for ${http_addr}"
}

main() {
    require_command docker
    require_command curl
    require_command jq
    require_command rg
    require_command go
    docker info >/dev/null 2>&1 || fail "Docker daemon is unavailable"
    docker compose version >/dev/null 2>&1 || fail "Docker Compose v2 is unavailable"

    start_telemetry_stack
    start_fixture
    login_and_publish
    verify_audit_trace_and_metric
    log "PASS: real Console audit and trace are correlated; Prometheus scrapes the Collector"
}

main "$@"
