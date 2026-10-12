---
title: CDC Connector 架构——多数据库数据源接入与能力声明（用户令 2026-10-11 整理）
---

# CDC Connector 架构（多数据库）

## 状态

- 状态: **契约已落码（2026-10-11 阶段一交付）；PostgreSQL P0 解码层与流式接线已落码（2026-10-13，4306 增量一/二），三源活栈冒烟已进 CI；runner 多 Connector 装配与三闸消费面切换已落码（2026-10-13，4306 增量四）**。本文是用户令的整理稿：多数据库优先级、SourceConnector 契约、统一 ChangeEvent、Connector 能力声明与能力感知发布校验、自研 vs 复用选型、部署形态与产品边界。§3 契约面已落地 `agents/capture/connector/`（ChangeEvent/SourceConnector/Capabilities，纯逻辑零驱动，单测锁定）；PG 实现落地 `agents/capture/connector/pg/`（pgoutput 解码层 + `PostgresConnector` 复制流接线，§8.1/8.2）；MySQL 迁移契约落地 `source.MySQLConnector`（§8.3，行像位置下标 + 凭据 DSNEnv 通道，活体冒烟）；runner 统一消费面落地（§8.4，`newConnector` 工厂 + ProviderConnector 桥接 + sourceType/dsnEnv 配置面）。全链（规则→面板）活栈验证未实施。任务拆分见 `todo.md`「CDC 平台化批」；与 [capture-agent 简档](capture-agent-design.md)（现有 MySQL 单源实现）、[CDC 过滤策略体系](cdc-filter-policy-design.md)（能力声明消费方）、[风控面板设计](risk-control-panel-design.md)（能力透明展示）互链。

## 8. 交付记录

### 8.1 PostgreSQL P0 解码层（4306 第一增量，2026-10-13）

`agents/capture/connector/pg/`（纯逻辑零驱动——不建网络连接、不引
pgx/pglogrepl；复制流握手是下一增量）：

- `lsn.go`：LSN 位点类型（"X/Y" 十六进制 wire 形态 ↔ 十进制 checkpoint
  形态互逆，connector.Position.Kind="lsn"）。
- `pgoutput.go`：pgoutput v1 二进制消息解析（Begin/Commit/Relation/Insert/
  Update/Delete/Truncate；'K' 身份键元组与 'O' 全行旧元组双分支）。
- `decoder.go`：解码状态机——Relation 缓存（OID→列名/身份键）+ 事务缓冲
  （Commit 点一次吐出整事务，从 EndLSN 恢复重放语义完整）+ 结构修订号
  （DDL 加列 bump → SchemaVersion 漂移信号）+ 契约自检（Commit 回填位点/
  时间后跑全量 Validate，产出不合规=解码器缺陷按错误暴露）。
- `capabilities.go`：PostgresCapabilities 诚实口径（afterImage/transaction
  Metadata/resumableCheckpoint=true；beforeImage 依赖源端 REPLICA IDENTITY
  从严 false；snapshot/pushdown=false）。

数据面决策：数值类型（int2/4/8、float4/8、numeric）文本直通为 JSON 数字，
bool 译真假（NaN/Infinity 回退字符串不伪造数字），其余一律 JSON 字符串；
TOAST 未重写列（'u'）行像缺键（非 null）；update 无旧像 Before=nil；
TRUNCATE 闭集无对应操作——跳过+计数（TruncatedRelations），不伪造 delete。

### 8.2 PostgreSQL P0 流式接线（4306 第二增量，2026-10-13）

`PostgresConnector` 实现 `SourceConnector`（`connector.go`），PG 复制流 →
Decoder → 统一 ChangeEvent 的组装层；新增依赖 `jackc/pglogrepl`（与既有
`jackc/pgx/v5` 同源作者栈，仅复制协议层，不复用其连接池）。

- **双连接形态**（PG 协议约束）：普通连接承载 Open/Discover/槽管理 SQL；
  `START_REPLICATION` 要求复制启动参数，Poll 时按 Open 解析的 `pgconn.Config`
  克隆一条复制连接（RuntimeParams `replication=database`）。槽管理走 SQL
  函数 `pg_create_logical_replication_slot`——复制命令版
  `CREATE_REPLICATION_SLOT` 在普通连接上报语法错（42601）。
- **槽与 publication**：slotName/publication 走 Options（必填、`[a-z0-9_]`
  白名单，因复制协议命令不支持参数绑定）；槽不存在则建持久槽（位点恢复
  依赖服务端保留 WAL），publication 必须预建——连接器不自动建
  FOR ALL TABLES，那会把全库变更卷进槽，属源端副作用须运维显式决策。
- **起点决策**：检查点位点续读优先；空 From 取槽 confirmed_flush_lsn（存量
  槽）或建槽 consistent point（新槽），契约要求的起点显式记日志。位点
  语义 at-least-once（边界事务可能重放），与 MySQL 源同口径。
- **心跳**：ReceiveMessage 挂 10s 截止，超时发 standby 状态上报（服务端
  wal_sender_timeout 默认 60s，10s 上报留 4 倍裕量）；keepalive 带
  ReplyRequested 时立即回。关闭路径按正常收敛（不记错误日志），
  `closed` 用 atomic 供 pump 并发判定。
- **Snapshot**：能力声明 false，显式返回 `ErrUnsupportedSnapshot`（不静默
  回空流）。

插件参数是完整 `name 'value'` 对（pgoutput v1：
`proto_version '1'`, `publication_names '<pub>'`）——pglogrepl 的
PluginArgs 原样逗号连接进括号，拆成裸键值对会被服务端按语法错拒收。

活栈验证（CI `capture-matrix` job + 本地 docker 同命令）：postgres:16
`-c wal_level=logical` 起源，integration 测试 `connector_integration_test.go`
（`TEST_POSTGRES_CAPTURE_DSN` 门控，缺席 skip）走完整链——建表建发布 →
Discover 断言列/PK/能力同源 → 空位点 Poll 自动建槽 → INSERT（列名映射/
主键/lsn 位点/时间戳）→ 非键列 UPDATE（Before=nil，REPLICA IDENTITY
DEFAULT 只在**键列变更**时带 'K' 旧元组，与 beforeImage=false 诚实口径
一致）→ 键列 UPDATE（'K' 元组全宽，非键列为 null 占位：
`{"id":1,"v":null}`，活体实测修正了「仅键列」的文档假设）→ Checkpoint
lsn 可解析 → Close → 从检查点续读收 DELETE 事件（槽保留位点语义）。

### 8.3 MySQL 迁移契约（4306 第三增量，2026-10-13）

`source.MySQLConnector`（`agents/capture/source/mysql_connector.go`）把
既有 `MySQLSource`（Provider 形态）包进统一 `SourceConnector` 契约，是
runner 多 Connector 装配与三闸消费面切换的前置件。与 PG 不同，MySQL 不
新建源实现，只做适配——binlog 解析/白名单/位点续读真值仍在
`MySQLSource`，契约壳只做形状翻译。

- **凭据只走契约通道**：`DSNEnv` 指向环境变量（go-sql-driver DSN 形态，
  `ParseDSN` 解析 host/port/user/password）；`Options` 只承载
  `serverId`/`flavor`/`tables` 等非密参数。明文密码不进契约、不落日志，
  错误消息只提环境变量名。
- **行像位置下标（关键映射）**：MySQL binlog ROW 事件是裸值数组，契约
  `ColumnsMapped=false` 的语义正是「行像键为十进制位置下标」——`After`/
  `Before` 键化 `{"0","1",…}`，**不升列名**。这与契约既定兼容决策
  （§7）吻合：闸层「位置下标取值」语义不受影响，列名映射随
  `Discover`（information_schema 摄取，含主键）在契约面单独落地，闸层
  不感知。
- **位点/操作/时间戳**：`GTID` → `Position{Kind:"gtid"}`；`Op` 闭集互转
  （未知操作显式报错，不静默映射）；账号/时间戳原样保留。桥接产出的
  事件过 `connector.ChangeEvent.Validate` 自检（契约层门槛）。
- **能力同源**：`Discover` 产出与 `MySQLCapabilities()` 同一声明；
  `Snapshot` 显式 `ErrUnsupportedSnapshot`（不静默回空流）。
- **活体冒烟**：`TEST_MYSQL_DSN` 门控（CI capture-matrix 复用 MySQL 腿，
  `-run 'TestCaptureSourceFlavors|TestMySQLConnectorContract'`）——Open
  （DSNEnv 通道）→ Discover 断言列/PK/能力 → Poll 空位点收 INSERT
  （位置下标行像/ColumnsMapped=false/gtid 位点）→ Checkpoint → Close。

**边界（本增量到此为止，诚实标注）**：runner 仍消费 Provider 形态的
`MySQLSource`，`MySQLConnector` 尚未被 runtime 调用（前置件，非死代码：
其契约映射口径已被活体冒烟验证）。MySQL 目前是唯一「双形态并存」的源
（Provider + SourceConnector），三闸消费面统一切换到
`connector.ChangeEvent` 属下一增量——切换前闸层读的是 source 形态事件，
与 runner 现状一致。

### 8.4 Runner 多 Connector 装配（4306 第四增量，2026-10-13）

runner 消费面整体切换到 `connector.SourceConnector` /
`connector.ChangeEvent` 契约（§8.3 边界的收口）：

- **runtime 装配**：`newConnector` 工厂按配置面分派——canonical
  `sourceType=postgres` → `pg.PostgresConnector`；`sourceType=mysql`/
  缺省 → `source.MySQLConnector`；legacy `capture.mysql` →
  `ProviderConnector` 桥接（凭据留 YAML，不进契约，warn 提示迁移）。
  `runSourceOnce` 全程契约面：`loadPosition` → `Open(SourceConfig)` →
  `Poll(PollRequest{From, Tables})` → 流结束/关停先 `flushBookmark`
  （Checkpoint 在 Close 后可能失败）再 Close。
- **ProviderConnector 桥接**：任意 `source.Provider` →
  `SourceConnector` 通用适配（MySQL 双形态在此收口）。`bridgeEvent`
  无损映射：行像数组 → 位置下标键 map（`ColumnsMapped=false`）、
  `GTID` → `Position{Kind: PositionKindOf(sourceType)}`（mysql=gtid，
  postgres=lsn）、操作闭集互转（未知操作终止流，不静默映射）、白名单
  二次过滤。legacy 与 canonical mysql 走同一桥接路径，事件形态一致。
- **消费面切换**：三闸（gate 包）与 runner 统计/上报面全部改读
  `connector.ChangeEvent`（`Operation`/`Before`/`After`/`Position`）。
  契约行像键与闸层既有「位置下标取值」语义一致，切换零语义变化。
- **数据源配置面**：`capture.sourceType`（闭集校验）+ `capture.dsnEnv`
  （凭据只走环境变量引用，不进配置文件/不落日志）+ `capture.options`
  （非密参数，`tables` 必填）；legacy `capture.mysql` 仅兼容代码容忍。
  位点文件按源分文件（`<bookmarkDir>/<sourceType>.json`，legacy 与
  canonical mysql 同名迁移不丢续读位点）；`loadPosition` 对 legacy 裸值
  位点按源类型回落 kind（旧位点文件平滑续读）。
- **Readiness 探测与能力自报**：`probeTarget` 经
  `source.AddressFromDSN` / `pg.AddressFromDSN` 从 DSNEnv 解析非凭据
  地址（unix socket 显式跳过探测，不伪造目标）；注册 labels 的
  `captureTables`/`captureConnectorCapabilities` 按源类型分派能力声明。

**边界（本增量到此为止，诚实标注）**：活栈全链验证（规则→面板，
4301 式清单）未实施，属 4306 收尾增量；分源分类型策略发布（每源能力
矩阵映射）随面板数据源页批落地。

## 1. 目标与非目标

**目标**：capture-agent 同一套运行时接入多类数据库数据源，事件模型统一，规则与面板不因数据源种类分叉。

**非目标**（第一批）：不做流式计算/Flink 级 Join；不追求毫秒级端到端延迟（告警秒级即可）；不做异构源之间的分布式事务一致性（各源独立位点管理）。

## 2. 数据库优先级（用户令）

| 批次 | 数据库                                                         | 说明                                         |
| ---- | -------------------------------------------------------------- | -------------------------------------------- |
| P0   | MySQL / PostgreSQL / MariaDB                                   | 首批，先完成 MySQL+PostgreSQL 完整闭环再扩展 |
| P1   | SQL Server / Oracle / MongoDB                                  | 第二梯队                                     |
| P2   | TiDB / CockroachDB / Db2 / 国产库（达梦/OceanBase/GaussDB 等） | 按需求排期                                   |

决策依据：**首批 MySQL+PostgreSQL 先完成完整闭环再扩展**——闭环含连接器、过滤策略、检测规则、告警链、面板能力展示全链，不是"能读 binlog"就算完成。

## 3. SourceConnector 契约（接口草案）

统一 runtime + 插件化 Connector，接口面（非最终 API）：

```go
type SourceConnector interface {
    Open(ctx context.Context, cfg SourceConfig) error
    // Discover 返回源的表/列元数据（列名、主键、before/after 能力等）
    Discover(ctx context.Context, req DiscoveryRequest) (*DiscoveryResult, error)
    // Snapshot 表级快照（按需回补/新表起点，能力不足时明确返回 UNSUPPORTED）
    Snapshot(ctx context.Context, req SnapshotRequest) (<-chan ChangeEvent, error)
    // Poll 增量变更流（binlog/WAL/oplog…），从指定位点续传
    Poll(ctx context.Context, req PollRequest) (<-chan ChangeEvent, error)
    // Checkpoint 返回当前消费位点（持久化 + 可恢复性）
    Checkpoint(ctx context.Context) (Position, error)
    Close(ctx context.Context) error
}

type ChangeEvent struct {
    Source    SourceRef   // connector 标识 + 源端连接描述
    Operation string      // insert/update/delete
    Table     string
    Before    map[string]json.RawMessage // 能力不足时空（见 §4）
    After     map[string]json.RawMessage
    Key       []string    // 主键/关联键列名集
    Transaction *TxInfo   // gtid/lsn/xid 等（可空）
    Position  Position    // 本事件在源端的位点
    SchemaVersion int64
    Timestamp time.Time
}
```

要点：

- **能力差异必须表达，不伪造缺失信息**：before 拿不到就是空 + 能力标志，规则层按"前像不可得"处理（如对账/血缘降级），不能填零值假装存在。
- `Position` 是各源位点抽象（GTID set / LSN / SCN / token），序列化为字符串存 checkpoint；位点语义见过滤策略简档 §4。

### 3.1 Connector 能力声明

每个 Connector 启动时上报能力集，策略发布校验按能力判定兼容性：

| 能力                  | 含义                             | 影响                                                  |
| --------------------- | -------------------------------- | ----------------------------------------------------- |
| `beforeImage`         | 可提供变更前像                   | 对账/血缘缺前像 → 降级或拒发规则                      |
| `afterImage`          | 可提供变更后像                   | 同上                                                  |
| `transactionMetadata` | 事务边界/gtid 级元数据           | 事务级告警聚合不可用                                  |
| `snapshot`            | 表级快照读取                     | 新表历史回补（过滤策略 §4）不可行时只能从当前位点生效 |
| `resumableCheckpoint` | 位点持久化 + 断点续传            | 断线恢复语义                                          |
| `pushdown`            | 条件下推到源端（log 订阅级过滤） | SourceFilter 可下推 vs 必须 Agent 侧过滤              |

## 4. 自研 vs 复用（提前决策项）

用户令要求评估并提前决策，三个候选：

| 方案                                   | 评估                                                                               |
| -------------------------------------- | ---------------------------------------------------------------------------------- |
| 轻量单 Go 二进制内嵌 CDC               | 运维最简单（现状形态）；多源要自研/移植各库客户端协议，长期成本随源数量线性增长    |
| 独立 CDC 进程（Debezium 等）+ 统一桥接 | 复用成熟 CDC；多一套运维面，连接器版本与 Go 运行时解耦，但引入进程间流与部署复杂度 |
| 自研各库 Go Connector                  | 统一契约可控；每源一个客户端协议实现 + 长期维护，成本最高                          |

**建议（待拍板）**：P0 采"轻量单 Go 二进制 + Go 连接器"起步——MySQL binlog 客户端（现状已有）、PostgreSQL pglogical/logical replication 输出插件（或 test_decoding）各实现一个 Connector，接口契约先行（§3），实现分层：接口与 runtime 先冻结，连接器可独立演进。若 P1 接入中发现自研维护成本失控，再评估把增量源迁移到独立 CDC 进程 + 桥接适配器（届时 SourceConnector 契约即抽象边界，改造成本可控）。**无论哪条路，Connector 契约与能力声明先落地**，这是可逆性的关键。

## 5. 部署形态与产品边界（用户令 6 条）

产品边界第一条：**capture-agent 不承诺多租户 SaaS**，按公司内多 game/env 隔离（现有 X-Game-ID/X-Env 模型沿用）。

| #   | 边界               | 说明                                                                   |
| --- | ------------------ | ---------------------------------------------------------------------- |
| 1   | 多数据库数据源管理 | 数据源注册（类型/连接/凭据）集中管理，连接串凭据走现有密钥面，不落日志 |
| 2   | 统一采集运行时     | 一个 capture-runtime 承载多 Connector，统一位点/checkpoint/统计        |
| 3   | 可扩展 Connector   | 新库 = 新增 Connector 实现 + 能力声明，不改 runtime                    |
| 4   | 统一过滤检测       | 过滤策略（过滤策略简档）+ 三闸检测对 Connector 类型透明                |
| 5   | 能力透明面板       | 每个数据源展示能力集（before/after/snapshot…），规则编辑时按能力提示   |
| 6   | 按需部署           | 单 game 可只装 MySQL Connector；多源再挂 PG/其他                       |

## 6. 落到现有代码

| 模块                                       | 改造                                                                                            |
| ------------------------------------------ | ----------------------------------------------------------------------------------------------- |
| `agents/capture/source/`                   | 现有 MySQL binlog 源拆为 SourceConnector 实现；ConfigSource/SourceFactory 按 connector 类型装配 |
| `agents/capture/runner/`                   | runtime 支持多 Connector 并行消费；每 Connector 独立 checkpoint 持久化；统一统计                |
| `agents/capture/`（新增 connector 注册表） | Connector 注册 + 能力声明采集（启动时上报）                                                     |
| Server 数据源管理                          | 数据源 REST（注册/列举/能力查询），game/env scope 隔离；发布校验消费能力集（过滤策略简档 §3）   |
| 面板                                       | 数据源页（采集范围 + 能力标签）；能力标签样式对齐 Nodes 页「捕获」列                            |

## 7. 已知边界

- 契约面已落码（`agents/capture/connector/`：event.go / capability.go / connector.go，单测 7 组含 fakeConnector 接口满足性）；接口草案与 §3.1 能力表以代码为准（实现期差异回写本文）。落码时的两个契约决定：①冲突检测只认精确表项同列两侧，库级 exclude 覆盖 include 是合法「关整库」手势（filter 包口径）；②零值位点 `Valid()=false`（事件必须带位点），空 checkpoint 由 `String()==""` 表达。
- 现有 MySQL 实现（`agents/capture/source/mysql*`）无 beforeImage 概念——现状行像是位置数组 JSON（简档 §9）；统一模型的兼容决策已定（ChangeEvent.ColumnsMapped=false 时行像键=十进制位置下标，与三闸「位置下标取值」语义一致），列名映射（information_schema 摄取）落地时切 true，闸层不受影响。
- MongoDB/国产库等 P1/P2 源无排期，仅留契约扩展点。
- 选型建议（§4）为待拍板项，用户裁决后本节定稿。
