#!/usr/bin/env bash
# C++ SDK 独立构建脚本
# 此脚本使 C++ SDK 能够在没有父项目的情况下独立构建

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SDK_ROOT="$SCRIPT_DIR"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }

# 显示帮助信息
show_help() {
    cat << EOF
C++ SDK 独立构建脚本

用法:
    $0 [选项] [构建类型]

构建类型:
    configure           配置构建环境（默认）
    build              编译项目
    test               运行测试
    install            安装到系统
    clean              清理构建文件
    package            创建发布包

选项:
    -h, --help          显示此帮助信息
    --with-grpc         启用 gRPC 支持（需要预生成文件或在线下载）
    --mock-grpc         使用 gRPC mock 实现（默认）
    --build-dir DIR     指定构建目录（默认：build）
    --install-dir DIR   指定安装目录（默认：install）
    --vcpkg            启用 vcpkg 包管理
    --system-libs      使用系统库（Linux/macOS 推荐）
    --release          Release 构建（默认）
    --debug            Debug 构建

示例:
    $0                          # 默认配置
    $0 build --with-grpc        # 构建并启用 gRPC
    $0 configure --system-libs  # 使用系统库配置
    $0 package                  # 创建发布包

独立构建说明:
    此 SDK 支持三种构建模式:

    1. 📦 完全独立模式（推荐）
       - 使用预提交的生成文件
       - 无需父项目或网络访问
       - 最快的构建速度

    2. 🌐 在线模式
       - 自动下载 protobuf 定义
       - 构建时生成 gRPC 代码
       - 需要网络连接

    3. 🔄 Mock 模式（开发/测试）
       - 使用 gRPC mock 实现
       - 无需 protobuf 文件
       - 快速原型开发

    SDK 会自动检测可用模式并选择最优策略。
EOF
}

# 检测构建模式
detect_build_mode() {
    log_info "检测构建模式..."

    # 检查预生成文件
    if [ -d "$SDK_ROOT/generated" ] && [ -n "$(find "$SDK_ROOT/generated" -name "*.cc" | head -1)" ]; then
        echo "prebuilt"
        return 0
    fi

    # 检查网络连接和工具
    if command -v protoc &> /dev/null && command -v curl &> /dev/null; then
        if curl -s --connect-timeout 5 "https://raw.githubusercontent.com/cuihairu/croupier/main/proto/croupier/function/v1/function.proto" > /dev/null 2>&1; then
            echo "online"
            return 0
        fi
    fi

    # 默认 mock 模式
    echo "mock"
}

# 下载 protobuf 文件
download_proto_files() {
    local proto_dir="$SDK_ROOT/downloaded_proto"

    log_info "下载 protobuf 文件..."

    mkdir -p "$proto_dir"

    # Croupier proto 文件列表
    local proto_files=(
        "croupier/agent/local/v1/local.proto"
        "croupier/control/v1/control.proto"
        "croupier/function/v1/function.proto"
        "croupier/edge/job/v1/job.proto"
        "croupier/tunnel/v1/tunnel.proto"
        "croupier/options/ui.proto"
        "croupier/options/function.proto"
    )

    local base_url="https://raw.githubusercontent.com/cuihairu/croupier/main/proto"
    local success_count=0

    for proto_file in "${proto_files[@]}"; do
        local url="$base_url/$proto_file"
        local local_path="$proto_dir/$proto_file"

        mkdir -p "$(dirname "$local_path")"

        log_info "下载: $proto_file"
        if curl -fsSL "$url" -o "$local_path"; then
            ((success_count++))
        else
            log_warn "下载失败: $proto_file"
        fi
    done

    if [ "$success_count" -eq 0 ]; then
        log_error "所有 protobuf 文件下载失败"
        return 1
    fi

    log_success "成功下载 $success_count/${#proto_files[@]} 个文件"
    return 0
}

# 配置构建
configure_build() {
    local build_mode="$1"
    local build_type="${2:-Release}"
    local build_dir="${3:-build}"
    local use_vcpkg="${4:-false}"
    local enable_grpc="${5:-auto}"

    log_info "配置 $build_type 构建 (模式: $build_mode)..."

    # 确定是否启用 gRPC
    local grpc_enabled="OFF"
    case "$enable_grpc" in
        "auto")
            if [ "$build_mode" != "mock" ]; then
                grpc_enabled="ON"
            fi
            ;;
        "true"|"on"|"ON")
            grpc_enabled="ON"
            ;;
        *)
            grpc_enabled="OFF"
            ;;
    esac

    # 构建 CMake 参数
    local cmake_args=(
        "-B" "$build_dir"
        "-DCMAKE_BUILD_TYPE=$build_type"
        "-DBUILD_EXAMPLES=ON"
        "-DBUILD_TESTS=ON"
        "-DENABLE_GRPC=$grpc_enabled"
        "-DCROUPIER_STANDALONE_BUILD=ON"
    )

    # 添加 vcpkg 支持
    if [ "$use_vcpkg" = "true" ] && [ -n "${VCPKG_ROOT:-}" ]; then
        cmake_args+=("-DCMAKE_TOOLCHAIN_FILE=$VCPKG_ROOT/scripts/buildsystems/vcpkg.cmake")
        cmake_args+=("-DENABLE_VCPKG=ON")
        log_info "启用 vcpkg 包管理"
    elif command -v pkg-config &> /dev/null; then
        cmake_args+=("-DUSE_SYSTEM_PACKAGES=ON")
        log_info "使用系统包管理"
    fi

    # 设置构建模式特定选项
    case "$build_mode" in
        "prebuilt")
            log_info "使用预构建的 protobuf 文件"
            cmake_args+=("-DCROUPIER_PREBUILT_PROTO=ON")
            ;;
        "online")
            log_info "在线模式：将下载并生成 protobuf 代码"
            cmake_args+=("-DCROUPIER_ONLINE_BUILD=ON")
            # 预下载文件
            if download_proto_files; then
                cmake_args+=("-DCROUPIER_PROTO_DIR=$SDK_ROOT/downloaded_proto")
            else
                log_warn "下载失败，回退到 mock 模式"
                cmake_args+=("-DENABLE_GRPC=OFF")
            fi
            ;;
        "mock")
            log_info "Mock 模式：使用 gRPC mock 实现"
            cmake_args+=("-DENABLE_GRPC=OFF")
            ;;
    esac

    # 执行配置
    log_info "运行 CMake 配置..."
    log_info "命令: cmake ${cmake_args[*]}"

    if cmake "${cmake_args[@]}"; then
        log_success "配置完成！"

        # 显示配置摘要
        echo
        echo "=== 构建配置摘要 ==="
        echo "构建类型: $build_type"
        echo "构建目录: $build_dir"
        echo "构建模式: $build_mode"
        echo "gRPC 支持: $grpc_enabled"
        echo "包管理: $([ "$use_vcpkg" = "true" ] && echo "vcpkg" || echo "system")"
        echo

        return 0
    else
        log_error "配置失败！"
        return 1
    fi
}

# 构建项目
build_project() {
    local build_dir="${1:-build}"
    local build_type="${2:-Release}"

    if [ ! -d "$build_dir" ]; then
        log_error "构建目录不存在，请先运行配置"
        return 1
    fi

    log_info "构建项目..."

    # 检测并行作业数
    local jobs=1
    if command -v nproc &> /dev/null; then
        jobs=$(nproc)
    elif [ "$(uname)" = "Darwin" ]; then
        jobs=$(sysctl -n hw.ncpu)
    fi

    log_info "使用 $jobs 个并行作业"

    if cmake --build "$build_dir" --config "$build_type" --parallel "$jobs"; then
        log_success "构建完成！"

        # 显示构建结果
        echo
        echo "=== 构建产物 ==="
        find "$build_dir" -name "libcroupier-sdk*" -o -name "croupier-*" -type f | head -10
        echo

        return 0
    else
        log_error "构建失败！"
        return 1
    fi
}

# 运行测试
run_tests() {
    local build_dir="${1:-build}"

    if [ ! -d "$build_dir" ]; then
        log_error "构建目录不存在，请先运行构建"
        return 1
    fi

    log_info "运行测试..."

    if cd "$build_dir" && ctest --output-on-failure; then
        log_success "所有测试通过！"
        cd "$SDK_ROOT"
        return 0
    else
        log_error "测试失败！"
        cd "$SDK_ROOT"
        return 1
    fi
}

# 清理构建文件
clean_build() {
    local build_dir="${1:-build}"

    log_info "清理构建文件..."

    if [ -d "$build_dir" ]; then
        rm -rf "$build_dir"
        log_success "已清理 $build_dir"
    fi

    if [ -d "$SDK_ROOT/downloaded_proto" ]; then
        rm -rf "$SDK_ROOT/downloaded_proto"
        log_success "已清理下载的 proto 文件"
    fi

    log_success "清理完成！"
}

# 主函数
main() {
    local action="configure"
    local build_type="Release"
    local build_dir="build"
    local install_dir="install"
    local use_vcpkg="false"
    local enable_grpc="auto"

    # 解析参数
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_help
                exit 0
                ;;
            configure|build|test|install|clean|package)
                action="$1"
                shift
                ;;
            --with-grpc)
                enable_grpc="true"
                shift
                ;;
            --mock-grpc)
                enable_grpc="false"
                shift
                ;;
            --build-dir)
                build_dir="$2"
                shift 2
                ;;
            --install-dir)
                install_dir="$2"
                shift 2
                ;;
            --vcpkg)
                use_vcpkg="true"
                shift
                ;;
            --system-libs)
                use_vcpkg="false"
                shift
                ;;
            --release)
                build_type="Release"
                shift
                ;;
            --debug)
                build_type="Debug"
                shift
                ;;
            *)
                log_error "未知选项: $1"
                show_help
                exit 1
                ;;
        esac
    done

    log_info "Croupier C++ SDK 独立构建工具"
    echo

    # 检测构建模式
    local build_mode
    build_mode=$(detect_build_mode)

    # 执行操作
    case "$action" in
        "configure")
            configure_build "$build_mode" "$build_type" "$build_dir" "$use_vcpkg" "$enable_grpc"
            ;;
        "build")
            if [ ! -d "$build_dir" ]; then
                configure_build "$build_mode" "$build_type" "$build_dir" "$use_vcpkg" "$enable_grpc"
            fi
            build_project "$build_dir" "$build_type"
            ;;
        "test")
            run_tests "$build_dir"
            ;;
        "clean")
            clean_build "$build_dir"
            ;;
        "install")
            log_info "安装功能开发中..."
            ;;
        "package")
            log_info "打包功能开发中..."
            ;;
        *)
            log_error "未知操作: $action"
            exit 1
            ;;
    esac
}

# 运行主函数
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi