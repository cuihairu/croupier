#!/usr/bin/env bash
#
# Croupier Agent 一键安装脚本（Linux / macOS）
#
# 三类 agent 统一入口（--agent 选型；差别见 docs/zh/operations/agent-comparison.md）：
#   sidecar  业务旁挂 agent：函数注册/调用、健康与指标上报（默认，历史行为）
#   devops   内网堡垒机式远程接入 agent（远程会话协议通道接线前只注册上线）
#   capture  CDC 数据变更捕获 agent（capture.enabled 缺省关，先注册后开闸）
#
# 远程执行（默认装 sidecar + 最新稳定版）：
#   curl -fsSL https://raw.githubusercontent.com/cuihairu/croupier/main/scripts/install.sh | bash -s --
#   curl -fsSL ... | bash -s -- --agent capture --server gm.example.com:19090 --game-id demo
#
# 参数（跟在 `bash -s --` 之后；本地执行直接跟在脚本名后）：
#   --agent <sidecar|devops|capture>   安装哪类 agent（默认 sidecar）
#   --version <latest|nightly|vX.Y.Z>  安装版本。latest=最新稳定版（默认）、
#                                      nightly=每日构建、vX.Y.Z=钉定版本
#   --local <dir>                      离线产物目录（内网/验证用）：直接取目录下
#                                      <bin-name> 或 croupier-bin-<os>-<arch>.tar.gz，
#                                      跳过 GitHub 下载
#   --bin-dir <dir>                    安装目录，默认 /usr/local/bin；
#                                      不可写且无免密 sudo 时自动回退 ~/.local/bin
#   --config-dir <dir>                 配置目录，默认 /etc/croupier（不可写回退 ~/.croupier）
#   --with-service                     注册开机自启（Linux: systemd root；
#                                      macOS: launchd，root 装系统级、否则用户级）
#   --server <addr>                    Server 上游地址（如 gm.example.com:19090），
#                                      写入已落盘配置（sidecar: server.addr，
#                                      devops/capture: agent.serverAddr）
#   --agent-id <id>                    注册用 agent 唯一 id（默认 <role>-主机短名）
#   --game-id <id>                     游戏域标签（默认 default；sidecar 启动强校验非空）
#   --env <name>                       环境标签（默认 dev；sidecar 启动强校验非空）
#   --silent                           静默安装：禁用一切交互提示（管道/CI 必用；
#                                      非 TTY 下自动等同静默）
#   --uninstall                        卸载（停止服务 + 删除二进制；配置不动数据保留）
#   -h, --help                         帮助
#
# 环境变量（piped 场景不便传参时）：
#   CROUPIER_INSTALL_AGENT / CROUPIER_INSTALL_VERSION / CROUPIER_INSTALL_LOCAL_DIR
#   CROUPIER_INSTALL_BIN_DIR / CROUPIER_INSTALL_CONFIG_DIR
#   CROUPIER_INSTALL_WITH_SERVICE=1
#   CROUPIER_INSTALL_SERVER / CROUPIER_INSTALL_AGENT_ID
#   CROUPIER_INSTALL_GAME_ID / CROUPIER_INSTALL_ENV / CROUPIER_INSTALL_SILENT=1
#
# 幂等：重复执行即覆盖升级。失败即停，报错带明确原因。
# 产物来自 GitHub Releases（匿名可下）：croupier-bin-<os>-<arch>.tar.gz
# （nightly 起含四二进制：croupier-server / croupier-agent / devops-agent /
# capture-agent），本脚本只安装所选 agent（server 另行部署）。
# 注意：三个 agent 的配置加载均无 ${ENV} 展开层——含密码的配置文件须
# root:0600 落盘，或部署侧模板渲染后再交付；密钥绝不入仓库。

set -euo pipefail

REPO="cuihairu/croupier"
RAW_BASE="https://raw.githubusercontent.com/${REPO}"
RELEASE_BASE="https://github.com/${REPO}/releases"

VERSION="${CROUPIER_INSTALL_VERSION:-latest}"
AGENT_KIND="${CROUPIER_INSTALL_AGENT:-sidecar}"
LOCAL_DIR="${CROUPIER_INSTALL_LOCAL_DIR:-}"
BIN_DIR="${CROUPIER_INSTALL_BIN_DIR:-}"
CONFIG_DIR="${CROUPIER_INSTALL_CONFIG_DIR:-}"
WITH_SERVICE="${CROUPIER_INSTALL_WITH_SERVICE:-0}"
SERVER_ADDR="${CROUPIER_INSTALL_SERVER:-}"
AGENT_ID="${CROUPIER_INSTALL_AGENT_ID:-}"
GAME_ID="${CROUPIER_INSTALL_GAME_ID:-default}"
ENV_NAME="${CROUPIER_INSTALL_ENV:-dev}"
SILENT="${CROUPIER_INSTALL_SILENT:-0}"
UNINSTALL=0

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[0;33m'; NC='\033[0m'
info() { printf "${GREEN}[INFO]${NC} %s\n" "$*"; }
warn() { printf "${YELLOW}[WARN]${NC} %s\n" "$*"; }
die()  { printf "${RED}[ERROR]${NC} %s\n" "$*" >&2; exit 1; }

usage() {
  sed -n '3,49p' "${BASH_SOURCE[0]}"
}

# ---- 参数解析 ------------------------------------------------------------

while [ $# -gt 0 ]; do
  case "$1" in
    --agent)        [ $# -ge 2 ] || die "--agent 需要取值"; AGENT_KIND="$2"; shift 2 ;;
    --agent=*)      AGENT_KIND="${1#--agent=}"; shift ;;
    --version)      [ $# -ge 2 ] || die "--version 需要取值"; VERSION="$2"; shift 2 ;;
    --version=*)    VERSION="${1#--version=}"; shift ;;
    --local)        [ $# -ge 2 ] || die "--local 需要取值"; LOCAL_DIR="$2"; shift 2 ;;
    --local=*)      LOCAL_DIR="${1#--local=}"; shift ;;
    --bin-dir)      [ $# -ge 2 ] || die "--bin-dir 需要取值"; BIN_DIR="$2"; shift 2 ;;
    --bin-dir=*)    BIN_DIR="${1#--bin-dir=}"; shift ;;
    --config-dir)   [ $# -ge 2 ] || die "--config-dir 需要取值"; CONFIG_DIR="$2"; shift 2 ;;
    --config-dir=*) CONFIG_DIR="${1#--config-dir=}"; shift ;;
    --with-service) WITH_SERVICE=1; shift ;;
    --server)       [ $# -ge 2 ] || die "--server 需要取值"; SERVER_ADDR="$2"; shift 2 ;;
    --server=*)     SERVER_ADDR="${1#--server=}"; shift ;;
    --agent-id)     [ $# -ge 2 ] || die "--agent-id 需要取值"; AGENT_ID="$2"; shift 2 ;;
    --agent-id=*)   AGENT_ID="${1#--agent-id=}"; shift ;;
    --game-id)      [ $# -ge 2 ] || die "--game-id 需要取值"; GAME_ID="$2"; shift 2 ;;
    --game-id=*)    GAME_ID="${1#--game-id=}"; shift ;;
    --env)          [ $# -ge 2 ] || die "--env 需要取值"; ENV_NAME="$2"; shift 2 ;;
    --env=*)        ENV_NAME="${1#--env=}"; shift ;;
    --silent)       SILENT=1; shift ;;
    --uninstall)    UNINSTALL=1; shift ;;
    -h|--help)      usage; exit 0 ;;
    *)              die "未知参数: $1（--help 查看用法）" ;;
  esac
done

# ---- agent 选型（二进制/服务名/配置名/launchd 标签四件套） -------------------

case "$AGENT_KIND" in
  sidecar)
    BIN_NAME="croupier-agent"
    SERVICE_NAME="croupier-agent"
    CFG_NAME="agent.yaml"
    AGENT_DESC="Croupier Sidecar Agent"
    LAUNCHD_LABEL="com.github.cuihairu.croupier.agent"
    ;;
  devops)
    BIN_NAME="devops-agent"
    SERVICE_NAME="croupier-devops-agent"
    CFG_NAME="devops-agent.yaml"
    AGENT_DESC="Croupier Devops Agent"
    LAUNCHD_LABEL="com.github.cuihairu.croupier.devops-agent"
    ;;
  capture)
    BIN_NAME="capture-agent"
    SERVICE_NAME="croupier-capture-agent"
    CFG_NAME="capture-agent.yaml"
    AGENT_DESC="Croupier Capture Agent"
    LAUNCHD_LABEL="com.github.cuihairu.croupier.capture-agent"
    ;;
  *)
    die "未知 agent 类型: $AGENT_KIND（可选 sidecar | devops | capture）" ;;
esac

# ---- 平台识别 ------------------------------------------------------------

OS_RAW="$(uname -s)"
case "$OS_RAW" in
  Linux)  OS=linux ;;
  Darwin) OS=darwin ;;
  *) die "不支持的操作系统: ${OS_RAW}（本脚本支持 Linux / macOS，Windows 用 scripts/install.ps1）" ;;
esac

ARCH_RAW="$(uname -m)"
case "$ARCH_RAW" in
  x86_64|amd64)    ARCH=amd64 ;;
  aarch64|arm64)   ARCH=arm64 ;;
  armv7l|armv7*|armv8l) ARCH=armv7 ;;  # armv8l = armv8 硬件跑 32 位用户态，可运行 armv7 产物
  *) die "不支持的 CPU 架构: ${ARCH_RAW}
已发布产物: linux-amd64 / linux-arm64 / linux-armv7 / darwin-amd64 / darwin-arm64 / windows-amd64
完整清单: ${RELEASE_BASE}/expanded_assets/${VERSION}" ;;
esac

have() { command -v "$1" >/dev/null 2>&1; }

download() { # $1=url $2=dest
  if have curl; then
    curl -fL --retry 3 --retry-delay 2 -o "$2" "$1" || die "下载失败: $1
检查网络连通性，或该平台产物是否已发布: ${RELEASE_BASE}"
  elif have wget; then
    wget -q -O "$2" "$1" || die "下载失败: $1
检查网络连通性，或该平台产物是否已发布: ${RELEASE_BASE}"
  else
    die "需要 curl 或 wget 之一下载文件"
  fi
}

# latest = 最新稳定版（releases/latest 302 跳转解析，不经 API、无速率限制、匿名可用）
resolve_latest_tag() {
  if have curl; then
    curl -fsSL -o /dev/null -w '%{url_effective}' "${RELEASE_BASE}/latest" | sed 's#.*/tag/##; s#/$##'
  elif have wget; then
    wget -qS --spider "${RELEASE_BASE}/latest" 2>&1 | awk 'tolower($1)=="location:"{print $2}' |
      head -1 | sed 's#.*/tag/##; s#/$##'
  else
    die "需要 curl 或 wget"
  fi
}

# ---- Server 地址（--server 静默 / TTY 交互提示） ---------------------------
# 目标：安装完成即可连上，不必二次手改配置；--silent 或非 TTY（curl|bash 管道）
# 下不交互——管道里 read 会读到脚本自身 stdin，必须防。

SERVER_APPLIED=0
# 交互前判定来源：此时 SERVER_ADDR 非空 = --server / env 显式提供（交互会
# 改写同名变量，故须在交互之前记账——显式给的非法地址必须 die，不能默默忽略）
if [ -n "$SERVER_ADDR" ]; then SERVER_PROVIDED=1; else SERVER_PROVIDED=0; fi

valid_server_addr() { # host / host:port / [v6]:port；拒引号反斜杠防 YAML 注入
  # 括号类零反斜杠：POSIX 括号类内 \ 不是转义，GNU grep 下 \] 会提前闭括号
  # 导致永不匹配；] 放最前、- 放最后即字面量，GNU grep / ugrep 语义一致
  printf '%s' "$1" | grep -Eq '^[][A-Za-z0-9._:-]+$'
}

apply_server_addr() { # $1=配置文件路径；sidecar 改 server.addr、devops/capture 改 agent.serverAddr
  [ -n "$SERVER_ADDR" ] || return 0
  [ -f "$1" ] || return 0
  local tmp="$1.tmp.$$"
  if awk -v new="$SERVER_ADDR" '
      /^server:[ \t]*(#.*)?$/  { in_server = 1; in_agent = 0; print; next }
      /^agent:[ \t]*(#.*)?$/   { in_agent = 1; in_server = 0; print; next }
      /^[^ \t]/                { in_server = 0; in_agent = 0 }
      in_server && /^[ \t]+addr:/        { sub(/addr:.*/, "addr: \"" new "\""); done = 1 }
      in_agent && /^[ \t]+serverAddr:/   { sub(/serverAddr:.*/, "serverAddr: \"" new "\""); done = 1 }
      { print }
      END { exit done ? 0 : 1 }
    ' "$1" > "$tmp"; then
    cat "$tmp" > "$1" && rm -f "$tmp"
    SERVER_APPLIED=1
    info "已写入上游地址 = ${SERVER_ADDR}（$1）"
  else
    rm -f "$tmp"
    warn "未在 $1 找到上游地址行（server.addr / agent.serverAddr），请手动设置"
  fi
}

if [ "$UNINSTALL" -eq 0 ] && [ -z "$SERVER_ADDR" ] && [ "$SILENT" != 1 ] && [ -t 0 ]; then
  printf 'Server 上游地址（host:port，如 gm.example.com:19090；回车跳过，稍后手配）> '
  read -r _server_reply || _server_reply=""
  SERVER_ADDR="${_server_reply:-}"
fi
if [ -n "$SERVER_ADDR" ] && ! valid_server_addr "$SERVER_ADDR"; then
  if [ "$SERVER_PROVIDED" -eq 1 ]; then
    die "server 地址含非法字符: ${SERVER_ADDR}"
  fi
  warn "输入的 server 地址含非法字符，已忽略（可稍后手动编辑 ${CFG_NAME}）"
  SERVER_ADDR=""
fi

# ---- agent id 默认值（<role>-主机短名；显式传入优先） ------------------------

if [ -z "$AGENT_ID" ]; then
  HOST_SHORT="$(hostname -s 2>/dev/null || hostname 2>/dev/null || echo host)"
  case "$AGENT_KIND" in
    sidecar)  AGENT_ID="sidecar-${HOST_SHORT}" ;;
    devops)   AGENT_ID="devops-${HOST_SHORT}" ;;
    capture)  AGENT_ID="capture-${HOST_SHORT}" ;;
  esac
fi

# ---- 配置目录与初始配置生成 --------------------------------------------------
# 三个 agent 均无 ${ENV} 展开层：初始配置不含任何密钥值（密码留空、证书给路径
# 占位注释）；含密码时须 root:0600。逐字段说明见 docs/operations/agent-config.md。

resolve_config_dir() {
  if [ -n "$CONFIG_DIR" ]; then
    printf '%s' "$CONFIG_DIR"; return 0
  fi
  if [ "$(id -u)" -eq 0 ] || [ -w /etc/croupier ] || { have sudo && sudo -n true 2>/dev/null; }; then
    printf '/etc/croupier'
  else
    printf '%s/.croupier' "$HOME"
  fi
}

write_initial_config() { # $1=目标文件路径；已存在则保留（幂等升级不改配置）
  local dest="$1"
  [ -f "$dest" ] && { info "保留既有配置: $dest"; return 0; }
  mkdir -p "$(dirname "$dest")" 2>/dev/null || true
  case "$AGENT_KIND" in
    sidecar)
      cat > "$dest" <<EOF
# croupier-agent（sidecar）初始配置 —— scripts/install.sh 生成
# 逐字段说明: docs/operations/config-agent.md（业务旁挂: 函数注册/调用 + 健康上报）
server:
  addr: "${SERVER_ADDR}"        # 上游 Croupier Server 控制面 host:port（空 = 不连上游）
  transport: tcp
  insecure: true                # 生产建议 TLS: false + 下方证书三件套
  # caFile: /etc/croupier/certs/ca.pem
  # tlsCertFile: /etc/croupier/certs/agent.pem
  # tlsKeyFile: /etc/croupier/certs/agent-key.pem
  # serverName: croupier-server.internal

agent:
  id: "${AGENT_ID}"             # 注册用唯一 id（空 = 自动 <hostname>-<ts>）
  gameId: "${GAME_ID}"          # 必填: 启动强校验非空
  env: "${ENV_NAME}"            # 必填: 启动强校验非空
  localAddr: "0.0.0.0:19091"    # 本地网关: 业务进程 SDK 注册函数的入口
  labels: {}

upstream:
  heartbeatInterval: 10         # 秒；核心默认 3s

log:
  level: info
  format: console

ops:
  enabled: true
  metricsInterval: 30s
  metricsEnabled: true
EOF
      ;;
    devops)
      cat > "$dest" <<EOF
# devops-agent 初始配置 —— scripts/install.sh 生成
# 逐字段说明: docs/operations/agent-config.md（内网堡垒机式远程接入）
agent:
  id: "${AGENT_ID}"             # 注册用唯一 id（必填）
  serverAddr: "${SERVER_ADDR}"  # 上游控制面 host:port（必填）
  gameId: "${GAME_ID}"          # 必填
  env: "${ENV_NAME}"
  heartbeatFile: /var/lib/croupier/devops/agent-heartbeat

outboundTLS:
  enabled: false                # 生产建议 true + caFile（校验服务端）+ cert/key（mTLS）
  # caFile: /etc/croupier/certs/ca.pem
  # certFile: /etc/croupier/certs/devops.pem
  # keyFile: /etc/croupier/certs/devops-key.pem

devops:
  enabled: false                # false = 只注册上线不接远程会话（会话协议通道接线前恒拒绝）
  sessionDir: /var/lib/croupier/devops/sessions
  execlogDir: /var/lib/croupier/devops/execlog   # 执行审计留痕（逐会话记录）
  allowlist: []                 # enabled: true 时必填非空的主机白名单，例如:
  # - host: 10.0.0.21
  #   ports: [22]
  #   protocols: [ssh]
EOF
      ;;
    capture)
      cat > "$dest" <<EOF
# capture-agent 初始配置 —— scripts/install.sh 生成
# 逐字段说明: docs/operations/agent-config.md（CDC 数据变更捕获 + 三闸风控）
agent:
  id: "${AGENT_ID}"             # 注册用唯一 id（必填）
  serverAddr: "${SERVER_ADDR}"  # 上游控制面 host:port（必填）
  gameId: "${GAME_ID}"          # 必填
  env: "${ENV_NAME}"
  heartbeatFile: /var/lib/croupier/capture/agent-heartbeat

outboundTLS:
  enabled: false                # 生产建议 true + caFile + cert/key

capture:
  enabled: false                # false = 只注册上线；开启前确认 MySQL 前置:
                                #   binlog_format=ROW / binlog_row_image=FULL /
                                #   gtid_mode=ON / 账号有 REPLICATION SLAVE / serverId 不冲突
  bookmarkDir: /var/lib/croupier/capture/bookmarks
  bookmarkFlushIntervalSec: 5
  mysql:
    host: 127.0.0.1
    port: 3306
    user: repl
    password: ""                # 明文字段（无 \${ENV} 展开）: 本文件须 root:0600
    serverId: 4242              # 与源实例及其他 binlog 订阅者不冲突
    tables: {}                  # enabled: true 时必填非空白名单（"库.表": true）
  gates: {}                     # 空 = 只采不判；规则建议走面板 capture.gates 热下发
EOF
      ;;
  esac
  chmod 0600 "$dest"
  info "已生成初始配置: $dest（已存在则保留不改）"
}

# ---- 卸载 ----------------------------------------------------------------

if [ "$UNINSTALL" -eq 1 ]; then
  FAILED=0
  # 安装可能经免密 sudo 落进 root 目录（/usr/local/bin），卸载须同权回收
  privileged() {
    if [ "$(id -u)" -eq 0 ]; then "$@"; return $?; fi
    if "$@" 2>/dev/null; then return 0; fi
    if have sudo && sudo -n "$@" 2>/dev/null; then return 0; fi
    return 1
  }
  try_remove() {
    [ -e "$1" ] || return 0
    if rm -f "$1" 2>/dev/null || { have sudo && sudo -n rm -f "$1" 2>/dev/null; }; then
      info "已删除: $1"
    else
      warn "无权限删除: $1（请重跑: curl -fsSL ${RAW_BASE}/main/scripts/install.sh | sudo bash -s -- --agent ${AGENT_KIND} --uninstall）"
      FAILED=1
    fi
  }

  if [ "$OS" = linux ] && have systemctl; then
    if privileged systemctl disable --now "${SERVICE_NAME}"; then
      info "已停止并禁用服务: ${SERVICE_NAME}"
    fi
    if [ -f "/etc/systemd/system/${SERVICE_NAME}.service" ]; then
      try_remove "/etc/systemd/system/${SERVICE_NAME}.service"
      privileged systemctl daemon-reload || true
      info "已移除 systemd 服务: ${SERVICE_NAME}"
    fi
  fi
  if [ "$OS" = darwin ]; then
    for PLIST in "/Library/LaunchDaemons/${LAUNCHD_LABEL}.plist" \
                 "$HOME/Library/LaunchAgents/${LAUNCHD_LABEL}.plist"; do
      if [ -f "$PLIST" ]; then
        launchctl bootout "system/${LAUNCHD_LABEL}" 2>/dev/null \
          || launchctl bootout "gui/$(id -u)/${LAUNCHD_LABEL}" 2>/dev/null || true
        try_remove "$PLIST"
        info "已移除 launchd 配置: $PLIST"
      fi
    done
  fi
  for CAND in "${BIN_DIR:-}/$BIN_NAME" /usr/local/bin/$BIN_NAME "$HOME/.local/bin/$BIN_NAME"; do
    try_remove "$CAND"
  done
  [ "$FAILED" -eq 0 ] || die "卸载未完成（见上方 WARN），请用 sudo 重跑"
  info "卸载完成（配置文件保留，如需清理请手动删除 ${CFG_NAME}）"
  exit 0
fi

# ---- 解析版本与取产物 ------------------------------------------------------

if [ "$ARCH" = "armv7" ] && [ "$AGENT_KIND" = "capture" ]; then
  die "capture-agent 的 CDC 解析依赖目前不支持 32 位 ARMv7，请使用 Linux ARM64 或 amd64；ARMv7 产物仅提供 sidecar/devops"
fi

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

if [ -n "$LOCAL_DIR" ]; then
  # 离线模式：本地目录直取二进制或官方归档（内网部署 / 干净环境验证）
  [ -d "$LOCAL_DIR" ] || die "本地产物目录不存在: $LOCAL_DIR"
  AGENT_SRC="$(find "$LOCAL_DIR" -maxdepth 2 -type f -name "$BIN_NAME" | head -1)"
  if [ -n "$AGENT_SRC" ]; then
    info "本地产物: $AGENT_SRC"
  else
    TARBALL="$(find "$LOCAL_DIR" -maxdepth 2 -type f -name "croupier-bin-${OS}-${ARCH}.tar.gz" | head -1)"
    [ -n "$TARBALL" ] || die "本地目录未找到 ${BIN_NAME} 或 croupier-bin-${OS}-${ARCH}.tar.gz: $LOCAL_DIR"
    tar -xzf "$TARBALL" -C "$TMP"
    AGENT_SRC="$(find "$TMP" -maxdepth 2 -type f -name "$BIN_NAME" | head -1)"
    [ -n "$AGENT_SRC" ] || die "归档中未找到 ${BIN_NAME}: $TARBALL"
    info "本地归档: $TARBALL"
  fi
else
  if [ "$VERSION" = "latest" ]; then
    TAG="$(resolve_latest_tag)"
    [ -n "$TAG" ] || die "无法解析最新稳定版 tag（网络问题或仓库无正式 release），可显式指定 --version nightly 或 --version vX.Y.Z"
    info "最新稳定版: $TAG"
  else
    TAG="$VERSION"
  fi

  URL="${RELEASE_BASE}/download/${TAG}/croupier-bin-${OS}-${ARCH}.tar.gz"
  info "目标产物: $URL"

  download "$URL" "$TMP/archive.tar.gz"

  # 归档由 nightly/CI 打包（tar -C bin .），含 server 与三类 agent；解包后取所选 agent
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP"
  AGENT_SRC="$(find "$TMP" -maxdepth 2 -type f -name "$BIN_NAME" | head -1)"
  [ -n "$AGENT_SRC" ] || die "归档中未找到 ${BIN_NAME}，产物内容异常（nightly 构建含全部四二进制）"
fi

# ---- 安装 ----------------------------------------------------------------

SUDO=""
if [ -z "$BIN_DIR" ]; then
  if [ "$(id -u)" -eq 0 ] || [ -w /usr/local/bin ]; then
    BIN_DIR=/usr/local/bin
  elif have sudo && sudo -n true 2>/dev/null; then
    BIN_DIR=/usr/local/bin
    SUDO="sudo"
  else
    BIN_DIR="$HOME/.local/bin"
  fi
elif [ ! -w "$BIN_DIR" ] && [ "$(id -u)" -ne 0 ] && have sudo && sudo -n true 2>/dev/null; then
  SUDO="sudo"
fi

mkdir -p "$BIN_DIR" 2>/dev/null || $SUDO mkdir -p "$BIN_DIR" ||
  die "无法创建安装目录: $BIN_DIR（用 --bin-dir 指定可写目录，如 --bin-dir ~/.local/bin）"

TARGET="${BIN_DIR%/}/$BIN_NAME"

OLD_VER=""
if [ -x "$TARGET" ]; then
  # 2>&1：sidecar 版本走 stdout（cobra），devops/capture 走 slog 时在 stderr，
  # 两类都要能捕到（老版本输出面不定，双兜底）
  OLD_VER="$("$TARGET" -version 2>&1 | head -1 || "$TARGET" --version 2>&1 | head -1 || true)"
fi

if [ -n "$SUDO" ]; then
  $SUDO install -m 0755 "$AGENT_SRC" "$TARGET"
else
  if command -v install >/dev/null 2>&1; then
    install -m 0755 "$AGENT_SRC" "$TARGET"
  else
    cp "$AGENT_SRC" "$TARGET" && chmod 0755 "$TARGET"
  fi
fi

# ---- 校验 ----------------------------------------------------------------
# sidecar 是 cobra（--version），devops/capture 是 Go flag（-version 亦可 --version）；
# 捕获带 2>&1——slog 系版本输出在 stderr，只捕 stdout 会误判为空

NEW_VER="$("$TARGET" -version 2>&1 | head -1 || "$TARGET" --version 2>&1 | head -1)" ||
  die "安装后校验失败: $TARGET -version 未正常输出"
[ -n "$NEW_VER" ] || die "安装后校验失败: $TARGET -version 输出为空"

if [ -n "$OLD_VER" ] && [ "$OLD_VER" != "$NEW_VER" ]; then
  info "已升级: $OLD_VER -> $NEW_VER"
elif [ -n "$OLD_VER" ]; then
  info "已重装（版本不变）: $NEW_VER"
else
  info "已安装: $NEW_VER"
fi
info "二进制位置: $TARGET"

case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) warn "$BIN_DIR 不在 PATH 中。请加入 shell 配置（如 ~/.bashrc / ~/.zshrc）:
    export PATH=\"$BIN_DIR:\$PATH\"" ;;
esac

# ---- 初始配置（始终生成；已存在则保留） --------------------------------------

CFG_ROOT="$(resolve_config_dir)"
CFG_PATH="${CFG_ROOT%/}/$CFG_NAME"

if [ -f "$CFG_PATH" ]; then
  info "保留既有配置: $CFG_PATH"
  apply_server_addr "$CFG_PATH"
elif mkdir -p "$(dirname "$CFG_PATH")" 2>/dev/null; then
  write_initial_config "$CFG_PATH"
  apply_server_addr "$CFG_PATH"
elif have sudo && sudo -n mkdir -p "$(dirname "$CFG_PATH")" 2>/dev/null; then
  TMP_CFG="$(mktemp)"
  write_initial_config "$TMP_CFG"
  sudo -n install -m 0600 "$TMP_CFG" "$CFG_PATH"
  rm -f "$TMP_CFG"
  info "已生成初始配置: $CFG_PATH"
  # 已存在路径不会走到这（上方 -f 分支拦住），此处必然是新文件，无需再 apply
else
  warn "无法写配置目录 $(dirname "$CFG_PATH")，请手动创建 ${CFG_NAME}（模板见 docs/operations/agent-config.md）"
fi

# ---- 可选：注册开机自启服务 --------------------------------------------------

if [ "$WITH_SERVICE" -eq 1 ]; then
  if [ "$OS" = linux ]; then
    have systemctl || die "--with-service 需要 systemd（当前系统无 systemctl），请手动配置服务"
    [ "$(id -u)" -eq 0 ] || die "--with-service 需要 root 权限：
    curl -fsSL ${RAW_BASE}/main/scripts/install.sh | sudo bash -s -- --agent ${AGENT_KIND} --with-service"

    cat > "$TMP/${SERVICE_NAME}.service" <<EOF
[Unit]
Description=${AGENT_DESC}
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=${TARGET} --config ${CFG_PATH}
Restart=on-failure
RestartSec=5
NoNewPrivileges=true
# 注意：三个 agent 的配置加载均无 \${ENV} 展开层——密码直接写配置文件并保持
# root:0600，或部署侧模板渲染后再交付；逐字段说明见 docs/operations/agent-config.md

[Install]
WantedBy=multi-user.target
EOF
    install -m 0644 "$TMP/${SERVICE_NAME}.service" "/etc/systemd/system/${SERVICE_NAME}.service"
    systemctl daemon-reload
    systemctl enable --now "${SERVICE_NAME}"
    info "服务已注册并启动: systemctl status ${SERVICE_NAME}"
    info "日志: journalctl -u ${SERVICE_NAME} -f"
  else
    # macOS launchd：root 装系统级 LaunchDaemon，普通用户装用户级 LaunchAgent
    if [ "$(id -u)" -eq 0 ]; then
      PLIST_DIR="/Library/LaunchDaemons"
      DOMAIN="system"
    else
      PLIST_DIR="$HOME/Library/LaunchAgents"
      DOMAIN="gui/$(id -u)"
    fi
    mkdir -p "$PLIST_DIR"
    cat > "$TMP/${LAUNCHD_LABEL}.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>${LAUNCHD_LABEL}</string>
    <key>ProgramArguments</key>
    <array>
        <string>${TARGET}</string>
        <string>--config</string>
        <string>${CFG_PATH}</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/tmp/${LAUNCHD_LABEL}.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/${LAUNCHD_LABEL}.err.log</string>
</dict>
</plist>
EOF
    launchctl bootout "$DOMAIN/$LAUNCHD_LABEL" 2>/dev/null || true
    cp "$TMP/${LAUNCHD_LABEL}.plist" "$PLIST_DIR/"
    launchctl bootstrap "$DOMAIN" "$PLIST_DIR/${LAUNCHD_LABEL}.plist"
    launchctl enable "$DOMAIN/$LAUNCHD_LABEL"
    info "launchd 服务已注册: launchctl print $DOMAIN/$LAUNCHD_LABEL"
    info "日志: /tmp/${LAUNCHD_LABEL}.log"
  fi
fi

if [ -n "$SERVER_ADDR" ] && [ "$SERVER_APPLIED" -eq 0 ]; then
  warn "本次未把 Server 地址写进配置（${CFG_PATH}），请手动补 ${CFG_NAME} 的上游地址字段"
fi

DOCS_HINT="docs/zh/operations/agent-comparison.md（选型）/ agent-config.md（配置逐字段）"
case "$AGENT_KIND" in
  sidecar)
    NEXT="业务进程经 SDK 连 agent.localAddr（默认 TCP 19091）注册函数" ;;
  devops)
    NEXT="当前仅注册上线（devops.enabled=false）；远程会话协议通道接线后按白名单开启" ;;
  capture)
    NEXT="当前只注册上线（capture.enabled=false）；开闸前配置 mysql.tables 白名单与源库权限" ;;
esac
info "完成。配置: ${CFG_PATH}
下一步: ${NEXT}
文档: ${DOCS_HINT}
启动: ${TARGET} --config ${CFG_PATH}"
