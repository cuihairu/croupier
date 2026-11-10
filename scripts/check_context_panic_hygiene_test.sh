#!/usr/bin/env bash
# context/panic 卫生守卫回归自测（CR-FIX-22）：干净树必须 PASS，注入合成
# 违规必须拒绝——证明守卫不是路径静默 no-op 的「全绿假象」。
set -euo pipefail

fixture_paths=(
  "internal/api/context_panic_hygiene_fixture.go"
  "internal/context_panic_hygiene_fixture.go"
)
created_fixtures=()

cleanup() {
  if [[ ${#created_fixtures[@]} -gt 0 ]]; then
    rm -f -- "${created_fixtures[@]}"
  fi
}
trap cleanup EXIT

for fixture_path in "${fixture_paths[@]}"; do
  if [[ -e "${fixture_path}" ]]; then
    echo "guard fixture path already exists: ${fixture_path}" >&2
    exit 1
  fi
done

# 干净树必须 PASS
bash "scripts/check_context_panic_hygiene.sh"

expect_rejection() {
  local name="$1"
  if bash "scripts/check_context_panic_hygiene.sh" > /dev/null 2>&1; then
    echo "FAIL (selftest): 守卫未拒绝合成违规: $name"
    exit 1
  fi
}

# 注入 1：internal/api 无豁免的裸 context.Background()
cat > internal/api/context_panic_hygiene_fixture.go <<'EOF'
package api

import "context"

func hygieneFixtureBare() context.Context {
	return context.Background()
}
EOF
created_fixtures+=("internal/api/context_panic_hygiene_fixture.go")
expect_rejection "bare context.Background without marker"

# 豁免标记就位后必须恢复 PASS（验证标记语法被守卫识别）
python3 - <<'PYEOF'
import pathlib
p = pathlib.Path("internal/api/context_panic_hygiene_fixture.go")
s = p.read_text(encoding="utf-8")
s = s.replace("\treturn context.Background()",
              "\t// bare-background-ok: 合成豁免样式（自测）。\n\treturn context.Background()")
p.write_text(s, encoding="utf-8")
PYEOF
if ! bash "scripts/check_context_panic_hygiene.sh" > /dev/null 2>&1; then
  echo "FAIL (selftest): bare-background-ok 标记未被守卫识别"
  exit 1
fi

rm -f internal/api/context_panic_hygiene_fixture.go
created_fixtures=()

# 注入 2：internal 无豁免的 panic
cat > internal/context_panic_hygiene_fixture.go <<'EOF'
package internal

func HygieneFixturePanic() {
	panic("hygiene fixture")
}
EOF
created_fixtures+=("internal/context_panic_hygiene_fixture.go")
expect_rejection "panic without marker"

python3 - <<'PYEOF'
import pathlib
p = pathlib.Path("internal/context_panic_hygiene_fixture.go")
s = p.read_text(encoding="utf-8")
s = s.replace("\tpanic(\"hygiene fixture\")",
              "\t// panic-ok: 合成豁免样式（自测）。\n\tpanic(\"hygiene fixture\")")
p.write_text(s, encoding="utf-8")
PYEOF
if ! bash "scripts/check_context_panic_hygiene.sh" > /dev/null 2>&1; then
  echo "FAIL (selftest): panic-ok 标记未被守卫识别"
  exit 1
fi

rm -f internal/context_panic_hygiene_fixture.go
created_fixtures=()

echo "context/panic hygiene guard selftest: PASSED"
