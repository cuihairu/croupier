# 分析中心外部数据插件化可行性调研（oddsmaker 为例）

状态：**调研汇报，待拍板，未动任何代码**。

## 问题

能否让分析中心接入外部数据（以 oddsmaker 为例），并做成「插件机制」？即：外部数据源的注册、权限、隔离、前端挂载点是否值得抽象成通用插件框架。

## 结论（先说答案）

**不建议新造「外部数据源插件」通用机制。** oddsmaker 这类外部数据源，用仓库既有机制即可完整接入，分两步走：

1. **起步（connector 即可）**：agent 侧 openapi provider 把 oddsmaker 注册为函数 + 页面工作室报表卡引用，零新机制，1-2 天量级；
2. **升级（若赔率域要长成产品能力）**：按仓库既定的 official.* 扩展域模式做 `official.odds`（对齐 approval/alerting/backup/notification 四个已插件化域的先例，以及 core-extension-mapping.md 已列明的 official.analytics 拆分方向）。

「通用外部数据源插件框架」不建议做：扩展机制本身就是插件机制（不是缺能力），而它最难的部分——动态前端挂载——已被仓库「UI 收敛为纯 spec + 受控 widget」的架构决策关闭，为单个数据源预付这层架构成本不成立。

## 仓库现状事实（调研依据）

### 分析中心数据面已经是「管道分层」的

- **三条既有数据链路**：① 服务端直写 MySQL（BehaviorModel/PaymentsModel，按 game 分库）；② 独立 ingest 网关（HMAC-SHA256 签名、per-game secret、时间戳校验、限流、Redis 去重）→ MQ（Redis Stream / Kafka）→ analytics-worker 批量入 ClickHouse；③ audit_records 聚合（调用分析）。
- **外部数据接入的「标准姿势」已经存在**：游戏方上报就是 ingest 网关——oddsmaker 若能推数据，走同一网关加一类事件即可；若只能拉数据，worker 侧加一个拉取 consumer，或 agent 侧 openapi provider 注册为函数。
- 前端 8 个视图全部是 REST `/api/v1/analytics/*` + spec 驱动渲染，没有任何视图依赖「插件式 UI」。

### 「插件机制」其实已经存在，且 analytics 已半插件化

- 扩展四表模型（catalog/releases/installations/runtime_bindings）+ 完整安装/配置/健康检查/启停 API + ScopeType（global/game/env）安装级隔离 + 前端 DomainEntry 域入口挂载，全部在产。
- **approval / alerting / backup / notification 四个域已完成插件化**：API 服务读扩展安装实例的 ConfigJSON 作为配置源，前端 DomainEntry 挂载，命名/权限/文档受 `official-extension-unified-pattern.md` 统一约束。
- **analytics 本身在既定迁移清单上**：`core-extension-mapping.md` 已把 `internal/api/analytics → official.analytics` 列为优先拆分方向，且 `internal/core/extension/runtime/service.go` 已有 official.analytics 的 page/capability/function bindings 雏形；采样过滤配置（analytics.filters）已持久化在扩展安装实例的 ConfigJSON 里——「数据已在扩展模型、UI 还在核心」的半插件化状态。
- agent 侧另有 providers.yaml + openapi provider（外部 REST → 函数注册，examples/openapi-provider 有完整示例），这是「外部系统 connector」的现成通道。

### 动态前端挂载点：架构上已关闭

- CLAUDE.md 所称「Function packs (.tgz) 携带 UI plugins 远程加载」经审计（docs/research/registration-ui-pipeline-audit-2026-09.md）确认**不存在于代码**；自定义 UI 已收敛为纯 PageSpec + 受控 widget 枚举。
- 这意味着「外部数据源插件自带 UI」没有合法实现路径——任何新视图都必须走 PageSpec/受控 widget，而不是插件注入任意组件。通用插件框架最值钱的「自由挂载」恰恰是被有意禁止的形态。

## oddsmaker 三条路径对比

> oddsmaker 在全仓无任何痕迹（代码/配置/文档/git 历史均无，仅有的 grep 命中是误提交的构建缓存二进制）。以下是假设其为外部赔率/盘口数据系统的方案对比。

### 路径 A：标准 connector（推荐起步）

```
oddsmaker API ──→ agent providers.yaml (type: openapi)
                    └─ 注册 oddsmaker.* 函数（函数目录可见、RBAC 生效）
页面工作室报表卡/分析视图 ──引用──→ oddsmaker.* 函数（PageSpec 数据源）
（若需入库留存：oddsmaker → ingest 网关加事件类 → ClickHouse 新表）
```

- **数据源注册**：providers.yaml 声明 base_url + OpenAPI spec，operationId 自动注册为函数；或 ingest 网关事件类。
- **权限**：函数目录既有 RBAC；ingest 走 per-game HMAC secret。
- **隔离**：agent provider 自带 scope 声明；ingest 事件按 game_id/env 天然隔离。
- **前端挂载点**：页面工作室配置报表卡引用函数——纯 spec，无新机制。
- 复杂度：低（配置 + 一张 ClickHouse 表可选）；收益：快速验证 oddsmaker 数据的业务价值。
- 局限：没有独立视图/独立权限域/安装生命周期——数据只是「另一个函数的数据源」。

### 路径 B：official.odds 扩展域（若升产品能力）

对齐 official.* 统一模式与四个已完成先例：

- **注册**：extension_catalogs + releases + installations（ScopeType 隔离安装到 global/game/env），ConfigJSON 存 oddsmaker 接入配置（endpoint/密钥引用走 SecretRefsJSON 分离）。
- **权限**：`odds.read/operate/admin` 三层（点号命名，对齐 unified pattern），runtime bindings 声明 required_permission。
- **隔离**：安装级 ScopeType + 数据面 game_id/env 列；注意扩展路由挂 protected 组（不经过 GameDBMiddleware），数据面查询需自行走 dbctx.Resolve。
- **前端挂载点**：runtime bindings 生成 page binding（route/icon/order/required_permission），DomainEntry 加一行域映射——四域先例同款。
- **数据面**：ingest 网关事件类或独立拉取 worker → ClickHouse `odds.*` 表；分析中心多一个「赔率」视图（official.analytics 雏形 bindings 已证明 page binding 指向 /analytics/* 路由可行）。
- 复杂度：中——全是「按先例填空」（四域模板 + official.analytics 雏形），无机制发明；收益：独立生命周期、权限域、配置管理、可拆卸。
- 前提：oddsmaker 需求被证实为长期能力（多个视图/多处引用/独立运维诉求），而非一次性看数。

### 路径 C：通用「外部数据源插件」框架（不建议）

需要新造的东西：数据源类型注册表、schema 映射层（任意源字段 → analytics 聚合语义）、动态视图生成、（被架构关闭的）动态 UI 挂载、动态权限点。

- **复杂度**：高。难点不在「注册一个源」，在语义映射——oddsmaker 的赔率快照（事件驱动、分钟级抖动、概率语义）与分析中心的聚合语义（DAU/留存/漏斗的时间窗聚合）不同构，映射层等于给每种源手写一个 ETL DSL；UI 侧与「受控 widget」决策直接冲突。
- **收益**：仅当预期有 N≥3 个异构外部源频繁接入时才成立。当前只有一个假设性需求（oddsmaker），且全仓无任何第二例迹象。
- **机会成本**：analytics 核心本身迁 official.analytics（既定方向）都比这个框架更接近用户价值。

## 决策建议

| 判断                         | 选择                                                                                         |
| ---------------------------- | -------------------------------------------------------------------------------------------- |
| 现在就要 oddsmaker 数据      | 路径 A（connector），验证数据价值                                                            |
| A 跑通后确认是长期能力       | 升路径 B（official.odds），按四域先例填空                                                    |
| 顺手把「外部源插件框架」做了 | 不做。等 official.analytics 迁移落地后复评，届时「分析域插件化」的样本才有两个，抽象才有依据 |

一句话：**插件机制不缺（扩展体系就是），缺的只是 oddsmaker 的接入本身；先 connector 后扩展域，通用框架等第二个真实需求再议。**

## 已知边界

- 本文不改变任何现有行为；official.analytics 迁移与路径 A/B 的实施均需另行排期与拍板。
- 「odds.read 点号权限」与前端 access 冒号命名（analytics:read）的双轨现状在扩展域内遵循 unified pattern，不做全局统一（该治理属另一议题）。
