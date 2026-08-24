# 数据分析系统修复设计（评审稿）

状态：已实施（P0/P1 于 2026-08-24 合入；P2 出口于 2026-08-24 合入，断点①-⑥ 全部修复，见 repair-plan.md 任务勾选）
日期：2026-08-23
前置分析：本文件由代码实测得出，所有断言带文件:行号证据

## 0. 一句话结论

三条数据链路（独立 ClickHouse 管道 / Server 内嵌 GORM / OTel bridge）各自代码完整、测试充分，
但互相之间和部署配置之间存在 **6 个结构性断点**。本设计不推翻任何已有架构，只做
"接线"：让既有代码按文档宣称的方式真正跑通，并把三链路收敛为一条主链路 + 两条支线。

## 1. 目标与非目标

### 目标

1. **P0 让独立管道跑通**：全新 `docker compose up` 后，ingest → Redis → worker → ClickHouse
   全链路落库成功（当前 3 处致命断点全部修复）
2. **P1 收敛数据链路**：OTel bridge 的事件进入同一条管道（当前写入无人消费的 stream）
3. **P1/P2 补齐部署与出口**：deploy compose 包含完整分析栈；后台 Dashboard 新增
   "数据仓库"页透出 ClickHouse 聚合数据（出口在后台而非 Grafana，与项目演进一致）
4. **P2 明确双写边界**：Server 内嵌 GORM 分析 API（链路 B）与 ClickHouse 管道（链路 A）
   的关系给出明确决策并落文档

### 非目标（明确不做）

- 不新增 FPS/崩溃率等性能指标计算（另立需求）
- 不实现 Kafka 消费端（生产端已有，消费仍走 Redis；Kafka 是 enhancement-plan M3 路线）
- 不做客户端 analytics SDK（sdk-reference.md 的设想，六语言 SDK 另立需求）
- 不做 attribution/segments（前端现为占位路由）
- 不改变 Server 内嵌 22 个分析 API 的行为（已在生产使用，GORM 数据源不动）

## 2. 现状链路与断点（实测）

```
链路A  ingest(:8088) ──MQ──> Redis Streams ──> analytics-worker ──> ClickHouse
       ✅代码完整          ⚠断点③默认noop     ✅代码完整          ⚠断点①②建表/镜像

链路B  POST /api/v1/analytics/ingest ──> GORM behavior_events ──> 22个查询API ──> 前端7页面
       ✅完整可用（Dashboard 当前唯一真实数据源）

链路C  OTel span ──> bridge ──XAdd──> Redis "game:events:<type>"
       ✅生产端完整   ⚠断点④三重不匹配，无任何消费者
```

| #   | 断点                                          | 证据                                                                                                                                                                                                | 影响                                                |
| --- | --------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------- |
| ①   | ClickHouse initdb 与 worker INSERT 列集不匹配 | worker.go:26-27 写 `analytics.events(game_id,env,...)`；initdb/010 建的是 `game_demo_prod.events`（无 game_id/env、多 server_id）；唯一匹配 DDL 藏在 scripts/e2e/analytics.sh:53-94 且缺 3 张聚合表 | 全新部署 worker 首笔写入即失败；聚合表连 E2E 都没建 |
| ②   | Dockerfile.ingest 构建路径不存在              | docker/Dockerfile.ingest:14 `go build ./services/ingest`（实际是 `cmd/ingest`）                                                                                                                     | ingestion 镜像构建必失败                            |
| ③   | ingest 的 MQ 默认 noop                        | factory.go:27-34；docker-compose.yml:441-461 与 quick-start.md 均未设 `ANALYTICS_MQ_TYPE`                                                                                                           | 按文档启动的 ingest **静默丢弃全部事件**            |
| ④   | OTel bridge 三重不匹配                        | stream 名 `game:events:<type>`(bridge:158) vs `analytics:events`(worker:77)；事件名点号 `session.start`(metrics.go:98) vs 下划线 `session_start`(worker:386)；payload camelCase vs snake_case       | bridge 事件无人消费，Redis 中堆积无效数据           |
| ⑤   | deploy compose 无分析栈                       | docker-compose.deploy.yml 服务清单（:2-282）无 ingestion/analytics-worker；otel-collector 挂载源 `docker/configs/otel-collector.yaml` 不存在（:81）；clickhouse 未挂 initdb（:54-56）               | 生产部署形态下独立管道整体缺席                      |
| ⑥   | 双采集路径并行                                | overview.go:210-248 直写 GORM；链路 A 写 ClickHouse；互不相通                                                                                                                                       | 同一事件类型两处定义、两处存储，口径漂移            |

另有一个次级问题：**聚合事件名命名漂移**（worker 匹配 `login/session_start/register/first_active`
下划线，规范 events.yaml 与 bridge 用点号）——归入断点④一并修复。

## 3. 设计决策

### D1 ClickHouse schema 以 worker 为准（单库多租户）

**决策**：删除 010 的 database-per-game 方案，以 E2E 脚本中与 worker 匹配的 DDL 为蓝本
重写 initdb，补齐 3 张聚合表。

**理由**：

- worker 的 `analytics.{events,payments,minute_online,daily_users,daily_revenue}`（含
  game_id/env 列）是唯一有消费者、有 E2E 验证的版本
- database-per-game（010）从未有代码写过它——worker 不支持按 game 路由库
- 单库 + `game_id/env` 列 + `ORDER BY (game_id, env, ...)` 已满足多游戏隔离与查询模式；
  数据量到需要分库时再演进（enhancement-plan M3 范畴）

**变更**：

- `configs/clickhouse/initdb/001_init.sql`：启用注释中的 DDL，格式对齐 worker 列序
- `configs/clickhouse/initdb/010_analytics.sql`：重写为聚合表 DDL（minute_online/daily_users/
  daily_revenue，列序 = worker.go:466-524 的 INSERT），删除 game_demo_prod/staging 演示库
- E2E 脚本改为复用 initdb（消除第三份 DDL 副本）

聚合表 DDL（新增，与 worker INSERT 一一对应）：

```sql
CREATE TABLE IF NOT EXISTS analytics.minute_online (
    m        DateTime,
    game_id  LowCardinality(String),
    env      LowCardinality(String),
    online   UInt32
) ENGINE = SummingMergeTree ORDER BY (game_id, env, m);

CREATE TABLE IF NOT EXISTS analytics.daily_users (
    d         Date,
    game_id   LowCardinality(String),
    env       LowCardinality(String),
    dau       UInt64,
    new_users UInt64,
    version   String
) ENGINE = ReplacingMergeTree ORDER BY (game_id, env, d, version);

CREATE TABLE IF NOT EXISTS analytics.daily_revenue (
    d             Date,
    game_id       LowCardinality(String),
    env           LowCardinality(String),
    revenue_cents UInt64,
    refunds_cents UInt64,
    failed        UInt64,
    version       String
) ENGINE = ReplacingMergeTree ORDER BY (game_id, env, d, version);
```

（worker 每个 flush 周期用 `version=时间戳` 重写当日值，ReplacingMergeTree 保留最新——
与 worker.go:495-524 现行为对齐，不引入新机制。）

### D2 ingest 默认 MQ 从 noop 改为 redis

**决策**：`ANALYTICS_MQ_TYPE` 未设置时的默认值从 `noop` 改为 `redis`；显式设 `noop`
仍可用（本地开发）。

**理由**：noop 是"配错即静默丢数据"的反模式；ingest 服务的全部意义就是转发到 MQ。
显式 noop 保留给无 Redis 的本地调试。

**变更**：

- factory.go 默认分支：`ANALYTICS_MQ_TYPE` 为空 → 尝试 redis，失败则启动报错退出
  （fail-fast，不再静默降级）
- docker-compose.yml ingestion 段：显式加 `ANALYTICS_MQ_TYPE: redis`
- quick-start.md 同步
- redis_pub.go 无需 build tag（factory.go:19 的 `-tags redis_mq` 注释已过时，删除）

### D3 OTel bridge 归流到主管道（断点④）

**决策**：bridge 改为写入 worker 已消费的 `analytics:events` 流，事件名与 payload
归一为 worker 的既有契约（snake_case 字段 + 事件名别名表）。

三处对齐：

1. **stream 名**：`XAdd` 到 `analytics:events`（`TopicPrefix` 配置语义改为"流前缀仅用于
   调试旁路"，默认关闭；或直接废弃 `game:events` 前缀——选后者，少一个配置项）
2. **事件名**：worker 侧加别名表而非改 bridge（bridge 的点号名是 OTel 语义命名，保持）：

```go
// worker.go 聚合匹配处
var eventAliases = map[string]string{
    "session.start": "session_start",
    "session.end":   "session_end",
    "user.login":    "login",
    "user.register": "register",
}
// evt = strings.ToLower(...); if canon, ok := eventAliases[evt]; evt = canon
```

3. **payload 字段**：bridge 的 XAdd 消息体从 camelCase（AnalyticsEvent json tag）改为
   worker 读取的 snake_case（`event/game_id/env/user_id/session_id/ts/props`）。
   bridge 位于 telemetry 包内部、无外部消费者，直接改结构体 json tag 即可。

**理由**：worker 是链路 A/B 的汇聚点，以它为契约中心改动最小；bridge 三个不匹配
一次修完。备选方案"worker 新增消费 `game:events:*`"被否：两套 stream/两套字段格式
长期并存，维护成本更高。

### D4 deploy compose 补全分析栈（断点⑤）

**决策**：docker-compose.deploy.yml 增加 `ingestion` + `analytics-worker` 服务
（复用 dev compose 的构建定义），并修两处挂载：

- otel-collector 挂载改为仓库实际存在的 `configs/otel-collector-config.yaml`
- clickhouse 挂载 `configs/clickhouse/initdb:/docker-entrypoint-initdb.d:ro`

两者均加 `profiles: [analytics]`，默认单机部署不拉起，按需 `--profile analytics` 启用
——分析栈非核心链路依赖，不应强制所有部署承担 ClickHouse 内存开销。

### D5 双链路关系：A 为主、B 保持现状（断点⑥）

**决策**：维持现状但**落文档明确边界**，不做代码双写合并：

- **链路 B（GORM）**：Dashboard 近实时分析（留存/漏斗/采纳，数据量小、查询灵活）
- **链路 A（ClickHouse）**：大规模事件明细与日聚合（数据量大、按 TTL 归档）

在 docs/analytics/index.md 增加"数据链路拓扑"一节说明两者关系与各自数据来源；
api-reference.md 补录 Server 的 22 个 `/api/v1/analytics/*` 端点（当前文档只写了
ingest 的 2 个）。

**理由**：双写合并（一次 ingest 同时落 GORM+Stream）涉及事务一致性，收益低复杂度高；
当前两条链路数据来源天然不同（B 来自游戏端主动上报、A 未来来自 SDK/OTel），保持
分离合理。真正的问题只是"没写清楚"，属文档债。

### D6 ClickHouse 数据出口：进后台 Dashboard（而非 Grafana）

**决策**：ClickHouse 聚合数据通过 **Server 只读查询 API → 后台 Dashboard 页面**透出，
与现有 7 个分析页面（链路 B）同一入口、同一鉴权体系。

**背景**：仓库演进已明确"看板在后台 Dashboard"——
2025-11 的独立管道期出口设计是 Grafana（configs/grafana/ 空骨架），但 2026-03~05
先后建成了 Server 内嵌 22 端点 API 和 web Dashboard 7 个分析页面，后台已成事实上的
分析展示层。 Grafana 仅保留为运维旁路（可选，不作为交付物）。

**实现**（最小切片）：

1. Server 新增 3 个只读端点（挂现有 `/api/v1/analytics` 组，走既有鉴权/scope 中间件）：

```
GET /api/v1/analytics/warehouse/dau?gameId=&env=&from=&to=     → daily_users
GET /api/v1/analytics/warehouse/online?gameId=&env=&minutes=   → minute_online
GET /api/v1/analytics/warehouse/revenue?gameId=&env=&from=&to= → daily_revenue
```

实现位置：internal/api/analytics/ 新增 warehouse.go；数据源用 clickhouse-go 直连
（DSN 复用 worker 的 `CLICKHOUSE_DSN` 环境变量；连接失败时端点返回 503 + 明确
错误"分析仓库未启用"，不影响链路 B 既有端点）2. Dashboard 前端新增"数据仓库"页（web/src/pages/Analytics/Warehouse/）：DAU 趋势、
分钟在线、日收入三个面板，复用现有 service 层模式（services/api/analytics.ts 加
3 个函数）；菜单挂在 Analytics 分组下 3. configs/grafana/ 空骨架**删除**（避免"半成品配置"误导部署），需要 Grafana 的
运维自行接 ClickHouse 数据源

**理由**：链路 A 数据当前零出口；出口选后台 Dashboard 而非 Grafana，因为
（a）与项目演进方向一致（后台已有 7 页 + SSE + 权限体系）；（b）游戏运营人员
不需要第二个系统；（c）Grafana 看板无法复用后台的 RBAC/game scope 隔离。

## 4. 变更清单（按文件）

| #   | 文件                                                           | 变更                                                                                            | 断点 | 优先级 |
| --- | -------------------------------------------------------------- | ----------------------------------------------------------------------------------------------- | ---- | ------ |
| 1   | configs/clickhouse/initdb/001_init.sql                         | 启用 events/payments DDL（对齐 worker 列序）                                                    | ①    | P0     |
| 2   | configs/clickhouse/initdb/010_analytics.sql                    | 重写为 3 张聚合表 DDL，删 database-per-game                                                     | ①    | P0     |
| 3   | docker/Dockerfile.ingest                                       | 构建路径 `./services/ingest` → `./cmd/ingest`                                                   | ②    | P0     |
| 4   | internal/analytics/mq/factory.go                               | 默认 noop → redis + fail-fast；删过时 tag 注释                                                  | ③    | P0     |
| 5   | docker/docker-compose.yml（ingestion 段）                      | 加 `ANALYTICS_MQ_TYPE: redis`                                                                   | ③    | P0     |
| 6   | internal/analytics/worker/worker.go                            | 事件名别名表（点号→下划线）                                                                     | ④    | P1     |
| 7   | internal/telemetry/analytics_bridge.go                         | stream 名改 `analytics:events`；payload json tag 改 snake_case                                  | ④    | P1     |
| 8   | docker/docker-compose.deploy.yml                               | +ingestion/analytics-worker（profile: analytics）；修 otel-collector 挂载；clickhouse 挂 initdb | ⑤    | P1     |
| 9   | internal/api/analytics/warehouse.go（新）                      | 3 个 ClickHouse 只读查询端点                                                                    | ⑤    | P2     |
| 10  | web/src/services/api/analytics.ts + pages/Analytics/Warehouse/ | 数据仓库页（DAU/在线/收入）                                                                     | ⑤    | P2     |
| 10b | configs/grafana/（删除空骨架）                                 | 移除半成品配置                                                                                  | ⑤    | P2     |
| 11  | docs/analytics/index.md                                        | 数据链路拓扑一节（A/B 关系）                                                                    | ⑥    | P2     |
| 12  | docs/analytics/api-reference.md                                | 补录 22 个 Server 端点                                                                          | ⑥    | P2     |
| 13  | docs/analytics/quick-start.md                                  | ANALYTICS_MQ_TYPE 说明 + deploy profile 用法                                                    | ③⑤   | P0     |
| 14  | scripts/e2e/analytics.sh                                       | DDL 改为复用 initdb；补聚合表断言                                                               | ①    | P0     |

## 5. 兼容性与风险

| 风险                                 | 评估                             | 缓解                                                         |
| ------------------------------------ | -------------------------------- | ------------------------------------------------------------ |
| D2 改默认值：已有环境显式依赖 noop？ | 低——noop 环境必然没在收数据      | 发布说明标注；显式 `ANALYTICS_MQ_TYPE=noop` 不受影响         |
| D1 换 DDL：已用 010 建表的环境       | 低——010 的表从未被任何代码写入   | initdb 仅首次初始化生效；存量环境表为空，可手动 DROP         |
| D3 改 bridge stream 名/payload       | 中——bridge 有 46+ 测试需同步     | payload 改 json tag 属机械变更；别名表方向不动 bridge 事件名 |
| D4 deploy 加 profile                 | 无——默认不启用                   | —                                                            |
| D6 Server 连 ClickHouse 失败         | 低——只影响 3 个新端点            | 端点返回 503，链路 B 端点不受影响                            |
| E2E 依赖 scripts 内联 DDL            | 改为复用 initdb 文件，单一事实源 | 脚本本地无法挂载时保留内联副本作 fallback                    |

## 6. 验收标准

1. **全新环境拉起**：`docker compose up redis clickhouse ingestion analytics-worker`
   → 向 ingest 发 HMAC 签名的 events/payments 请求 → ClickHouse `analytics.events/payments`
   出现记录，`minute_online/daily_users/daily_revenue` 在 flush 周期后出现聚合值
2. **E2E 扩展**：scripts/e2e/analytics.sh 增加聚合表断言 + 点号事件名
   （`session.start`）触发 HLL 的用例
3. **bridge 归流**：`ANALYTICS_BRIDGE_ENABLED=true` 的 Server 产生的业务事件出现在
   `analytics:events` 流并被 worker 消费落库
4. **回归**：链路 B 的 22 个 API 全部既有测试通过（不动）；bridge 的 telemetry 测试
   更新后全绿；六 SDK 合同测试不受影响（未触碰 SDK）
5. **deploy**：`docker compose -f docker-compose.deploy.yml --profile analytics config`
   校验通过；后台 Dashboard"数据仓库"页在本地 compose 栈下能出图（DAU/在线/收入三面板）

## 7. 工作量估算

| 批次                         | 内容                                                     | 估算     |
| ---------------------------- | -------------------------------------------------------- | -------- |
| P0（断点①②③）                | DDL×2 + Dockerfile + factory 默认值 + compose/quickstart | 1 人日   |
| P1（断点④⑤）                 | bridge 归流（含测试更新）+ deploy compose                | 1.5 人日 |
| P2（断点⑥ + Dashboard 出口） | warehouse API + 前端页 + 文档                            | 1.5 人日 |

合计约 4 人日；P0 单独可先行合入。
