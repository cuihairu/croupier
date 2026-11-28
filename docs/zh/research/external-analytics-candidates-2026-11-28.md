# 外部专业分析：候选产品矩阵调查

日期：2026-11-28。DATA-010（资料调查）交付物。专业需求基线即 owner 划定的
边界（2026-10-29，[core-and-external](../analytics/core-and-external.md)）：
复杂自助查询/下钻、高级漏斗/路径、归因/ROI、预测 LTV、专业分群归**外部**
专业分析。本页收集候选产品的官方核实事实形成对接矩阵。**本页不选首接目标、
不宣称任何供应商集成已完成**——选型、验收环境与集成契约（DATA-011）保持
owner 决策。

事实收集方法：2026-11-28 抓取官方产品页与 GitHub 仓库。本轮无法从官方页
确认的单元格标 **NOT VERIFIED**——选型后须对所选产品逐项复核再进契约工作。

## 1. 候选矩阵

| 维度                                | Metabase                                                                                                              | Apache Superset                                                            | Grafana OSS                                                                                                                  | PostHog                                                                                                       |
| ----------------------------------- | --------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------- |
| 版本形态                            | Open Source（自托管 Docker/JAR）；Starter；Pro；Enterprise；云或自托管                                                | Apache 项目，自托管                                                        | OSS（自管）；Cloud（托管，免费档）；Enterprise（自管）                                                                       | 云（免费档）；自托管仓库；`posthog-foss` 纯净仓库                                                             |
| 许可                                | OSS 版免费；高级特性（SAML/JWT/OIDC SSO、行/列级安全）锁 Pro/Enterprise——**本轮未核到具体 OSS 许可文本 NOT VERIFIED** | Apache License 2.0（站点页脚）                                             | **AGPL-3.0-only** 含 Apache-2.0 例外（README）                                                                               | MIT expat，`ee/` 目录除外（独立许可）（README）                                                               |
| 数据输入                            | 列示 24+ 数据源含 **ClickHouse**、PostgreSQL、MySQL、BigQuery、Snowflake…；API/CLI/MCP 编程访问                       | 任何 SQL 方言库；列示画廊含 **ClickHouse**、PostgreSQL、MySQL 等；REST API | 150+ 插件；ClickHouse 插件**本轮未核 NOT VERIFIED**（grafana.com 插件画廊广泛列示）                                          | 事件摄取（SDK/autocapture/手工埋点）+ warehouse + 数据管道（README）；capture API **本轮未核 NOT VERIFIED**   |
| 需求契合（漏斗/路径/归因/LTV/分群） | SQL 优先自助；无原生产品分析漏斗/归因模型                                                                             | SQL 优先自助；无原生产品分析模型                                           | 观测看板；非产品分析工具                                                                                                     | 产品分析原生：事件分析、群组分析、session replay、feature flags/实验；漏斗/留存见文档（README 级：分析+指南） |
| 鉴权/SSO                            | JWT/SAML SSO + OIDC 仅 **Pro/Enterprise**；OSS 基础鉴权（官方页）                                                     | **本轮未核 NOT VERIFIED**                                                  | OSS：OAuth（GitHub/Google/GitLab/Generic…）、LDAP、Auth Proxy、JWT proxy；**SAML/team-sync 仅 Enterprise/Cloud**（官方文档） | **本轮未核 NOT VERIFIED**                                                                                     |
| 嵌入/深链                           | 产品内嵌看板/图表；多租户鉴权、行级权限、白标；React SDK；官方页确认                                                  | **本轮未核 NOT VERIFIED**                                                  | 看板分享/嵌入**本轮未核 NOT VERIFIED**                                                                                       | 可分享看板**本轮未核 NOT VERIFIED**                                                                           |
| 数据回流（回 croupier）             | 查询/API 导出；无原生回推                                                                                             | REST API 导出                                                              | API 导出；数据管道**未核**                                                                                                   | 数据管道/warehouse 导出存在（README）；契约形态**未核**                                                       |
| 成本模型                            | 分档（Starter/Pro/Enterprise），云或自托管；抓取页无金额                                                              | 免费（Apache），仅自托管基建成本                                           | OSS 免费；Cloud 免费档（10k 指标/50GB 日志…）；Enterprise 授权                                                               | 云免费档；超出用量计费（细节未核）                                                                            |
| 对 croupier 的部署注记              | 直读 ClickHouse——与既有 `analytics.*` 仓库契合                                                                        | 直读 ClickHouse——与既有仓库契合                                            | 经插件读 ClickHouse——观测形态                                                                                                | **自行摄取事件**——第二条并行摄取路径，非 ClickHouse 读取器                                                    |

## 2. 核实事实要点（2026-11-28）

- **Grafana 许可**为 AGPL-3.0-only（Apache-2.0 例外）——若未来 vendor 任何
  Grafana 源码需考虑 copyleft（作为外部服务使用则不受影响）。
- **PostHog** 为 MIT 但 `ee/` 例外；`posthog-foss` 仓库剔除专有特性——
  有干净许可要求自托管时相关。
- **Superset** 为 Apache-2.0（四者许可最干净）且原生直读 ClickHouse。
- **Metabase** 的 SSO 与行/列级安全是付费档特性——仅免费版给不出 DATA-011
  契约假设的 SSO scope。
- **PostHog 是唯一自行摄取事件的候选**（SDK/autocapture）；其余三个是既有
  仓库的读取器。采用 PostHog 意味着**第二条事件管道**，除非 croupier 把
  `analytics.events` 导出给它——那是 DATA-011 的契约问题，不是调查结论。

## 3. 边界

- 不选首接目标；选择需要 owner 的成本/部署/运维姿态（自托管 vs 云、干净许可
  要求、仓库读取器 vs 并行摄取取舍）。
- 所有 NOT VERIFIED 单元格必须在 DATA-011 契约起草前对所选产品官方文档复核。
- 不存在也未宣称任何供应商集成；croupier 侧唯一涉及的机制仍是 extension
  基础设施（[core-and-external](../analytics/core-and-external.md) §4）。
