#!/usr/bin/env bash
#
# 代码风格迁移脚本
# 将所有服务从 gozero 风格迁移到 go_zero 风格（下划线分隔）
#
# 功能：
# 1. 备份所有现有代码
# 2. 重新生成所有服务的代码（使用 go_zero 风格）
# 3. 保留业务逻辑代码（logic 文件）
# 4. 生成迁移报告
#
# 使用方法：
#   ./migrate-to-go-zero.sh           # 交互式确认
#   ./migrate-to-go-zero.sh --yes     # 自动确认
#   ./migrate-to-go-zero.sh --dry-run # 仅显示计划，不执行
#

set -euo pipefail

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKUP_DIR="${SCRIPT_DIR}/backup_$(date +%Y%m%d_%H%M%S)"
DRY_RUN=false
AUTO_YES=false

# 解析参数
while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run)
      DRY_RUN=true
      shift
      ;;
    --yes|-y)
      AUTO_YES=true
      shift
      ;;
    --help|-h)
      echo "使用方法: $0 [选项]"
      echo ""
      echo "选项:"
      echo "  --dry-run    仅显示计划，不执行操作"
      echo "  --yes, -y    自动确认，不询问"
      echo "  --help, -h   显示此帮助信息"
      exit 0
      ;;
    *)
      echo "未知选项: $1"
      echo "使用 --help 查看帮助"
      exit 1
      ;;
  esac
done

# 检查 goctl
if ! command -v goctl >/dev/null 2>&1; then
  GOPATH_BIN="$(go env GOPATH 2>/dev/null)/bin"
  if [[ -x "${GOPATH_BIN}/goctl" ]]; then
    export PATH="${PATH}:${GOPATH_BIN}"
  else
    echo -e "${RED}错误: goctl 未安装${NC}"
    echo "请运行: go install github.com/zeromicro/go-zero/tools/goctl@latest"
    exit 1
  fi
fi

# 查找所有服务
services=()
for dir in "${SCRIPT_DIR}"/*; do
  if [[ -d "$dir" ]] && [[ -f "$dir"/*.api || -f "$dir"/*.proto ]]; then
    services+=("$(basename "$dir")")
  fi
done

if [[ ${#services[@]} -eq 0 ]]; then
  echo -e "${YELLOW}警告: 未找到任何服务${NC}"
  exit 0
fi

# 显示迁移计划
echo -e "${BLUE}╔═══════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║         代码风格迁移：gozero → go_zero                    ║${NC}"
echo -e "${BLUE}╚═══════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${YELLOW}发现以下服务:${NC}"
for service in "${services[@]}"; do
  echo -e "  - ${GREEN}${service}${NC}"
done
echo ""

echo -e "${YELLOW}迁移计划:${NC}"
echo "  1. 备份所有服务代码到: ${BACKUP_DIR}"
echo "  2. 重新生成所有服务代码（使用 go_zero 风格）"
echo "  3. 生成迁移报告"
echo ""

echo -e "${YELLOW}文件命名变化示例:${NC}"
echo -e "  ${RED}adminfunctionpermissionsgethandler.go${NC}"
echo -e "  ↓"
echo -e "  ${GREEN}admin_function_permissions_get_handler.go${NC}"
echo ""

echo -e "${RED}⚠️  警告:${NC}"
echo "  - handler 和 types 文件会被完全重新生成"
echo "  - logic 文件会被重新生成（需要手动合并业务逻辑）"
echo "  - 建议先提交 git，以便回滚"
echo ""

# 确认
if [[ "$DRY_RUN" == true ]]; then
  echo -e "${BLUE}[Dry Run] 仅显示计划，不执行操作${NC}"
  exit 0
fi

if [[ "$AUTO_YES" != true ]]; then
  read -p "是否继续？[y/N] " -n 1 -r
  echo
  if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "已取消"
    exit 0
  fi
fi

# 创建备份目录
echo ""
echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}步骤 1: 备份现有代码${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
mkdir -p "${BACKUP_DIR}"

for service in "${services[@]}"; do
  service_dir="${SCRIPT_DIR}/${service}"
  if [[ -d "${service_dir}/internal" ]]; then
    echo -e "备份: ${GREEN}${service}${NC}"
    cp -r "${service_dir}/internal" "${BACKUP_DIR}/${service}_internal"
  fi
done

echo -e "${GREEN}✓ 备份完成: ${BACKUP_DIR}${NC}"
echo ""

# 重新生成代码
echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}步骤 2: 重新生成代码（go_zero 风格）${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"

# 查找所有 .api 文件
api_files=()
while IFS= read -r file; do
  api_files+=("$file")
done < <(find "${SCRIPT_DIR}" -maxdepth 2 -name "*.api" -print | sort)

if [[ ${#api_files[@]} -eq 0 ]]; then
  echo -e "${YELLOW}警告: 未找到 .api 文件${NC}"
else
  for api_path in "${api_files[@]}"; do
    service_dir="$(dirname "${api_path}")"
    service_name="$(basename "${service_dir}")"

    echo ""
    echo -e "${YELLOW}生成服务: ${service_name}${NC}"

    # 验证 API 文件
    if ! goctl api validate --api "${api_path}" 2>/dev/null; then
      echo -e "${RED}  ✗ API 文件验证失败，跳过${NC}"
      continue
    fi

    # 生成代码
    if goctl api go --api "${api_path}" --dir "${service_dir}" --style go_zero; then
      echo -e "${GREEN}  ✓ 生成成功${NC}"
    else
      echo -e "${RED}  ✗ 生成失败${NC}"
    fi
  done
fi

echo ""
echo -e "${GREEN}✓ 代码生成完成${NC}"
echo ""

# 生成迁移报告
echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}步骤 3: 生成迁移报告${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"

REPORT_FILE="${SCRIPT_DIR}/migration_report_$(date +%Y%m%d_%H%M%S).md"

cat > "${REPORT_FILE}" << EOF
# 代码风格迁移报告

**日期**: $(date '+%Y-%m-%d %H:%M:%S')
**迁移类型**: gozero → go_zero

## 备份信息

**备份目录**: \`${BACKUP_DIR}\`

## 已迁移的服务

EOF

for service in "${services[@]}"; do
  echo "### ${service}" >> "${REPORT_FILE}"
  echo "" >> "${REPORT_FILE}"

  service_dir="${SCRIPT_DIR}/${service}/internal"

  if [[ -d "${service_dir}/handler" ]]; then
    handler_count=$(find "${service_dir}/handler" -name "*.go" 2>/dev/null | wc -l | tr -d ' ')
    echo "- Handler 文件: ${handler_count} 个" >> "${REPORT_FILE}"
  fi

  if [[ -d "${service_dir}/logic" ]]; then
    logic_count=$(find "${service_dir}/logic" -name "*.go" 2>/dev/null | wc -l | tr -d ' ')
    echo "- Logic 文件: ${logic_count} 个" >> "${REPORT_FILE}"
  fi

  echo "" >> "${REPORT_FILE}"
done

cat >> "${REPORT_FILE}" << 'EOF'

## 文件命名变化示例

**旧风格 (gozero)**:
```
adminfunctionpermissionsgethandler.go
userprofilehandler.go
authloginhandler.go
```

**新风格 (go_zero)**:
```
admin_function_permissions_get_handler.go
user_profile_handler.go
auth_login_handler.go
```

## 后续步骤

### 1. 验证生成的代码

```bash
# 检查文件命名
ls services/*/internal/handler/ | head -10
ls services/*/internal/logic/ | head -10

# 尝试编译
cd services/server && go build
cd services/agent && go build
```

### 2. 合并业务逻辑

Logic 文件被重新生成，需要手动合并业务逻辑：

```bash
# 对比差异
diff -r backup_*/server_internal/logic services/server/internal/logic

# 使用 Git 查看变化
git diff services/server/internal/logic/
```

**推荐工具**:
- VS Code: 使用 Compare 功能
- Meld: 可视化对比工具
- Beyond Compare: 三向合并工具

### 3. 更新导入路径（如果需要）

检查是否有代码直接导入了旧的文件名（很少见）。

### 4. 运行测试

```bash
# 运行所有测试
cd services && go test ./...

# 针对特定服务
cd services/server && go test ./...
```

### 5. 提交代码

```bash
git add .
git commit -m "refactor: migrate to go_zero naming style"
```

## 回滚方法

如果迁移出现问题，可以从备份恢复：

```bash
# 恢复特定服务
cp -r backup_*/server_internal services/server/internal

# 或使用 git reset
git reset --hard HEAD
```

## 注意事项

- ✅ handler 文件：完全重新生成，无需合并
- ✅ types.go：完全重新生成，无需合并
- ⚠️ logic 文件：需要手动合并业务逻辑
- ⚠️ middleware：如果有自定义中间件，检查是否受影响

## 常见问题

### Q: 编译错误 "undefined: XXX"
A: 可能是 types.go 中的类型定义变化，检查 API 定义是否完整

### Q: Logic 文件中的业务逻辑丢失
A: 从备份目录中复制业务逻辑代码

### Q: 是否需要更新数据库或配置
A: 不需要，文件命名变化不影响运行时行为

EOF

echo -e "${GREEN}✓ 迁移报告已生成: ${REPORT_FILE}${NC}"
echo ""

# 完成
echo -e "${GREEN}╔═══════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║                    迁移完成！                              ║${NC}"
echo -e "${GREEN}╚═══════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${YELLOW}后续步骤:${NC}"
echo "  1. 查看迁移报告: ${REPORT_FILE}"
echo "  2. 检查生成的文件命名"
echo "  3. 合并业务逻辑（从 ${BACKUP_DIR} 复制）"
echo "  4. 运行测试验证"
echo "  5. 提交代码"
echo ""
echo -e "${YELLOW}回滚方法:${NC}"
echo "  cp -r ${BACKUP_DIR}/server_internal services/server/internal"
echo ""
echo -e "${BLUE}提示: 使用 git diff 查看具体变化${NC}"
