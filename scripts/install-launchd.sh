#!/usr/bin/env bash
#
# Croupier Server macOS launchd 服务安装脚本
# 用法: sudo ./scripts/install-launchd.sh [install|uninstall|reinstall|load|unload|start|stop|restart|status]
#
# 环境变量 (可覆盖):
#   CROUPIER_USER       - 运行用户 (默认: croupier，若不存在则使用当前用户)
#   CROUPIER_HOME       - 安装目录 (默认: /usr/local/croupier)
#   CROUPIER_CONFIG_DIR - 配置目录 (默认: /usr/local/etc/croupier)
#   CROUPIER_DATA_DIR   - 数据目录 (默认: /usr/local/var/croupier)
#   CROUPIER_LOG_DIR    - 日志目录 (默认: /usr/local/var/log/croupier)
#   CROUPIER_BIN_DIR    - 二进制目录 (默认: /usr/local/croupier/bin)
#   CROUPIER_BIN_SRC    - 二进制源文件路径 (用于安装时复制)
#

set -euo pipefail

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_debug() {
    echo -e "${BLUE}[DEBUG]${NC} $1"
}

# 默认配置
CROUPIER_USER="${CROUPIER_USER:-croupier}"
CROUPIER_HOME="${CROUPIER_HOME:-/usr/local/croupier}"
CROUPIER_CONFIG_DIR="${CROUPIER_CONFIG_DIR:-/usr/local/etc/croupier}"
CROUPIER_DATA_DIR="${CROUPIER_DATA_DIR:-/usr/local/var/croupier}"
CROUPIER_LOG_DIR="${CROUPIER_LOG_DIR:-/usr/local/var/log/croupier}"
CROUPIER_BIN_DIR="${CROUPIER_BIN_DIR:-/usr/local/croupier/bin}"
CROUPIER_BIN_SRC="${CROUPIER_BIN_SRC:-}"
SERVICE_NAME="com.github.cuihairu.croupier.server"
PLIST_FILE="/Library/LaunchDaemons/${SERVICE_NAME}.plist"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# 检测是否为 root
check_root() {
    if [[ $EUID -ne 0 ]]; then
        log_error "此脚本需要 root 权限运行"
        log_info "请使用: sudo $0 $*"
        exit 1
    fi
}

# 检测系统
detect_system() {
    if [[ "$(uname)" != "Darwin" ]]; then
        log_error "此脚本仅适用于 macOS 系统"
        exit 1
    fi

    macOS_VERSION=$(sw_vers -productVersion)
    log_info "检测到 macOS 版本: $macOS_VERSION"
}

# 创建用户和组
create_user() {
    log_info "检查用户: ${CROUPIER_USER}"

    # 检查用户是否存在
    if ! id "$CROUPIER_USER" &>/dev/null; then
        log_warn "用户 ${CROUPIER_USER} 不存在"
        log_info "创建系统用户: ${CROUPIER_USER}"

        # macOS 创建系统用户
        sysadminctl -addUser "$CROUPIER_USER" \
            -home "$CROUPIER_HOME" \
            -shell /usr/bin/false \
            -comment "Croupier Server" \
            -password "*" 2>/dev/null || true

        # 检查是否创建成功
        if ! id "$CROUPIER_USER" &>/dev/null; then
            log_warn "自动创建用户失败，使用当前用户: $(whoami)"
            CROUPIER_USER="$(whoami)"
        fi
    else
        log_info "用户 ${CROUPIER_USER} 已存在"
    fi

    # 获取用户 UID 和 GID
    CROUPIER_UID=$(id -u "$CROUPIER_USER")
    CROUPIER_GID=$(id -g "$CROUPIER_USER")
    log_debug "用户 UID: ${CROUPIER_UID}, GID: ${CROUPIER_GID}"
}

# 创建目录结构
create_directories() {
    log_info "创建目录结构"

    mkdir -p "$CROUPIER_HOME"
    mkdir -p "$CROUPIER_BIN_DIR"
    mkdir -p "$CROUPIER_CONFIG_DIR"
    mkdir -p "$CROUPIER_DATA_DIR"
    mkdir -p "$CROUPIER_LOG_DIR"

    # 设置权限
    chown -R "${CROUPIER_USER}:${CROUPIER_GID}" "$CROUPIER_HOME" 2>/dev/null || true
    chown -R "${CROUPIER_USER}:${CROUPIER_GID}" "$CROUPIER_DATA_DIR" 2>/dev/null || true
    chown -R "${CROUPIER_USER}:${CROUPIER_GID}" "$CROUPIER_LOG_DIR" 2>/dev/null || true
    chmod 755 "$CROUPIER_BIN_DIR"

    log_info "目录创建完成"
}

# 安装二进制文件
install_binary() {
    log_info "安装二进制文件"

    if [[ -n "$CROUPIER_BIN_SRC" ]]; then
        if [[ -f "$CROUPIER_BIN_SRC" ]]; then
            cp "$CROUPIER_BIN_SRC" "${CROUPIER_BIN_DIR}/croupier-server"
            chmod +x "${CROUPIER_BIN_DIR}/croupier-server"
            log_info "二进制文件已从 $CROUPIER_BIN_SRC 复制到 ${CROUPIER_BIN_DIR}/croupier-server"
        else
            log_warn "源二进制文件不存在: $CROUPIER_BIN_SRC"
            log_warn "请手动将 croupier-server 复制到 ${CROUPIER_BIN_DIR}/"
        fi
    else
        # 检查 bin 目录
        LOCAL_BIN="${SCRIPT_DIR}/../bin/croupier-server"
        if [[ -f "$LOCAL_BIN" ]]; then
            cp "$LOCAL_BIN" "${CROUPIER_BIN_DIR}/croupier-server"
            chmod +x "${CROUPIER_BIN_DIR}/croupier-server"
            log_info "二进制文件已从开发目录复制"
        else
            log_warn "未找到二进制文件，请手动安装到 ${CROUPIER_BIN_DIR}/croupier-server"
        fi
    fi
}

# 安装配置文件
install_config() {
    log_info "安装配置文件"

    if [[ ! -f "${CROUPIER_CONFIG_DIR}/server.yaml" ]]; then
        if [[ -f "${SCRIPT_DIR}/../services/server/etc/server.yaml" ]]; then
            cp "${SCRIPT_DIR}/../services/server/etc/server.yaml" "${CROUPIER_CONFIG_DIR}/server.yaml"
            log_info "配置文件已创建，请编辑: ${CROUPIER_CONFIG_DIR}/server.yaml"
        else
            cat > "${CROUPIER_CONFIG_DIR}/server.yaml" << 'EOF'
# Croupier Server 配置文件
# 请根据实际环境修改

Name: croupier-server
Host: 0.0.0.0
Port: 18780
Mode: prod

Database:
  Driver: postgres
  DataSource: "postgres://croupier:password@localhost:5432/croupier?sslmode=disable"

Control:
  Addr: ":19090"

Auth:
  JWTSecret: "change-me-to-random-string"

Log:
  Level: info
  Format: json
  Output: stdout
EOF
            log_info "默认配置文件已创建，请编辑: ${CROUPIER_CONFIG_DIR}/server.yaml"
        fi
    else
        log_info "配置文件已存在: ${CROUPIER_CONFIG_DIR}/server.yaml"
    fi

    # 设置配置文件权限
    chown "${CROUPIER_USER}:${CROUPIER_GID}" "${CROUPIER_CONFIG_DIR}/server.yaml" 2>/dev/null || true
    chmod 640 "${CROUPIER_CONFIG_DIR}/server.yaml"
}

# 生成 LaunchDaemon plist 文件
generate_plist() {
    cat << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>${SERVICE_NAME}</string>

    <key>ProgramArguments</key>
    <array>
        <string>${CROUPIER_BIN_DIR}/croupier-server</string>
        <string>--config</string>
        <string>${CROUPIER_CONFIG_DIR}/server.yaml</string>
    </array>

    <key>RunAtLoad</key>
    <true/>

    <key>KeepAlive</key>
    <dict>
        <key>SuccessfulExit</key>
        <false/>
        <key>Crashed</key>
        <true/>
    </dict>

    <key>WorkingDirectory</key>
    <string>${CROUPIER_HOME}</string>

    <key>UserName</key>
    <string>${CROUPIER_USER}</string>

    <key>GroupName</key>
    <string>staff</string>

    <key>StandardOutPath</key>
    <string>${CROUPIER_LOG_DIR}/croupier-server.log</string>

    <key>StandardErrorPath</key>
    <string>${CROUPIER_LOG_DIR}/croupier-server.err</string>

    <key>EnvironmentVariables</key>
    <dict>
        <key>CROUPIER_CONFIG_DIR</key>
        <string>${CROUPIER_CONFIG_DIR}</string>
        <key>CROUPIER_DATA_DIR</key>
        <string>${CROUPIER_DATA_DIR}</string>
        <key>CROUPIER_LOG_DIR</key>
        <string>${CROUPIER_LOG_DIR}</string>
    </dict>

    <key>SoftResourceLimits</key>
    <dict>
        <key>NumberOfFiles</key>
        <integer>65536</integer>
    </dict>

    <key>HardResourceLimits</key>
    <dict>
        <key>NumberOfFiles</key>
        <integer>65536</integer>
    </dict>

    <key>ProcessType</key>
    <string>Interactive</string>

    <key>Nice</key>
    <integer>0</integer>
</dict>
</plist>
EOF
}

# 安装 LaunchDaemon
install_launchd() {
    log_info "安装 LaunchDaemon: ${PLIST_FILE}"

    generate_plist > "$PLIST_FILE"

    # 设置权限
    chmod 644 "$PLIST_FILE"

    log_info "LaunchDaemon 文件已创建"
}

# 安装服务
install() {
    check_root "$@"
    detect_system

    log_info "开始安装 Croupier Server macOS launchd 服务"
    log_info "安装目录: ${CROUPIER_HOME}"
    log_info "配置目录: ${CROUPIER_CONFIG_DIR}"
    log_info "数据目录: ${CROUPIER_DATA_DIR}"
    log_info "日志目录: ${CROUPIER_LOG_DIR}"
    log_info "运行用户: ${CROUPIER_USER}"

    create_user
    create_directories
    install_binary
    install_config
    install_launchd

    log_info "安装完成！"
    log_info "请执行以下步骤完成配置："
    log_info "  1. 编辑配置文件: ${CROUPIER_CONFIG_DIR}/server.yaml"
    log_info "  2. 加载服务: sudo $0 load"
    log_info "  3. 启动服务: sudo $0 start"
    log_info "  4. 查看日志: tail -f ${CROUPIER_LOG_DIR}/croupier-server.log"
}

# 卸载服务
uninstall() {
    check_root "$@"

    log_info "卸载 Croupier Server launchd 服务"

    # 停止服务
    if launchctl list | grep -q "$SERVICE_NAME"; then
        log_info "停止服务..."
        launchctl bootout system "$SERVICE_NAME" 2>/dev/null || true
    fi

    # 删除 plist 文件
    if [[ -f "$PLIST_FILE" ]]; then
        rm -f "$PLIST_FILE"
        log_info "LaunchDaemon 文件已删除"
    fi

    log_info "卸载完成"
    log_warn "用户 ${CROUPIER_USER}、目录 ${CROUPIER_HOME} 和 ${CROUPIER_CONFIG_DIR} 保留未删除"
    log_info "如需完全删除，请手动执行："
    log_info "  sudo dscl . delete /Users/${CROUPIER_USER}"
    log_info "  sudo rm -rf ${CROUPIER_HOME} ${CROUPIER_CONFIG_DIR} ${CROUPIER_DATA_DIR} ${CROUPIER_LOG_DIR}"
}

# 重装服务
reinstall() {
    uninstall "$@"
    install "$@"
}

# 加载服务
load_service() {
    check_root "$@"

    log_info "加载 LaunchDaemon: ${SERVICE_NAME}"

    if [[ -f "$PLIST_FILE" ]]; then
        launchctl load -w "$PLIST_FILE"
        log_info "LaunchDaemon 已加载"
    else
        log_error "LaunchDaemon 文件不存在: $PLIST_FILE"
        log_info "请先运行: sudo $0 install"
        exit 1
    fi
}

# 卸载服务
unload_service() {
    check_root "$@"

    log_info "卸载 LaunchDaemon: ${SERVICE_NAME}"

    if launchctl list | grep -q "$SERVICE_NAME"; then
        launchctl bootout system "$SERVICE_NAME" 2>/dev/null || \
        launchctl unload -w "$PLIST_FILE" 2>/dev/null || true
        log_info "LaunchDaemon 已卸载"
    else
        log_warn "服务未加载"
    fi
}

# 启动服务
start() {
    check_root "$@"

    log_info "启动服务: ${SERVICE_NAME}"

    # 检查服务是否已加载
    if ! launchctl list | grep -q "$SERVICE_NAME"; then
        log_warn "服务未加载，正在加载..."
        load_service "$@"
    fi

    # 使用 launchctl start 启动服务
    launchctl start "$SERVICE_NAME" 2>/dev/null || log_warn "服务可能已启动或启动失败"

    log_info "服务已启动"
}

# 停止服务
stop() {
    check_root "$@"

    log_info "停止服务: ${SERVICE_NAME}"

    if launchctl list | grep -q "$SERVICE_NAME"; then
        launchctl stop "$SERVICE_NAME" 2>/dev/null || log_warn "服务可能未运行"
        log_info "服务已停止"
    else
        log_warn "服务未加载"
    fi
}

# 重启服务
restart() {
    check_root "$@"

    log_info "重启服务: ${SERVICE_NAME}"
    stop "$@"
    sleep 2
    start "$@"
}

# 查看状态
status() {
    log_info "服务状态: ${SERVICE_NAME}"

    if launchctl list | grep -q "$SERVICE_NAME"; then
        echo "服务已加载"

        # 检查进程
        PID=$(pgrep -f "croupier-server" || true)
        if [[ -n "$PID" ]]; then
            echo "运行中: PID $PID"

            # 显示进程信息
            ps -p "$PID" -o pid,ppid,user,%cpu,%mem,etime,command
        else
            echo "进程未运行"
        fi

        # 显示日志文件位置
        echo ""
        echo "日志文件:"
        echo "  标准输出: ${CROUPIER_LOG_DIR}/croupier-server.log"
        echo "  错误输出: ${CROUPIER_LOG_DIR}/croupier-server.err"
    else
        echo "服务未加载"
        echo "LaunchDaemon 文件: $PLIST_FILE"
    fi
}

# 显示帮助
show_help() {
    cat << EOF
Croupier Server macOS launchd 服务安装脚本

用法: sudo $0 <命令> [选项]

命令:
  install     安装服务 (创建用户、目录、配置、LaunchDaemon)
  uninstall   卸载服务 (保留用户和数据目录)
  reinstall   重装服务
  load        加载 LaunchDaemon (注册到系统)
  unload      卸载 LaunchDaemon
  start       启动服务
  stop        停止服务
  restart     重启服务
  status      查看服务状态
  help        显示此帮助信息

环境变量:
  CROUPIER_USER       运行用户 (默认: croupier)
  CROUPIER_HOME       安装目录 (默认: /usr/local/croupier)
  CROUPIER_CONFIG_DIR 配置目录 (默认: /usr/local/etc/croupier)
  CROUPIER_DATA_DIR   数据目录 (默认: /usr/local/var/croupier)
  CROUPIER_LOG_DIR    日志目录 (默认: /usr/local/var/log/croupier)
  CROUPIER_BIN_DIR    二进制目录 (默认: /usr/local/croupier/bin)
  CROUPIER_BIN_SRC    二进制源文件路径 (可选)

示例:
  # 安装服务
  sudo ./scripts/install-launchd.sh install

  # 指定二进制源路径安装
  sudo CROUPIER_BIN_SRC=/tmp/croupier-server ./scripts/install-launchd.sh install

  # 加载并启动服务
  sudo ./scripts/install-launchd.sh load
  sudo ./scripts/install-launchd.sh start

  # 查看服务状态
  ./scripts/install-launchd.sh status

  # 查看日志
  tail -f /usr/local/var/log/croupier/croupier-server.log

  # 停止服务
  sudo ./scripts/install-launchd.sh stop

  # 卸载服务
  sudo ./scripts/install-launchd.sh unload
  sudo ./scripts/install-launchd.sh uninstall

macOS 特定说明:
  - LaunchDaemon 文件位置: /Library/LaunchDaemons/
  - 服务随系统自动启动 (RunAtLoad = true)
  - 服务崩溃后自动重启 (KeepAlive = true)
  - 日志输出到标准文件，方便排查问题
EOF
}

# 主入口
main() {
    local command="${1:-help}"

    case "$command" in
        install)
            install "${@:2}"
            ;;
        uninstall)
            uninstall "${@:2}"
            ;;
        reinstall)
            reinstall "${@:2}"
            ;;
        load)
            load_service "${@:2}"
            ;;
        unload)
            unload_service "${@:2}"
            ;;
        start)
            start "${@:2}"
            ;;
        stop)
            stop "${@:2}"
            ;;
        restart)
            restart "${@:2}"
            ;;
        status)
            status "${@:2}"
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            log_error "未知命令: $command"
            show_help
            exit 1
            ;;
    esac
}

main "$@"
