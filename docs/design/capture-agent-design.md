---
title: capture-agent 设计简档——游戏库变更捕获与道具反作弊三道闸（立项待批）
---

# capture-agent 设计简档（道具反作弊监控）

## 状态

- 状态: **Proposed（简档待用户过目，未动码）**。批准后按 §8 分期实施。
- 定名（用户拍板，2026-10-09）：本 agent 定名 **`capture-agent`**（capture = Change Data Capture 的 C）；旧候选 cdc-agent 弃用。三 agent 定稿 sidecar-agent / devops-agent / capture-agent，全景见 [Agent 清单](agents-inventory.md)。
- 立项口径（用户令，2026-10-09）：
  1. CDC（Change Data Capture，变更数据捕获）读游戏业务库变更日志，**可开关**；
  2. 三道闸：对账 / CEP（Complex Event Processing，复杂事件处理）/ 血缘，异常进 croupier 现有告警页并带规则命中详情；
  3. capture-agent 归入**现有 agent 注册链**，进程纳入 supervisor（进程监管）；
  4. **源层可插拔 Provider**：MySQL（binlog）+ PostgreSQL（逻辑复制）默认提供，SQLite/SQL Server/MongoDB 留扩展位；三道闸规则层与源解耦，所有源吐**统一变更事件流**；
  5. **极简**：规则白名单/阈值全部走面板配置不写死；规则 DSL 择最简。
- 关联: [Agent 清单](agents-inventory.md)（三 agent 定名/职责边界/面板分页/scope 归属）· [agent-core 通用能力层简档](agent-core-design.md)（capture-agent = core + 业务插件的载体）· [Agent Supervisor 简档](agent-supervisor-design.md)（core 即被监管对象，协议一份）· [数据库监控设计](../research/db-monitoring-design.md)（同库部署的姊妹件：那个管指标健康，本件管数据变更合规）· [Provider 插件设计](provider-plugin-design.md)（可插拔原则同源）

## 0. 作用（Agent 清单口径，先读这节）

| 维度               | 内容                                                                                                                                                                |
| ------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 职责一句话         | **防私改数据库**：读游戏库变更日志，变更发生即告警——谁、以什么方式、改了什么、有没有业务流水背书                                                                    |
| 部署位置（贴谁跑） | 游戏网络内、贴近游戏业务库（MySQL/PostgreSQL），与 sidecar-agent 同机或邻近部署                                                                                     |
| 监控对象           | 道具相关表（货币/物品/装备）的行级变更（insert/update/delete）                                                                                                      |
| 输入               | 源库变更日志（binlog / WAL 逻辑复制）+ 面板下发的规则（白名单/阈值，game-scoped 配置）                                                                              |
| 输出               | 告警事件（进现有告警页，带规则命中详情）+ 位点/命中统计上报                                                                                                         |
| 与其他 agent 边界  | 只管**数据变更合规**：不做库指标健康（db-monitoring 姊妹件的面）、不碰函数注册调用（sidecar-agent 专属）、不碰 CI/CD（devops-agent 专属）；发现异常**只告警不处置** |
| scope 归属         | 注册必须携带 `game_id`+`env` 标签（无标签不进面板）；告警与变更事件一律按 scope 过滤，跨环境不串（见 Agent 清单 §scope）                                            |

## 1. 问题

道具（货币/物品/装备）异常发放是游戏运营最高频的资金级事故：直接改库、绕过业务流水的人工补偿、非授权时段的批量刷取、小量连续试探绕阈值。事后审计依赖 binlog 回放，发现滞后以天计。需要的不是事后取证，而是**变更发生即告警**：谁（连接账号）、以什么方式（应用写入 vs 人工 SQL）、改了什么（库.表.行.前后值）、有没有业务流水背书。

## 2. 定位与部署形态

```
游戏业务库(MySQL/PostgreSQL)
   │ binlog / 逻辑复制(WAL)
   ▼
capture-agent（独立进程，游戏网络内，与 sidecar-agent 同机或邻近部署）
   │  统一变更事件流
   ├─▶ 三道闸（对账/CEP/血缘）──异常──▶ croupier server 告警页（现有体系）
   │
   └─ 位点持久化（GTID/LSN bookmark，崩溃安全）
```

- capture-agent 是**标准 agent**：走既有注册上线链（AgentRegister/心跳/任务通道），注册携带 `game_id`+`env` 标签，面板可见、可停启。
- 进程监管复用 supervisor：`managedProcesses` 配置 capture-agent 自身，崩溃自动拉起/退避/熔断/事件环全部复用 S2，**不另造监管**；自身存活/依赖就绪探测走 `core/healthprobe`（Liveness/Readiness，探库连通即 Readiness 语义）。
- 与 sidecar-agent 的关系：既可独立二进制（`agents/capture/cmd/capture-agent`）由 sidecar-agent 托管拉起，也可并置同进程（分期再定，默认独立进程 + supervisor 托管——崩溃域隔离，CDC 挂不拖累函数通道）。

## 3. 源层：可插拔 Provider（统一变更事件流）

### 3.1 统一事件契约（规则层只认这个）

```proto
message ChangeEvent {
  string source_type = 1;      // "mysql" | "postgres"（闭集，随源扩展）
  string database = 2;         // 库（PG 为 database+schema 拼接约定）
  string table = 3;
  string op = 4;               // insert/update/delete（闭集）
  bytes  before_json = 5;      // 前像（update/delete 有；PG 需 REPLICA IDENTITY FULL）
  bytes  after_json = 6;       // 后像（insert/update 有）
  string gtid = 7;             // 源原生位点标识：MySQL GTID / PG LSN（幂等键的组成部分）
  string account = 8;          // 连接账号（血缘闸核心字段；PG 见 §3.3 已知边界）
  string client_addr = 9;      // 连接来源（尽力而为）
  int64  ts_unix_ms = 10;      // 事件时间（毫秒）
  string server_id = 11;       // 源实例标识（多实例分流）
}
```

- 幂等键 = `gtid + table + 主键 + op`：at-least-once 语义下闸层/告警层按幂等键去重。
- 位点 bookmark 持久化（本地文件 + 上报 server 双写），崩溃拉起后从 bookmark 续读——**不丢事件、幂等去重**，与 supervisor 自动拉起天然拼合。

### 3.2 源 Provider 接口

```go
// 每个 Provider 只做一件事：把源日志翻译成 ChangeEvent 流。
type ChangeSource interface {
    Open(ctx, SourceConfig, bookmark) (eventCh <-chan ChangeEvent, err)
    Bookmark() []byte        // 当前位点（崩溃前落盘）
    Close(ctx)
}
```

- 开关：`capture.enabled: false` 缺省关；开启亦按库粒度 `tables` 白名单订阅（只订道具相关表，噪声与带宽双控）。

### 3.3 源成熟度与接入成本（文档标注，选型依据）

| 源         | 机制                       | 库                                            | 成熟度                                                      | 前置条件                                                                              | 已知边界                                                              |
| ---------- | -------------------------- | --------------------------------------------- | ----------------------------------------------------------- | ------------------------------------------------------------------------------------- | --------------------------------------------------------------------- |
| MySQL      | binlog ROW 模式            | github.com/go-mysql（siddontang，canal 同源） | **高**（国内游戏业事实标准，社区大）                        | `binlog_format=ROW`、`binlog_row_image=FULL`、REPLICATION SLAVE 权限、独立 server_id  | 无实质边界；GTID 原生                                                 |
| PostgreSQL | 逻辑复制（pgoutput）       | github.com/cockroachdb/pglogrepl + jackc/pgx  | **中高**（pglogrepl 是 CockroachDB 维护的逻辑复制协议实现） | `wal_level=logical`、publication、replication slot、`REPLICA IDENTITY FULL`（要前像） | **WAL 不携带会话账号**——血缘闸的 account 字段 PG 侧拿不到原生值，见下 |
| SQLite     | （无日志流）触发器镜像表   | 自写 trigger→change 表轮询                    | 低                                                          | 每张受监控表加 AFTER 触发器                                                           | 侵入 schema；留扩展位不进默认两源                                     |
| SQL Server | 内置 CDC / Change Tracking | 官方 sqlserver 驱动                           | 中                                                          | 版本/许可依赖                                                                         | 留扩展位                                                              |
| MongoDB    | Change Streams             | mongo-driver                                  | 中                                                          | 副本集                                                                                | 留扩展位                                                              |

**PG 血缘边界（诚实标注）**：PG 逻辑解码输出不含执行事务的会话角色。可选补齐路径：① pgaudit/`log_statement` 侧信道关联（部署成本+）；② 库上触发器写审计表（侵入）；③ 血缘闸在 PG 源降级为「连接来源不可知，仅对账+CEP 生效」。默认按 ③ 落地并在面板标注降级态，①② 留作后续增强——**不静默假装有账号字段**。

## 4. 三道闸（规则层，与源解耦）

所有闸消费同一事件流，逐事件评估、输出「命中详情」结构进告警。

### 4.1 闸一：对账（道具变更 ↔ 业务流水）

- **语义**：道具表变更必须能 JOIN 到业务流水（订单/邮件附件/GM 补偿单等）在时间窗内的对应记录；无流水 = 异常。
- **配置面（面板）**：道具表清单 + 每表的「流水关联规则」——流水表、JOIN 键（如 `item.change_reason_id ↔ ledger.trace_id`）、时间窗（默认 ±5 分钟）、豁免名单（如离线结算批处理账号）。
- **最简实现**：事件进 (table, join_key) 待对账队列 + 流水侧同样可订阅（流水表也在订阅清单内），窗口内双向匹配销账，超窗未销 = 告警。**不引对账引擎，环形窗口 + map 即可。**

### 4.2 闸二：CEP（时序模式，三个内置模式起步）

- **非白名单时段**：表/操作 × 时段白名单（如道具表仅 02:00-04:00 允许批量变更，其余时段变更即告警）。
- **数量突变**：单事件 delta 或滑动窗口累计 delta 超阈值（按表/字段配置绝对值或百分比基线）。
- **连续试探**：同 (账号, 表) 滑动窗口内 N 次小变更（每次低于突变阈值）——低频慢刷形态，纯计数器实现。
- **最简实现**：固定模式模板 + 参数，**不引 CEP 引擎**（Flink/Esper 级别全否）。滑窗用环形桶计数，内存 O(配置基数)。

### 4.3 闸三：血缘（谁在写）

- **语义**：区分「应用写入」与「人工 SQL」：非白名单账号对受监控表的写入 = 异常（人工直改库未走审批流）。
- **配置面（面板）**：账号白名单（按 库/表 粒度），应用账号与运维豁免账号分列。
- MySQL 侧 `account` 原生可得；PG 侧按 §3.3 边界降级。

### 4.4 命中详情（告警载荷）

```
rule: lineage.whitelist_violation | reconcile.missing_ledger | cep.burst_delta | cep.off_hours | cep.probing
severity: warn | critical（面板按规则配置）
event: ChangeEvent 关键字段（表/行主键/delta/before→after 摘要）
context: 账号、GTID、时间窗内相邻事件摘要、规则参数快照
```

## 5. 规则 DSL 择最简（用户令：择最简）

**结论：v1 不做表达式语言，做「参数化规则模板」。**

- 三道闸的全部规则形态收敛为**固定模板集合**（上面 5 个内置模式），每条规则 = `{模板, 表清单, 参数(json), 豁免名单, severity, enabled}`。
- 模板集合是代码（闸的实现），规则是纯数据——无解析器、无表达式求值、无脚本注入面、无 DSL 版本漂移。
- 面板编辑 = 结构化表单（模板决定了字段），不是文本框里写代码。
- 预留：确有复杂布尔组合需求时，二期评估受限表达式（google/cel-go 一类），但**默认不进**——历史经验（本仓 openapi 推导、迁移 DSL）表明表达力换来的第一件事永远是逃逸面与调试成本。

## 6. 配置与下发（白名单/阈值不写死）

- 规则与源订阅配置存 **server 侧 game-scoped 配置**（复用现有 game 库配置资源机制），面板增删改查。
- 下发复用 agent 既有**配置同步拉取通道**（extension sync puller 同族机制）：规则版本号轮询 → 变更拉取 → 热生效；capture-agent 本地缓存最后版本，断连时按旧规则继续跑（降级不中断）。
- 规则命中统计（各规则触发次数）随常规指标上报，面板可见「哪些规则在响」。

## 7. 告警接入（现有体系）

- capture-agent 经现有 agent→server 上行通道新增**告警事件上报**（消息类型新增，wire 协议同族扩展），server 落现有告警存储/告警页，复用静默、认领、升级链路——**不另立告警面**。
- 告警页可见：来源 agent、规则命中详情（§4.4）、事件摘要与跳转参数。
- 面板 **capture-agent 页**（见 [Agent 清单](agents-inventory.md) §面板）：实例列表 + 实时变更事件流与命中告警，全部按 `game_id`+`env` scope 过滤；日志入口复用 supervisor 日志下载链。
- **投递出口 herald**：规则命中告警在落告警页（真值）后，可经 [herald 对接](agent-herald-integration.md) 投递到 IM/短信等渠道（croupier 只报事件，渠道/收件人 herald 侧配置；herald 不可达不影响告警落库）。

## 8. 分期

| 期  | 内容                                                                                                                         | 依赖                |
| --- | ---------------------------------------------------------------------------------------------------------------------------- | ------------------- |
| C1  | agent-core 骨架 + capture-agent 骨架（注册上线/心跳/监管挂接）+ MySQL 源 + 统一事件流 + 位点持久化；`capture.enabled` 缺省关 | agent-core 简档获批 |
| C2  | 闸三血缘 + 闸二 CEP（off_hours/burst_delta/probing）+ 规则模板与面板配置面 + 告警上报进告警页                                | C1                  |
| C3  | 闸一对账（流水订阅 + 窗口销账）+ PG 源（含血缘降级态标注）                                                                   | C2                  |
| C4  | 扩展位评估（SQLite 触发器镜像/SQL Server/Mongo）+ pgaudit 血缘增强评估                                                       | 按需                |

## 9. 已知边界（诚实清单）

- PG 血缘账号不可得（§3.3），默认降级运行并有面板标注。
- 对账闸要求流水表同库可订阅；跨库流水（分库分表）C3 不承诺，需按游戏方拓扑定制路由。
- binlog/WAL 保留期短于 capture-agent 停机时长时位点失效，从可得起点重扫（对账闸窗口兜底误报）；监控位点滞后并告警。
- 闸层评估为尽力而为（agent 侧内存态）：agent 重启丢失滑窗状态，血缘/对账不受影响（纯逐事件）。
- 前像依赖 `binlog_row_image=FULL` / `REPLICA IDENTITY FULL`，未满足时 update 前像为空，对账/突变按后像降级。
