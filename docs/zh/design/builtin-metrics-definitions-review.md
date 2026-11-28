---
title: 内置核心指标定义方案评审
---

# 内置核心指标定义方案评审

日期：2026-11-28。状态：**提案待 owner 决策**。本页是 DATA-001 方案决策交付物。
产品边界已由 owner 划定（2026-10-29，[core-and-external](../analytics/core-and-external.md)）：
croupier 内置可复用的**核心游戏运营指标**；专业分析走外部系统集成。悬而未决的是
**定义**——本页按指标组逐项给出定义提案，以当前代码为地基。本页不实施、不验收
任何分析能力；DATA-002–008 落地计算之前，此处一切定义仅是纸面（`口径未验收不称已可用`）。

## 1. 现状代码事实（grounding，2026-11-28）

| 面                                                                                                                 | 形态                                                                               | 对定义的约束                                                                        |
| ------------------------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------- |
| `analytics.events`（CH，MergeTree）                                                                                | `user_id` 扁平 String、`event_id UUID`、`session_id`、`event_time` UTC、TTL 6 个月 | 事件流中不存在账号/角色区分；身份必须由定义钉死                                     |
| `analytics.payments`（CH，MergeTree）                                                                              | `order_id` String、`amount_cents UInt64`、有 `currency`、`status`                  | 表级订单去重不存在——重放即重复行；去重必须在查询/汇总粒度上定义                     |
| `analytics.daily_users` / `daily_revenue` / `minute_online`（ReplacingMergeTree(version)，键 `game_id, env, d/m`） | worker 产出汇总                                                                    | `daily_revenue` 键缺 `currency`（DATA-004）——定义必须要求 currency 进粒度           |
| `retention_cohorts`（关系模型）+ `UpsertCohort`                                                                    | 模型在；未找到生产计算调用                                                         | cohort 形态在；DATA-002 接生产者之前由定义钉死语义                                  |
| warehouse 读端点（`internal/api/analytics/warehouse.go`）                                                          | DAU/新增、分钟在线、收入查询；缺 DSN 即禁用                                        | 读形态在；聚合语义继承本页定义                                                      |
| 平台 `Player` 模型                                                                                                 | 账号级（username/nickname/game_id/status），无 role/server 列                      | 平台身份=账号；工单/cohort 模型中的 `ServerID` 是游戏侧的，只能当维度、不能当身份键 |

## 2. 定义（按指标组提案）

### D1 — 身份键

`user_id` = 平台 **Player ID**（账号级），强制 `(game_id, env)` 作用域。
角色/人物与区服是**维度、绝不是身份键**——一个账号在两个区服活跃仍是一个 DAU。
无有效 `user_id` 的事件不进活跃指标，计入数据健康（§D9），绝不静默丢弃。

### D2 — 活跃事件集、DAU/WAU/MAU

- **活跃日** = 该业务日内 ≥1 条带有效 `user_id` 的事件（登录、心跳、玩家行为
  事件都算；系统来源事件经显式的玩家事件名 allowlist 排除，不做 props 字段猜测）。
- **DAU** = 每业务日 distinct `user_id`。**WAU/MAU** = 滚动近 7 / 28 天 distinct
  （不用 ISO 自然周——滚动窗口贴合运营用法，避开周边界伪影）。
- 近似策略：看板查询可用 HLL（`uniq`）并声明误差；审计级导出用精确
  （`uniqExact`）。所用模式随查询响应记录。

### D3 — 新增

`(game_id, env, user_id)` 的首个活跃日——**不是** `Player` 行创建时间。Player
CRUD 是平台注册，明确**不是**权威游戏注册（[core-and-external](../analytics/core-and-external.md)
§1 已确立）。需要 first-seen 水位线；数据缺口后的首条事件是否真首见取决于水位线
覆盖该缺口——水位线属于 DATA-002 的计算契约。

### D4 — 业务日与时区

- 按 `(game_id, env)` 配置**业务时区**，缺省 UTC；一切日级聚合按业务当地日分桶；
  存储保持 UTC（`event_time`）。任何指标不混时区；每个聚合响应随附生效时区。

### D5 — 近实时在线

- 心跳制：用户在分钟 `m` **在线** = 在 `m` 的后溯窗口 `W = 90s` 内发过心跳。
  分钟桶 = `event_time` 截断；日峰值 = 分钟值最大。
- 语义声明为**近似并发用户（HLL）**——绝不表述为精确连接数。`W` 按游戏配置门控。

### D6 — 留存（D1/D7/D30）

- **Cohort 日** = 首个活跃日（与 D3 对齐）。**回流** = cohort 日 + N 当天活跃。
  **分母** = cohort 规模（当日首见 distinct 用户）。成熟 cohort 的成员一经写入不可变。
- **成熟窗口**：D-N cohort 仅在业务日历过去 N 天后可报；未成熟 cohort 报
  `immature`——**绝不报 0%**（0% 展示是伪造流失）。
- 多区服（MMORPG）cohort 可加 `ServerID` 可选维度；平台级 cohort 保持账号粒度。

### D7 — 付费（付费人数/订单/流水/退款/净额/付费率/ARPU/ARPPU）

- **订单粒度**：`(game_id, env, order_id)` 去重、首条记录胜出（表级重放重复存在
  ——去重规则属于汇总/查询契约；存储修复归 DATA-004）。
- **状态闭集（v1）**：`paid`、`refunded`。**流水（gross）** = Σ paid 金额；
  **退款** = Σ refunded 金额、**记入退款事件自身的业务日**（v1 不回溯改写购买日）；
  **净额** = gross − refunds，同日粒度。
- **付费人数** = 窗口内 ≥1 笔 `paid` 订单的 distinct 用户；**付费率** = 付费人数 /
  DAU（同窗口）；**ARPU** = gross / DAU；**ARPPU** = gross / 付费人数。一切比率随附分母。
- **单位**：整数最小单位 + 强制币种码。**不做跨币种加总**——聚合按币种分行；
  折算报告币种是带汇率快照的显式后续工作（非 v1）。`daily_revenue` 键必须带
  currency（与 DATA-004 对齐）。

### D8 — 迟到与更正

- 按 `(game_id, env)` **水位线**：日级活跃/收入聚合在成熟期（`T+3` 业务日缺省，
  配置门控）内重算；retention cohort 在其窗口成熟前重算（D6）。
- 成熟后的更正产生**带版本的 restatement**（ReplacingMergeTree `version` 列已支持），
  且 restatement 在数据健康面可见——绝不静默改写。

### D9 — 缺测 vs 真零

- 无 ingest 数据 ≠ 零：每个聚合响应区分「窗口无数据」与「真实为零」（数据健康组
  ——ingest 滞后、积压、死信——是伴生面，复用 `pipeline_monitor`）。
- 源表不可用的指标报**不可用**，绝不回退到语义不同的表（承
  [core-and-external](../analytics/core-and-external.md) 约束）。

## 3. 边界确认（owner 2026-10-29 已定，重述）

内置范围 = 上述六组（获客/活跃、在线、留存、付费、共享探索、数据健康）。复杂
自助分析、漏斗/路径、归因、预测 LTV、专业分群 = 外部专业分析（DATA-010 调查；
DATA-011 契约）。既有行为/关卡页保留并带诚实边界。本页不选供应商、不接集成。

## 4. 验收映射（定义 → DATA-002–008 的验证场景）

| #   | 场景                              | 期望                                         |
| --- | --------------------------------- | -------------------------------------------- |
| A1  | 同日重复事件重放（同 `event_id`） | DAU 该用户只计一次；ingest 健康记录重复      |
| A2  | 账号同日在两个区服活跃            | 一个 DAU；按区服拆分只作为维度出现           |
| A3  | UTC+8 游戏 23:59 UTC 的事件       | 计入下一业务日                               |
| A4  | 用户有史以来首条事件              | 当日计 `new_users`                           |
| A5  | cohort 日后 5 天查 D7 cohort      | 报 `immature`；同日 D1 cohort 报真实值       |
| A6  | 订单重放 / 重复 `order_id`        | 只计一次；gross 不变                         |
| A7  | 购买 3 天后退款                   | 购买日 gross 不变；退款日净额减少            |
| A8  | 同日两币种                        | 两条按币种行；无显式换算不出加总             |
| A9  | ingest 中断 2 小时                | 受影响日缺口标「无数据」而非零；健康面见滞后 |
| A10 | 成熟后 restatement                | 新版本行；旧值在健康/审计轨迹可见            |

## 5. 待 owner 决策

- [ ] **D1** 身份——批准账号级 `user_id`（Player ID），角色/区服仅作维度
- [ ] **D2** 活跃——批准任意玩家事件活跃集 + 滚动 WAU/MAU + HLL 带声明误差缺省
- [ ] **D3** 新增——批准首活跃日（非 Player CRUD）
- [ ] **D4** 时区——批准按 (game,env) 业务时区、缺省 UTC
- [ ] **D5** 在线——批准心跳窗 90s + 近似语义
- [ ] **D6** 留存——批准 cohort=首活跃日、`immature` 非 0%
- [ ] **D7** 付费——批准订单去重粒度、退款日记账、按币种不加总
- [ ] **D8** 更正——批准 T+3 成熟期 + 带版本 restatement
- [ ] 验收映射 §4（A1–A10）——确认其为 DATA-002–008 的验证基准
