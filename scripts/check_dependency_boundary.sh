#!/usr/bin/env bash
# 执行领域依赖方向边界守卫（ARCH-019）。
#
# 契约（core/README.md）：core/ 与 agents/ 的生产代码不得 import internal/。
# ARCH-019 收敛：internal/platform/functioninvoke 生产代码不得 import
# internal/api（分配闸门下沉到 internal/logic/assignment 后，执行领域只
# 依赖领域包，不依赖 API 层）。
#
# 负向检查对扫描路径静默 no-op：路径消失即「全绿假象」（历史教训）。
# 因此脚本先断言全部扫描路径存在，路径缺失视为 FAIL（守卫清单失效需同步
# 脚本）。自测见 scripts/check_dependency_boundary_test.sh（注入合成违规
# 断言拒绝）。
set -uo pipefail
cd "$(dirname "$0")/.."

fail=0

SCAN_PATHS=(
  "core"
  "agents"
  "internal/platform/functioninvoke"
)

# 1) 扫描路径必须存在（负向检查路径消失=静默 no-op，不判 PASS）
for p in "${SCAN_PATHS[@]}"; do
  if [[ ! -d "$p" ]]; then
    echo "FAIL: 扫描路径缺失: $p（守卫清单失效=静默 no-op，需同步脚本）"
    fail=1
  fi
done

# 2) core/ + agents/ 生产代码不得 import internal/
#    匹配「引号紧邻路径」的 import 行（import 形如
#    "github.com/cuihairu/croupier/internal/..." 或 alias "..."；不匹配
#    仅注释引用的路径，避免注释误报）。
core_agents=$(grep -rnE '"[^"]*cuihairu/croupier/internal/' \
  --include='*.go' --exclude='*_test.go' core agents 2>/dev/null || true)
if [[ -n "$core_agents" ]]; then
  echo "FAIL: core/ 或 agents/ 生产代码 import internal/（core/README.md 契约禁止）:"
  echo "$core_agents"
  fail=1
fi

# 3) internal/platform/functioninvoke 生产代码不得 import internal/api
#    （执行领域依赖方向收敛：闸门已下沉领域包）
fn_api=$(grep -rnE '"[^"]*cuihairu/croupier/internal/api' \
  --include='*.go' --exclude='*_test.go' internal/platform/functioninvoke 2>/dev/null || true)
if [[ -n "$fn_api" ]]; then
  echo "FAIL: internal/platform/functioninvoke 生产代码 import internal/api（ARCH-019 收敛禁止）:"
  echo "$fn_api"
  fail=1
fi

if [[ "$fail" -eq 0 ]]; then
  echo "PASSED: dependency boundary guard passed（core/agents 不 import internal/；functioninvoke 不 import internal/api）"
fi
exit "$fail"
