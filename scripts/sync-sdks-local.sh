#!/bin/bash
# 本地 SDK 同步脚本 - 从 croupier 同步 proto 到各个 SDK 仓库
# 用法: ./scripts/sync-sdks-local.sh [sdk1,sdk2,...]
# 示例: ./scripts/sync-sdks-local.sh go,python,cpp
#      ./scripts/sync-sdks-local.sh              # 同步所有 SDK

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# 默认 SDK 列表 (相对于 ../ 目录)
ALL_SDKS=(
    "croupier-sdk-go:go:Go"
    "croupier-sdk-python:python:Python"
    "croupier-sdk-cpp:cpp:C++"
    "croupier-sdk-js:js:JavaScript"
    "croupier-sdk-csharp:csharp:C#"
)

# 函数：打印使用说明
usage() {
    echo "用法: $0 [选项] [SDK列表]"
    echo ""
    echo "选项:"
    echo "  -h, --help              显示此帮助信息"
    echo "  -p, --path BASE_PATH    指定 SDK 仓库的基础路径 (默认: ../)"
    echo "  -d, --dry-run           预演模式，不执行实际操作"
    echo "  --skip-generate         跳过 buf generate 步骤"
    echo ""
    echo "SDK列表 (逗号分隔):"
    echo "  go, python, cpp, js, csharp, all"
    echo ""
    echo "示例:"
    echo "  $0                      # 同步所有 SDK"
    echo "  $0 go,python            # 只同步 Go 和 Python SDK"
    echo "  $0 -p ~/Workspaces go   # 指定 SDK 基础路径"
    echo "  $0 -d cpp               # 预演 C++ SDK 同步"
    exit 0
}

# 函数：打印消息
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

# 函数：检查命令是否存在
check_command() {
    if ! command -v "$1" &> /dev/null; then
        log_error "命令 $1 未找到，请先安装"
        return 1
    fi
}

# 解析参数
BASE_PATH="../"
DRY_RUN=false
SKIP_GENERATE=false
REQUESTED_SDKS=()

while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            usage
            ;;
        -p|--path)
            BASE_PATH="$2"
            shift 2
            ;;
        -d|--dry-run)
            DRY_RUN=true
            shift
            ;;
        --skip-generate)
            SKIP_GENERATE=true
            shift
            ;;
        go|python|js|cpp|csharp|all)
            IFS=',' read -ra SDK_ARRAY <<< "$1"
            for sdk in "${SDK_ARRAY[@]}"; do
                REQUESTED_SDKS+=("$sdk")
            done
            shift
            ;;
        *)
            if [[ $1 == *,* ]]; then
                IFS=',' read -ra SDK_ARRAY <<< "$1"
                for sdk in "${SDK_ARRAY[@]}"; do
                    REQUESTED_SDKS+=("$sdk")
                done
            else
                REQUESTED_SDKS+=("$1")
            fi
            shift
            ;;
    esac
done

# 如果没有指定 SDK，默认同步所有
if [ ${#REQUESTED_SDKS[@]} -eq 0 ]; then
    REQUESTED_SDKS=("all")
fi

# 检查必要的工具
check_command buf

log_info "=== Croupier SDK 本地同步脚本 ==="
log_info "基础路径: $BASE_PATH"
log_info "预演模式: $DRY_RUN"
log_info "跳过生成: $SKIP_GENERATE"
echo ""

# 函数：同步 Go SDK
sync_go_sdk() {
    local sdk_path="$1"
    log_info "开始同步 Go SDK..."

    cd "$sdk_path"

    if [ "$DRY_RUN" = true ]; then
        log_warn "[预演] 将删除 proto/ 并重新生成"
        return
    fi

    # 删除旧的 proto 目录
    rm -rf proto
    mkdir -p proto

    # 复制新的 proto 结构
    cp -r "$REPO_ROOT/proto/croupier" proto/

    # 删除旧的生成代码
    rm -rf pkg/pb proto/pkg

    # 创建 buf.gen.yaml
    cat > proto/buf.gen.yaml << 'EOF'
version: v2
plugins:
  - remote: buf.build/protocolbuffers/go:v1.36.11
    out: pkg/pb
    opt:
      - paths=source_relative
EOF

    # 创建 buf.yaml
    cat > proto/buf.yaml << 'EOF'
version: v2
modules:
  - path: .
deps:
  - buf.build/protocolbuffers/wellknowntypes:v25.1
EOF

    if [ "$SKIP_GENERATE" = false ]; then
        # 生成代码
        cd proto && buf generate && cd ..
        # 将生成的代码移动到正确的位置
        if [ -d "proto/pkg/pb" ]; then
            mv proto/pkg/pb/* pkg/pb/
            rm -rf proto/pkg
        fi
        # 修复生成代码中的 import 路径
        find pkg/pb -name "*.pb.go" -exec sed -i 's|github.com/cuihairu/croupier/sdks/go/pkg/pb/croupier/|github.com/cuihairu/croupier/sdks/go/pkg/pb/|g' {} \;
        # 修复 SDK 代码中的 import 路径
        find pkg -name "*.go" -not -path "*/pb/*" -exec sed -i 's|github.com/cuihairu/croupier/sdks/go/pkg/pb/croupier/|github.com/cuihairu/croupier/sdks/go/pkg/pb/|g' {} \;
        log_success "生成的 Go 文件:"
        find pkg/pb -name "*.pb.go" 2>/dev/null | wc -l
    else
        log_warn "跳过 buf generate"
    fi
}

# 函数：同步 Python SDK
sync_python_sdk() {
    local sdk_path="$1"
    log_info "开始同步 Python SDK..."

    cd "$sdk_path"

    if [ "$DRY_RUN" = true ]; then
        log_warn "[预演] 将删除 proto/ 并重新生成"
        return
    fi

    rm -rf proto
    mkdir -p proto

    cp -r "$REPO_ROOT/proto/croupier" proto/

    rm -rf generated

    cat > proto/buf.gen.yaml << 'EOF'
version: v2
plugins:
  - remote: buf.build/protocolbuffers/python:v25.1
    out: generated
EOF

    cat > proto/buf.yaml << 'EOF'
version: v2
modules:
  - path: .
deps:
  - buf.build/protocolbuffers/wellknowntypes:v25.1
EOF

    if [ "$SKIP_GENERATE" = false ]; then
        cd proto && buf generate && cd ..
        # 将生成的代码移动到正确的位置
        if [ -d "proto/generated" ]; then
            # 合并 proto/generated 到 generated
            if [ -d "generated" ]; then
                rm -rf generated
            fi
            mv proto/generated generated
        fi
        # 创建 __init__.py
        [ -d "generated" ] && find generated -type d -exec touch {}/__init__.py \;
        log_success "生成的 Python 文件:"
        find generated -name "*.py" 2>/dev/null | wc -l || echo "0"
    else
        log_warn "跳过 buf generate"
    fi
}

# 函数：同步 C++ SDK
sync_cpp_sdk() {
    local sdk_path="$1"
    log_info "开始同步 C++ SDK..."

    cd "$sdk_path"

    if [ "$DRY_RUN" = true ]; then
        log_warn "[预演] 将删除 proto/ 并重新生成"
        return
    fi

    rm -rf proto
    mkdir -p proto

    cp -r "$REPO_ROOT/proto/croupier" proto/

    rm -rf generated

    cat > proto/buf.gen.yaml << 'EOF'
version: v2
plugins:
  - remote: buf.build/protocolbuffers/cpp:v25.1
    out: generated
EOF

    cat > proto/buf.yaml << 'EOF'
version: v2
modules:
  - path: .
deps:
  - buf.build/protocolbuffers/wellknowntypes:v25.1
EOF

    if [ "$SKIP_GENERATE" = false ]; then
        cd proto && buf generate && cd ..
        # 将生成的代码移动到正确的位置
        if [ -d "proto/generated" ]; then
            if [ -d "generated" ]; then
                rm -rf generated
            fi
            mv proto/generated generated
        fi
        log_success "生成的 C++ 文件:"
        find generated -name "*.pb.*" 2>/dev/null | wc -l || echo "0"
    else
        log_warn "跳过 buf generate"
    fi
}

# 函数：同步 JavaScript SDK
sync_js_sdk() {
    local sdk_path="$1"
    log_info "开始同步 JavaScript SDK..."

    cd "$sdk_path"

    if [ "$DRY_RUN" = true ]; then
        log_warn "[预演] 将删除 proto/ 并重新生成"
        return
    fi

    # 确保 PATH 包含 node_modules/.bin
    export PATH="$PWD/node_modules/.bin:$PATH"

    # 完全删除旧的 proto 目录（而不是 rm -rf proto && mkdir，确保删除多余的旧文件）
    rm -rf proto
    mkdir -p proto

    # 只复制主仓库的 proto 文件
    cp -r "$REPO_ROOT/proto/croupier" proto/

    # 删除旧的生成代码
    rm -rf src/gen

    cat > proto/buf.gen.yaml << 'EOF'
version: v2
plugins:
  - local: protoc-gen-es
    out: src/gen
    opt:
      - target=ts
EOF

    cat > proto/buf.yaml << 'EOF'
version: v2
modules:
  - path: .
deps:
  - buf.build/protocolbuffers/wellknowntypes:v25.1
EOF

    if [ "$SKIP_GENERATE" = false ]; then
        # 确保 PATH 包含 node_modules/.bin
        export PATH="$PWD/node_modules/.bin:$PATH"
        cd proto && buf generate && cd .. || {
            log_warn "buf generate 失败 (可能需要先安装 protoc-gen-es: npm install)"
        }
        log_success "生成的 TypeScript 文件:"
        find src/gen -name "*.ts" 2>/dev/null | wc -l || echo "0"
    else
        log_warn "跳过 buf generate"
    fi
}

# 函数：同步 C# SDK
sync_csharp_sdk() {
    local sdk_path="$1"
    log_info "开始同步 C# SDK..."

    cd "$sdk_path"

    if [ "$DRY_RUN" = true ]; then
        log_warn "[预演] 将删除 proto/ 并重新生成"
        return
    fi

    rm -rf proto
    mkdir -p proto

    cp -r "$REPO_ROOT/proto/croupier" proto/

    rm -rf generated

    cat > proto/buf.gen.yaml << 'EOF'
version: v2
plugins:
  - remote: buf.build/protocolbuffers/csharp:v25.1
    out: generated
EOF

    cat > proto/buf.yaml << 'EOF'
version: v2
modules:
  - path: .
deps:
  - buf.build/protocolbuffers/wellknowntypes:v25.1
EOF

    if [ "$SKIP_GENERATE" = false ]; then
        cd proto && buf generate && cd ..
        # 将生成的代码移动到正确的位置
        if [ -d "proto/generated" ]; then
            if [ -d "generated" ]; then
                rm -rf generated
            fi
            mv proto/generated generated
        fi
        log_success "生成的 C# 文件:"
        find generated -name "*.cs" 2>/dev/null | wc -l || echo "0"
    else
        log_warn "跳过 buf generate"
    fi
}

# 函数：获取 SDK 信息
get_sdk_info() {
    local sdk_id="$1"
    for sdk_info in "${ALL_SDKS[@]}"; do
        IFS=':' read -ra PARTS <<< "$sdk_info"
        if [ "${PARTS[1]}" = "$sdk_id" ]; then
            echo "${PARTS[0]}|${PARTS[2]}"
            return 0
        fi
    done
    return 1
}

# 处理每个请求的 SDK
for requested in "${REQUESTED_SDKS[@]}"; do
    if [ "$requested" = "all" ]; then
        for sdk_info in "${ALL_SDKS[@]}"; do
            IFS=':' read -ra PARTS <<< "$sdk_info"
            repo_name="${PARTS[0]}"
            sdk_id="${PARTS[1]}"
            sdk_display="${PARTS[2]}"

            # 确保 BASE_PATH 以斜杠结尾
            if [[ ! "$BASE_PATH" =~ /$ ]] && [[ ! "$BASE_PATH" =~ \\$ ]]; then
                BASE_PATH="${BASE_PATH}/"
            fi
            sdk_full_path="${BASE_PATH}${repo_name}"

            if [ ! -d "$sdk_full_path" ]; then
                log_warn "跳过 $sdk_display: 路径不存在 ($sdk_full_path)"
                continue
            fi

            echo ""
            log_info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            log_info "处理 $sdk_display SDK"
            log_info "路径: $sdk_full_path"
            echo ""

            case "$sdk_id" in
                go) sync_go_sdk "$sdk_full_path" ;;
                python) sync_python_sdk "$sdk_full_path" ;;
                js) sync_js_sdk "$sdk_full_path" ;;
                cpp) sync_cpp_sdk "$sdk_full_path" ;;
                csharp) sync_csharp_sdk "$sdk_full_path" ;;
            esac

            if [ "$DRY_RUN" = false ]; then
                log_success "✓ $sdk_display SDK 同步完成"
            fi
        done
    else
        sdk_info=$(get_sdk_info "$requested")
        if [ -n "$sdk_info" ]; then
            IFS='|' read -ra INFO <<< "$sdk_info"
            repo_name="${INFO[0]}"
            sdk_display="${INFO[1]}"

            if [[ ! "$BASE_PATH" =~ /$ ]] && [[ ! "$BASE_PATH" =~ \\$ ]]; then
                BASE_PATH="${BASE_PATH}/"
            fi
            sdk_full_path="${BASE_PATH}${repo_name}"

            if [ ! -d "$sdk_full_path" ]; then
                log_error "跳过 $sdk_display: 路径不存在 ($sdk_full_path)"
                continue
            fi

            echo ""
            log_info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            log_info "处理 $sdk_display SDK"
            log_info "路径: $sdk_full_path"
            echo ""

            case "$requested" in
                go) sync_go_sdk "$sdk_full_path" ;;
                python) sync_python_sdk "$sdk_full_path" ;;
                js) sync_js_sdk "$sdk_full_path" ;;
                cpp) sync_cpp_sdk "$sdk_full_path" ;;
                csharp) sync_csharp_sdk "$sdk_full_path" ;;
            esac

            if [ "$DRY_RUN" = false ]; then
                log_success "✓ $sdk_display SDK 同步完成"
            fi
        else
            log_warn "未知的 SDK: $requested"
        fi
    fi
done

echo ""
log_success "=== 所有同步操作完成 ==="

if [ "$DRY_RUN" = true ]; then
    log_warn "这是预演模式，没有实际修改文件"
    log_info "使用 $0 <sdk> 来执行实际同步"
fi
