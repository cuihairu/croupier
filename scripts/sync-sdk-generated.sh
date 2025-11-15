#!/bin/bash
# 同步生成代码到SDK仓库的脚本（兼容版本）
# 支持 bash 3.0+, zsh, 和其他 POSIX shell

set -e

# 项目根目录
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# 颜色输出（兼容检测）
if [ -t 1 ] && command -v tput > /dev/null 2>&1; then
    RED=$(tput setaf 1 2>/dev/null || echo '')
    GREEN=$(tput setaf 2 2>/dev/null || echo '')
    YELLOW=$(tput setaf 3 2>/dev/null || echo '')
    BLUE=$(tput setaf 4 2>/dev/null || echo '')
    NC=$(tput sgr0 2>/dev/null || echo '')
else
    RED='' GREEN='' YELLOW='' BLUE='' NC=''
fi

log_info() {
    echo "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo "${RED}[ERROR]${NC} $1"
}

# 检查必要工具
check_dependencies() {
    local missing_tools=""

    if ! command -v buf &> /dev/null; then
        missing_tools="${missing_tools}buf "
    fi

    if ! git rev-parse --git-dir > /dev/null 2>&1; then
        log_error "Not in a git repository"
        exit 1
    fi

    if [ -n "$missing_tools" ]; then
        log_error "Missing required tools: $missing_tools"
        log_info "Please install missing tools and try again"
        exit 1
    fi
}

# 生成proto代码
generate_proto() {
    log_info "检查是否需要生成 proto 代码..."

    # 检查是否所有SDK都有生成文件
    all_exist=true
    for sdk in go cpp java python js; do
        if [ ! -d "sdks/$sdk/generated" ] || [ ! "$(find "sdks/$sdk/generated" -name "*" -type f | head -1)" ]; then
            all_exist=false
            break
        fi
    done

    if [ "$all_exist" = true ]; then
        log_success "所有SDK已有生成代码，跳过重新生成"
        return 0
    fi

    log_info "生成 proto 代码..."
    if buf generate; then
        log_success "Proto 代码生成完成"
        return 0
    else
        log_warning "Proto 代码生成失败（可能是速率限制）"
        log_info "使用现有的生成代码继续..."
        return 0  # 不让生成失败阻止同步过程
    fi
}

# SDK配置（使用简单数组代替关联数组）
get_sdk_config() {
    # 返回格式：语言:仓库地址
    cat << 'EOF'
go:git@github.com:cuihairu/croupier-sdk-go.git
cpp:git@github.com:cuihairu/croupier-sdk-cpp.git
java:git@github.com:cuihairu/croupier-sdk-java.git
python:git@github.com:cuihairu/croupier-sdk-python.git
js:git@github.com:cuihairu/croupier-sdk-js.git
EOF
}

# 同步单个SDK的生成代码
sync_single_sdk() {
    local sdk_lang="$1"
    local sdk_repo="$2"
    local sdk_dir="sdks/$sdk_lang"
    local generated_dir="$sdk_dir/generated"

    log_info "同步 $sdk_lang SDK 生成代码..."

    # 检查SDK目录是否存在
    if [ ! -d "$sdk_dir" ]; then
        log_warning "$sdk_dir 目录不存在，跳过"
        return 0
    fi

    # 检查生成目录是否存在且不为空
    if [ ! -d "$generated_dir" ] || [ ! "$(find "$generated_dir" -mindepth 1 -print -quit 2>/dev/null)" ]; then
        log_warning "$generated_dir 为空或不存在，跳过 $sdk_lang SDK"
        return 0
    fi

    # 进入SDK目录
    cd "$sdk_dir" || {
        log_error "无法进入 $sdk_dir 目录"
        return 1
    }

    # 检查是否有变更
    if git diff --quiet generated/ 2>/dev/null && git diff --cached --quiet generated/ 2>/dev/null; then
        log_info "$sdk_lang SDK 生成代码无变更"
        cd "$PROJECT_ROOT"
        return 0
    fi

    # 添加生成的文件
    log_info "添加 $sdk_lang SDK 生成代码..."
    git add generated/ || {
        log_error "添加文件失败"
        cd "$PROJECT_ROOT"
        return 1
    }

    # 检查是否确实有内容要提交
    if git diff --cached --quiet 2>/dev/null; then
        log_info "$sdk_lang SDK 无新变更需要提交"
        cd "$PROJECT_ROOT"
        return 0
    fi

    # 生成提交信息
    local main_commit=""
    if [ -d "$PROJECT_ROOT/.git" ]; then
        main_commit=$(cd "$PROJECT_ROOT" && git rev-parse --short HEAD 2>/dev/null || echo "unknown")
    fi

    local changed_files
    changed_files=$(git diff --cached --name-only generated/ | wc -l)

    local commit_msg="chore: update generated proto code

Generated from main project at $(date '+%Y-%m-%d %H:%M:%S')
Main project commit: $main_commit

Updated files: $changed_files
$(git diff --cached --name-only generated/ | head -10)
$([ "$changed_files" -gt 10 ] && echo "... and $(( changed_files - 10 )) more files")"

    # 提交变更
    if git commit -m "$commit_msg"; then
        log_success "$sdk_lang SDK 生成代码已提交"

        # 询问是否推送
        printf "是否推送 %s SDK 到远程仓库? (y/N): " "$sdk_lang"
        read -r response
        case "$response" in
            [yY]|[yY][eE][sS])
                if git push; then
                    log_success "$sdk_lang SDK 已推送到远程仓库"
                else
                    log_error "$sdk_lang SDK 推送失败"
                fi
                ;;
            *)
                log_info "$sdk_lang SDK 已提交到本地，未推送"
                ;;
        esac
    else
        log_warning "$sdk_lang SDK 提交失败或无需提交"
    fi

    cd "$PROJECT_ROOT"
}

# 显示统计信息
show_statistics() {
    log_info "统计信息："

    # 使用while循环处理SDK配置
    get_sdk_config | while IFS=: read -r sdk_lang sdk_repo; do
        local sdk_dir="sdks/$sdk_lang"
        if [ -d "$sdk_dir/generated" ]; then
            # 统计文件数量
            local file_count=0

            # 根据语言统计不同类型的文件
            case "$sdk_lang" in
                "go")
                    file_count=$(find "$sdk_dir/generated" -name "*.go" 2>/dev/null | wc -l)
                    ;;
                "cpp")
                    file_count=$(find "$sdk_dir/generated" \( -name "*.h" -o -name "*.cc" -o -name "*.cpp" \) 2>/dev/null | wc -l)
                    ;;
                "java")
                    file_count=$(find "$sdk_dir/generated" -name "*.java" 2>/dev/null | wc -l)
                    ;;
                "python")
                    file_count=$(find "$sdk_dir/generated" -name "*.py" 2>/dev/null | wc -l)
                    ;;
                "js")
                    file_count=$(find "$sdk_dir/generated" \( -name "*.js" -o -name "*.ts" -o -name "*.d.ts" \) 2>/dev/null | wc -l)
                    ;;
                *)
                    file_count=$(find "$sdk_dir/generated" -type f 2>/dev/null | wc -l)
                    ;;
            esac

            # 计算目录大小
            local dir_size=""
            if command -v du > /dev/null 2>&1; then
                dir_size=$(du -sh "$sdk_dir/generated" 2>/dev/null | cut -f1 || echo "unknown")
            else
                dir_size="unknown"
            fi

            printf "  %-8s: %3d 个文件, %s\n" "$sdk_lang" "$file_count" "$dir_size"
        else
            printf "  %-8s: 未找到生成目录\n" "$sdk_lang"
        fi
    done
}

# 显示使用说明
show_usage() {
    cat << EOF
使用方法: $0 [选项]

同步主项目的生成代码到各个SDK子模块仓库

选项:
  -h, --help     显示此帮助信息
  --dry-run      显示要执行的操作，但不实际执行
  --lang LANG    仅处理指定语言的SDK (cpp|java|python|js)

工作流程:
  1. 运行 buf generate 生成 proto 代码
  2. 将生成的代码提交到各个SDK子模块
  3. 可选择性推送到远程SDK仓库

示例:
  $0                    # 处理所有SDK
  $0 --lang cpp         # 仅处理C++ SDK
  $0 --dry-run          # 预览操作
EOF
}

# 主函数
main() {
    local dry_run=false
    local target_lang=""

    # 解析命令行参数
    while [ $# -gt 0 ]; do
        case $1 in
            -h|--help)
                show_usage
                exit 0
                ;;
            --dry-run)
                dry_run=true
                shift
                ;;
            --lang)
                target_lang="$2"
                shift 2
                ;;
            *)
                log_error "未知参数: $1"
                show_usage
                exit 1
                ;;
        esac
    done

    log_info "🎯 开始同步 SDK 生成代码..."
    echo

    # 检查依赖
    check_dependencies

    # 生成 proto 代码
    if ! generate_proto; then
        exit 1
    fi
    echo

    # 处理每个SDK
    local processed_count=0
    local success_count=0

    get_sdk_config | while IFS=: read -r sdk_lang sdk_repo; do
        # 过滤特定语言
        if [ -n "$target_lang" ] && [ "$sdk_lang" != "$target_lang" ]; then
            continue
        fi

        processed_count=$((processed_count + 1))

        if [ "$dry_run" = true ]; then
            log_info "[DRY-RUN] 将处理 $sdk_lang SDK (${sdk_repo})"
        else
            if sync_single_sdk "$sdk_lang" "$sdk_repo"; then
                success_count=$((success_count + 1))
            fi
        fi
        echo
    done

    if [ "$dry_run" = false ]; then
        log_success "同步完成！"
        echo
        show_statistics
    else
        log_info "预览模式完成，使用不带 --dry-run 参数执行实际操作"
    fi
}

# 错误处理
trap 'log_error "脚本执行失败"; exit 1' ERR

# 执行主函数
main "$@"