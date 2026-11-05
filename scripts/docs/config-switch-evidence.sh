#!/usr/bin/env bash
# DOC-001 config-switch evidence pipeline.
#
# Captures off/on screenshots of the operator console sidebar for a feature
# flag, stitches them into a labeled comparison PNG and a crossfade GIF, and
# writes the artifacts under docs/public/evidence/config-effects/<flag>/.
#
# The pipeline boots a throwaway local croupier-server (SQLite, temp dir) on
# port 28780 with the flag set off then on, driving a headless Chromium
# (Playwright) login against a web dev server that proxies /api to it. The
# sidebar difference (a top-level menu appearing/disappearing) is the evidence.
#
# Prerequisites:
#   - go toolchain (server binary is built on demand to a temp path)
#   - web/node_modules present (Playwright + umi) -- run `pnpm --dir web i`
#   - Playwright chromium browsers -- `pnpm --dir web exec playwright install chromium`
#   - ffmpeg (labels + GIF), Noto Sans CJK font for CJK labels (optional)
#
# Usage:
#   scripts/docs/config-switch-evidence.sh --flag=featureFlags.ops
#
# Env overrides:
#   EVIDENCE_WEB_PORT (default 8001), EVIDENCE_SERVER_PORT (default 28780),
#   EVIDENCE_OUT_DIR (default docs/public/evidence/config-effects)
#
# The web dev server is reused if already serving on $EVIDENCE_WEB_PORT (fast);
# otherwise one is started (and stopped) by this script.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
WEB="$ROOT/web"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

FLAG=""
for arg in "$@"; do
  case "$arg" in
    --flag=*) FLAG="${arg#--flag=}" ;;
    *) echo "unknown arg: $arg" >&2; exit 2 ;;
  esac
done
if [[ -z "$FLAG" ]]; then
  echo "usage: $0 --flag=featureFlags.<name>" >&2
  exit 2
fi

WEB_PORT="${EVIDENCE_WEB_PORT:-8001}"
SERVER_PORT="${EVIDENCE_SERVER_PORT:-28780}"
OUT_DIR="${EVIDENCE_OUT_DIR:-$ROOT/docs/public/evidence/config-effects}"
WEB_BASE="http://localhost:$WEB_PORT"
SERVER_BASE="http://localhost:$SERVER_PORT"
FONT="/usr/share/fonts/opentype/noto/NotoSansCJK-Bold.ttc"

WORK="$(mktemp -d /tmp/evid-XXXXXX)"
SERVER_BIN="$WORK/croupier-server"
SERVER_LOG="$WORK/server.log"
WEB_LOG="$WORK/web.log"
STARTED_WEB=0
SERVER_PID=""

cleanup() {
  # Kill the evidence server before deleting $WORK (its sqlite DB lives there).
  # The pid-file capture makes SERVER_PID reliable; the port kill is a belt-
  # and-braces net against anything left on the dedicated evidence port.
  if [[ -n "${SERVER_PID:-}" ]] && kill -0 "$SERVER_PID" 2>/dev/null; then
    kill "$SERVER_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
  fi
  if [[ -f "$WORK/server.pid" ]]; then
    kill "$(cat "$WORK/server.pid")" 2>/dev/null || true
  fi
  fuser -k "$SERVER_PORT/tcp" 2>/dev/null || true
  if [[ "$STARTED_WEB" = "1" ]] && [[ -n "${WEB_PID:-}" ]] && kill -0 "$WEB_PID" 2>/dev/null; then
    kill "$WEB_PID" 2>/dev/null || true
  fi
  rm -rf "$WORK"
}
trap cleanup EXIT

log() { echo "[evid] $*"; }

build_server() {
  if [[ ! -x "$SERVER_BIN" ]]; then
    log "building server binary..."
    (cd "$ROOT" && go build -o "$SERVER_BIN" ./cmd/server)
  fi
}

ensure_web() {
  if curl -sf -o /dev/null "$WEB_BASE/umi.js"; then
    log "reusing web dev server on $WEB_BASE"
    return
  fi
  log "starting web dev server on $WEB_BASE (proxying to $SERVER_BASE)..."
  # $! is only visible inside the subshell that backgrounds the job; export it
  # via a pid file (set -u makes the outer $! unbound).
  (cd "$WEB" && \
    PORT="$WEB_PORT" REACT_APP_ENV=dev MOCK=none UMI_ENV=dev \
    CROUPIER_SERVER_BASE_URL="$SERVER_BASE" \
    nohup ./node_modules/.bin/max dev >"$WEB_LOG" 2>&1 & echo $! >"$WORK/web.pid")
  WEB_PID=$(cat "$WORK/web.pid")
  STARTED_WEB=1
  for _ in $(seq 1 60); do
    if curl -sf -o /dev/null "$WEB_BASE/umi.js"; then
      log "web dev server ready"
      return
    fi
    sleep 2
  done
  echo "web dev server did not become ready; log: $WEB_LOG" >&2
  exit 1
}

write_config() { # $1 = flag value (true/false)
  local value="$1"
  local cfg="$WORK/server.yaml"
  cp "$ROOT/configs/server-sqlite.yaml" "$cfg"
  python3 - "$cfg" "$WORK" "$SERVER_PORT" "$FLAG" "$value" <<'PY'
import sys
cfg, work, port, flag, value = sys.argv[1:6]
s = open(cfg).read()
s = s.replace("port: 18780", f"port: {port}")
s = s.replace('dataSource: "data/croupier.db"', f'dataSource: "{work}/data/evidence.db"')
s = s.replace("  mode: dev\n", f"  mode: dev\n\nfeatureFlags:\n  {flag.split('.',1)[1]}: {value}\n", 1)
s = s.replace('assignmentsPath: "data/assignments.json"', f'assignmentsPath: "{work}/data/assignments.json"')
s = s.replace('analyticsFiltersPath: "schemas/analytics_filters.json"', f'analyticsFiltersPath: "{work}/data/analytics_filters.json"')
s = s.replace('rateLimitsPath: "data/rate_limits.json"', f'rateLimitsPath: "{work}/data/rate_limits.json"')
s = s.replace('jobRoutingDir: "data"', f'jobRoutingDir: "{work}/data"')
s = s.replace('baseDir: "data/uploads"', f'baseDir: "{work}/data/uploads"')
open(cfg, "w").write(s)
PY
}

start_server() { # $1 = flag value
  # Guard against a foreign listener on the dedicated port: the readiness probe
  # would otherwise hit *that* server and the pipeline would capture the wrong
  # flag state silently. Refuse to run instead.
  if ss -ltn | grep -q ":$SERVER_PORT "; then
    echo "port $SERVER_PORT is busy; kill the listener or set EVIDENCE_SERVER_PORT" >&2
    exit 1
  fi
  write_config "$1"
  # --bootstrap-data-dir is passed explicitly: the flag's computed default
  # (execDir/configs, cmd/server/root.go) always overrides the YAML
  # bootstrapData.baseDir, and a stray <execDir>/configs dir would silently
  # hijack admin seeding. Pin it to the repo configs dir for determinism.
  (cd "$ROOT" && nohup "$SERVER_BIN" server --config "$WORK/server.yaml" \
    --bootstrap-data-dir "$ROOT/configs" >"$SERVER_LOG" 2>&1 & echo $! >"$WORK/server.pid")
  SERVER_PID=$(cat "$WORK/server.pid")
  for _ in $(seq 1 30); do
    if curl -sf -o /dev/null "$SERVER_BASE/api/v1/auth/providers"; then
      return
    fi
    sleep 1
  done
  echo "server did not become ready; log tail:" >&2
  tail -20 "$SERVER_LOG" >&2
  exit 1
}

stop_server() {
  # kill by port (fuser) rather than by pid: the pid file captures a wrapper
  # subshell (cd && nohup ... &), whose child is the real server, so a pid
  # kill would leave an orphan holding the port.
  fuser -k "$SERVER_PORT/tcp" 2>/dev/null || true
  for _ in $(seq 1 40); do
    if ! ss -ltn | grep -q ":$SERVER_PORT "; then
      break
    fi
    sleep 0.25
  done
  SERVER_PID=""
}

capture() { # $1 = out png
  # 直接截取侧栏元素（而非全页再 crop）：不依赖侧栏宽度假设，侧栏非 260px
  # 时也不会裁错/裁断。--full 只影响 JS 内截图方式（元素 vs 全页）。
  NODE_PATH="$WEB/node_modules" node "$SCRIPT_DIR/config-evidence-capture.js" "$1"
}

resolve_font() { # echo 可用字体路径或空（缺省 ffmpeg 默认）
  if [[ -f "$FONT" ]]; then
    echo "$FONT"
    return
  fi
  local f
  f="$(fc-match -f '%{file}' 'Noto Sans CJK' 2>/dev/null | head -1)"
  if [[ -n "$f" && -f "$f" ]]; then
    echo "$f"
    return
  fi
  echo ""
}

label_frame() { # $1 = in png, $2 = out png, $3 = label text, $4 = color
  local font="$(resolve_font)"
  local fontopt=""
  if [[ -n "$font" ]]; then
    fontopt="fontfile=$font:"
  else
    log "warning: CJK font not found ($FONT); label may render with fallback glyphs"
  fi
  ffmpeg -y -loglevel error -i "$1" \
    -vf "pad=ceil(iw/2)*2:ih+52:0:52:color=white,drawtext=${fontopt}text='$3':x=14:y=18:fontsize=16:fontcolor=$4" \
    "$2"
}

main() {
  build_server
  ensure_web
  mkdir -p "$OUT_DIR/$FLAG"

  local name="${FLAG#featureFlags.}"
  local dir="$OUT_DIR/$FLAG"

  # OFF
  start_server false
  sleep 2
  capture "$WORK/off.png"
  label_frame "$WORK/off.png" "$dir/${name}-off.png" "$FLAG = false" 0xc03030
  stop_server
  sleep 1

  # ON
  start_server true
  sleep 2
  capture "$WORK/on.png"
  label_frame "$WORK/on.png" "$dir/${name}-on.png" "$FLAG = true" 0x1f7a3d
  stop_server

  # Comparison + GIF
  ffmpeg -y -loglevel error -i "$dir/${name}-off.png" -i "$dir/${name}-on.png" \
    -filter_complex "[0][1]hstack=inputs=2" "$dir/${name}-compare.png"
  ffmpeg -y -loglevel error -loop 1 -t 2.2 -i "$dir/${name}-off.png" -loop 1 -t 2.2 -i "$dir/${name}-on.png" \
    -filter_complex "[0][1]xfade=transition=fade:duration=0.6:offset=1.6,format=rgb24,split[a][b];[a]palettegen=stats_mode=diff[p];[b][p]paletteuse" \
    -loop 0 "$dir/${name}.gif"

  log "artifacts written to $dir/"
  ls -la "$dir/" | sed 's/^/  /'
}

main
