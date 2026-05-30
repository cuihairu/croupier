#!/bin/bash
# Croupier SDK Skynet 示例启动脚本
# Copyright 2025 Croupier Authors

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 打印函数
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SDK_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

log_info "Croupier SDK Skynet 示例启动脚本"
echo "SDK 路径: $SDK_ROOT"
echo ""

# 检查 Skynet 路径
SKYNET_PATH="${SKYNET_PATH:-/tmp/skynet}"

if [ ! -d "$SKYNET_PATH" ]; then
    log_error "Skynet 未找到: $SKYNET_PATH"
    echo ""
    echo "请先安装 Skynet:"
    echo "  cd /tmp"
    echo "  git clone https://github.com/cloudwu/skynet.git"
    echo "  cd skynet && make"
    echo ""
    echo "或设置 SKYNET_PATH 环境变量指向你的 Skynet 目录"
    exit 1
fi

log_info "Skynet 路径: $SKYNET_PATH"

# 检查编译好的库文件
LIB_FILE="$SDK_ROOT/build/lib/libcroupier-sdk.so"

if [ ! -f "$LIB_FILE" ]; then
    log_error "找不到编译好的库文件: $LIB_FILE"
    echo ""
    echo "请先编译 Croupier SDK with Lua 绑定:"
    echo "  cd $SDK_ROOT"
    echo "  cmake -B build -DCMAKE_TOOLCHAIN_FILE=./cmake/vcpkg-bootstrap.cmake \\"
    echo "    -DENABLE_LUA_BINDING=ON -DBUILD_SHARED_LIBS=ON"
    echo "  cmake --build build"
    exit 1
fi

log_info "找到库文件: $LIB_FILE"

# 设置输出目录
OUTPUT_DIR="$SKYNET_PATH/croupier-sdk"
mkdir -p "$OUTPUT_DIR/cservice"
mkdir -p "$OUTPUT_DIR/luaclib"
mkdir -p "$OUTPUT_DIR/service"

# 复制文件
log_info "复制文件到 Skynet 目录..."

# 复制 .so 文件
cp "$LIB_FILE" "$OUTPUT_DIR/cservice/"
log_info "✓ 复制 libcroupier-sdk.so"

# 复制 Lua 模块
if [ -d "$SDK_ROOT/lua" ]; then
    cp -r "$SDK_ROOT/lua/"* "$OUTPUT_DIR/luaclib/"
    log_info "✓ 复制 Lua 模块"
fi

# 复制服务文件
if [ -f "$SDK_ROOT/skynet/service/croupier_service.lua" ]; then
    cp "$SDK_ROOT/skynet/service/croupier_service.lua" "$OUTPUT_DIR/service/"
    log_info "✓ 复制 croupier_service.lua"
fi

# 复制示例文件
cp "$SCRIPT_DIR/config.lua" "$OUTPUT_DIR/"
cp "$SCRIPT_DIR"/*.lua "$OUTPUT_DIR/"
log_info "✓ 复制示例文件"

# 创建配置文件
cat > "$OUTPUT_DIR/skynet.conf" << EOF
# Croupier SDK Skynet 配置
root = "."
thread = 8
logger = nil
logpath = "."
harbor = 0

# Lua 服务路径
luaservice = root.."service/?.lua;"..root.."?.lua"
lualoader = root.."lualoader.lua"
lua_path = root.."luaclib/?.lua;"..root.."luaclib/?/init.lua"
lua_cpath = root.."luaclib/?.so"

# C 服务路径
cpath = root.."cservice/?.so"

# Bootstrap
bootstrap = "snlua bootstrap"

# Croupier 服务器配置（可选，也可通过环境变量设置）
croupier_server = "localhost:50051"
auth_token = nil
EOF

log_info "✓ 创建配置文件"

# 创建启动脚本
cat > "$OUTPUT_DIR/run.sh" << 'EOF'
#!/bin/bash
# Croupier SDK Skynet 运行脚本

# 检查 Skynet 是否存在
if [ ! -f "./skynet" ]; then
    echo "错误: 找不到 skynet 可执行文件"
    echo "请确保在 Skynet 根目录运行此脚本"
    exit 1
fi

# 显示菜单
echo "=========================================="
echo "Croupier SDK Skynet 示例"
echo "=========================================="
echo ""
echo "请选择要运行的示例:"
echo "  1) 简单示例 (simple_example.lua)"
echo "  2) 完整示例 (main.lua)"
echo "  3) 自定义..."
echo ""
read -p "请输入选项 (1-3): " choice

case $choice in
    1)
        echo ""
        echo "运行简单示例..."
        ./skynet examples/simple_example.lua
        ;;
    2)
        echo ""
        echo "运行完整示例..."
        ./skynet examples/main.lua
        ;;
    3)
        echo ""
        read -p "请输入 Lua 文件名: " lua_file
        if [ -f "$lua_file" ]; then
            ./skynet "$lua_file"
        else
            echo "错误: 找不到文件 $lua_file"
            exit 1
        fi
        ;;
    *)
        echo "无效的选项"
        exit 1
        ;;
esac
EOF

chmod +x "$OUTPUT_DIR/run.sh"
log_info "✓ 创建运行脚本"

# 完成
echo ""
log_info "=========================================="
log_info "设置完成！"
log_info "=========================================="
echo ""
echo "输出目录: $OUTPUT_DIR"
echo ""
echo "下一步:"
echo "  cd $OUTPUT_DIR"
echo "  ./run.sh"
echo ""
echo "或直接运行:"
echo "  cd $OUTPUT_DIR"
echo "  ./skynet simple_example.lua"
echo ""
