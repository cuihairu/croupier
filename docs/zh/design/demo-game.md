---
title: Demo Game（持久化模拟接入方）
---

# Demo Game（持久化模拟接入方）

> **这是模拟接入方，不是真实游戏。** `sdks/go/cmd/demo-game` 的存在目的是：在没有真实接入方
> 的情况下解锁游戏运营面的游戏侧验收（todo GAME-002/004/005/006、UI-001）。它只 import 公共
> Go SDK（`sdks/go`），禁止 import `internal/`，与真实游戏走完全相同的接入路径（dogfood）。
> 它**不**关闭 GAME-001（真实接入方契约调查），也不能替代真实环境的验收记录。

## 1. 定位

一个独立的游戏进程：

- 经公共 Go SDK 拨号 sidecar-agent 的 SDK 网关（默认 `127.0.0.1:19091`）；
- 注册完整游戏运营契约面——**34 个函数**（`player.*`、`mail_task.*`、`ban.*`、`order.*`、
  `activity.*`，外加任务态 `report.generate`），ID 与 schema 与 `sdks/go/examples/gameops-demo`
  完全一致；
- 以**持久化 exactly-once 存储**（经 DSN 切 postgres/sqlite，测试可退内存态）承载这些函数，
  不再是浏览器/进程内存；
- 以固定 PRNG 种子加固定锚点日期生成确定性数据集。

前序形态与 demo-game 补上的缺口：

| 前序                            | demo-game 补上的缺口                   |
| ------------------------------- | -------------------------------------- |
| `web` 浏览器内存 demo 运行时    | 无进程、无 provider 注册、无持久化     |
| `sdks/go/examples/gameops-demo` | 内存 / state-file 存储，无持久重放证明 |
| `sdks/go/examples/gameops-rpg`  | 17 函数不同契约面；快照存储而非 SQL    |

## 2. 契约面

34 个 descriptor 逐字复制自 `examples/gameops-demo/contracts.go`。三方漂移锁
（`contracts_drift_test.go`）断言：

```
gameops-demo 集合 == demo-game 集合 == 模板集合（33 个 binding ID）+ report.generate
```

- `report.generate` 是唯一 `Execution: "task"` 契约；其余全部 `sync`。
- List 契约返回 `{items, total, page, pageSize}` envelope；幂等标记
  `already_credited`（order.recredit）与 `no_failed_targets`（mail_task.retry_failed）保留。
- 对外 JSON 键为 lowerCamelCase；schema 变更的 review 闸门是漂移锁测试。

## 3. Exactly-Once 存储

表（全部 `demo_` 前缀，gorm AutoMigrate——这是 demo 自有数据库，不是 `internal/model`，
编号迁移契约不适用）：

| 表                 | 用途                                                                 |
| ------------------ | -------------------------------------------------------------------- |
| `demo_idempotency` | `UNIQUE(game_id, env, op, operation_id)` + `fingerprint` + 回执 JSON |
| `demo_players`     | 玩家                                                                 |
| `demo_mail_tasks`  | 邮件 / 补偿任务                                                      |
| `demo_bans`        | 封禁记录                                                             |
| `demo_orders`      | 订单与发货状态                                                       |
| `demo_activities`  | 活动与 `demo_activity_versions`（`UNIQUE(activity_id, version)`）    |
| `demo_audit_log`   | 只追加的操作审计                                                     |
| `demo_meta`        | 种子标记与 report 运行序列                                           |

协议（每个变更 handler，单事务）：

1. `Begin(gameID, env, op, operationId, fingerprint)` 仲裁：
   - committed 行存在 → **原样回放存储的回执**，不产生副作用；
   - 同 `operationId` 不同 fingerprint → `operation_fingerprint_mismatch` 错误，客户端必须换新 id；
   - 残留 `pending` 行 → 释放，重试继续。
2. 业务变更与审计行同事务写入。
3. `Commit` 存回执；`Fail` 释放 pending 行（提交前崩溃可重试）。

`operationId` 由调用方显式传入。平台侧 operationId 的签发与传播仍归 GAME-009/GAME-010；
Fail 释放之外的「结果未知」收敛同样归 GAME-010。

存储拒绝库名为 `croupier` 的 DSN——demo 表绝不落平台库。

## 4. 确定性种子

- `-seed N` 驱动 `mulberry32(N)`；全部时间戳派生自固定锚点 `-anchor`
  （默认 `2026-11-01T00:00:00Z`），绝不用挂钟。任一 N 的黄金行数都可复算。
- 执行器依赖的 fixture ID（如 `task-mail-1002`、订单 `2026102308`、`act-4002`）对所有 N 相同；
  只有数量与派生属性随 N 变化。
- 同 N 重跑 no-op；不同 N 被拒绝，除非 `-reseed` 清库重播。
- 种子 7：26 玩家、30 订单、12 封禁、邮件/活动集合——E2E 与截图依赖它。

## 5. 运行

Flags：`-agent`、`-game-id`（默认 `demo-game`）、`-env`（默认 `demo`）、`-dsn`
（`postgres://…` 或 sqlite 路径；空 = 内存态）、`-seed`、`-reseed`、`-http-addr`
（默认 `127.0.0.1:8090`，提供 `/healthz` + `/readyz`）、`-register-only`、`-anchor`、
`-insecure`。

本地单机三进程栈（`scripts/e2e/demo-game.sh` 自动化的就是它）：

```bash
./scripts/dev-certs.sh && make build
./bin/croupier-server --config configs/test-sqlite.yaml
./bin/croupier-agent agent --config configs/agent.demo.yaml   # 网关 19091，scope demo-game/demo
go run ./cmd/demo-game -agent 127.0.0.1:19091 -seed 7 -dsn /tmp/demo.db
```

控制台切到 `demo-game` / `demo` scope（scope 烘焙在 `configs/games.json`）。存量库上烘焙清单仅在
显式设置 `auth.gamesSeedReconcile: true` 时对账补种（默认保持首装仅跑语义）；games REST API 的
create/update 已支持显式 `gameId`——带连字符的标识如 `demo-game` 原样保留
（`POST /api/v1/games {"name":"...","gameId":"demo-game"}`，连字符可用，改名仅限未绑定环境的
游戏；GAME-013 交付）。历史注记：GAME-013 之前 192.168.5.5 曾按 seed 语义一次性 DB 直插（games 行
与 game_envs 绑定）。随后打开 player-manage 模板页即可对真实函数执行 binding。

## 6. 部署（192.168.5.5 demo 机）

`docker/docker-compose.demo-game.yml` 加入既有 `croupier` compose 网络：

- `demo-postgres`（postgres:15-alpine，`demo_game` 库）；
- `demo-agent`（croupier-agent 镜像 + `configs/agent.demo.yaml`，拨号 `croupier-haproxy:19090`）；
- `demo-game`（`docker/Dockerfile.demo-game`，CGO off；健康口 `8090` 发布为
  `${DEMO_GAME_HTTP_PORT:-18090}`——192.168.5.5 上 18090 被其他项目占用，实际用 `18091`）；
- demo-internal 网络把 db 与 game 和平台网络隔离。

```bash
docker compose -f docker/docker-compose.demo-game.yml up -d
docker restart docker-demo-game-1   # 重放演练：余额必须纹丝不动
# （容器名随 compose 项目名变化；192.168.5.5 上是 docker-demo-game-1）
```

## 7. E2E

`bash scripts/e2e/demo-game.sh` 启动真实三进程栈（sqlite server、agent、demo-game），端到端
断言以下链路：

1. provider 注册使 `player.list` 进入 scope 注册表（34 个函数）；
2. 发布闭环：草稿（fixture 从 player-manage TSX 模板导出）→ 发布 → 已发布 console 快照
   （11 个 binding、本地化标题）；
3. binding 执行：list / create / detail，均带直连 invoke 的权威回读；
4. 跨重启 exactly-once：杀掉 demo-game，同 DSN 重启，重放同充值 payload + operationId →
   回执逐字节一致、余额不变；同 operationId 换 payload → 指纹冲突拒绝。

CI：`.github/workflows/ci-demo-game.yml` 对 `sdks/go/**`、`internal/**`、server/agent cmd
路径跑同一脚本。pagespec fixture 由
`web/src/components/PageRenderer/templates/__tests__/playerManagePagespecFixture.test.ts`
与 TSX 常量漂移锁定。

## 8. 已知边界

1. 模拟接入方：GAME-001 保持 BLOCKED；真实环境验收记录单独走。
2. GAME-002/004/005/006 的游戏半边可对 demo-game 验收；**项关闭**仍等平台半边
   （GAME-010 持久凭据、GAME-009 用户拍板、ARCH-024）。
3. GAME-003 关闭额外等 GAME-012（批次账本）；UI-001 等 GAME-002 + GAME-010 + ARCH-024。
4. E2E fixture 只导出 player-manage 一个模板；其余四模板由同一契约二进制覆盖，但无专属
   已发布页 E2E。
5. 分析事件发射是编译期桩（未设 `ANALYTICS_INGEST_SECRET` 即 no-op；HMAC 契约已验证）。
   探针/压测钩子为注释占位。
6. 种子锚点默认固定 2026-11-01；改 `-anchor` 只改派生时间戳，不改 fixture ID。
7. games.json bootstrap 默认仍跳过非空 games 表；设置 `auth.gamesSeedReconcile: true` 后按启动
   幂等补种缺失的烘焙条目（已存在的游戏绝不修改；被刻意删除的烘焙条目重启会被重建，在意此语义时
   保持开关关闭）。dashboard 手建 scope 已支持显式连字符 gameId（GAME-013）；名称派生的回落路径
   仍把 `-` 归一成 `_`。
