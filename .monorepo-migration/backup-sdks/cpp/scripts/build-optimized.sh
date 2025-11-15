#!/bin/bash
# CI 构建优化脚本 - 智能选择最快的依赖安装策略

set -e

# 配置
PROJECT_NAME="croupier-cpp-sdk"
BUILD_DIR="build-ci"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[✅]${NC} $1"; }
log_warning() { echo -e "${YELLOW}[⚠️]${NC} $1"; }
log_error() { echo -e "${RED}[❌]${NC} $1"; }
log_step() { echo -e "${PURPLE}[🚀]${NC} $1"; }

# 检测最优依赖策略
detect_dependency_strategy() {
    log_step "检测最优依赖安装策略..."

    # 检查系统包可用性
    local system_packages_available=false

    if command -v apt-get &> /dev/null; then
        log_info "检测到 Ubuntu/Debian 系统"
        if apt-cache show libgrpc++-dev &> /dev/null; then
            system_packages_available=true
            log_success "系统 gRPC 包可用"
        fi
    elif command -v dnf &> /dev/null; then
        log_info "检测到 Fedora/CentOS 系统"
        if dnf list grpc-devel &> /dev/null; then
            system_packages_available=true
            log_success "系统 gRPC 包可用"
        fi
    elif command -v brew &> /dev/null; then
        log_info "检测到 macOS 系统"
        if brew list grpc &> /dev/null || brew info grpc &> /dev/null; then
            system_packages_available=true
            log_success "Homebrew gRPC 包可用"
        fi
    fi

    # 决策逻辑
    if [ "$system_packages_available" = true ]; then
        echo "system"
    elif [ -n "$VCPKG_ROOT" ] || [ -d "/vcpkg" ]; then
        echo "vcpkg"
    else
        echo "minimal"
    fi
}

# 安装系统依赖
install_system_dependencies() {
    log_step "安装系统依赖包..."

    if command -v apt-get &> /dev/null; then
        # Ubuntu/Debian
        sudo apt-get update -qq
        sudo apt-get install -y \
            build-essential \
            cmake \
            libgrpc++-dev \
            libprotobuf-dev \
            protobuf-compiler-grpc \
            nlohmann-json3-dev \
            pkg-config

        log_success "Ubuntu/Debian 系统包安装完成"

    elif command -v dnf &> /dev/null; then
        # Fedora/CentOS/RHEL
        sudo dnf install -y \
            gcc-c++ \
            cmake \
            grpc-devel \
            protobuf-devel \
            json-devel \
            pkg-config

        log_success "Fedora/CentOS 系统包安装完成"

    elif command -v brew &> /dev/null; then
        # macOS
        brew install \
            grpc \
            protobuf \
            nlohmann-json \
            pkg-config

        log_success "macOS Homebrew 包安装完成"

    else
        log_error "不支持的操作系统，无法安装系统包"
        return 1
    fi
}

# 配置 vcpkg（优化版）
setup_vcpkg() {
    log_step "配置优化的 vcpkg 环境..."

    # 设置 vcpkg 环境
    if [ -z "$VCPKG_ROOT" ]; then
        if [ -d "/vcpkg" ]; then
            export VCPKG_ROOT="/vcpkg"
        else
            log_error "VCPKG_ROOT 未设置且 /vcpkg 目录不存在"
            return 1
        fi
    fi

    log_info "使用 vcpkg: $VCPKG_ROOT"

    # 创建优化的 triplet 文件
    local triplet_file=""
    local triplet_name=""

    if [[ "$OSTYPE" == "linux-gnu"* ]]; then
        triplet_name="x64-linux-release"
        triplet_file="$VCPKG_ROOT/triplets/x64-linux-release.cmake"
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        if [[ $(uname -m) == "arm64" ]]; then
            triplet_name="arm64-osx-release"
            triplet_file="$VCPKG_ROOT/triplets/arm64-osx-release.cmake"
        else
            triplet_name="x64-osx-release"
            triplet_file="$VCPKG_ROOT/triplets/x64-osx-release.cmake"
        fi
    fi

    # 创建 release-only triplet（如果不存在）
    if [ ! -f "$triplet_file" ] && [ -n "$triplet_name" ]; then
        log_info "创建优化的 triplet: $triplet_name"

        cat > "$triplet_file" << EOF
# 优化的 release-only triplet
# 避免 Debug/Release 重复编译，节省 50% 构建时间

set(VCPKG_TARGET_ARCHITECTURE x64)
set(VCPKG_CRT_LINKAGE dynamic)
set(VCPKG_LIBRARY_LINKAGE shared)
set(VCPKG_BUILD_TYPE release)

# 启用并行编译
set(VCPKG_BUILD_TYPE release)
set(VCPKG_CONCURRENCY 4)
EOF

        log_success "创建 triplet: $triplet_file"
    fi

    echo "$triplet_name"
}

# 构建 SDK
build_sdk() {
    local strategy="$1"
    local triplet="$2"

    log_step "构建 SDK (策略: $strategy)..."

    # 清理之前的构建
    rm -rf "$BUILD_DIR"
    mkdir -p "$BUILD_DIR"
    cd "$BUILD_DIR"

    # 设置 CMake 参数
    local cmake_args=(
        "-DCMAKE_BUILD_TYPE=Release"
        "-DBUILD_EXAMPLES=ON"
        "-DBUILD_TESTS=OFF"
        "-DCMAKE_VERBOSE_MAKEFILE=OFF"
    )

    case "$strategy" in
        "system")
            cmake_args+=(
                "-DUSE_SYSTEM_PACKAGES=ON"
                "-DENABLE_GRPC=ON"
            )
            log_info "使用系统包构建（最快）"
            ;;
        "vcpkg")
            cmake_args+=(
                "-DCMAKE_TOOLCHAIN_FILE=$VCPKG_ROOT/scripts/buildsystems/vcpkg.cmake"
                "-DVCPKG_TARGET_TRIPLET=$triplet"
                "-DVCPKG_RELEASE_ONLY=ON"
                "-DENABLE_GRPC=ON"
            )
            log_info "使用 vcpkg 构建（release-only）"
            ;;
        "minimal")
            cmake_args+=(
                "-DENABLE_GRPC=OFF"
                "-DUSE_SYSTEM_PACKAGES=ON"
            )
            log_warning "最小化构建（禁用 gRPC）"
            ;;
    esac

    # 配置项目
    log_info "配置 CMake..."
    if cmake "${cmake_args[@]}" ..; then
        log_success "CMake 配置成功"
    else
        log_error "CMake 配置失败"
        return 1
    fi

    # 编译
    log_info "开始并行编译..."
    local cpu_count=$(nproc 2>/dev/null || echo 4)
    if make -j$cpu_count; then
        log_success "编译成功！"
    else
        log_error "编译失败"
        return 1
    fi

    # 显示构建结果
    log_success "📦 构建产物："
    find . -name "*.so" -o -name "*.dylib" -o -name "*.dll" -o -name "croupier-*" -type f | head -10

    cd ..
}

# 主流程
main() {
    echo
    log_step "🎯 优化的 Croupier C++ SDK 构建流程"
    echo

    # 检查预生成文件
    if [ ! -d "generated/croupier" ]; then
        log_warning "未找到预生成的 proto 文件"
        log_info "请运行: scripts/sync-sdk-generated.sh"
        log_info "或者将禁用 gRPC 支持"
        echo
    fi

    # 检测最优策略
    strategy=$(detect_dependency_strategy)
    log_success "选择策略: $strategy"
    echo

    # 记录开始时间
    start_time=$(date +%s)

    case "$strategy" in
        "system")
            install_system_dependencies
            build_sdk "system" ""
            ;;
        "vcpkg")
            triplet=$(setup_vcpkg)
            build_sdk "vcpkg" "$triplet"
            ;;
        "minimal")
            build_sdk "minimal" ""
            ;;
    esac

    # 计算构建时间
    end_time=$(date +%s)
    duration=$((end_time - start_time))

    echo
    log_success "🎉 构建完成！"
    log_info "⏱️  总耗时: ${duration}s"
    log_info "💰 预计节省构建时间: ~50% (相比传统 vcpkg)"
    echo
}

# 错误处理
trap 'log_error "构建失败"; exit 1' ERR

# 执行主流程
main "$@"