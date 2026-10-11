---
title: capture-agent 设计简档——游戏库变更捕获与道具反作弊三道闸（立项待批）
---

# capture-agent 设计简档（道具反作弊监控）

## 状态

- 状态: **C2 已交付（2026-10-11）**。C1 骨架（2026-10-10：注册上线/心跳/事件流/位点）+ 闸层三闸 reconcile/cep/lineage（§8.1–8.3）+ 告警上报链（§8.4）+ 闸规则下行拉取（§8.5）+ 面板编辑面（§8.6，下行链闭环）+ 事件粗过滤接线（§8.7，CDC 平台化批阶段二起步）+ appliedRevision 上报链（§8.8，阶段三首项）+ 回滚 API（§8.9）落地（`agents/capture/{source,gate,config,runner}` + `cmd/capture-agent`，`capture.enabled` 缺省关）。后续演进（过滤策略体系/多数据库 Connector/风控面板）见 [CDC 过滤策略体系](cdc-filter-policy-design.md)、[CDC Connector 架构](cdc-connector-architecture.md)、[风控面板设计](risk-control-panel-design.md)。
- 定名（用户拍板，2026-10-09）：本 agent 定名 **`capture-agent`**（capture = Change Data Capture 的 C）；旧候选 cdc-agent 弃用。三 agent 定稿 sidecar-agent / devops-agent / capture-agent，全景见 [Agent 清单](agents-inventory.md)。
- 立项口径（用户令，2026-10-09）：
  1. CDC（Change Data Capture，变更数据捕获）读游戏业务库变更日志，**可开关**；
  2. 三道闸：对账 / CEP（Complex Event Processing，复杂事件处理）/ 血缘，异常进 croupier 现有告警页并带规则命中详情；
  3. capture-agent 归入**现有 agent 注册链**，进程纳入 supervisor（进程监管）；
  4. **源层可插拔 Provider**：MySQL（binlog）+ PostgreSQL（逻辑复制）默认提供，SQLite/SQL Server/MongoDB 留扩展位；三道闸规则层与源解耦，所有源吐**统一变更事件流**；
  5. **极简**：规则白名单/阈值全部走面板配置不写死；规则 DSL 择最简。
- 关联: [Agent 清单](agents-inventory.md)（三 agent 定名/职责边界/面板分页/scope 归属）· [agent-core 通用能力层简档](agent-core-design.md)（capture-agent = core + 业务插件的载体）· [Agent Supervisor 简档](agent-supervisor-design.md)（core 即被监管对象，协议一份）· [数据库监控设计](../research/db-monitoring-design.md)（同库部署的姊妹件：那个管指标健康，本件管数据变更合规）· [Provider 插件设计](provider-plugin-design.md)（可插拔原则同源）

## 0. 作用（Agent 清单口径，先读这节）

| 维度               | 内容                                                                                                                                                                                                  |
| ------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 职责一句话         | **防私改数据库**：读游戏库变更日志，变更发生即告警——谁、以什么方式、改了什么、有没有业务流水背书                                                                                                      |
| 部署位置（贴谁跑） | 游戏网络内、贴近游戏业务库（MySQL/PostgreSQL），与 sidecar-agent 同机或邻近部署                                                                                                                       |
| 监控对象           | 道具相关表（货币/物品/装备）的行级变更（insert/update/delete）                                                                                                                                        |
| 输入               | 源库变更日志（binlog / WAL 逻辑复制）+ 面板下发的规则（白名单/阈值，game-scoped 配置）                                                                                                                |
| 输出               | 告警事件（进现有告警页，带规则命中详情）+ 位点/命中统计上报                                                                                                                                           |
| 与其他 agent 边界  | 只管**数据变更合规**：不做库指标健康（db-monitoring 姊妹件的面）、不碰函数注册调用（sidecar-agent 专属）、不做内网远程接入/堡垒机（devops-agent 专属，2026-10-11 口径重定）；发现异常**只告警不处置** |
| scope 归属         | 注册必须携带 `game_id`+`env` 标签（无标签不进面板）；告警与变更事件一律按 scope 过滤，跨环境不串（见 Agent 清单 §scope）                                                                              |

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

| 源         | 机制                       | 库                                            | 成熟度                                                      | 前置条件                                                                              | 已知边界                                                                                                                                            |
| ---------- | -------------------------- | --------------------------------------------- | ----------------------------------------------------------- | ------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| MySQL      | binlog ROW 模式            | github.com/go-mysql（siddontang，canal 同源） | **高**（国内游戏业事实标准，社区大）                        | `binlog_format=ROW`、`binlog_row_image=FULL`、REPLICATION SLAVE 权限、独立 server_id  | **binlog 事件不携带会话账号**（C1 实证：go-mysql replication 事件无 session account 字段）——account 字段 C1 恒空，血缘闸按 §3.3 边界降级；GTID 原生 |
| PostgreSQL | 逻辑复制（pgoutput）       | github.com/cockroachdb/pglogrepl + jackc/pgx  | **中高**（pglogrepl 是 CockroachDB 维护的逻辑复制协议实现） | `wal_level=logical`、publication、replication slot、`REPLICA IDENTITY FULL`（要前像） | **WAL 不携带会话账号**——血缘闸的 account 字段 PG 侧拿不到原生值，见下                                                                               |
| SQLite     | （无日志流）触发器镜像表   | 自写 trigger→change 表轮询                    | 低                                                          | 每张受监控表加 AFTER 触发器                                                           | 侵入 schema；留扩展位不进默认两源                                                                                                                   |
| SQL Server | 内置 CDC / Change Tracking | 官方 sqlserver 驱动                           | 中                                                          | 版本/许可依赖                                                                         | 留扩展位                                                                                                                                            |
| MongoDB    | Change Streams             | mongo-driver                                  | 中                                                          | 副本集                                                                                | 留扩展位                                                                                                                                            |

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
- MySQL 与 PG 两侧 `account` 均不可原生获得（binlog/WAL 事件均不含会话账号，C1 实证）；血缘闸 account 字段 C1 恒空，两侧同按 §3.3 边界降级。

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

**分期口径（2026-10-11 拍板）**：上表为原始设计批次；实际推进按用户令闸序
**reconcile → cep → lineage** 执行（对账从 C3 提到 C2 首位，血缘最后）。批次划分
不改变各闸的依赖关系，仅改变推进次序与验收次序。

### 8.1 C2 交付记录：闸一·对账（2026-10-11）

代码面：`agents/capture/gate`（规则评估层，与源解耦）+ `agents/capture/config`
的 `capture.gates.reconcile.rules`（文件配置，纯数据）+ `agents/capture/runner`
接线（`consume` 内联 `Evaluate`、位点 tick 内 `Sweep`、命中 → `slog warn` +
`Stats.GateHits`）。规则 ID `reconcile.missing_ledger`，severity 缺省 `warn`。

与原设计的实现差异（诚实清单，逐项记明理由）：

1. **键形态**：§4.1 以字段名表述 JOIN 键（`item.change_reason_id ↔ ledger.trace_id`）；
   C1 行像是位置数组 JSON、无列名映射（§9 末条），故 `itemKey`/`ledgerKey` 规格为
   **整数串 = 行像位置下标**（如 `"1"`）。字段名形态保留为前向兼容：在数组行像上
   取不到值即按「键缺失」处理，列名映射（information_schema 摄取）到位后同一
   规格字符串可无缝切换为字段名——配置无需改。
2. **双向销账**：道具侧入 debit 队列；流水侧优先销同键最早 debit，无待销则挂
   credit（流水先于道具变更到达的时序）；道具侧到达时同样先销同键 credit。
   credit 超窗**静默清**——流水无对应道具变更不是异常（如退款冲销、GM 回收）。
3. **键缺失降级**：道具侧取不到键（NULL 关联键、键规格与行像列数不匹配）按
   never-match 哨兵入窗并超窗告警（命中 Key 为空串即该信号），宁误报不漏报；
   流水侧取不到键跳过并计入 `SkippedNoKey`（无法定位销账对象，通常是规格配错，
   供运维排查配置）。
4. **豁免降级**：§4.1 豁免按账号（离线结算批处理账号）；C1 `account` 恒空（§3.3），
   故本批豁免为**关联键值精确豁免** `exemptKeys`。账号维度豁免待血缘数据源
   解决（含 pgaudit/MySQL general log 侧通道）后再恢复，面板配置面同步。
5. **容量边界**：单规则在窗条目上限 `maxPending`（默认 10000），超限按入窗
   FIFO 驱逐——debit 驱逐产出命中（`detail` 标注 `cap exceeded`），credit 驱逐
   静默；内存有界且不阻断采流。
6. **命中出口**：§7 的命中进告警页为本批之外——本批命中仅落 agent 日志
   （字段 `rule`/`severity`/`table`/`key`/`detail`/`gtid`/`tsUnixMs`）+
   `Stats.GateHits` 计数。服务端 outlet kind `capture.rule_hit` 闭集已就绪
   （`internal/platform/outlet`），告警上报 wire 消息是独立增量（§7 接线）。
7. **校验面**：规则随 `config.Load` 启动即全量校验（进程不起优于运行期错判）：
   表须 `库.表` 两段、同表不得跨规则复用（道具/流水角色路由必须无歧义）、
   `itemTable ≠ ledgerTable`、`severity ∈ {warn, critical}`、`windowSec`/`maxPending ≥ 0`。
   §6 的面板配置面尚未接线，本批仅文件配置。
8. **并发契约**：闸内状态无锁，依赖 runner 单 goroutine 消费循环
   （`Evaluate` 在事件分支、`Sweep` 在位点 tick 分支，同循环串行）。
9. **时间语义**：入窗时刻 = `max(事件 TsUnixMs, now)`——binlog 回放补扫（位点
   失效重扫）不会即刻超窗产生批量误报；到期判定 `now - ts >= windowSec`。

测试：`agents/capture/gate/reconcile_test.go` 13 例（双向匹配、超窗命中、窗口
边界、豁免、键缺失双侧降级、insert/update/delete 行像选取、回放补扫、容量驱逐、
多规则隔离、命中排序确定性、校验错误集）+ `runner_gate_test.go` 3 例（接线、
未配置直通、config 校验拦截）。

### 8.2 C2 交付记录：闸二·CEP（2026-10-11）

代码面：`agents/capture/gate/cep.go` 三模板（`cep.off_hours` / `cep.burst_delta`
/ `cep.probing`）+ `capture.gates.cep.{offHours,burst,probing}` 文件配置 +
runner 接线（事件到达即评即命中；位点 tick Sweep 只做清桶卫生）。与对账闸不同，
**同一表允许挂多实例**（时段 + 突变并存是常态），无角色路由唯一性约束。

三模板实现口径与差异：

1. **off_hours**：`windows` 为每日分钟偏移段（`startMin > endMin` = 跨零点，
   左闭右开）；`tzOffsetMin` 显式配置（0=UTC，东八区 480）——不引 IANA 时区库，
   避免 agent 二进制携带 tzdata 依赖。判定取**事件时间**（源真值）：位点失效
   重扫会对历史时段重复告警（重扫本身即异常路径，可接受）；命中 detail 带本地
   时钟与 tz 偏移，人读可核。
2. **burst_delta**：`mode=single` 判单事件 `|delta|`；`mode=window` 判滑动窗口
   累计 `|delta|`（双向突变都计入）。delta 语义：insert = 后像值（新增全额），
   delete = 前像值（清空全额），update = 后像 − 前像。**命中即清零再计时**
   （每窗一条命中，不逐事件刷屏）。累计桶按事件时间裁剪 + `maxEvents` 容量
   （默认 10000，FIFO 丢最老）。
3. **probing**：键降级为 **(行键, 表)**——设计原文 (账号, 表)，C1 `account`
   恒空（§3.3）。同一行反复小额改动即命中；大额变更打断该键序列（大额归
   burst 管）；命中即重置该键窗口（每键每窗一条）。`maxTracked` 键容量
   （默认 10000，FIFO 确定性驱逐）。
4. **数量字段**：`deltaField` 键形态同对账闸（整数串=位置下标）；不可数值化
   → 跳过并计数 `SkippedNoDelta`（规格配错信号）。float64 精度：整数金额
   < 2^53 安全，超限币量（>9e15）有精度损失——钱以分存储的游戏不受影响。
5. **百分比基线**（§4.2 提及）v1 不做：基线需历史分布来源（analytics/面板侧），
   事件流给不出基线值；绝对阈值先行，基线随面板配置面评估。
6. **时序边界**：CEP 命中依赖事件按序到达（binlog 保证单流有序）；agent 重启
   丢桶状态（§9 既有边界，probing/burst 窗口重新起算）。

测试：`agents/capture/gate/cep_test.go` 14 例（时段窗内/窗外/跨零点/时区偏移/
操作过滤/severity、单笔与累计突变、insert/delete 全额 delta、命中清零、越窗
裁剪、试探计数与重置、键隔离与大额打断、Sweep 清桶、键容量 FIFO 驱逐、校验
错误集、同表多实例）+ runner 接线 2 例。

### 8.3 C2 交付记录：闸三·血缘（2026-10-11）

代码面：`agents/capture/gate/lineage.go`（`lineage.whitelist_violation`）+
`capture.gates.lineage.rules` 文件配置 + runner 接线。实现口径：

1. **账号白名单**：`allowAccounts` 必填非空（至少含应用写入账号——白名单
   空的血缘规则=全部写入都告警，属配置错误，启动即拦）。
2. **C1 降级态（主差异）**：binlog/WAL 事件不携带会话账号，`account` 恒空
   （§3.3）——空账号事件**跳过并计数 `Degraded`**（不误报：无归因主体的
   告警是纯噪声；不放过：一旦源侧补上账号通道，本闸零改动生效）。闸启用
   时 runner 启动日志显式标注 `degraded: source has no session account`；
   面板「降级态」标注随面板配置面批。
3. **降级不等于关闸**：规则与校验面按有账号形态交付，事件带账号即按白名单
   判定——general log / audit 插件侧通道（C4 评估项）接入后无需改闸。
4. 操作过滤与表路由同 CEP（`ops` 空名单 = 全部变更；非订阅表直通且不计
   degraded——降级计数只反映真实评估量）。

测试：`agents/capture/gate/lineage_test.go` 5 例（白名单外命中、白名单内直通、
空账号降级计数、操作过滤与表路由不误计、校验错误集）。

### 8.4 C2 交付记录：告警上报链（2026-10-11）

代码面：wire 对 `MsgAlertEventRequest/Response = 0x040119/0x04011A`（Ops 族
下一个空闲对，奇数请求/偶数响应，`IsRequest`/`GetResponseMsgID` 契约满足）；
proto 消息对 `opsv1.CaptureRuleHitReport`（agent/game/env/rule/severity/table/
key/detail/gtid/event_op/event_ts_unix_ms）+ `CaptureRuleHitAck`（accepted/
duplicate）；agent 侧 `controlconn.SendAlertReport` + runner 异步上报循环
（`startHitSender`：`hitCh` 缓冲 512，非阻塞入队满即丢、单次发送 5s 超时、
失败丢弃不重试、连接跟随 Dial 重连换新）；server 侧 `ControlService.
handleAlertEvent`（`SetAlertModel`/`SetOutletManager` 注入，cmd/server/root.go
装配）。口径与边界：

1. **幂等键**：`AlertID = "capture." + sha256(agent|rule|gtid|table|key|ts)`
   前 16 hex——对 `model.Alert.AlertID`（`size:64` 唯一索引）安全；重连重放
   服务端折叠为 `duplicate`（回执体现，不重复落库）。
2. **Level 归一**：闸层 severity 闭集 `{warn, critical}`，映射
   `outlet.SeverityWarning/Critical`；未知值防御性按 warning 处理（不乱开
   new level）。
3. **game/env 只进 Details**：`Scope` 三元组随事件送 outlet（面板按
   scope 过滤），但 alerts 表身份不存 game 列（表为全公司单库）——与
   `alert.inbound` 同形态（scope 在 Details）。
4. **丢弃不背压**：上报全程 best effort——队列满（`AlertDropped++`）、连接
   未就绪、超时/失败一律丢弃+warn 日志；本地 slog warn 日志是持久记录
   （命中可核），上报链不重试、不排队、不阻塞消费循环。
5. **混版兼容**：旧 server 收到未知 msgID 返回 unknown message type 错误，
   agent 侧按发送失败丢弃（同上）；新 server + 旧 agent 不受影响（旧 agent
   不发该消息）。无配置开关——闸关则无命中无噪声，闸开即启用上报。
6. **server 双存储语义**：`alertModel` nil → 显式错误（同 taskStore）；
   `outletManager` nil → 只落库不分发（outlet 链未装配时降级）。

测试：server `internal/server/control_handler_alert_test.go` 7 例（落库+
幂等折叠、severity 映射与归一、校验拒绝、nil model 拒绝、occurredAt 缺省、
outlet 分发含 scope/dedup、msgID 对契约）；runner `runner_alert_test.go` 4 例
（上报载荷完整性、发送失败计数、duplicate 回执、队列满丢弃）。

### 8.5 C2 交付记录：闸规则下行拉取（面板配置面·wire 半边，2026-10-11）

代码面：wire 对 `MsgCaptureConfigRequest/Response = 0x04011B/0x04011C`
（Ops 族续对）；proto `opsv1.CaptureGatesConfigRequest`（agent_id）/
`CaptureGatesConfigResponse`（version/gates_json）；server
`ControlService.handleCaptureConfig`（`SetConfigVersionModel` +
`SetGameDBResolver` 注入，root.go 装配）；agent 侧 `controlconn.
PullCaptureGates` + runner 闸层热更（`gateSet` 快照整体 `atomic.Pointer`
切换，consume/sweep 单 goroutine 读指针，拉取 goroutine 写，免锁维持
§8.1 并发契约）+ `core/configsync.Puller` 版本轮询（30s 默认档）。
口径与差异：

1. **通道选型（拒绝 REST）**：sidecar 的 `GET /api/v1/agents/{id}/extensions`
   坐在 JWT Authority + `requireReadPermission` 后面，agents 无 JWT——
   下行走**已注册 TCP 会话上的 wire 拉取**（注册即认证，与告警上行同通道）。
2. **scope 取会话不取请求**：server 从 `AgentSession{GameID,Env}` 解析
   agent 归属（`req.agent_id` 只做会话寻址），config_versions 按
   `FindLatestByScope("capture.gates", gameID, env)` 读取——agent 无法
   越 scope 拉别人的规则；未注册会话/缺 game 直接错误回绝。
3. **gates_json 对 server 不透明**：server 只存转发（`config_versions`
   键 `capture.gates`、namespace=ops，无 schema 变更）；闸层结构由
   agent 在 apply 时全量校验（validator of last resort）——与配置文件
   同一校验函数集（`gate.Validate*Rules`），任一闸非法**整体拒绝**
   （keep last-good，不半换）。
4. **热更语义**：`buildGateSet` 先校验后构建、整体换指针；apply 失败版本
   游标不推进（下轮重试）；空 version（服务端未发布）= no-op 保持现状；
   连接未就绪按 Debug 静默（注册未完成/重连窗口是常态），真实读错误 warn。
5. **multiGame 分库**：server 侧经 `GameDBResolver`（结构性满足
   `*router.Router`，control 包不引 internal/db）按 (game,env) 解析 game 库
   并经 dbctx 注入——规则存在游戏方自己的库里；单库模式不注入即走默认连接。
   分库解析失败/规则读取失败统一回空 version（= agent 保持现状），拉取
   是周期轮询，单轮读异常不放大为通道错误。
6. **闸层快照化（行为不变）**：runner 三闸字段收敛为 `gateSet` 快照，
   启动仍=文件规则；面板发布即热替换——本批是 §6 下发通道的 agent 侧
   落地，面板编辑面（web 卡片 + configs API 写 config_versions）随后批。

测试：server `control_handler_config_test.go` 7 例（最新版本透传、未发布
回空、scope 隔离、未知 agent/nil model 拒绝、分库覆盖生效、分库解析失败
回空、msgID 对契约）；runner `runner_config_test.go` 3 例（热换快照、
非法规则 keep last-good、拉取器版本跳过/空载荷/错误/非法规则/新版本全路径）。

### 8.6 C2 交付记录：面板编辑面（下行链闭环，2026-10-11）

代码面：REST `GET/PUT /api/v1/ops/capture/gates`（ops 组，scoped +
Authority）读写 `config_versions` 键 `capture.gates`（namespace=ops，
`CreateWithMeta` 乐观锁递增）；服务端**仅结构 lint**（reconcile/cep/
lineage 三段对象 + 数组形状，lint 败 422 `errorx.CodeError`）；web
`/ops/capture` 页（JSON textarea v1 + 插入模板 + Popconfirm 发布 +
未发布/版本 Tag + 「发布≠生效」边界 Alert）+ `services/api/opsCapture`；
Ops/Nodes 新增「捕获」列（注册 labels `captureLineage=degraded` →
金色「血缘降级」徽标，复用 §8.3 既有 labels，无新 API）。口径与差异：

1. **服务端结构 lint / agent 语义校验分层不变**：server 只查形状
   （对象/数组），不解释规则内容——语义真值仍在 agent apply 全量校验
   （validator of last resort，§8.5 第 3 条口径延续）；面板发布成功
   ≠ agent 已生效，真值是 agent 日志 `capture gates version applied`。
2. **发布必须 game+env scope 齐全**：空 scope 的发布会被任何 agent
   拉到（FindLatestByScope 要求三元组非空）且无从归属，直接 422 拒绝。
3. **降级徽标零新 API**：血缘降级态走注册 labels 链路（OpsAgentInfo
   已暴露），面板列只做展示映射。
4. **编辑器 v1 是 JSON textarea**：结构化表单（模板 + 参数 + 测试）
   归 [风控面板设计](risk-control-panel-design.md) 阶段四；本文只交付
   下行链闭环所需的最小编辑面。
5. **appliedRevision 未上报**：面板无「待生效」态可见性，随 [CDC
   过滤策略体系](cdc-filter-policy-design.md) 阶段三补上行通道。

测试：ops `capture_gates_test.go` 4 例（递增版本+元信息、未发布回空、
scope 隔离、5 种 lint 败 + scope 缺失 + lint 败后不落库）；web 页 7 例
（已发布/未发布渲染、模板插入、发布链、发布失败、加载失败、重载）+
service 3 例（URL/method/参数形状）；Nodes 75 例回归绿。

### 8.7 交付记录：事件粗过滤接线（CDC 平台化批阶段二起步，2026-10-11）

代码面：新契约包 `agents/capture/filter`（`policy.go` 统一策略头 +
`match.go` 库表授权 + `event.go` EventFilter + `connector` 包 ChangeEvent/
SourceConnector/能力声明，阶段一）；runner 接线（`config.Gates` 新增
`event.ops` 段 → `gateSet.event` → consume 进闸前粗过滤）；Stats 新增
`FilteredNoMatch` / `FilteredError`；面板 lint 认识 `event.ops` 数组形状。
口径与差异：

1. **EventFilter v1 只做操作类型白名单**：库表范围归 SourceFilter（授权
   语义默认拒绝，见 [CDC 过滤策略体系](cdc-filter-policy-design.md)
   §1.1/§1.2）——事件过滤是放行语义（空白名单=全放行），两者语义不同
   不共用匹配器，防「exclude-only 配置被默认拒绝误伤」这类边界。
2. **缺省段 = 全放行**（向后兼容）：既有 capture.gates JSON 与文件配置
   无 `event` 段时 gateSet.event=nil，直通零开销，行为零变化。
3. **失败语义**：未列入白名单的合法操作 = FilteredNoMatch 丢弃；未知
   操作（形态不可评估）= FilteredError 丢弃——有计数可观测的丢弃，不
   静默（简档 §5）；闸层评估失败保留事件的红线不变（gate 评估当前无
   错误路径，错误计数随闸层演进补）。
4. **原子切换免费获得**：event filter 挂在 gateSet 快照内，§8.5 的整体
   热换/keep last-good 语义对过滤层同样生效，无新增并发面。

测试：filter 包 3 组（策略头/授权匹配/事件过滤）+ connector 包 7 组；
runner 4 例（接线与计数、未知操作 Error 计数、非法段 keep last-good、
buildGateSet 闭集拒绝）；ops lint 2 例新增（event 非法形状 422、合法
放行）。

### 8.8 交付记录：appliedRevision 上报链（CDC 平台化批阶段三首项，2026-10-11）

代码面：runner 新增 `appliedGatesRevision`（atomic.Pointer[string]，拉取器
`WithOnApply` 回调写入）+ `reg`（register.Client 句柄，Run 装配）；生效后
异步 `SyncOnce` 重注册，`buildPayload` 以 label `captureGatesRevision` 捎带
生效版本——server session labels 经既有 register/UpsertAgent 全量覆盖刷新，
零 server 改动、零 proto 变更；面板 `/ops/capture` 新增「生效对账」行
（`listOpsNodes` labels 对账 desired vs applied：全生效绿 / 部分生效金
tag+tooltip 点名 / 无 agent 灰）。口径与差异：

1. **通道选型（复用注册面，不新增 wire 对）**：appliedRevision 是低频
   低速率状态（每次发布生效一次），注册 labels 面已有承载（captureLineage
   先例）；心跳 proto 无 labels 且扩展 HeartbeatRequest 面更大——重注册
   即刷新，语义等价且实现面最小。
2. **尽力而为不回滚**：重注册失败不回滚已生效规则（规则热替换已完成），
   下次心跳断连重注册自然补报；面板对账允许滞后（30s 拉取 + 重注册），
   不作强一致读。
3. **生效失败不推进**：apply 失败（非法规则 keep last-good）时 onApply
   不触发，appliedGatesRevision 保持上一生效值——面板不会把未生效版本
   显示成已生效。
4. **空语义**：未生效过任何面板版本时不带键（面板侧 = 文件规则；对账行
   只在 desired>0 时渲染）。

测试：runner_revision_test 3 例（label 跟随拉取推进/生效失败保持上一
生效值/fetch 失败不记录 + reg 缺失容忍）；web 页 4 例新增（全生效/
部分生效/对账失败退化/未发布不渲染对账行）。

### 8.9 交付记录：回滚（历史版本重发一版，CDC 平台化批阶段三，2026-10-11）

代码面：REST `POST /api/v1/ops/capture/gates/rollback`（body
`{version, message?}`）——`FindByScope` 取目标版本内容 → 重新过当前
结构 lint → `publishCaptureGates` 落新版本（message 自动带
`rollback from version N` 前缀 + 可选原因）；PUT 与回滚共用发布路径
（lint + scope 检查 + CreateWithMeta 一处收敛）。口径与差异：

1. **版本日志不可变**：回滚不改写历史、不删版本，目标版本内容复制为
   新版本号发布——agent 侧照常 30s 版本轮询热生效，不引入第三种下行
   形态（无「回滚指令」wire 对）。
2. **历史内容重新过当前 lint**：lint 规则随服务端演进，回退到会被当前
   规则拒绝的内容显式 422，不静默发布坏版本（存量坏行直插场景单测
   锁定：失败回滚不落新版本）。
3. **scope 隔离照旧**：跨 game/env 回滚 = 404（Find 不到即不存在，不
   泄漏其他 scope 的版本存在性）。
4. **回滚当前版本合法**：重发同内容 = 显式重新生效（版本继续递增，
   生效对账行随之走完 生效→重报 链）。

测试：ops `capture_gates_test.go` 3 例新增（历史内容重发 + message
前缀 + 历史完好 + 回滚当前版本递增；版本非法 422 / 不存在 404 / 跨
scope 404 / 缺 scope 422；lint 演进拒绝 + 失败不落库）。边界：面板侧
回滚按钮未做（归风控策略页，阶段四；REST 已就绪）。

## 9. 已知边界（诚实清单）

- 血缘账号两侧均不可得（§3.3）：MySQL binlog 与 PG WAL 事件都不携带会话账号，account 字段 C1 恒空，默认降级运行并有面板标注。
- 对账闸要求流水表同库可订阅；跨库流水（分库分表）C3 不承诺，需按游戏方拓扑定制路由。
- binlog/WAL 保留期短于 capture-agent 停机时长时位点失效，从可得起点重扫（对账闸窗口兜底误报）；监控位点滞后并告警。
- 闸层评估为尽力而为（agent 侧内存态）：agent 重启丢失滑窗状态，血缘/对账不受影响（纯逐事件）。
- 前像依赖 `binlog_row_image=FULL` / `REPLICA IDENTITY FULL`，未满足时 update 前像为空，对账/突变按后像降级。
- 告警上报 best effort：队列满/断连/超时/发送失败即丢（不重试不排队），alerts 表可能少条；命中真相在 agent `slog warn` 日志（`capture gate hit`），可对账核对，丢弃计数见 `Stats.AlertDropped`。
- 告警上报对旧 server 静默降级：server 不认识 msgID → 发送失败丢弃，agent 不告警不重试（混版部署期无上报，仅本地日志）。
- C1 行像为位置数组 JSON（binlog ROW 事件裸形态，无列名映射）——闸层血缘/对账可按位置对齐，但面板摘要可读性受限；列名映射（information_schema 摄取）随 C2。
