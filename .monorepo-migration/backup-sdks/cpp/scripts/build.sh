#!/bin/bash

# Croupier C++ SDK Build Script
# Supports cross-platform building with vcpkg

set -e

# ========== Configuration ==========
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SDK_ROOT="$(dirname "$SCRIPT_DIR")"
PROJECT_ROOT="$(dirname "$(dirname "$SDK_ROOT")")"

# Default values
BUILD_TYPE="Release"
ENABLE_GRPC="ON"
ENABLE_VCPKG="ON"
BUILD_EXAMPLES="ON"
BUILD_TESTS="OFF"
VCPKG_ROOT=""
INSTALL_PREFIX=""
PLATFORM=""
ARCH=""
CLEAN_BUILD="false"

# ========== Help Function ==========
show_help() {
    cat << EOF
Croupier C++ SDK Build Script

USAGE:
    $0 [OPTIONS]

OPTIONS:
    -h, --help              Show this help message
    -c, --clean             Clean build (remove build directory)
    -t, --type TYPE         Build type: Debug, Release, RelWithDebInfo (default: Release)
    -p, --platform PLATFORM Target platform: windows, linux, macos (auto-detect)
    -a, --arch ARCH         Target architecture: x64, x86, arm64 (auto-detect)
    --vcpkg-root PATH       Path to vcpkg installation
    --install-prefix PATH   Installation prefix
    --examples BOOL         Build examples (default: ON)
    --tests BOOL            Build tests (default: OFF)
    --grpc BOOL             Enable gRPC integration (default: ON)
    --vcpkg BOOL            Enable vcpkg integration (default: ON)

EXAMPLES:
    # Basic build
    $0

    # Clean build with tests
    $0 --clean --tests ON

    # Cross-compile for ARM64 on macOS
    $0 --arch arm64 --platform macos

    # Debug build with custom vcpkg
    $0 --type Debug --vcpkg-root /path/to/vcpkg

ENVIRONMENT VARIABLES:
    VCPKG_ROOT             Path to vcpkg installation
    CMAKE_BUILD_PARALLEL_LEVEL Number of parallel build jobs
EOF
}

# ========== Argument Parsing ==========
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_help
            exit 0
            ;;
        -c|--clean)
            CLEAN_BUILD="true"
            shift
            ;;
        -t|--type)
            BUILD_TYPE="$2"
            shift 2
            ;;
        -p|--platform)
            PLATFORM="$2"
            shift 2
            ;;
        -a|--arch)
            ARCH="$2"
            shift 2
            ;;
        --vcpkg-root)
            VCPKG_ROOT="$2"
            shift 2
            ;;
        --install-prefix)
            INSTALL_PREFIX="$2"
            shift 2
            ;;
        --examples)
            BUILD_EXAMPLES="$2"
            shift 2
            ;;
        --tests)
            BUILD_TESTS="$2"
            shift 2
            ;;
        --grpc)
            ENABLE_GRPC="$2"
            shift 2
            ;;
        --vcpkg)
            ENABLE_VCPKG="$2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# ========== Platform Detection ==========
detect_platform() {
    if [[ -z "$PLATFORM" ]]; then
        case "$(uname -s)" in
            Linux*)     PLATFORM="linux" ;;
            Darwin*)    PLATFORM="macos" ;;
            CYGWIN*|MINGW*|MSYS*) PLATFORM="windows" ;;
            *)          PLATFORM="unknown" ;;
        esac
    fi

    if [[ -z "$ARCH" ]]; then
        case "$(uname -m)" in
            x86_64|amd64)   ARCH="x64" ;;
            i?86)           ARCH="x86" ;;
            arm64|aarch64)  ARCH="arm64" ;;
            *)              ARCH="unknown" ;;
        esac
    fi

    echo "Detected platform: $PLATFORM-$ARCH"
}

# ========== vcpkg Setup ==========
setup_vcpkg() {
    if [[ "$ENABLE_VCPKG" == "ON" ]]; then
        # Find vcpkg root
        if [[ -z "$VCPKG_ROOT" ]] && [[ -n "$VCPKG_ROOT_ENV" ]]; then
            VCPKG_ROOT="$VCPKG_ROOT_ENV"
        fi

        if [[ -z "$VCPKG_ROOT" ]]; then
            # Try common locations
            local common_paths=(
                "/vcpkg"
                "/usr/local/vcpkg"
                "$HOME/vcpkg"
                "C:/vcpkg"
                "C:/dev/vcpkg"
            )

            for path in "${common_paths[@]}"; do
                if [[ -d "$path" ]] && [[ -f "$path/vcpkg.exe" || -f "$path/vcpkg" ]]; then
                    VCPKG_ROOT="$path"
                    break
                fi
            done
        fi

        if [[ -n "$VCPKG_ROOT" ]]; then
            echo "Using vcpkg from: $VCPKG_ROOT"
            VCPKG_TOOLCHAIN="$VCPKG_ROOT/scripts/buildsystems/vcpkg.cmake"
        else
            echo "WARNING: vcpkg not found. Set VCPKG_ROOT environment variable or use --vcpkg-root"
            echo "Falling back to system packages..."
            ENABLE_VCPKG="OFF"
        fi
    fi
}

# ========== vcpkg Triplet Detection ==========
get_vcpkg_triplet() {
    case "$PLATFORM-$ARCH" in
        windows-x64)    echo "x64-windows" ;;
        windows-x86)    echo "x86-windows" ;;
        linux-x64)      echo "x64-linux" ;;
        linux-arm64)    echo "arm64-linux" ;;
        macos-x64)      echo "x64-osx" ;;
        macos-arm64)    echo "arm64-osx" ;;
        *)              echo "x64-linux" ;;  # Default fallback
    esac
}

# ========== Build Directory Setup ==========
setup_build_dir() {
    local build_dir="$SDK_ROOT/build"

    if [[ "$CLEAN_BUILD" == "true" ]] && [[ -d "$build_dir" ]]; then
        echo "Cleaning build directory..."
        rm -rf "$build_dir"
    fi

    mkdir -p "$build_dir"
    echo "Build directory: $build_dir"
}

# ========== CMake Configuration ==========
configure_cmake() {
    local build_dir="$SDK_ROOT/build"
    local cmake_args=()

    # Basic configuration
    cmake_args+=("-S" "$SDK_ROOT")
    cmake_args+=("-B" "$build_dir")
    cmake_args+=("-DCMAKE_BUILD_TYPE=$BUILD_TYPE")

    # Install prefix
    if [[ -n "$INSTALL_PREFIX" ]]; then
        cmake_args+=("-DCMAKE_INSTALL_PREFIX=$INSTALL_PREFIX")
    else
        cmake_args+=("-DCMAKE_INSTALL_PREFIX=$build_dir/install")
    fi

    # vcpkg configuration
    if [[ "$ENABLE_VCPKG" == "ON" ]] && [[ -n "$VCPKG_TOOLCHAIN" ]]; then
        cmake_args+=("-DCMAKE_TOOLCHAIN_FILE=$VCPKG_TOOLCHAIN")
        cmake_args+=("-DVCPKG_TARGET_TRIPLET=$(get_vcpkg_triplet)")
    fi

    # Build options
    cmake_args+=("-DENABLE_VCPKG=$ENABLE_VCPKG")
    cmake_args+=("-DENABLE_GRPC=$ENABLE_GRPC")
    cmake_args+=("-DBUILD_EXAMPLES=$BUILD_EXAMPLES")
    cmake_args+=("-DBUILD_TESTS=$BUILD_TESTS")
    cmake_args+=("-DBUILD_SHARED_LIBS=ON")
    cmake_args+=("-DBUILD_STATIC_LIBS=ON")

    # Cross-compilation setup
    if [[ "$PLATFORM" == "linux" && "$ARCH" == "arm64" && "$(uname -m)" != "aarch64" ]]; then
        cmake_args+=("-DCMAKE_C_COMPILER=aarch64-linux-gnu-gcc")
        cmake_args+=("-DCMAKE_CXX_COMPILER=aarch64-linux-gnu-g++")
    fi

    echo "Configuring CMake with:"
    printf "  %s\n" "${cmake_args[@]}"

    cmake "${cmake_args[@]}"
}

# ========== Build ==========
build_project() {
    local build_dir="$SDK_ROOT/build"
    local parallel_jobs="${CMAKE_BUILD_PARALLEL_LEVEL:-$(nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 4)}"

    echo "Building with $parallel_jobs parallel jobs..."

    cmake --build "$build_dir" --config "$BUILD_TYPE" --parallel "$parallel_jobs"

    if [[ "$?" -eq 0 ]]; then
        echo "✅ Build completed successfully!"
    else
        echo "❌ Build failed!"
        exit 1
    fi
}

# ========== Run Tests ==========
run_tests() {
    if [[ "$BUILD_TESTS" == "ON" ]]; then
        local build_dir="$SDK_ROOT/build"

        echo "Running tests..."
        cd "$build_dir"
        ctest --build-config "$BUILD_TYPE" --output-on-failure

        if [[ "$?" -eq 0 ]]; then
            echo "✅ All tests passed!"
        else
            echo "❌ Some tests failed!"
            exit 1
        fi
    fi
}

# ========== Create Packages ==========
create_packages() {
    local build_dir="$SDK_ROOT/build"

    echo "Creating packages..."

    # Build packages
    cmake --build "$build_dir" --config "$BUILD_TYPE" --target package

    # Install to staging directory
    cmake --install "$build_dir" --prefix "$build_dir/package"

    # Create additional archive
    local package_name="croupier-cpp-sdk-$(date +%Y%m%d)-$PLATFORM-$ARCH"

    cd "$build_dir/package"
    if [[ "$PLATFORM" == "windows" ]]; then
        powershell -Command "Compress-Archive -Path * -DestinationPath ../${package_name}.zip"
    else
        tar -czf "../${package_name}.tar.gz" .
    fi

    echo "✅ Packages created in $build_dir/"
}

# ========== Main Function ==========
main() {
    echo "🚀 Croupier C++ SDK Build Script"
    echo "================================="

    detect_platform
    setup_vcpkg
    setup_build_dir

    echo ""
    echo "Configuration:"
    echo "  Platform: $PLATFORM-$ARCH"
    echo "  Build Type: $BUILD_TYPE"
    echo "  vcpkg: $ENABLE_VCPKG"
    echo "  gRPC: $ENABLE_GRPC"
    echo "  Examples: $BUILD_EXAMPLES"
    echo "  Tests: $BUILD_TESTS"
    echo ""

    configure_cmake
    build_project
    run_tests
    create_packages

    echo ""
    echo "🎉 Build completed successfully!"
    echo "📦 Packages available in: $SDK_ROOT/build/"

    # Show next steps
    echo ""
    echo "📖 Next steps:"
    echo "  • Run examples: $SDK_ROOT/build/bin/virtual-object-demo"
    echo "  • Install SDK: cmake --install $SDK_ROOT/build"
    echo "  • Package distribution: $SDK_ROOT/build/*.tar.gz"
}

# ========== Error Handling ==========
trap 'echo "❌ Build failed at line $LINENO"' ERR

# ========== Run Main ==========
main "$@"