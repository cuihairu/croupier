#!/usr/bin/env bash
#
# Croupier Server systemd 服务安装脚本
# 用法: sudo ./scripts/install-systemd.sh [install|uninstall|reinstall|status|enable|disable|start|stop|restart]
#
# 环境变量 (可覆盖):
#   CROUPIER_USER       - 运行用户 (默认: croupier)
#   CROUPIER_GROUP      - 运行组 (默认: croupier)
#   CROUPIER_HOME       - 安装目录 (默认: /opt/croupier)
#   CROUPIER_CONFIG_DIR - 配置目录 (默认: /etc/croupier)
#   CROUPIER_DATA_DIR   - 数据目录 (默认: /var/lib/croupier)
#   CROUPIER_LOG_DIR    - 日志目录 (默认: /var/log/croupier)
#   CROUPIER_BIN_DIR    - 二进制目录 (默认: /opt/croupier/bin)
#   CROUPIER_BIN_SRC    - 二进制源文件路径 (用于安装时复制)
#

set -euo pipefail

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
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

# 默认配置
CROUPIER_USER="${CROUPIER_USER:-croupier}"
CROUPIER_GROUP="${CROUPIER_GROUP:-croupier}"
CROUPIER_HOME="${CROUPIER_HOME:-/opt/croupier}"
CROUPIER_CONFIG_DIR="${CROUPIER_CONFIG_DIR:-/etc/croupier}"
CROUPIER_DATA_DIR="${CROUPIER_DATA_DIR:-/var/lib/croupier}"
CROUPIER_LOG_DIR="${CROUPIER_LOG_DIR:-/var/log/croupier}"
CROUPIER_BIN_DIR="${CROUPIER_BIN_DIR:-/opt/croupier/bin}"
CROUPIER_BIN_SRC="${CROUPIER_BIN_SRC:-}"
SERVICE_NAME="croupier-server"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
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
    if [[ -f /etc/os-release ]]; then
        . /etc/os-release
        OS_ID="$ID"
        OS_VERSION="$VERSION_ID"
    else
        log_error "无法检测操作系统类型"
        exit 1
    fi
}

# 创建用户和组
create_user() {
    log_info "创建用户和组: ${CROUPIER_USER}:${CROUPIER_GROUP}"

    if ! id "$CROUPIER_USER" &>/dev/null; then
        useradd --system \
            --home "$CROUPIER_HOME" \
            --create-home \
            --shell /bin/false \
            --comment "Croupier Server" \
            "$CROUPIER_USER"
        log_info "用户 ${CROUPIER_USER} 创建成功"
    else
        log_info "用户 ${CROUPIER_USER} 已存在"
    fi
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
    chown -R "${CROUPIER_USER}:${CROUPIER_GROUP}" "$CROUPIER_HOME"
    chown -R "${CROUPIER_USER}:${CROUPIER_GROUP}" "$CROUPIER_DATA_DIR"
    chown -R "${CROUPIER_USER}:${CROUPIER_GROUP}" "$CROUPIER_LOG_DIR"
    chmod 750 "$CROUPIER_CONFIG_DIR"
    chmod 755 "$CROUPIER_BIN_DIR"

    log_info "目录创建完成"
}

# 安装二进制文件
install_binary() {
    log_info "安装二进制文件"

    if [[ -n "$CROUPIER_BIN_SRC" ]]; then
        if [[ -f "$CROUPIER_BIN_SRC" ]]; then
            cp "$CROUPIER_BIN_SRC" "${CROUPIER_BIN_DIR}/${SERVICE_NAME}"
            chmod +x "${CROUPIER_BIN_DIR}/${SERVICE_NAME}"
            log_info "二进制文件已从 $CROUPIER_BIN_SRC 复制到 ${CROUPIER_BIN_DIR}/${SERVICE_NAME}"
        else
            log_warn "源二进制文件不存在: $CROUPIER_BIN_SRC"
            log_warn "请手动将 croupier-server 复制到 ${CROUPIER_BIN_DIR}/"
        fi
    else
        # 检查 bin 目录
        LOCAL_BIN="${SCRIPT_DIR}/../bin/${SERVICE_NAME}"
        if [[ -f "$LOCAL_BIN" ]]; then
            cp "$LOCAL_BIN" "${CROUPIER_BIN_DIR}/${SERVICE_NAME}"
            chmod +x "${CROUPIER_BIN_DIR}/${SERVICE_NAME}"
            log_info "二进制文件已从开发目录复制"
        else
            log_warn "未找到二进制文件，请手动安装到 ${CROUPIER_BIN_DIR}/${SERVICE_NAME}"
        fi
    fi
}

# 安装配置文件
install_config() {
    log_info "安装配置文件"

    if [[ ! -f "${CROUPIER_CONFIG_DIR}/server.yaml" ]]; then
        if [[ -f "${SCRIPT_DIR}/../configs/server.example.yaml" ]]; then
            cp "${SCRIPT_DIR}/../configs/server.example.yaml" "${CROUPIER_CONFIG_DIR}/server.yaml"
            log_info "配置文件已创建，请编辑: ${CROUPIER_CONFIG_DIR}/server.yaml"
        else
            cat > "${CROUPIER_CONFIG_DIR}/server.yaml" << 'EOF'
# Croupier Server 配置文件
# 请根据实际环境修改

server:
  addr: ":8443"
  http_addr: ":8080"

database:
  driver: postgres
  dsn: "postgres://croupier:password@localhost:5432/croupier?sslmode=disable"

jwt:
  secret: "change-me-to-random-string"

log:
  level: info
  format: json
EOF
            log_info "默认配置文件已创建，请编辑: ${CROUPIER_CONFIG_DIR}/server.yaml"
        fi
    else
        log_info "配置文件已存在: ${CROUPIER_CONFIG_DIR}/server.yaml"
    fi

    # 创建环境变量文件
    if [[ ! -f "${CROUPIER_CONFIG_DIR}/croupier.conf" ]]; then
        cat > "${CROUPIER_CONFIG_DIR}/croupier.conf" << 'EOF'
# Croupier Server 环境变量
# 此文件中的变量会被 systemd 服务读取

# 数据库连接
# DATABASE_URL=postgres://croupier:password@localhost:5432/croupier?sslmode=disable

# 监听地址
# CROUPIER_SERVER_ADDR=:8443
# CROUPIER_SERVER_HTTP_ADDR=:8080

# 日志级别
# CROUPIER_LOG_LEVEL=info
EOF
        log_info "环境变量文件已创建: ${CROUPIER_CONFIG_DIR}/croupier.conf"
    fi
}

# 安装 systemd 服务文件
install_service_file() {
    log_info "安装 systemd 服务文件"

    if [[ -f "${SCRIPT_DIR}/${SERVICE_NAME}.service" ]]; then
        sed "s|/opt/croupier|${CROUPIER_HOME}|g" "${SCRIPT_DIR}/${SERVICE_NAME}.service" > "$SERVICE_FILE"
    else
        log_warn "服务模板文件不存在，创建默认服务文件"
        cat > "$SERVICE_FILE" << EOF
[Unit]
Description=Croupier Server - Game Operations Control Plane
Documentation=https://github.com/cuihairu/croupier
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${CROUPIER_USER}
Group=${CROUPIER_GROUP}
WorkingDirectory=${CROUPIER_HOME}
ExecStart=${CROUPIER_BIN_DIR}/${SERVICE_NAME} --config ${CROUPIER_CONFIG_DIR}/server.yaml
Restart=always
RestartSec=5
EnvironmentFile=-${CROUPIER_CONFIG_DIR}/croupier.conf
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=${CROUPIER_LOG_DIR} ${CROUPIER_DATA_DIR} /tmp
StandardOutput=journal
StandardError=journal
SyslogIdentifier=${SERVICE_NAME}

[Install]
WantedBy=multi-user.target
EOF
    fi

    systemctl daemon-reload
    log_info "systemd 服务文件已安装"
}

# 安装服务
install() {
    check_root "$@"
    detect_system

    log_info "开始安装 Croupier Server systemd 服务"
    log_info "安装目录: ${CROUPIER_HOME}"
    log_info "配置目录: ${CROUPIER_CONFIG_DIR}"
    log_info "数据目录: ${CROUPIER_DATA_DIR}"
    log_info "日志目录: ${CROUPIER_LOG_DIR}"

    create_user
    create_directories
    install_binary
    install_config
    install_service_file

    log_info "安装完成！"
    log_info "请执行以下步骤完成配置："
    log_info "  1. 编辑配置文件: ${CROUPIER_CONFIG_DIR}/server.yaml"
    log_info "  2. 启用并启动服务: sudo systemctl enable --now ${SERVICE_NAME}"
    log_info "  3. 查看服务状态: sudo systemctl status ${SERVICE_NAME}"
    log_info "  4. 查看日志: sudo journalctl -u ${SERVICE_NAME} -f"
}

# 卸载服务
uninstall() {
    check_root "$@"

    log_info "卸载 Croupier Server systemd 服务"

    # 停止服务
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        log_info "停止服务..."
        systemctl stop "$SERVICE_NAME"
    fi

    # 禁用服务
    if systemctl is-enabled --quiet "$SERVICE_NAME"; then
        log_info "禁用服务..."
        systemctl disable "$SERVICE_NAME"
    fi

    # 删除服务文件
    if [[ -f "$SERVICE_FILE" ]]; then
        rm -f "$SERVICE_FILE"
        systemctl daemon-reload
        log_info "服务文件已删除"
    fi

    log_info "卸载完成"
    log_warn "用户 ${CROUPIER_USER}、目录 ${CROUPIER_HOME} 和 ${CROUPIER_CONFIG_DIR} 保留未删除"
    log_info "如需完全删除，请手动执行："
    log_info "  sudo userdel ${CROUPIER_USER}"
    log_info "  sudo rm -rf ${CROUPIER_HOME} ${CROUPIER_CONFIG_DIR} ${CROUPIER_DATA_DIR} ${CROUPIER_LOG_DIR}"
}

# 重装服务
reinstall() {
    uninstall "$@"
    install "$@"
}

# 查看状态
status() {
    systemctl status "$SERVICE_NAME" "$@"
}

# 启用服务
enable() {
    check_root "$@"
    log_info "启用 ${SERVICE_NAME} 服务"
    systemctl enable "$SERVICE_NAME"
    log_info "服务已启用，使用 'sudo systemctl start ${SERVICE_NAME}' 启动服务"
}

# 禁用服务
disable() {
    check_root "$@"
    log_info "禁用 ${SERVICE_NAME} 服务"
    systemctl disable "$SERVICE_NAME"
    log_info "服务已禁用"
}

# 启动服务
start() {
    check_root "$@"
    log_info "启动 ${SERVICE_NAME} 服务"
    systemctl start "$SERVICE_NAME"
    log_info "服务已启动"
}

# 停止服务
stop() {
    check_root "$@"
    log_info "停止 ${SERVICE_NAME} 服务"
    systemctl stop "$SERVICE_NAME"
    log_info "服务已停止"
}

# 重启服务
restart() {
    check_root "$@"
    log_info "重启 ${SERVICE_NAME} 服务"
    systemctl restart "$SERVICE_NAME"
    log_info "服务已重启"
}

# 显示帮助
show_help() {
    cat << EOF
Croupier Server systemd 服务安装脚本

用法: sudo $0 <命令> [选项]

命令:
  install     安装服务 (创建用户、目录、配置)
  uninstall   卸载服务 (保留用户和数据目录)
  reinstall   重装服务
  enable      启用服务 (开机自启)
  disable     禁用服务
  start       启动服务
  stop        停止服务
  restart     重启服务
  status      查看服务状态
  help        显示此帮助信息

环境变量:
  CROUPIER_USER       运行用户 (默认: croupier)
  CROUPIER_GROUP      运行组 (默认: croupier)
  CROUPIER_HOME       安装目录 (默认: /opt/croupier)
  CROUPIER_CONFIG_DIR 配置目录 (默认: /etc/croupier)
  CROUPIER_DATA_DIR   数据目录 (默认: /var/lib/croupier)
  CROUPIER_LOG_DIR    日志目录 (默认: /var/log/croupier)
  CROUPIER_BIN_DIR    二进制目录 (默认: /opt/croupier/bin)
  CROUPIER_BIN_SRC    二进制源文件路径 (可选)

示例:
  # 安装服务
  sudo ./scripts/install-systemd.sh install

  # 指定二进制源路径安装
  sudo CROUPIER_BIN_SRC=/tmp/croupier-server ./scripts/install-systemd.sh install

  # 查看服务状态
  ./scripts/install-systemd.sh status

  # 启用并启动服务
  sudo ./scripts/install-systemd.sh enable
  sudo ./scripts/install-systemd.sh start

  # 查看日志
  sudo journalctl -u croupier-server -f
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
        status)
            status "${@:2}"
            ;;
        enable)
            enable "${@:2}"
            ;;
        disable)
            disable "${@:2}"
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
