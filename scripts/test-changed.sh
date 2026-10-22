#!/usr/bin/env bash
# scripts/test-changed.sh — 按改动范围选择校验范围（本地开发增量口径）。
#
# CI 门禁保持全量不变；本地开发按"改动了什么"决定跑什么，避免改一行文档
# 也要全量编译测试。口径文档：docs/development/repository-guidelines.md
# "Selective Testing" 小节。
#
# 用法：
#   scripts/test-changed.sh                # base..工作区（含未提交改动）
#   scripts/test-changed.sh --committed    # base..HEAD（只看已提交）
#   scripts/test-changed.sh <base-ref>     # 指定 base（如 origin/main）
# base 缺省依次取 @{u} → origin/main → origin/master → 空树（全量视为改动）。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

# /tmp 常是 tmpfs 且容量有限：-race 测试链接的临时文件写满即
# "disk quota exceeded"（假红）。/tmp 空闲不足 10G 时把 GOTMPDIR 挪到 $HOME。
if [ -z "${GOTMPDIR:-}" ]; then
	avail_kb="$(df -Pk /tmp 2>/dev/null | awk 'NR==2 {print $4}')"
	if [ "${avail_kb:-0}" -lt 10485760 ]; then
		mkdir -p "${HOME}/.gotmp"
		export GOTMPDIR="${HOME}/.gotmp"
		echo "[test-changed] /tmp free < 10G, GOTMPDIR=$GOTMPDIR"
	fi
fi

COMMITTED=0
BASE_REF=""
for arg in "$@"; do
	case "$arg" in
	--committed) COMMITTED=1 ;;
	*) BASE_REF="$arg" ;;
	esac
done

if [ -z "$BASE_REF" ]; then
	if git rev-parse --verify -q '@{u}' >/dev/null 2>&1; then
		BASE_REF='@{u}'
	elif git rev-parse --verify -q origin/main >/dev/null 2>&1; then
		BASE_REF=origin/main
	elif git rev-parse --verify -q origin/master >/dev/null 2>&1; then
		BASE_REF=origin/master
	else
		BASE_REF="$(git hash-object -t tree /dev/null)"
	fi
fi

echo "[test-changed] repo=$ROOT base=$BASE_REF scope=$([ "$COMMITTED" = 1 ] && echo 'base..HEAD' || echo 'base..worktree')"

# ---- 分类（python 辅助：文件桶 + go 包路径 + 纯注释判定 + 反向依赖闭包）----
CLASS_OUT="$(python3 - "$BASE_REF" "$COMMITTED" <<'PYEOF'
import os
import subprocess
import sys

ROOT = os.getcwd()
base = sys.argv[1]
committed = sys.argv[2] == "1"

def git(*args):
    return subprocess.run(["git", *args], cwd=ROOT, capture_output=True, text=True).stdout

changed = {}  # path -> status letter
def add(line):
    parts = line.split("\t")
    st = parts[0][0]
    path = parts[-1]
    changed[path] = st

if committed:
    for line in git("diff", "--name-status", base, "HEAD").splitlines():
        add(line)
else:
    for line in git("diff", "--name-status", base).splitlines():
        add(line)
    for line in git("status", "--porcelain", "--untracked-files=all").splitlines():
        if line.startswith("??"):
            add("A\t" + line[3:])

if not changed:
    sys.exit(0)

FULL_FILES = {"go.mod", "go.sum", "Makefile", "makefile", "CMakeLists.txt", "go.work"}
FULL_PREFIXES = (".github/", "proto/", "buf.yaml", "buf.gen.yaml", "buf.work.yaml")
GO_PREFIXES = ("internal/", "cmd/", "pkg/", "core/", "database/", "agents/", "examples/")
SHARED = ("internal/model", "internal/config", "internal/common", "internal/svc", "internal/db", "pkg/pb")
WEB_FULL_FILES = {"web/package.json", "web/pnpm-lock.yaml"}
WEB_FULL_PREFIXES = ("web/jest.config", "web/config/", "web/.umirc", ".umirc", "web/src/services/api/", "web/src/types/", "web/.env")

def classify(p):
    if p in FULL_FILES or any(p.startswith(x) for x in FULL_PREFIXES):
        return "FULL"
    if any(p == s or p.startswith(s + "/") for s in SHARED):
        return "SHARED"
    if p.endswith(".go") and any(p.startswith(x) for x in GO_PREFIXES):
        return "GO"
    if p in WEB_FULL_FILES or any(p.startswith(x) for x in WEB_FULL_PREFIXES):
        return "WEB_FULL"
    if p.startswith("web/"):
        return "WEB"
    if p.startswith("docs/") or p.endswith(".md"):
        return "DOCS"
    if p.startswith("docker/"):
        return "DOCKER"
    if p.startswith("configs/"):
        return "CONFIGS"
    if p.startswith("scripts/") or p.startswith("tools/"):
        return "SCRIPTS"
    return "OTHER"

def is_comment_only(p):
    # 未跟踪文件无 diff，保守视为代码改动。
    out = git("diff", "--unified=0", "--", p)
    if not out.strip():
        return False
    for line in out.splitlines():
        if line.startswith(("+++", "---")):
            continue
        if line.startswith(("+", "-")):
            c = line[1:].strip()
            if not c or c.startswith("//") or c.startswith("/*") or c.startswith("*"):
                continue
            return False
    return True

modpath = subprocess.run(["go", "list", "-m"], cwd=ROOT, capture_output=True, text=True).stdout.strip()

def nested_module_root(p):
    # 独立 Go module（如 sdks/go）：自文件目录向上找 go.mod，非仓库根即嵌套模块。
    d = os.path.dirname(os.path.join(ROOT, p))
    while d.startswith(ROOT + "/") and d != ROOT:
        if os.path.isfile(os.path.join(d, "go.mod")):
            return os.path.relpath(d, ROOT)
        d = os.path.dirname(d)
    return None

for path in sorted(changed):
    cls = classify(path)
    if cls == "GO":
        co = is_comment_only(path)
        tag = "G_CO" if co else "G"
        print(f"{tag}\t{path}")
        if not co:
            print(f"GOPKG\t{modpath}/{os.path.dirname(path)}")
    elif cls == "OTHER":
        nmod = nested_module_root(path)
        if nmod:
            print(f"NESTED\t{nmod}")
        else:
            print(f"{cls}\t{path}")
    else:
        print(f"{cls}\t{path}")
PYEOF
)"

# ---- 汇总桶 ----
full_files=() shared_files=() go_files=() go_co_files=() go_pkgs=() web_files=() web_full_files=() docs_files=() docker_files=() config_files=() script_files=() nested_files=() other_files=()

while IFS=$'\t' read -r tag path; do
	[ -z "$tag" ] && continue
	case "$tag" in
	FULL) full_files+=("$path") ;;
	SHARED) shared_files+=("$path") ;;
	G) go_files+=("$path") ;;
	G_CO) go_co_files+=("$path") ;;
	GOPKG) go_pkgs+=("$path") ;;
	WEB_FULL) web_full_files+=("$path") ;;
	WEB) web_files+=("$path") ;;
	DOCS) docs_files+=("$path") ;;
	DOCKER) docker_files+=("$path") ;;
	CONFIGS) config_files+=("$path") ;;
	SCRIPTS) script_files+=("$path") ;;
	NESTED) nested_files+=("$path") ;;
	OTHER) other_files+=("$path") ;;
	esac
done <<<"$CLASS_OUT"

# 嵌套模块按目录去重（每个模块只跑一次 build/test）。
if [ "${#nested_files[@]}" -gt 0 ]; then
	mapfile -t nested_files < <(printf '%s\n' "${nested_files[@]}" | sort -u)
fi

all_count=$(( ${#full_files[@]} + ${#shared_files[@]} + ${#go_files[@]} + ${#go_co_files[@]} + ${#web_files[@]} + ${#web_full_files[@]} + ${#docs_files[@]} + ${#docker_files[@]} + ${#config_files[@]} + ${#script_files[@]} + ${#nested_files[@]} + ${#other_files[@]} ))

if [ "$all_count" -eq 0 ]; then
	echo "[test-changed] no changes vs $BASE_REF, nothing to do"
	exit 0
fi

# ---- FULL 判定：显式触发文件 或 共享层改动 ----
MODE=FULL
if [ "${#full_files[@]}" -eq 0 ] && [ "${#shared_files[@]}" -eq 0 ]; then
	MODE=SCOPED
fi

echo "[test-changed] changed entries: $all_count (nested modules collapsed)"
for f in "${full_files[@]:-}" "${shared_files[@]:-}" "${go_files[@]:-}" "${go_co_files[@]:-}" "${web_full_files[@]:-}" "${web_files[@]:-}" "${docs_files[@]:-}" "${docker_files[@]:-}" "${config_files[@]:-}" "${script_files[@]:-}" "${nested_files[@]:-}" "${other_files[@]:-}"; do
	[ -n "$f" ] && echo "[test-changed]   $f"
done
if [ "${#other_files[@]}" -gt 0 ]; then
	echo "[test-changed] [warn] unrecognized scope (no automated check mapped), verify manually:"
	for f in "${other_files[@]}"; do echo "[test-changed]   ? $f"; done
fi

run_docs_checks() {
	local docs_site=0
	for f in "${docs_files[@]:-}"; do
		case "$f" in docs/*) docs_site=1 ;; esac
	done
	if [ "$docs_site" = 1 ]; then
		echo "[docs] build docs site (link check)..."
		pnpm --dir docs build
	fi
	local md=()
	for f in "${docs_files[@]:-}"; do
		case "$f" in *.md) md+=("$f") ;; esac
	done
	if [ "${#md[@]}" -gt 0 ]; then
		if npx --no-install prettier --version >/dev/null 2>&1; then
			echo "[docs] prettier --check on changed md..."
			npx --no-install prettier --check "${md[@]}"
		else
			echo "[docs] prettier unavailable (no node_modules), skip format check"
		fi
	fi
}

if [ "$MODE" = FULL ]; then
	echo "[test-changed] scope: FULL gate"
	if [ "${#shared_files[@]}" -gt 0 ]; then
		echo "[test-changed] full trigger: shared layer -> ${shared_files[*]}"
	fi
	if [ "${#full_files[@]}" -gt 0 ]; then
		echo "[test-changed] full trigger: ${full_files[*]}"
	fi
	echo "[go] gofmt check..."
	gofmt_out="$(gofmt -l .)"
	if [ -n "$gofmt_out" ]; then
		echo "$gofmt_out"
		echo "[go] gofmt dirty, run: make gofmt-fix"
		exit 1
	fi
	echo "[go] go build ./..."
	go build ./...
	# 不带 -race：与 make test / CI（go test ./internal/... ./agents/...）口径一致。
	# -race 全量在本仓当前不可绿（非新增改动所致）：pkg/protocol 溢出用例
	# 触发 checkptr fatal、internal/server 等测试代码存在 atomic/裸读混用、
	# hotreload 插件用例未透传 -race 导致 ABI 不匹配；且 -race 全量会让
	# 开发机过载出现 600s 假红。修复这些是独立质量工程项，见 todo。
	echo "[go] go test -short ./..."
	go test -short ./...
	echo "[web] tsc..."
	pnpm --dir web run tsc
	echo "[web] jest full..."
	pnpm --dir web test
	echo "[test-changed] docs + format checks..."
	run_docs_checks
	echo "[guard] dashboard_vnext_guard..."
	bash scripts/dashboard_vnext_guard.sh
	echo "[format] pnpm format:check..."
	pnpm run format:check
	echo "[test-changed] PASS (full gate)"
	exit 0
fi

# ---- SCOPED 模式 ----
ran_any=0

# GO：改动包 + 传递反向依赖者（含测试导入边）
if [ "${#go_pkgs[@]}" -gt 0 ]; then
	ran_any=1
	pkg_list="${go_pkgs[*]}"
	echo "[go] changed packages: $(echo "$pkg_list" | tr ' ' '\n' | sort -u | tr '\n' ' ')"
	closure="$(python3 - "$pkg_list" <<'PYEOF'
import os
import subprocess
import sys

ROOT = os.getcwd()
mod = subprocess.run(["go", "list", "-m"], cwd=ROOT, capture_output=True, text=True).stdout.strip()
changed = set(sys.argv[1].split())
out = subprocess.run(
    ["go", "list", "-f", '{{.ImportPath}} {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}', "./..."],
    cwd=ROOT, capture_output=True, text=True,
).stdout
imps = {}
for line in out.splitlines():
    parts = line.split(" ", 1)
    ip = parts[0]
    imports = parts[1].split() if len(parts) > 1 else []
    imports = [i for i in imports if i.startswith(mod + "/")]
    imps[ip] = imports
rev = {}
for ip, lst in imps.items():
    for dep in lst:
        rev.setdefault(dep, set()).add(ip)
closure = set()
queue = list(changed)
while queue:
    p = queue.pop()
    if p in closure:
        continue
    closure.add(p)
    queue.extend(rev.get(p, ()))
known = sorted(p for p in closure if p in imps)
print("\n".join(known))
PYEOF
)"
	if [ -z "$closure" ]; then
		echo "[go] no resolvable packages (deleted?), skip go build/test"
	else
		echo "[go] test scope (closure incl. test importers):"
		awk '{print "[go]   " $0}' <<<"$closure"
		mapfile -t pkgs <<<"$closure"
		echo "[go] gofmt check on changed files..."
		gofmt_out="$(gofmt -l "${go_files[@]}")"
		if [ -n "$gofmt_out" ]; then
			echo "$gofmt_out"
			echo "[go] gofmt dirty, run: make gofmt-fix"
			exit 1
		fi
		echo "[go] go build <closure>..."
		go build "${pkgs[@]}"
		echo "[go] go test <closure>..."
		go test "${pkgs[@]}"
	fi
elif [ "${#go_co_files[@]}" -gt 0 ]; then
	echo "[go] comment-only .go changes, skip compile/test: ${go_co_files[*]}"
fi

# 嵌套 Go module（如 sdks/go）
if [ "${#nested_files[@]}" -gt 0 ]; then
	ran_any=1
	for mod in "${nested_files[@]}"; do
		echo "[go] nested module $mod: go build ./... && go test ./..."
		( cd "$mod" && go build ./... && go test ./... )
	done
fi

# WEB
if [ "${#web_full_files[@]}" -gt 0 ] || [ "${#web_files[@]}" -gt 0 ]; then
	ran_any=1
	echo "[web] tsc..."
	pnpm --dir web run tsc
	if [ "${#web_full_files[@]}" -gt 0 ]; then
		echo "[web] full trigger (${web_full_files[*]}) -> full jest..."
		pnpm --dir web test
	else
		# 定向 jest：改动测试文件本身 + 导入了改动源文件的测试文件
		targets=()
		for f in "${web_files[@]}"; do
			case "$f" in
			*.test.ts | *.test.tsx) targets+=("$(basename "$f" | sed 's/\.test\.tsx\?$//;s/\.test\.ts$//')") ;;
			esac
		done
		for f in "${web_files[@]}"; do
			case "$f" in
			*.test.ts | *.test.tsx) continue ;;
			*.ts | *.tsx)
				rel="${f#web/}"
				stem="${rel%.*}"
				hits="$(grep -rl --include='*.test.ts' --include='*.test.tsx' -F "$stem" web/src 2>/dev/null || true)"
				if [ -z "$hits" ]; then
					base="$(basename "$stem")"
					hits="$(grep -rl --include='*.test.ts' --include='*.test.tsx' -F "$base" web/src 2>/dev/null || true)"
				fi
				if [ -n "$hits" ]; then
					while IFS= read -r t; do
						targets+=("$(basename "$t" | sed 's/\.test\.tsx\?$//;s/\.test\.ts$//')")
					done <<<"$hits"
				else
					echo "[web] no test importer found for $f (tsc covers it)"
				fi
				;;
			esac
		done
		if [ "${#targets[@]}" -gt 0 ]; then
			uniq_targets="$(printf '%s\n' "${targets[@]}" | sort -u | tr '\n' '|' | sed 's/|$//')"
			echo "[web] targeted jest: $uniq_targets"
			pnpm --dir web test --testPathPatterns "$uniq_targets"
		else
			echo "[web] no jest targets, tsc only"
		fi
	fi
fi

# DOCS
if [ "${#docs_files[@]}" -gt 0 ]; then
	ran_any=1
	run_docs_checks
fi

# DOCKER
if [ "${#docker_files[@]}" -gt 0 ]; then
	ran_any=1
	if command -v docker >/dev/null 2>&1; then
		for f in "${docker_files[@]}"; do
			case "$f" in
			docker/docker-compose*.yml | docker/compose*.yml)
				echo "[docker] compose config validate: $f"
				docker compose -f "$f" config -q
				;;
			*/Dockerfile* | Dockerfile*)
				echo "[docker] build image: $f"
				docker build -q -f "$f" "$(dirname "$f")"
				;;
			esac
		done
	else
		echo "[docker] docker CLI unavailable, skip compose/image checks"
	fi
fi

# CONFIGS
if [ "${#config_files[@]}" -gt 0 ]; then
	ran_any=1
	echo "[configs] go test ./internal/config/..."
	go test ./internal/config/...
fi

# SCRIPTS
if [ "${#script_files[@]}" -gt 0 ]; then
	ran_any=1
	for f in "${script_files[@]}"; do
		case "$f" in
		*.sh)
			echo "[scripts] bash -n $f"
			bash -n "$f"
			if command -v shellcheck >/dev/null 2>&1; then
				echo "[scripts] shellcheck $f"
				shellcheck -S warning "$f"
			fi
			;;
		esac
	done
fi

if [ "$ran_any" = 0 ] && [ "${#other_files[@]}" -eq "${all_count:-0}" ]; then
	echo "[test-changed] only unrecognized-scope changes; nothing ran (see warn list above)"
	exit 0
fi

echo "[test-changed] PASS (scoped)"
