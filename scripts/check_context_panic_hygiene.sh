#!/usr/bin/env bash
# context / panic 卫生守卫（CR-FIX-22）。
#
# 规则 1（裸 context 禁止）：internal/api 与 internal/handler 的非测试代码
#   不得裸起 context.Background()——请求链上的 context 必须从入参 ctx 派生
#   （客户端断开不取消的长操作用 context.WithoutCancel(ctx)，限时的用
#   WithTimeout(WithoutCancel(ctx), ...)）。固定签名/启动装配缝隙用
#   `bare-background-ok:` 豁免注释（panic/调用行向上 9 行窗口内）。
# 规则 2（panic 限定）：全仓非测试 panic( 调用必须有 `panic-ok:` 豁免注释
#   （9 行窗口内），注明属 init/注册不变量、启动不可恢复、构造期契约
#   三类之一。新增 panic 必须先论证，否则守卫拒绝。
#
# 负向检查对扫描路径静默 no-op：路径消失即「全绿假象」（历史教训）。
# 因此脚本先断言全部扫描路径存在，路径缺失视为 FAIL。自测见
# scripts/check_context_panic_hygiene_test.sh（注入合成违规断言拒绝）。
set -uo pipefail
cd "$(dirname "$0")/.."

fail=0

CONTEXT_SCAN_PATHS=(
  "internal/api"
  "internal/handler"
)
PANIC_SCAN_PATHS=(
  "internal"
  "agents"
  "pkg"
  "core"
  "cmd"
)

# 1) 扫描路径必须存在（负向检查路径消失=静默 no-op，不判 PASS）
for p in "${CONTEXT_SCAN_PATHS[@]}" "${PANIC_SCAN_PATHS[@]}"; do
  if [[ ! -d "$p" ]]; then
    echo "FAIL: 扫描路径缺失: $p（守卫清单失效=静默 no-op，需同步脚本）"
    fail=1
  fi
done

# 2) 邻近窗口标记检查（python3 遍历，避免逐行 grep -B 的管道裁剪）。
#    argv[1]=裸 context 扫描域数；其余为 panic 扫描域（含 internal 全量）。
python3 - "${CONTEXT_SCAN_PATHS[@]}" -- "${PANIC_SCAN_PATHS[@]}" <<'PYEOF'
import pathlib
import re
import sys

WINDOW = 9
argv = sys.argv[1:]
split = argv.index("--")
context_paths, panic_paths = argv[:split], argv[split + 1:]

def violations(paths, pattern, marker):
    hits = []
    rx = re.compile(pattern)
    for raw in paths:
        root = pathlib.Path(raw)
        if not root.is_dir():
            continue
        for f in sorted(root.rglob("*.go")):
            sp = str(f)
            if sp.endswith("_test.go"):
                continue
            lines = f.read_text(encoding="utf-8", errors="replace").splitlines()
            for i, line in enumerate(lines):
                if rx.search(line):
                    window = "\n".join(lines[max(0, i - WINDOW):i])
                    if marker not in window:
                        hits.append(f"{sp}:{i + 1}: {line.strip()[:120]}")
    return hits

bare = violations(context_paths, r"context\.Background\(\)", "bare-background-ok")
if bare:
    print("FAIL: 裸 context.Background()（请求链须从入参 ctx 派生；固定签名/启动缝隙用 bare-background-ok 豁免）:")
    for h in bare:
        print("  " + h)

panics = violations(panic_paths, r"\bpanic\(", "panic-ok")
if panics:
    print("FAIL: panic 缺 panic-ok 豁免注释（init/不可恢复/构造期不变量三类之一，9 行窗口内）:")
    for h in panics:
        print("  " + h)

sys.exit(1 if (bare or panics) else 0)
PYEOF
if [[ $? -ne 0 ]]; then
  fail=1
fi

if [[ "$fail" -ne 0 ]]; then
  echo "context/panic hygiene guard: FAILED"
  exit 1
fi
echo "context/panic hygiene guard: PASSED"
