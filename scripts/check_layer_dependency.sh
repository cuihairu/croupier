#!/usr/bin/env bash
# 四层 import 方向守卫（CR-FIX-21 / 审计 P2-21）。
#
# 定四层（transport → core → platform → api）并以 import 方向强制：
#   传输层（最底层）不得依赖任何上层；
#   core 不得依赖 platform/api；
#   platform 不得依赖 api；
#   api 为顶层，可依赖一切下层。
# 方向为「下层不得 import 上层」，即依赖只能由上层指向下层。
#
# 与 check_dependency_boundary.sh 同款机制：生产代码（排除 *_test.go）
# 的 import 行逐层 grep 违禁上层路径。集成测试可跨层装配真实栈
# （如 platform/registry 的 integration test 引 api/page 构建 PageSpec），
# 故按既有边界守卫先例排除 *_test.go——生产代码 import 方向是门禁对象。
#
# 负向检查对扫描路径静默 no-op：路径消失即「全绿假象」（历史教训）。
# 因此脚本先断言全部扫描路径存在，路径缺失视为 FAIL（守卫清单失效需
# 同步脚本）。自测见 scripts/check_layer_dependency_test.sh（注入合成
# 违规断言拒绝）。
set -uo pipefail
cd "$(dirname "$0")/.."

fail=0

# 1) 扫描路径必须存在（负向检查路径消失=静默 no-op，不判 PASS）
SCAN_PATHS=(
  "internal/transport"
  "internal/core"
  "internal/platform"
  "internal/api"
)
for p in "${SCAN_PATHS[@]}"; do
  if [[ ! -d "$p" ]]; then
    echo "FAIL: 扫描路径缺失: $p（守卫清单失效=静默 no-op，需同步脚本）"
    fail=1
  fi
done

# 2) 各层生产代码不得 import 更上层。
#    匹配「引号紧邻路径」的 import 行（import 形如
#    "github.com/cuihairu/croupier/internal/..." 或 alias "..."；不匹配
#    仅注释引用的路径，避免注释误报）。
check_layer() {
  local layer="$1" denied="$2" desc="$3"
  local hits
  hits=$(grep -rnE "\"[^\"]*cuihairu/croupier/internal/(${denied})" \
    --include='*.go' --exclude='*_test.go' "internal/${layer}" 2>/dev/null || true)
  if [[ -n "$hits" ]]; then
    echo "FAIL: internal/${layer} 生产代码 import 上层（${desc}）:"
    echo "$hits"
    fail=1
  fi
}

check_layer "transport" "core|platform|api" "transport 不得依赖 core/platform/api"
check_layer "core" "platform|api" "core 不得依赖 platform/api"
check_layer "platform" "api" "platform 不得依赖 api"

if [[ "$fail" -eq 0 ]]; then
  echo "PASSED: layer dependency guard passed（transport→core→platform→api import 方向成立）"
fi
exit "$fail"
