#!/usr/bin/env bash
#
# 迁移验证脚本
# 验证代码风格迁移是否成功
#

set -euo pipefail

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PASS_COUNT=0
FAIL_COUNT=0

echo -e "${BLUE}╔═══════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║              代码风格迁移验证                              ║${NC}"
echo -e "${BLUE}╚═══════════════════════════════════════════════════════════╝${NC}"
echo ""

# 验证函数
check_pass() {
  echo -e "${GREEN}✓${NC} $1"
  ((PASS_COUNT++))
}

check_fail() {
  echo -e "${RED}✗${NC} $1"
  ((FAIL_COUNT++))
}

check_warn() {
  echo -e "${YELLOW}⚠${NC} $1"
}

# 1. 检查配置文件
echo -e "${BLUE}[1/6] 检查配置文件${NC}"
if [[ -f "${SCRIPT_DIR}/.goctl.yaml" ]]; then
  if grep -q "style: go_zero" "${SCRIPT_DIR}/.goctl.yaml"; then
    check_pass "配置文件存在且设置为 go_zero 风格"
  else
    check_fail "配置文件存在但风格设置不正确"
  fi
else
  check_fail "配置文件 .goctl.yaml 不存在"
fi
echo ""

# 2. 检查文件命名风格
echo -e "${BLUE}[2/6] 检查文件命名风格${NC}"

old_style_count=0
new_style_count=0

for service_dir in "${SCRIPT_DIR}"/*; do
  if [[ ! -d "$service_dir" ]] || [[ ! -d "${service_dir}/internal" ]]; then
    continue
  fi

  service_name=$(basename "$service_dir")

  # 检查 handler 文件
  if [[ -d "${service_dir}/internal/handler" ]]; then
    # 查找旧风格文件（连续小写字母超过15个的文件名）
    while IFS= read -r file; do
      filename=$(basename "$file" .go)
      # 检查是否包含下划线
      if [[ ! "$filename" =~ _ ]]; then
        # 检查是否是短文件名（可能是合理的）
        if [[ ${#filename} -gt 15 ]]; then
          check_warn "${service_name}: 发现旧风格文件 ${filename}.go"
          ((old_style_count++))
        fi
      else
        ((new_style_count++))
      fi
    done < <(find "${service_dir}/internal/handler" -name "*.go" -not -name "*_test.go" 2>/dev/null || true)
  fi
done

if [[ $old_style_count -eq 0 ]] && [[ $new_style_count -gt 0 ]]; then
  check_pass "所有文件使用 go_zero 风格（${new_style_count} 个文件）"
elif [[ $old_style_count -gt 0 ]]; then
  check_fail "发现 ${old_style_count} 个旧风格文件"
else
  check_warn "未找到 handler 文件"
fi
echo ""

# 3. 检查编译
echo -e "${BLUE}[3/6] 检查编译${NC}"

for service_dir in "${SCRIPT_DIR}"/*; do
  if [[ ! -d "$service_dir" ]] || [[ ! -f "${service_dir}/go.mod" && ! -f "${SCRIPT_DIR}/../go.mod" ]]; then
    continue
  fi

  service_name=$(basename "$service_dir")

  # 查找主文件
  main_file=""
  for f in "${service_dir}"/*.go; do
    if [[ -f "$f" ]] && grep -q "func main()" "$f" 2>/dev/null; then
      main_file="$f"
      break
    fi
  done

  if [[ -n "$main_file" ]]; then
    if (cd "$service_dir" && go build -o /dev/null "$main_file" 2>/dev/null); then
      check_pass "${service_name}: 编译成功"
    else
      check_fail "${service_name}: 编译失败"
    fi
  fi
done
echo ""

# 4. 检查测试
echo -e "${BLUE}[4/6] 检查测试${NC}"

test_failed=false
for service_dir in "${SCRIPT_DIR}"/*; do
  if [[ ! -d "$service_dir" ]] || [[ ! -d "${service_dir}/internal" ]]; then
    continue
  fi

  service_name=$(basename "$service_dir")

  # 查找测试文件
  test_count=$(find "${service_dir}" -name "*_test.go" 2>/dev/null | wc -l | tr -d ' ')

  if [[ $test_count -gt 0 ]]; then
    if (cd "$service_dir" && go test ./... 2>/dev/null); then
      check_pass "${service_name}: 测试通过（${test_count} 个测试文件）"
    else
      check_fail "${service_name}: 测试失败"
      test_failed=true
    fi
  fi
done

if [[ "$test_failed" == false ]]; then
  check_pass "所有测试通过"
fi
echo ""

# 5. 检查文件结构
echo -e "${BLUE}[5/6] 检查文件结构${NC}"

for service_dir in "${SCRIPT_DIR}"/*; do
  if [[ ! -d "$service_dir" ]] || [[ ! -d "${service_dir}/internal" ]]; then
    continue
  fi

  service_name=$(basename "$service_dir")
  missing=()

  [[ ! -d "${service_dir}/internal/handler" ]] && missing+=("handler")
  [[ ! -d "${service_dir}/internal/logic" ]] && missing+=("logic")
  [[ ! -d "${service_dir}/internal/svc" ]] && missing+=("svc")
  [[ ! -f "${service_dir}/internal/types/types.go" ]] && missing+=("types")

  if [[ ${#missing[@]} -eq 0 ]]; then
    check_pass "${service_name}: 目录结构完整"
  else
    check_warn "${service_name}: 缺少目录 ${missing[*]}"
  fi
done
echo ""

# 6. 检查备份
echo -e "${BLUE}[6/6] 检查备份${NC}"

backup_dirs=($(find "${SCRIPT_DIR}" -maxdepth 1 -type d -name "backup_*" 2>/dev/null | sort -r))

if [[ ${#backup_dirs[@]} -gt 0 ]]; then
  latest_backup="${backup_dirs[0]}"
  backup_time=$(basename "$latest_backup" | sed 's/backup_//')
  check_pass "找到备份: $(basename "$latest_backup")"

  # 检查备份内容
  backup_files=$(find "$latest_backup" -type f -name "*.go" 2>/dev/null | wc -l | tr -d ' ')
  echo -e "  备份文件数: ${backup_files}"
else
  check_warn "未找到备份目录（如果是首次生成则正常）"
fi
echo ""

# 总结
echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}验证结果总结${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"

total=$((PASS_COUNT + FAIL_COUNT))
if [[ $total -gt 0 ]]; then
  pass_rate=$((PASS_COUNT * 100 / total))
else
  pass_rate=0
fi

echo -e "${GREEN}通过: ${PASS_COUNT}${NC}"
echo -e "${RED}失败: ${FAIL_COUNT}${NC}"
echo -e "通过率: ${pass_rate}%"
echo ""

if [[ $FAIL_COUNT -eq 0 ]]; then
  echo -e "${GREEN}╔═══════════════════════════════════════════════════════════╗${NC}"
  echo -e "${GREEN}║              ✓ 迁移验证通过！                             ║${NC}"
  echo -e "${GREEN}╚═══════════════════════════════════════════════════════════╝${NC}"
  exit 0
else
  echo -e "${RED}╔═══════════════════════════════════════════════════════════╗${NC}"
  echo -e "${RED}║              ✗ 发现问题，请检查                           ║${NC}"
  echo -e "${RED}╚═══════════════════════════════════════════════════════════╝${NC}"
  echo ""
  echo -e "${YELLOW}建议:${NC}"
  echo "  1. 查看上面的错误信息"
  echo "  2. 阅读 MIGRATION-GUIDE.md 中的常见问题"
  echo "  3. 使用 git diff 查看具体变化"
  echo "  4. 如需回滚，参考 MIGRATION-GUIDE.md 的回滚方法"
  exit 1
fi
