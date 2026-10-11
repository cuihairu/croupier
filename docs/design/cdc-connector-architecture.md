---
title: CDC Connector 架构——多数据库数据源接入与能力声明（用户令 2026-10-11 整理）
---

# CDC Connector 架构（多数据库）

## 状态

- 状态: **契约已落码（2026-10-11 阶段一交付），实现未做**。本文是用户令的整理稿：多数据库优先级、SourceConnector 契约、统一 ChangeEvent、Connector 能力声明与能力感知发布校验、自研 vs 复用选型、部署形态与产品边界。§3 契约面已落地 `agents/capture/connector/`（ChangeEvent/SourceConnector/Capabilities，纯逻辑零驱动，单测锁定）；连接器实现（MySQL 迁移/PostgreSQL P0）未实施。任务拆分见 `todo.md`「CDC 平台化批」；与 [capture-agent 简档](capture-agent-design.md)（现有 MySQL 单源实现）、[CDC 过滤策略体系](cdc-filter-policy-design.md)（能力声明消费方）、[风控面板设计](risk-control-panel-design.md)（能力透明展示）互链。

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
