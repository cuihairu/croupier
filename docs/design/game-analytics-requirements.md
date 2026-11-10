# 游戏数据分析横向需求分析（MOB-ANA-001）

- Date: 2026-11-11. Status: **analysis delivered with this document**; facade landed (`internal/api/analytics/facade.go`), mobile dashboard landed (`mobile/lib/features/analytics/`). Missing data sources are declared honestly, not faked.
- Scope: 游戏行业常见横向分析域逐域盘点——现在有没有、缺什么、优先级；同时落 analytics 门面（facade）供移动端一次拉多域摘要，避免后续加需求翻新查询层。

## 1. 用户三问与本档回答

1. **移动端要支持审批和数据分析**——审批移动端闭环（批准/拒绝/OTP step-up/409 竞态）已交付并有测试（`mobile/lib/features/approvals/`，`approval_detail_page_test.dart` 覆盖 otp_required→弹码→重批与 otp_invalid 纠错重试）；本档补数据分析移动端（facade 摘要 + 分域骨架）。
2. **数据分析够不够**——§3 逐域结论表。
3. **预留门面**——§4：facade 统一查询入口 + 每域独立接口，新域只加实现不改门面。

## 2. 服务端 analytics 现状基线（逐条核实）

来源：`internal/api/analytics/`（handler.go/dto.go/各域文件）+ `internal/handler/routes_alert.go` registerAnalyticsRoutes（挂 `/api/v1/analytics`，scoped 组=JWT+游戏/环境头）：

| 已有面                                                                   | 内容                                                                 |
| ------------------------------------------------------------------------ | -------------------------------------------------------------------- |
| `/overview`                                                              | DAU/MAU/newUsers/revenue/ARPU/ARPPU/payingRate + trends              |
| `/invocations{,/trend,/summary}`                                         | audit_records 聚合（调用次数/失败/成功率/耗时 P95/Top 函数）         |
| `/warehouse/{dau,online,revenue}`                                        | ClickHouse 聚合表（未配 DSN 如实 503）                               |
| `/realtime{,/series}`                                                    | 瞬时 onlineUsers/activeSessions + 时间序列                           |
| `/ingest` + `/filters`                                                   | 事件入站 + 过滤器持久化                                              |
| `/behavior{,/events,/paths}` + `POST /funnel` + `/adoption{,/breakdown}` | topActions/userFlows/heatMap + 事件明细/路径/漏斗/功能采用           |
| `/payments{,/summary,/product-trend,/transactions}` + `POST /ingest`     | 收入/交易/付费用户 + 商品趋势                                        |
| `/retention`                                                             | 队列留存（cohort → 多日留存率数组）                                  |
| `/levels{,/episodes,/maps}`                                              | 关卡 attempts/completions/completionRate/avgDuration/avgRetries      |
| pipeline_monitor                                                         | 数据管道健康后台监控（ClickHouse/Redis 窗口对比→告警中心，非查询域） |

## 3. 横向需求逐域结论表（✅ 够用 / ⚠️ 有但需补 / ❌ 缺）

| 域       | 游戏业横向标准                               | 现状核实                                                                                  | 结论           | 缺口优先级                                                        |
| -------- | -------------------------------------------- | ----------------------------------------------------------------------------------------- | -------------- | ----------------------------------------------------------------- |
| 宏观大盘 | DAU/WAU/MAU、新增、留存、在线峰值            | overview 有 DAU/MAU/新增；retention 有队列留存；realtime 有瞬时在线；warehouse 有在线序列 | ⚠️             | P2：WAU 口径、在线峰值（窗口内最大而非瞬时）                      |
| 玩家行为 | 路径漏斗、功能热图、会话时长分布             | behavior 有 paths/funnel/heatMap/adoption                                                 | ⚠️             | P2：会话时长分布（当前事件流无会话聚合口径）                      |
| 经济系统 | 通货产出/消耗流动、通胀指标、商城转化        | payments 只覆盖真实资金流，游戏内通货无 sink/source 模型                                  | ❌             | **P1**：经济事件入站 + 通货流动聚合（产出/消耗/库存水位）         |
| 游戏进度 | 关卡通过率、卡点、任务完成漏斗               | levels/episodes/maps 有通过率/卡点(avgRetries)/时长                                       | ✅（关卡维度） | P2：任务(quest)完成漏斗复用 behavior funnel 即可起步              |
| 社交     | 聊天量、好友、公会活跃                       | 无                                                                                        | ❌             | P2：依赖游戏侧事件上报约定                                        |
| 商业化   | ARPU/ARPPU/LTV、首充转化、礼包分析、支付漏斗 | overview 有 ARPU/ARPPU/付费率；payments 有收入/交易/付费用户/商品趋势                     | ⚠️             | **P1**：LTV（队列收入追踪）、首充转化；礼包分析随商品维度已有雏形 |
| 稳定性   | 崩溃率、卡顿、客户端错误聚合（按版本/机型）  | pipeline_monitor 是数据管道健康，非客户端稳定性；客户端崩溃无采集通道                     | ❌             | P2：需客户端 SDK 上报约定（跨端改造，非纯服务端）                 |
| 反作弊   | 异常行为标记（速度/资源突变）                | behavior events 有原始流；无规则/标记                                                     | ❌             | P2：先约定异常事件 schema，再谈规则引擎                           |
| 运营活动 | 活动参与率、奖励发放量、活动 ROI             | 无                                                                                        | ❌             | P2：活动事件约定 + 参与漏斗                                       |
| 实时监控 | 实时在线、实时进出                           | realtime + series + pipeline_monitor→告警中心                                             | ✅             | —                                                                 |

优先级原则：**P1 = 直接影响运营决策且数据侧可自洽落地**（经济、LTV/首充——都只依赖现有 ingest 通道扩展事件类型）；P2 = 依赖游戏侧上报约定扩展或口径定义（WAU/峰值、会话分布、任务漏斗、社交、稳定性、反作弊、活动）。

## 4. 门面设计（本档同步落代码）

- `internal/api/analytics/facade.go`：
  - 每域独立查询接口（`OverviewQueryer`/`BehaviorQueryer`/`PaymentsQueryer`/`RetentionQueryer`/`RealtimeQueryer`/`InvocationsQueryer`/`LevelsQueryer`），`*Service` 隐式实现，facade 只依赖接口不依赖实现——新域加实现+注册，不改门面。
  - `FacadeSummary(gameId, env)`：一次返回全部域摘要卡片；**逐域错误隔离**（单域失败不影响其它域，失败域标 `unavailable`+原因）；未建数据源的域声明 `pending_data_source`（economy/social/stability/anticheat/campaign），不伪造数字。
  - `AnalyticsQuery{domain, metrics, filter, timeRange}`：统一查询路由 `FacadeQuery`，按 domain 分发到对应域接口；未知域 422、未建域 422（错误码 `domain_pending`）。
  - 路由（加法，不动现有 handler）：`GET /analytics/facade/summary`、`POST /analytics/facade/query`（同一 scoped 组）。
- 移动端 `mobile/lib/features/analytics/`：仿 approvals 模式（service 解析 fail-fast / riverpod controller / dashboard 页 + 分域详情骨架）；入口挂监控大盘（设备/告警/审计同款入口模式）。

## 5. 不做什么（边界诚实）

- 不在本档实现任何 ❌ 域的数据模型——先有游戏侧事件上报约定，再建模（防假数据门面）。
- 不重构现有分域 handler/路由——facade 是聚合加法。
- 移动端不做图表渲染框架引入——摘要卡片用数字+趋势箭头表达，复杂图表归 Web 端。
- pipeline_monitor 是管道健康监控不是分析查询域，不进 facade 域列表。

## 6. 验证计划

- facade 单测：mock 各域接口 → 聚合合并、逐域错误隔离（单域 fail 其余 ok）、未知域/未建域 422、路由分发正确。
- 移动端：`flutter analyze` 0 错；dashboard 页 widget 测试（mock service：加载/空态/错误态/摘要卡渲染）；既有 approvals OTP 用例回归。
- 真机联调待环境（无部署后端时 mobile 用 mock 流）。
