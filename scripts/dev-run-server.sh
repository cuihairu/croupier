#!/usr/bin/env bash
set -euo pipefail

# Simple local dev runner for croupier-server with IP2Location enabled if BINs exist under ./configs
# Usage: scripts/dev-run-server.sh

ROOT_DIR="$(cd "$(dirname "$0")"/.. && pwd)"
BIN="${ROOT_DIR}/bin/croupier-server"

if [[ ! -x "$BIN" ]]; then
  echo "[dev-run] server binary not found. Building..."
  make -C "$ROOT_DIR" server >/dev/null
fi

# Configure IP2Location BIN paths if present
IPV4_BIN="${ROOT_DIR}/configs/IP2LOCATION-LITE-DB3.BIN"
IPV6_BIN="${ROOT_DIR}/configs/IP2LOCATION-LITE-DB3.IPV6.BIN"
if [[ -f "$IPV4_BIN" ]]; then export IP2LOCATION_BIN_PATH="$IPV4_BIN"; fi
if [[ -f "$IPV6_BIN" ]]; then export IP2LOCATION_BIN_PATH_V6="$IPV6_BIN"; fi
export GEOIP_TIMEOUT_MS="800"

mkdir -p "$ROOT_DIR/logs"
CONFIG_FILE="${ROOT_DIR}/services/server/etc/server.yaml"

if [[ ! -f "$CONFIG_FILE" ]]; then
  echo "[dev-run] config file not found: $CONFIG_FILE"
  exit 1
fi

exec "$BIN" --config "$CONFIG_FILE"
