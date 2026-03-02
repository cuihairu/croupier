#!/usr/bin/env bash
#
# Croupier Server systemd 服务安装脚本 (使用内置 CLI)
# 用法: sudo ./scripts/install-systemd.sh [install|uninstall|reinstall|status|enable|disable|start|stop|restart]
#
# 环境变量 (可覆盖):
#   CROUPIER_USER       - 运行用户 (默认: croupier)
#   CROUPIER_CONFIG_DIR  - 配置目录 (默认: /etc/croupier)
#   CROUPIER_BIN_DIR     - 二进制目录 (默认: /opt/croupier/bin)
#   CROUPIER_BIN_SRC     - 二进制源文件路径 (可选)
#

set -euo pipefail

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

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
CROUPIER_CONFIG_DIR="${CROUPIER_CONFIG_DIR:-/etc/croupier}"
CROUPIER_BIN_DIR="${CROUPIER_BIN_DIR:-/opt/croupier/bin}"
CROUPIER_BIN_SRC="${CROUPIER_BIN_SRC:-}"
SERVICE_NAME="croupier-server"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# 检测是否为 root
check_root() {
    if [[ $EUID -ne 0 ]]; then
        log_error "此脚本需要 root 权限运行"
        log_info "请使用: sudo $0 $*"
        exit 1
    fi
}

# 获取二进制路径
get_binary_path() {
    if [[ -n "$CROUPIER_BIN_SRC" && -f "$CROUPIER_BIN_SRC" ]]; then
        echo "$CROUPIER_BIN_SRC"
        return
    fi

    LOCAL_BIN="${SCRIPT_DIR}/../bin/${SERVICE_NAME}"
    if [[ -f "$LOCAL_BIN" ]]; then
        echo "$LOCAL_BIN"
        return
    fi

    # 检查系统 PATH
    if command -v "$SERVICE_NAME" &>/dev/null; then
        command -v "$SERVICE_NAME"
        return
    fi

    return 1
}

# 创建用户 (如果需要)
ensure_user() {
    if ! id "$CROUPIER_USER" &>/dev/null; then
        log_info "创建用户: ${CROUPIER_USER}"
        useradd --system \
            --home "/var/lib/croupier" \
            --create-home \
            --shell /bin/false \
            --comment "Croupier Server" \
            "$CROUPIER_USER"
    fi
}

# 创建目录
create_directories() {
    log_info "创建目录结构"

    mkdir -p "$CROUPIER_BIN_DIR"
    mkdir -p "$CROUPIER_CONFIG_DIR"
    mkdir -p "/var/lib/croupier"
    mkdir -p "/var/log/croupier"

    chmod 755 "$CROUPIER_BIN_DIR"
    chmod 750 "$CROUPIER_CONFIG_DIR"
}

# 安装二进制
install_binary() {
    log_info "安装二进制文件"

    local bin_src
    bin_src="$(get_binary_path)" || true

    if [[ -n "$bin_src" ]]; then
        cp "$bin_src" "${CROUPIER_BIN_DIR}/${SERVICE_NAME}"
        chmod +x "${CROUPIER_BIN_DIR}/${SERVICE_NAME}"
        log_info "二进制文件已安装到 ${CROUPIER_BIN_DIR}/${SERVICE_NAME}"
    else
        log_warn "未找到二进制文件，请手动安装到 ${CROUPIER_BIN_DIR}/${SERVICE_NAME}"
    fi
}

# 创建配置文件
create_config() {
    local config_path="${CROUPIER_CONFIG_DIR}/server.yaml"

    if [[ ! -f "$config_path" ]]; then
        log_info "创建默认配置文件: $config_path"
        cat > "$config_path" << 'EOF'
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
    fi
}

# 使用 CLI 安装服务
install_service() {
    log_info "使用内置 CLI 安装服务"

    local bin_path
    bin_path="${CROUPIER_BIN_DIR}/${SERVICE_NAME}"

    if [[ ! -f "$bin_path" ]]; then
        log_error "二进制文件不存在: $bin_path"
        log_info "请先安装二进制文件或设置 CROUPIER_BIN_SRC"
        exit 1
    fi

    # 使用 CLI 安装命令
    "$bin_path" service install \
        --config "${CROUPIER_CONFIG_DIR}/server.yaml" \
        --config-dir "$CROUPIER_CONFIG_DIR"

    log_info "安装完成！"
    log_info "后续操作:"
    log_info "  启动服务:   sudo $0 start"
    log_info "  查看状态:   sudo $0 status"
    log_info "  启用开机自启: sudo $0 enable"
}

# 卸载服务
uninstall_service() {
    log_info "卸载服务"

    local bin_path
    bin_path="${CROUPIER_BIN_DIR}/${SERVICE_NAME}"

    if [[ -f "$bin_path" ]]; then
        "$bin_path" service uninstall --name "$SERVICE_NAME"
    else
        # 回退到 systemctl
        if systemctl is-active --quiet "$SERVICE_NAME"; then
            systemctl stop "$SERVICE_NAME"
        fi
        if systemctl is-enabled --quiet "$SERVICE_NAME"; then
            systemctl disable "$SERVICE_NAME"
        fi
        rm -f "/etc/systemd/system/${SERVICE_NAME}.service"
        systemctl daemon-reload
    fi

    log_info "卸载完成"
}

# 启动服务
start_service() {
    log_info "启动服务"

    local bin_path="${CROUPIER_BIN_DIR}/${SERVICE_NAME}"

    if [[ -f "$bin_path" ]]; then
        "$bin_path" service start --name "$SERVICE_NAME"
    else
        systemctl start "$SERVICE_NAME"
    fi
}

# 停止服务
stop_service() {
    log_info "停止服务"

    local bin_path="${CROUPIER_BIN_DIR}/${SERVICE_NAME}"

    if [[ -f "$bin_path" ]]; then
        "$bin_path" service stop --name "$SERVICE_NAME"
    else
        systemctl stop "$SERVICE_NAME"
    fi
}

# 重启服务
restart_service() {
    log_info "重启服务"

    local bin_path="${CROUPIER_BIN_DIR}/${SERVICE_NAME}"

    if [[ -f "$bin_path" ]]; then
        "$bin_path" service restart --name "$SERVICE_NAME"
    else
        systemctl restart "$SERVICE_NAME"
    fi
}

# 查看状态
show_status() {
    local bin_path="${CROUPIER_BIN_DIR}/${SERVICE_NAME}"

    if [[ -f "$bin_path" ]]; then
        "$bin_path" service status --name "$SERVICE_NAME"
    else
        systemctl status "$SERVICE_NAME"
    fi
}

# 启用服务
enable_service() {
    log_info "启用开机自启"
    systemctl enable "$SERVICE_NAME"
    log_info "服务已启用"
}

# 禁用服务
disable_service() {
    log_info "禁用开机自启"
    systemctl disable "$SERVICE_NAME"
    log_info "服务已禁用"
}

# 显示帮助
show_help() {
    cat << EOF
Croupier Server systemd 服务安装脚本 (使用内置 CLI)

用法: sudo $0 <命令> [选项]

命令:
  install     安装服务 (使用内置 CLI 命令)
  uninstall   卸载服务
  start       启动服务
  stop        停止服务
  restart     重启服务
  enable      启用开机自启
  disable     禁用开机自启
  status      查看服务状态
  help        显示此帮助信息

环境变量:
  CROUPIER_USER        运行用户 (默认: croupier)
  CROUPIER_CONFIG_DIR  配置目录 (默认: /etc/croupier)
  CROUPIER_BIN_DIR    二进制目录 (默认: /opt/croupier/bin)
  CROUPIER_BIN_SRC    二进制源文件路径

示例:
  # 安装服务
  sudo ./scripts/install-systemd.sh install

  # 指定二进制源路径安装
  sudo CROUPIER_BIN_SRC=/tmp/croupier-server ./scripts/install-systemd.sh install

  # 查看服务状态
  sudo ./scripts/install-systemd.sh status

  # 启动服务
  sudo ./scripts/install-systemd.sh start

  # 查看日志
  sudo journalctl -u croupier-server -f

  # 直接使用 CLI 命令
  /opt/croupier/bin/croupier-server service install
  /opt/croupier/bin/croupier-server service status
EOF
}

# 主入口
main() {
    local command="${1:-help}"

    case "$command" in
        install)
            check_root "$@"
            ensure_user
            create_directories
            install_binary
            create_config
            install_service
            ;;
        uninstall)
            check_root "$@"
            uninstall_service
            ;;
        start)
            check_root "$@"
            start_service
            ;;
        stop)
            check_root "$@"
            stop_service
            ;;
        restart)
            check_root "$@"
            restart_service
            ;;
        status)
            show_status
            ;;
        enable)
            check_root "$@"
            enable_service
            ;;
        disable)
            check_root "$@"
            disable_service
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
