#!/usr/bin/env bash
# 六语言 demo 契约兜底文案基线守护（Go 基准）。
# 根因背景：六个 demo 共享同一 (game,env,functionId) 契约槽位，心跳重注册
# 互相覆盖契约行——任何语言的 summary/description 兜底文案差异都会表现为
# 契约版本反复翻转（B2 function_contract_versions 可见 updated 振荡）与
# 已发布页面 bindingFreshness 永久 stale。历史修复：cpp/csharp 对齐（9a85d137c）、
# python/java schema 对齐（40ea68422）、六语言 summary/description 统一（2026-09）。
# 修改任一 demo 的描述文案必须六语言同步，否则本脚本失败。
set -euo pipefail
cd "$(dirname "$0")/.."

fail=0
err() { echo "DEMO-BASELINE FAIL: $*" >&2; fail=1; }

J="sdks/java/examples/game-demo/src/main/java/com/croupier/sdk/examples/GameDemo.java"
C="sdks/cpp/examples/game_demo.cpp"
CS="sdks/csharp/examples/GameDemo/Program.cs"
PY="sdks/python/examples/game_demo.py"
JS="sdks/js/examples/game_demo.ts"
GO="sdks/go/examples/demo/main.go"

# 1) description 兜底模板必须为 "Demo function {id} for {resource} {operation} operations."
grep -q 'Demo function %s for %s %s operations\.' "$GO" || err "go: description 模板漂移"
grep -q '"Demo function %s for %s %s operations\."' "$J" || err "java: description 模板漂移"
grep -q '"Demo function " + desc.id + " for " + desc.resource + " " + desc.operation + " operations\."' "$C" || err "cpp: description 模板漂移"
grep -q 'Demo function {desc.Id} for {desc.Resource} {desc.Operation} operations\.' "$CS" || err "csharp: description 模板漂移"
grep -q 'Demo function {desc.id} for {desc.resource or .unscoped.} {desc.operation or .invoke.} operations\.' "$PY" || err "python: description 模板漂移"
grep -q 'Demo function ${desc.id} for ${desc.resource || "unscoped"} ${desc.operation || "invoke"} operations\.' "$JS" || err "js: description 模板漂移"

# 2) 禁止历史漂移形态回流
grep -qn 'for %s %s action\.' "$J" "$C" "$GO" 2>/dev/null && err "java/cpp/go: 出现旧的 'action.' 文案"
grep -q '" action\."' "$C" && err "cpp: 出现旧的 'action.' 文案"
grep -q 'action\."' "$CS" && err "csharp: 出现旧的 'action.' 文案"
grep -q 'desc\.summary = desc\.id' "$PY" && err "python: summary 兜底回退为 id（基线是 '{resource} {operation}'）"
grep -q 'desc.summary || desc.id' "$JS" && err "js: summary 兜底回退为 id（基线是 '{resource} {operation}'）"

# 3) summary 基线：`{resource} {operation}` 空格连接（六语言同源兜底）
grep -q 'fmt.Sprintf("%s %s", desc.Resource, desc.Operation)' "$GO" || err "go: summary 模板漂移"
grep -q 'setSummary(desc.getResource() + " " + desc.getOperation())' "$J" || err "java: summary 模板漂移"
grep -q 'desc.summary = desc.resource + " " + desc.operation;' "$C" || err "cpp: summary 模板漂移"
grep -q 'desc.Summary ??= $"{desc.Resource} {desc.Operation}"' "$CS" || err "csharp: summary 模板漂移"
grep -q 'f"{desc.resource} {desc.operation}"' "$PY" || err "python: summary 模板漂移"
grep -q 'summary: desc.summary || `${desc.resource || "function"} ${desc.operation || "invoke"}`' "$JS" || err "js: summary 模板漂移"

# 4) tags 基线：[resource, operation]
grep -q 'setTags(List.of(desc.getResource(), desc.getOperation()))' "$J" || err "java: tags 模板漂移"
grep -q 'desc.tags = {desc.resource, desc.operation};' "$C" || err "cpp: tags 模板漂移"

if [ "$fail" -eq 0 ]; then
  echo "DEMO-BASELINE OK: 六语言 demo 契约兜底文案与 Go 基线一致"
fi
exit "$fail"
