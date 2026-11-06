#!/usr/bin/env bash
# release-quality-gate.sh — CR-FIX-04 (P0-4) 发布门禁
#
# 发布前质量门禁：全量单元测试 + e2e + 契约守卫。任何一步失败即非零退出，
# 调用方（release.yml / nightly.yml）据此拒绝出包。CI 绿是发布的前提；本脚本
# 把「人工确认 CI 绿」升级为发布工作流内强制串跑，杜绝「CI 红仍出包」。
#
# 用法：bash scripts/release-quality-gate.sh
# 前置：仓库已 checkout；本脚本只在 ubuntu 运行（e2e 起本地 SQLite server）。
set -euo pipefail

echo "==> [1/3] 全量单元测试 (go test ./...)"
TEST_LOG=$(mktemp)
all_green=0
for attempt in 1 2 3; do
  echo "==> 全量测试（第 ${attempt}/3 次）"
  if go test -count=1 ./... 2>&1 | tee "$TEST_LOG"; then
    all_green=1
    break
  fi
  # 只允许「整轮全量测试」重跑通过，绝不把失败包隔离重跑通过当成放行：
  # 并发/共享资源触发的真实回归在隔离环境可能消失，隔离通过不能证明发布
  # 候选没有回归。全量失败即阻止发布，除非下一次完整全量运行全绿。
  failed_pkgs=$(grep -E '^FAIL\s+' "$TEST_LOG" | awk '{print $2}' | sort -u || true)
  if [ -z "$failed_pkgs" ]; then
    # 无 FAIL 行但管道失败（如超时/编译错）：真实失败，不放行。
    echo "::error::全量测试失败但无 FAIL 包行（超时或构建错误），拒绝放行"
    rm -f "$TEST_LOG"
    exit 1
  fi
  echo "::warning::第 ${attempt} 次全量测试失败包: $(echo "$failed_pkgs" | tr '\n' ' ')"
done
rm -f "$TEST_LOG"
if [ "$all_green" -ne 1 ]; then
  echo "::error::全量测试 3 次均未全绿（持续 flake 或回归），拒绝发布"
  exit 1
fi
echo "==> 全量测试全绿"

echo "==> [2/3] 契约守卫"
bash .github/scripts/contracts_guard.sh
bash .github/scripts/api_response_guard.sh
bash .github/scripts/architecture_guard.sh
bash scripts/check-sdk-matrix.sh
bash scripts/check-demo-contract-baseline.sh
bash scripts/check_dependency_boundary.sh
bash scripts/check-no-grpc.sh

echo "==> [3/3] E2E（SQLite server + 代理探针）"
# 与 ci.yml e2e job 对齐：构建 server + 两个辅助二进制，用 SQLite 配置起本地
# 实例跑核心请求链，验证发布产物可真实启动并完成握手/任务生命周期。
mkdir -p bin
go build -o bin/croupier-server ./cmd/server
go build -o bin/e2e-agent-probe ./examples/cmd/e2e-agent-probe
go build -o bin/e2e-function-seed ./examples/cmd/e2e-function-seed

E2E_DIR="${RUNNER_TEMP:-/tmp}/croupier-release-gate-$$"
mkdir -p "$E2E_DIR/uploads"
: > "$E2E_DIR/server.log"
# 生成临时配置，避免与本机既有实例的固定端口 18780/19090 冲突。
# 基于 configs/test-sqlite.yaml（SQLite、无外部依赖），只覆盖端口、数据库与
# 存储路径。
HTTP_PORT=$(python3 - <<'PY'
import socket
s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()
PY
)
AGENT_PORT=$(python3 - <<'PY'
import socket
s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()
PY
)
cat > "$E2E_DIR/config.yaml" <<YAML
server:
  host: 127.0.0.1
  port: ${HTTP_PORT}
  timeout: 600000
  mode: dev
database:
  driver: sqlite
  dataSource: ${E2E_DIR}/croupier.db
auth:
  jwtSecret: localdevsecret-change-me
  rbacConfig: configs/rbac.json
  usersConfig: configs/users.json
  gamesConfig: configs/games.json
bootstrapData:
  baseDir: configs
cache:
  enabled: false
log:
  level: debug
  format: console
  output: stdout
control:
  addr: 127.0.0.1:${AGENT_PORT}
YAML
./bin/croupier-server --config "$E2E_DIR/config.yaml" >> "$E2E_DIR/server.log" 2>&1 &
SERVER_PID=$!
cleanup() { kill "$SERVER_PID" 2>/dev/null || true; }
trap cleanup EXIT

ready=0
for i in $(seq 1 120); do
  # 同时等待 HTTP 就绪和 Agent 控制端口 19090 就绪，确保 e2e-agent-probe 可连。
  if curl -sf "http://127.0.0.1:${HTTP_PORT}/healthz" > /dev/null && \
     (exec 3<>/dev/tcp/127.0.0.1/${AGENT_PORT}) 2>/dev/null; then
    ready=1
    break
  fi
  if ! kill -0 "$SERVER_PID" 2>/dev/null; then
    echo "::error::server exited before ready; log:"; cat "$E2E_DIR/server.log"; exit 1
  fi
  sleep 1
done
[ "$ready" -eq 1 ] || { echo "::error::server not ready in 120s"; cat "$E2E_DIR/server.log"; exit 1; }

SERVER_URL="http://127.0.0.1:${HTTP_PORT}" AGENT_ADDR="127.0.0.1:${AGENT_PORT}" DSN="$E2E_DIR/croupier.db" bash scripts/e2e/happy-path.sh
SERVER_URL="http://127.0.0.1:${HTTP_PORT}" AGENT_ADDR="127.0.0.1:${AGENT_PORT}" DSN="$E2E_DIR/croupier.db" bash scripts/e2e/task-lifecycle.sh

echo "==> 发布质量门禁全部通过"
