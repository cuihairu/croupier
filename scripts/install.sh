#!/usr/bin/env bash
#
# Croupier Agent 一键安装脚本（Linux / macOS）
#
# 远程执行（默认装最新稳定版）：
#   curl -fsSL https://raw.githubusercontent.com/cuihairu/croupier/main/scripts/install.sh | bash -s --
#
# 参数（跟在 `bash -s --` 之后；本地执行直接跟在脚本名后）：
#   --version <latest|nightly|vX.Y.Z>  安装版本。latest=最新稳定版（默认）、
#                                      nightly=每日构建、vX.Y.Z=钉定版本
#   --bin-dir <dir>                    安装目录，默认 /usr/local/bin；
#                                      不可写且无免密 sudo 时自动回退 ~/.local/bin
#   --with-service                     注册开机自启（Linux: systemd root；
#                                      macOS: launchd，root 装系统级、否则用户级）
#   --server <addr>                    Server 上游地址（如 gm.example.com:19090），
#                                      写入已落盘配置的 server.addr；缺省时若在
#                                      交互终端会提示输入（回车跳过）
#   --silent                           静默安装：禁用一切交互提示（管道/CI 必用；
#                                      非 TTY 下自动等同静默）
#   --uninstall                        卸载（停止服务 + 删除二进制与配置不动数据）
#   -h, --help                         帮助
#
# 环境变量（piped 场景不便传参时）：
#   CROUPIER_INSTALL_VERSION / CROUPIER_INSTALL_BIN_DIR
#   CROUPIER_INSTALL_WITH_SERVICE=1
#   CROUPIER_INSTALL_SERVER / CROUPIER_INSTALL_SILENT=1
#
# 幂等：重复执行即覆盖升级。失败即停，报错带明确原因。
# 产物来自 GitHub Releases（匿名可下）：croupier-bin-<os>-<arch>.tar.gz，
# 只安装其中的 croupier-agent（server 另行部署）。

set -euo pipefail

REPO="cuihairu/croupier"
RAW_BASE="https://raw.githubusercontent.com/${REPO}"
RELEASE_BASE="https://github.com/${REPO}/releases"
BIN_NAME="croupier-agent"
SERVICE_NAME="croupier-agent"            # systemd 单元名
LAUNCHD_LABEL="com.github.cuihairu.croupier.agent"  # 与 scripts/install-launchd.sh 命名同族

VERSION="${CROUPIER_INSTALL_VERSION:-latest}"
BIN_DIR="${CROUPIER_INSTALL_BIN_DIR:-}"
WITH_SERVICE="${CROUPIER_INSTALL_WITH_SERVICE:-0}"
SERVER_ADDR="${CROUPIER_INSTALL_SERVER:-}"
SILENT="${CROUPIER_INSTALL_SILENT:-0}"
UNINSTALL=0

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[0;33m'; NC='\033[0m'
info() { printf "${GREEN}[INFO]${NC} %s\n" "$*"; }
warn() { printf "${YELLOW}[WARN]${NC} %s\n" "$*"; }
die()  { printf "${RED}[ERROR]${NC} %s\n" "$*" >&2; exit 1; }

usage() {
  sed -n '3,26p' "${BASH_SOURCE[0]}"
}

# ---- 参数解析 ------------------------------------------------------------

while [ $# -gt 0 ]; do
  case "$1" in
    --version)      [ $# -ge 2 ] || die "--version 需要取值"; VERSION="$2"; shift 2 ;;
    --version=*)    VERSION="${1#--version=}"; shift ;;
    --bin-dir)      [ $# -ge 2 ] || die "--bin-dir 需要取值"; BIN_DIR="$2"; shift 2 ;;
    --bin-dir=*)    BIN_DIR="${1#--bin-dir=}"; shift ;;
    --with-service) WITH_SERVICE=1; shift ;;
    --server)       [ $# -ge 2 ] || die "--server 需要取值"; SERVER_ADDR="$2"; shift 2 ;;
    --server=*)     SERVER_ADDR="${1#--server=}"; shift ;;
    --silent)       SILENT=1; shift ;;
    --uninstall)    UNINSTALL=1; shift ;;
    -h|--help)      usage; exit 0 ;;
    *)              die "未知参数: $1（--help 查看用法）" ;;
  esac
done

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

apply_server_addr() { # $1=agent.yaml 路径；只改 server 段下的 addr 行
  [ -n "$SERVER_ADDR" ] || return 0
  [ -f "$1" ] || return 0
  local tmp="$1.tmp.$$"
  if awk -v new="$SERVER_ADDR" '
      /^server:[ \t]*(#.*)?$/ { in_server = 1; print; next }
      /^[^ \t]/               { in_server = 0 }
      in_server && /^[ \t]+addr:/ { sub(/addr:.*/, "addr: \"" new "\""); done = 1 }
      { print }
      END { exit done ? 0 : 1 }
    ' "$1" > "$tmp"; then
    cat "$tmp" > "$1" && rm -f "$tmp"
    SERVER_APPLIED=1
    info "已写入 server.addr = ${SERVER_ADDR}（$1）"
  else
    rm -f "$tmp"
    warn "未在 $1 的 server 段找到 addr 行，请手动设置 server.addr=${SERVER_ADDR}"
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
  warn "输入的 server 地址含非法字符，已忽略（可稍后手动编辑 agent.yaml 的 server.addr）"
  SERVER_ADDR=""
fi

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
      warn "无权限删除: $1（请重跑: curl -fsSL ${RAW_BASE}/main/scripts/install.sh | sudo bash -s -- --uninstall）"
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
  info "卸载完成（/etc/croupier/ 配置保留，如需清理请手动删除）"
  exit 0
fi

# ---- 解析版本与下载地址 ----------------------------------------------------

if [ "$VERSION" = "latest" ]; then
  TAG="$(resolve_latest_tag)"
  [ -n "$TAG" ] || die "无法解析最新稳定版 tag（网络问题或仓库无正式 release），可显式指定 --version nightly 或 --version vX.Y.Z"
  info "最新稳定版: $TAG"
else
  TAG="$VERSION"
fi

URL="${RELEASE_BASE}/download/${TAG}/croupier-bin-${OS}-${ARCH}.tar.gz"
info "目标产物: $URL"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

download "$URL" "$TMP/archive.tar.gz"

# 归档由 nightly/CI 打包（tar -C bin .），含 server+agent 等；解到临时目录后取 agent
tar -xzf "$TMP/archive.tar.gz" -C "$TMP"
AGENT_SRC="$(find "$TMP" -maxdepth 2 -type f -name "$BIN_NAME" | head -1)"
[ -n "$AGENT_SRC" ] || die "归档中未找到 ${BIN_NAME}，产物内容异常"
chmod +x "$AGENT_SRC"

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
  OLD_VER="$("$TARGET" --version 2>/dev/null | head -1 || true)"
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

NEW_VER="$("$TARGET" --version 2>/dev/null | head -1)" ||
  die "安装后校验失败: $TARGET --version 未正常输出"
[ -n "$NEW_VER" ] || die "安装后校验失败: $TARGET --version 输出为空"

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

# ---- 可选：注册开机自启服务 --------------------------------------------------

if [ "$WITH_SERVICE" -eq 1 ]; then
  if [ "$OS" = linux ]; then
    have systemctl || die "--with-service 需要 systemd（当前系统无 systemctl），请手动配置服务"
    [ "$(id -u)" -eq 0 ] || die "--with-service 需要 root 权限：
    curl -fsSL ${RAW_BASE}/main/scripts/install.sh | sudo bash -s -- --with-service"

    mkdir -p /etc/croupier
    if [ ! -f /etc/croupier/agent.yaml ]; then
      info "拉取配置模板 configs/agent.yaml -> /etc/croupier/agent.yaml（tag $TAG）"
      download "${RAW_BASE}/${TAG}/configs/agent.yaml" /etc/croupier/agent.yaml
    else
      info "保留既有配置 /etc/croupier/agent.yaml"
    fi
    apply_server_addr /etc/croupier/agent.yaml

    cat > "$TMP/${SERVICE_NAME}.service" <<'EOF'
[Unit]
Description=Croupier Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=__BIN__ --config /etc/croupier/agent.yaml
Restart=on-failure
RestartSec=5
NoNewPrivileges=true
# 敏感值建议在 YAML 写 $(ENV_VAR) 占位（加载时 os.ExpandEnv 展开），秘钥不落盘；
# 配置项详见 docs/operations/config-agent.md

[Install]
WantedBy=multi-user.target
EOF
    sed -i.bak "s#__BIN__#${TARGET}#" "$TMP/${SERVICE_NAME}.service" && rm -f "$TMP/${SERVICE_NAME}.service.bak"
    install -m 0644 "$TMP/${SERVICE_NAME}.service" "/etc/systemd/system/${SERVICE_NAME}.service"
    systemctl daemon-reload
    systemctl enable --now "${SERVICE_NAME}"
    info "服务已注册并启动: systemctl status ${SERVICE_NAME}"
  else
    # macOS launchd：root 装系统级 LaunchDaemon，普通用户装用户级 LaunchAgent
    if [ "$(id -u)" -eq 0 ]; then
      PLIST_DIR="/Library/LaunchDaemons"
      DOMAIN="system"
      CFG_DIR="/etc/croupier"
      mkdir -p "$CFG_DIR"
      if [ ! -f "$CFG_DIR/agent.yaml" ]; then
        download "${RAW_BASE}/${TAG}/configs/agent.yaml" "$CFG_DIR/agent.yaml"
      fi
      CFG_PATH="$CFG_DIR/agent.yaml"
    else
      PLIST_DIR="$HOME/Library/LaunchAgents"
      DOMAIN="gui/$(id -u)"
      CFG_PATH="${CROUPIER_AGENT_CONFIG:-$HOME/etc/croupier/agent.yaml}"
      mkdir -p "$(dirname "$CFG_PATH")"
      if [ ! -f "$CFG_PATH" ]; then
        download "${RAW_BASE}/${TAG}/configs/agent.yaml" "$CFG_PATH"
      fi
    fi
    apply_server_addr "$CFG_PATH"
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
  fi
fi

if [ -n "$SERVER_ADDR" ] && [ "$SERVER_APPLIED" -eq 0 ]; then
  # 无 service 分支不落配置文件：地址没处写，明说别让用户以为已生效
  info "完成。本次未落配置文件：Server 地址 ${SERVER_ADDR} 需写入 agent.yaml 的
  server.addr（配置模板与说明见 docs/operations/config-agent.md），然后: $TARGET --config <agent.yaml>"
else
  info "完成。下一步：编辑配置连上 Server（上游地址见 docs/operations/config-agent.md），
  然后: $TARGET --config <agent.yaml>"
fi
