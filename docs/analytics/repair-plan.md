# 数据分析系统修复 — 原子任务清单

状态：已完成（2026-08-29 验收收口；代码任务此前分批合入，4 个验收门当日于 192.168.5.5 deploy 栈实测通过，见文末「验收记录」）
来源：`docs/analytics/repair-design.md`（设计决策 D1–D6，已评审）
拆分原则：每个任务独立可开发、可验证、可单独提交合入；标注依赖与验收命令。

## 批次总览

| 批次 | 目标                                      | 任务数  | 依赖                     |
| ---- | ----------------------------------------- | ------- | ------------------------ |
| P0   | 独立管道全新部署可跑通（断点①②③）         | T1–T8   | 无                       |
| P1   | OTel bridge 归流 + 生产部署补全（断点④⑤） | T9–T15  | P0                       |
| P2   | 后台 Dashboard 出口 + 文档（断点⑥）       | T16–T22 | P0（T16 依赖 D1 聚合表） |

验证环境：192.168.5.5 自托管 runner（ssh 别名 `runner-docker`），dev compose 已含
redis/clickhouse/analytics-worker/ingestion 服务定义。

---

## 批次 P0 — 独立管道跑通

### ✅ T1 修复 Dockerfile.ingest 构建路径

- 文件：`docker/Dockerfile.ingest`
- 改动：`./services/ingest` → `./cmd/ingest`（第 14 行）
- 验收：`docker build -f docker/Dockerfile.ingest .` 成功产出镜像
- 依赖：无 ｜ 预估：0.1d ｜ 对应断点②

### ✅ T2 initdb/001 启用明细表 DDL

- 文件：`configs/clickhouse/initdb/001_init.sql`
- 改动：去掉注释，启用 `analytics.events` / `analytics.payments` DDL；
  列序逐列对齐 `internal/analytics/worker/worker.go:26-27` 的 INSERT
  （events: event_time, game_id, env, user_id, session_id, event, channel,
  platform, country, app_version, event_id, props_json；payments 同理）；
  保留 `TTL 6/12 MONTH` 与 `ORDER BY (game_id, env, event, user_id, event_time)`
- 验收：`docker exec clickhouse clickhouse-client < 001_init.sql` 后
  `DESCRIBE analytics.events` 列序与 worker INSERT 一致
- 依赖：无 ｜ 预估：0.25d ｜ 对应断点①（明细半）

### ✅ T3 initdb/010 重写为聚合表 DDL

- 文件：`configs/clickhouse/initdb/010_analytics.sql`（整文件重写）
- 改动：删除 database-per-game 全部内容（game_demo_prod/staging 两套演示库），
  替换为 3 张聚合表（worker.go:466/495/524 的 INSERT 一一对应）：
  - `analytics.minute_online (m, game_id, env, online) SummingMergeTree`
  - `analytics.daily_users (d, game_id, env, dau, new_users, version) ReplacingMergeTree`
  - `analytics.daily_revenue (d, game_id, env, revenue_cents, refunds_cents, failed, version) ReplacingMergeTree`
- 验收：`DESCRIBE` 三表列序 = worker INSERT；`SHOW DATABASES` 无 game_demo_prod
- 依赖：无 ｜ 预估：0.25d ｜ 对应断点①（聚合半）

### ✅ T4 E2E 脚本复用 initdb + 补聚合断言

- 文件：`scripts/e2e/analytics.sh`
- 改动：内联 DDL 段（53-94 行）改为 `docker exec -i clickhouse clickhouse-client < configs/clickhouse/initdb/001_init.sql`（本地无挂载时回退内联副本）；测试事件追加 `session.start`（点号，验证 T9 前先保持下划线）；断言聚合表 flush 后有行
- 验收：`bash scripts/e2e/analytics.sh` 全绿（CI ci-analytics.yml 跟随）
- 依赖：T2 T3 ｜ 预估：0.25d ｜ 对应断点①

### ✅ T5 ingest 默认 MQ 改 redis + fail-fast

- 文件：`internal/analytics/mq/factory.go`
- 改动：`ANALYTICS_MQ_TYPE` 为空时从 noop → redis（构造失败返回 error 终止进程，
  不再静默降级）；显式 `noop` 保留；删除 `redis_mq` build tag 过时注释（19 行）；
  同步更新 factory 单测（默认值/失败路径）
- 验收：`go test ./internal/analytics/mq/...` 全绿；无 REDIS_URL 时 ingest 启动报错退出
- 依赖：无 ｜ 预估：0.25d ｜ 对应断点③（代码半）

### ✅ T6 compose ingestion 显式设 MQ 类型

- 文件：`docker/docker-compose.yml`（ingestion 段 441-461）
- 改动：environment 追加 `ANALYTICS_MQ_TYPE: redis`
- 验收：`docker compose config` 展开可见该变量
- 依赖：T5 ｜ 预估：0.1d ｜ 对应断点③（配置半）

### ✅ T7 quick-start 文档同步

- 文件：`docs/analytics/quick-start.md`
- 改动：启动命令补 `ANALYTICS_MQ_TYPE=redis` 说明 + 显式 noop 的本地调试场景
- 验收：按文档裸跑命令事件不再丢失
- 依赖：T5 ｜ 预估：0.1d

### ✅ T8 P0 端到端验证（验收门）

- 环境：192.168.5.5 dev compose
- 步骤：`docker compose up -d redis clickhouse ingestion analytics-worker` →
  按 api-reference.md 构造 HMAC 签名的 events/payments 请求打 `:18081` →
  等 flush 周期 → 查 ClickHouse 五张表全部有数据
- 验收：明细+聚合五表均非空；记录到本任务评论作为证据
- 依赖：T1–T7 ｜ 预估：0.25d

> P0 完成即达成「全新部署管道跑通」，可单独合入发布。

---

## 批次 P1 — bridge 归流 + 生产部署

### ✅ T9 worker 事件名别名表

- 文件：`internal/analytics/worker/worker.go`（384-406 聚合匹配处）
- 改动：加 `session.start→session_start`、`session.end→session_end`、
  `user.login→login`、`user.register→register` 归一映射（点号事件不改变
  bridge 侧 OTel 语义命名）；更新 worker 单测覆盖别名
- 验收：`go test ./internal/analytics/worker/...` 含点号事件触发 HLL 用例
- 依赖：无（可与 P0 并行）｜ 预估：0.25d ｜ 对应断点④（命名第三重）

### ✅ T10 bridge stream 名 + payload 对齐

- 文件：`internal/telemetry/analytics_bridge.go`
- 改动：① XAdd 目标流 `game:events:{type}` → `analytics:events`/
  `analytics:payments`（按事件性质）；② AnalyticsEvent 结构体 json tag
  camelCase → worker 读取的 snake_case（event/game_id/env/user_id/session_id/ts/props）；
  ③ `ANALYTICS_TOPIC_PREFIX` 配置废弃（保留解析但不再拼流名，日志提示）
- 验收：bridge XAdd 的消息能被 worker 正常 insertEvent 解析
- 依赖：T9 ｜ 预估：0.5d ｜ 对应断点④（stream + payload 两重）

### ✅ T11 bridge/telemetry 测试更新

- 文件：`internal/telemetry/` 相关 9 个测试文件（46+ 用例）
- 改动：流名/payload 断言同步 `analytics:events` 与 snake_case
- 验收：`go test ./internal/telemetry/...` 全绿
- 依赖：T10 ｜ 预估：0.5d

### ✅ T12 bridge 归流端到端验证（验收门）

- 步骤：dev compose 起 `ANALYTICS_BRIDGE_ENABLED=true` 的 server（或独立
  测试进程）→ 触发一次业务事件 → 断言 `analytics.events` 表出现对应行
- 验收：表中有 bridge 来源事件；无未消费的 `game:events:*` stream 残留
- 依赖：T8 T10 T11 ｜ 预估：0.25d

### ✅ T13 deploy compose 补分析栈服务

- 文件：`docker/docker-compose.deploy.yml`
- 改动：从 dev compose 移植 `ingestion` + `analytics-worker` 两个服务
  （image 指向 ghcr `croupier-ingest`/`croupier-analytics-worker`——若 GHCR
  无此二镜像，Docker workflow 需同步加 build 目标）；均挂 `profiles: [analytics]`
- 验收：`docker compose -f docker-compose.deploy.yml --profile analytics config` 通过
- 依赖：T1（ingest 镜像可构建）｜ 预估：0.5d ｜ 对应断点⑤

### ✅ T14 deploy compose 修挂载源

- 文件：`docker/docker-compose.deploy.yml`
- 改动：otel-collector 挂载 `docker/configs/otel-collector.yaml`（不存在）→
  `configs/otel-collector-config.yaml`；clickhouse 补挂
  `./configs/clickhouse/initdb:/docker-entrypoint-initdb.d:ro`；
  deploy workflow 的 Prepare 步骤同步拷贝该目录
- 验收：`--profile analytics up` 后 clickhouse 自动建出五张表
- 依赖：T2 T3 T13 ｜ 预估：0.25d ｜ 对应断点⑤

### ✅ T15 生产部署验证（验收门）

- 步骤：触发 Deploy Self Hosted（runner-docker/192.168.5.5），启用 analytics
  profile → 全链路复验 T8 场景于 deploy 栈
- 验收：deploy 栈五表落库；otel-collector 正常启动
- 依赖：T13 T14 ｜ 预估：0.25d

---

## 批次 P2 — 后台出口 + 文档

### ✅ T16 warehouse 只读 API

- 文件：`internal/api/analytics/warehouse.go`（新）+ routes 注册
- 改动：3 端点 `GET /api/v1/analytics/warehouse/{dau,online,revenue}`，
  clickhouse-go 直连（DSN 复用 `CLICKHOUSE_DSN`），查询 D1 三张聚合表；
  连接失败返回 503 `分析仓库未启用`；挂现有鉴权/scope 中间件
- 验收：`go test ./internal/api/analytics/...` 新增用例（mock CH 连接）全绿
- 依赖：T2 T3（表存在）｜ 预估：0.5d ｜ 对应 D6

### ✅ T17 前端数据仓库页

- 文件：`web/src/services/api/analytics.ts`（+3 函数）、
  `web/src/pages/Analytics/Warehouse/index.tsx`（新）、`web/config/routes.ts` 注册
- 改动：DAU 趋势/分钟在线/日收入三面板（复用现有图表组件模式）；
  503 时显示"未启用分析仓库"引导态
- 验收：dev 栈下页面出图；`pnpm --dir web test` 全绿
- 依赖：T16 ｜ 预估：0.5d

### ✅ T18 删除 Grafana 空骨架

- 文件：`configs/grafana/`（整目录）、引用它的 compose/文档段落
- 验收：全仓 grep 无 configs/grafana 残留引用
- 依赖：无 ｜ 预估：0.1d ｜ 对应 D6

### ✅ T19 链路拓扑文档

- 文件：`docs/analytics/index.md`
- 改动：新增「数据链路拓扑」节：链路 A（ClickHouse 管道）/B（Server GORM）
  /C（OTel bridge）各自数据来源、查询出口、适用场景；修正链路图出口
  `Grafana / 报表` → `后台 Dashboard 数据仓库页`
- 验收：docs-link 检查通过
- 依赖：T17（出口真实存在后落档）｜ 预估：0.25d ｜ 对应断点⑥

### ✅ T20 api-reference 补 Server 端点

- 文件：`docs/analytics/api-reference.md`
- 改动：补录 22 个 `/api/v1/analytics/*` 端点（从 internal/handler/routes.go
  488-523 逐个核对）+ 3 个 warehouse 端点
- 验收：端点数与路由注册一致（脚本抽查）
- 依赖：T16 ｜ 预估：0.25d ｜ 对应断点⑥

### ✅ T21 Dashboard 出口端到端验证（验收门）

- 步骤：5.5 deploy 栈灌入数据 → 后台切 scope → Analytics → 数据仓库页三面板出图
- 验收：截图/数据行数证据
- 依赖：T15 T17 ｜ 预估：0.25d

### ✅ T22 收尾：设计文档状态更新 + 发布说明

- 文件：`docs/analytics/repair-design.md`（标记各断点已修复）、
  repair-plan.md（任务勾选）、发布说明提及 `ANALYTICS_MQ_TYPE` 默认值变更
- 依赖：T21 ｜ 预估：0.1d

---

## 汇总

- 任务数：22（含 4 个验收门）
- 工作量：P0 ≈ 1.4d ｜ P1 ≈ 2.25d ｜ P2 ≈ 2d，合计 ≈ 5.7d（设计稿 4d 的细化版，
  差额来自 T13 可能需要 Docker workflow 加镜像目标 + 测试更新单列）
- 关键路径：T1→T5→T6→T8（P0 门）→ T10→T12（归流门）→ T13→T15（部署门）→ T16→T17→T21（出口门）
- 风险项：GHCR 无 ingest/worker 镜像（T13 前置确认）；telemetry 测试量大（T11）

## 附：本轮实战带入的注意点

1. 镜像 CI 缓存曾产出过旧代码镜像（sdk-demo-go 先例）：T1 合入后确认
   docker-go-sdk-examples 同级的 Docker workflow 对 ingest 真重建而非全 CACHED
2. 5.5 部署通道 = workflow_dispatch 触发 Deploy Self Hosted（runner-docker）；
   analytics profile 验证（T15）走同一通道
3. dev compose 多语言 demo 同 scope 互踩的教训：analytics 各服务使用独立
   REDIS_DB/前缀，无共享命名空间冲突，无此风险

---

## 验收记录（2026-08-29，192.168.5.5 deploy 栈）

| 验收门             | 结果 | 证据                                                                                                                                                                   |
| ------------------ | ---- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| T8 管道跑通        | ✅   | HMAC 签名 events/payments 打 `:18081` 返回 202；flush 后 ClickHouse 五表非空（events=4 / payments=1 / minute_online=1 / daily_users=2 / daily_revenue=1，game=t8game） |
| T12 bridge 归流    | ✅   | invoke `mail.send`×3 → `analytics.events` 出现 `function.call`×3 + `function.call.error`×3（含 trace_id/error.message）；`analytics:events` 流消费正常零死信           |
| T15 生产部署       | ✅   | deploy-self-hosted 加 `analytics_bridge` 输入项；ingestion/worker/clickhouse 于 deploy 栈常驻（profile analytics）                                                     |
| T21 Dashboard 出口 | ✅   | `/api/v1/analytics/warehouse/{dau,online,revenue}` 全部出数：dau=2、online=1（HLL 分钟）、revenueCents=9900                                                            |

### 验收过程中抓出并修复的缺陷（均已在生产栈复现→修复→复验）

1. **bridge 零值 FlushInterval panic**：YAML 路径未设 flushInterval 时 `NewTicker(0)`
   打崩 server（重启循环）。构造器补 30s/100 条默认值。
2. **bridge 发送端从未接线**：`TrackFunctionCall` 等入口全库无调用方，归流事件数为零。
   `functionInvoke` 首尾经 `BridgeFunctionCall` 发 `function.call(/.error)`。
3. **bridge ts 毫秒当秒**：`time.Unix(UnixMilli, 0)` 把事件抛到 58630 年，worker 端
   `event_time` 解析失败 100% 进死信。改 `time.UnixMilli` + 单位契约单测。
4. **ingest 文档示例错误**：`ts`/`game_id`/`env` 应为顶层字符串字段，原数字 ts + attrs
   嵌套示例被校验器拒。api-reference 已修正。

### 运维备注

- deploy 栈启用归流：`gh workflow run deploy-self-hosted.yml -f analytics_bridge=true ...`
- worker 毒消息会整批拖累 insert 并重试写回原流；如再遇解析类死信刷屏，
  停 worker → 清 `analytics:events` 与 `analytics:checkpoint:*` → 重启。
