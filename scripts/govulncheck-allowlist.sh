#!/usr/bin/env bash
# govulncheck + 显式豁免清单。
#
# 用途：vulndb 对个别漏洞不登记修复版本（Fixed in: N/A）时，即便代码已
# 通过 replace 换入上游修复 commit，govulncheck 仍按版本区间报红。本脚本
# 在保持「未豁免漏洞仍然失败」语义下放行已处置条目。
#
# 维护规则：豁免条目必须注明处置方式与移除条件，上游可用版本发布后立即
# 删除对应条目（replace 与豁免同 PR 撤除）。
set -euo pipefail

# ---- 豁免清单（格式：GOID|处置说明） ----
ALLOWLIST=(
  "GO-2026-6452|excelize GetRows panic（CVE-2026-59162）；已 replace 到上游 master f0b1c24ee69c（含修复 #2331/#2366），vulndb 尚无 fixed 版本可登记；上游发版后撤 replace 与本条目"
)

govulncheck_bin="${GOVULNCHECK_BIN:-$HOME/go/bin/govulncheck}"
output="$("$govulncheck_bin" ./... 2>&1)" && exit 0
status=$?

if [ "$status" -eq 2 ]; then
  # 工具自身错误（网络/DB 不可达等），不属于漏洞判定，原样透传
  echo "$output"
  exit "$status"
fi

echo "$output"
echo
echo "=== 豁免清单核对 ==="
ids=$(printf '%s\n' "$output" | grep -oE 'GO-[0-9]{4}-[0-9]+' | sort -u)
if [ -z "$ids" ]; then
  echo "未解析到漏洞 ID，按失败处理"
  exit 1
fi

bad=0
while IFS= read -r id; do
  hit=""
  for entry in "${ALLOWLIST[@]}"; do
    if [ "$id" = "${entry%%|*}" ]; then
      hit="$entry"
      break
    fi
  done
  if [ -n "$hit" ]; then
    echo "::warning::$id 已豁免：${hit#*|}"
  else
    echo "::error::$id 未豁免，存在未处置漏洞"
    bad=1
  fi
done <<< "$ids"

exit "$bad"
