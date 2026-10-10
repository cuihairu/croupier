---
title: 功能分层审计——核心 / 可选 / 可切换服务商 / 不可换
---

# 功能分层审计（feature tiers）

## 状态

- 状态: Accepted（2026-10-10，用户令：功能分层审计落盘，面板设置页照 §6 总表出开关）
- 方法：通读代码现状（带 file:line 锚点，2026-10-10 巡检）+ 既有设计稿（plugin-mechanism / provider-plugin-design / server-status-provider / agents-inventory / incident-reports），不拍脑补。
- 关联：[配置整理](config-organization.md)（分层结论直接作为设置页/配置文件归类的输入）· [插件机制正式设计](plugin-mechanism.md) · [Provider 插件设计](provider-plugin-design.md) · [服务器状态源](server-status-provider.md)

## 1. 三层定义与判定规则

| 层                  | 定义                                                                                         | 裁掉会发生什么                                           |
| ------------------- | -------------------------------------------------------------------------------------------- | -------------------------------------------------------- |
| **L1 核心功能**     | 平台存在理由，默认必有、不可裁。个别带「部署开关」但语义上不可移除（开关只管启停，不管去留） | 产品不成立                                               |
| **L2 可选功能**     | 按业务域开关，关掉不影响主链（注册→调用→审计）。开关默认值见 §3                              | 面板少一个业务域，主链无感                               |
| **L3 可切换服务商** | 同一能力位的可替换实现（Provider/Driver 契约），默认实现内建、可选项按配置接入               | 无感——默认实现顶上（默认 no-op 第一原则：零配置=零外发） |

判定规则：

1. 裁掉后「注册→鉴权→调用→审计→执行」主链断裂 → L1。
2. 有 `featureFlags` 五域软开关或独立 enabled 配置、且关闭后仅少面板入口/少数据源 → L2。
3. 存在（或已设计定稿的）接口契约 + 至少一个内建默认实现 → L3。
4. 与 L1 重叠但实现写死、无接口抽象的 → 归 §5「不可换」，不硬造接口。

## 2. L1 核心功能（不可裁）

| 功能                                                                                    | 为什么是核心                                                                      | 部署开关                                                                                                |
| --------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------- |
| 权限控制层（RBAC/ABAC + casbin 求值 + scope 检查）                                      | 第一层架构定位（CLAUDE.md）：一切 API 的前置门                                    | 无（恒开）                                                                                              |
| 自建 TCP transport（19090，length-prefix + protobuf）                                   | agent/SDK 唯一连接入口；gRPC 已裁决不用（docs/architecture/transport-no-grpc.md） | TLS 可选（prod 应开）                                                                                   |
| 函数注册表 + 调用路由（`(game_id, function_id)` 索引 + lb/broadcast/targeted/hash）     | 游戏控制层的本体                                                                  | 无                                                                                                      |
| agent 执行审计 execlog（`execution_logs`）                                              | 风控溯源底座；高危操作事后追责唯一凭据                                            | `executionLog.enabled` 缺省**开**（nil→true，config.go:732）；关=同步执行不写留痕，属运营期降档不可移除 |
| 审批工作流（two-person rule，状态机 draft→pending→approved/rejected/cancelled/expired） | 高危操作双人规则的载体                                                            | `approval` 段存在即启用                                                                                 |
| 幂等 + 异步任务（idempotency-key、事件流、CancelTask）                                  | 分布式重复副作用防线                                                              | 无                                                                                                      |
| 审计链（audit_records + hash 完整性 + 敏感字段掩码）                                    | 合规底座，篡改可检出                                                              | 无                                                                                                      |
| 认证（JWT 签发/校验 + local 口令源恒在级联首位）                                        | 无认证=无系统                                                                     | local 缺省开（nil→true，config.go:857）；外接身份源见 §4                                                |
| 多 game 隔离（game_id/env 传播 + database-per-game 路由）                               | 单公司多游戏的行级/库级隔离契约                                                   | `database.multiGame` 缺省 false（单库行级隔离）                                                         |
| 事故登记（incidents + incident_categories CRUD）                                        | 事故报表体系的数据源（incident-reports.md §2.2）                                  | 无（ops 域内，域关则入口藏、数据与 API 在）                                                             |
| 站内通知（in-app，出口链首不可删）                                                      | 一切告警/审批/公告的兜底触达；外发出口全挂时仍可达                                | 无（恒开）                                                                                              |
| 编号迁移链（goose，`MinimumRequiredVersion` 门禁）                                      | 存量库 schema 演进唯一真值（迁移契约）                                            | 无                                                                                                      |
| CDC 风控（capture agent，MySQL binlog → 风险闸门）                                      | 数据变更的事前/事后风控依据                                                       | `capture.enabled` 缺省**关**（需 binlog 权限与位点目录，零配置开不了也不该开；属「核心但部署缺省关」）  |

## 3. L2 可选功能（按需开关）

软 flag 五域（`featureFlags` map，`internal/config/config.go:158-181`）：**缺省全部开启（fail-open）**——map 为 nil / 键不存在 / 未知 flag 名一律 true，仅显式 `false` 才关。双门守卫：L2 物理不注册路由 + L3 `softFlags.guard` 403。前端同构 fail-open（`web/src/access.ts:49` `featureOn = features?.[key] !== false`），设置页 FeatureFlagsTab 可翻转（L3 settings 落库）。

| 域         | flag         | 内容                                                  | 默认 |
| ---------- | ------------ | ----------------------------------------------------- | ---- |
| 研发域     | `dev`        | 缺陷追踪（bugs）、任务安排                            | 开   |
| 客服域     | `support`    | 工单、FAQ、玩家反馈                                   | 开   |
| 数据分析域 | `analytics`  | 面板 analytics 页（invocations/warehouse/realtime）   | 开   |
| 运维中心域 | `ops`        | 监控、告警、探活、事故登记/报表、服务器状态、菜单管理 | 开   |
| 扩展中心域 | `extensions` | Store、pack 导入、扩展安装/生命周期、provider 绑定    | 开   |

非 flag 的独立 enabled 配置（关=少数据源/少外发，主链无感）：

| 功能                                                 | 配置位                                                                       | 默认                   |
| ---------------------------------------------------- | ---------------------------------------------------------------------------- | ---------------------- |
| 告警出口 herald                                      | `herald.enabled` + baseUrl/tokenEnv（service_context.go:548-551 齐备才注册） | **关**                 |
| 告警入站 webhook（alertmanager/generic）             | `alertInbound.sources.<name>.secretEnv`（空=未启用，config.go:81-95）        | **关**                 |
| CDC capture                                          | `capture.enabled`                                                            | **关**（见 §2 核心表） |
| 身份源 ldap / oidc / github / wechat / generic-oauth | `auth.<source>.enabled`                                                      | 全**关**（local 除外） |
| ClickHouse warehouse                                 | `analytics` DSN 未设 → 接口 503                                              | **关**                 |
| SSE 实时推送                                         | `sse` 段                                                                     | 按配置                 |
| metrics / telemetry 导出                             | `metrics` / `telemetry` 段                                                   | 按配置                 |
| 移动端 companion                                     | mobile-companion-design.md                                                   | 未实施（设计稿阶段）   |

## 4. L3 可切换服务商的能力

逐个列：能力 / 契约（Provider 接口）/ 默认实现 / 可选项 / 配置位。

| #   | 能力                         | 契约                                                                                                                                                    | 默认实现                                                          | 可选项                                                                                         | 配置位                                                                       |
| --- | ---------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------- | ---------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------- |
| 1   | 告警出口                     | `outlet.Outlet`：`Name() + Deliver(ctx, AlertEvent) error`（outlet.go:69-73；`DeliveryOutcome` 回执与降级链为 plugin-mechanism §5.1 设计态，落地在 M6） | **站内通知**（链首内建不可删）+ manager 扇出                      | herald（唯一已注册实现，herald.go）；generic webhook / email 留位                              | `herald.*`（enabled/baseUrl/app/tokenEnv/target）；改造后归 `notification.*` |
| 2   | 服务器状态源                 | `ServerStatusProvider`（server-status-provider.md 设计定稿，未落地）                                                                                    | **no-op**（零配置零外发）                                         | atlas（第一个真实现，骨架）；Zabbix 留位                                                       | `serverStatus.*`（enabled/provider/matchBy/cacheTtlSeconds/providers.atlas） |
| 3   | CDC 数据源                   | `capture/source.Provider`：`Open/Bookmark/Close`（source.go:11-18）+ 本地位点 Store                                                                     | MySQLSource（唯一实现，runner 硬编码 newSource，runner.go:70-71） | Postgres/SQLite/Mongo 源未落地（capture-agent-design §3 的可插拔未兑现）                       | `capture.mysql.*` + `capture.bookmarkDir`                                    |
| 4   | CI/CD 供应商                 | `cicd.Provider`：`Kind/TriggerBuild/GetBuild/ListArtifacts` + `Factory` 注册表 init 自注册（provider.go:69-126）                                        | 无默认——配置实例必填 Kind，未注册报 ErrUnknownKind                | generic / gitlab-ci / github-actions / jenkins（4 个已注册）                                   | DB 行 `cicd_integrations`（Kind/Endpoint/Token/Extra，Enabled 缺省 true）    |
| 5   | 身份源                       | `identity.PasswordProvider` + `OAuthProvider`（provider.go:45-61）——**接口有、注册表无**，显式装配在 api/auth/providers.go buildIdentityProviders       | **local**（恒为密码级联首位，api/auth/service.go:304-306）        | ldap / oidc / github / wechat / generic-oauth（构建失败失效降级跳过，LDAP 配置不完整直接报错） | `auth.<source>.*`（L3 白名单全字段）                                         |
| 6   | 外部平台调用                 | `drivers.Driver`：`Kind/Call/Close` + Factory 注册表（driver.go:44-100，#66 P0 契约骨架）                                                               | **空注册表**（零 driver，P1 迁 openapi、P2 webhook）              | openapi / webhook（type 闭集，manifest provider 块三键定稿）                                   | extension manifest `provider` 块 → installation runtime binding `spec_json`  |
| 7   | 供应商数据接入（#66 前身）   | `platform/provider.Registry`                                                                                                                            | 未接生产（生产走 extension externalfunc dispatcher）              | ——                                                                                             | 归档不接线（provider-plugin-design §5.3 裁决）                               |
| 8   | 对象存储                     | storage driver 接口                                                                                                                                     | `file`                                                            | 按实现                                                                                         | `storage.driver` + `storage.baseDir`                                         |
| 9   | 缓存                         | cache 抽象                                                                                                                                              | 内存                                                              | redis                                                                                          | `cache.enabled` + `cache.type`                                               |
| 10  | 数据库                       | gorm driver                                                                                                                                             | mysql / postgres / sqlite 三方言均在用                            | ——                                                                                             | `database.driver` + `database.dataSource`                                    |
| 11  | 负载均衡策略                 | Strategy 接口                                                                                                                                           | round-robin                                                       | consistent hash / least conn                                                                   | 调用路由参数                                                                 |
| 12  | 报告分发渠道（事故报表批 4） | 复用 Outlet 出口链                                                                                                                                      | 站内（leader 分片 + 管理员总聚合）                                | herald/webhook 等外发出口                                                                      | `notification.*`（设计态）                                                   |
| 13  | 告警出口以外的通知渠道       | `notify` 服务渠道矩阵（service/notify/service.go：Dispatch→inApp+external）                                                                             | 站内信（默认开）                                                  | dingtalk / wecom / feishu / webhook / email                                                    | `notification.*`（L3 白名单）                                                |

> 4 与 5 是 provider-plugin-design §8 拍板「保留内置不迁移」的两处抽象；6 是它们的统一方向（extension × driver × provider 三层模型）。

## 5. 不可换的（写死在核心，无接口抽象）

| 项            | 现状锚点                                                                                                                                                                             | 为什么不抽象                                                                                                     |
| ------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------- |
| RBAC 策略引擎 | PermissionService GORM 直读 roles/permissions + 包级 casbin 内存求值（service/permission/service.go:22, rbac/logical_permissions.go:40-63）；legacy Policy/CasbinPolicy 无生产调用点 | 权限模型即产品本体，换引擎=重写鉴权语义；端口（ports/permissions.go:16-20）只覆盖 scope 检查，刻意不覆盖引擎实现 |
| JWT 签发/校验 | HS256 + 共享密钥写死（jwtutil/token.go:50），TTL 24h 常量（token.go:23）                                                                                                             | 单公司自托管无多算法需求；换算法=全量强制重登录，等真实需求再立项                                                |
| 审批状态机    | WorkflowEngine + 六态闭集进程内 GORM（platform/approvals/workflow.go:24-30,175）                                                                                                     | two-person rule 是平台底座语义；可插拔的只有**通知渠道**（MultiChannelNotifier，notification.go:95），引擎不外置 |
| execlog 落库  | GORM 直写 execution_logs（model/execution_log.go:38-49），无 sink 抽象                                                                                                               | 审计完整性要求本地落库不可绕过；外发 sink 等明确合规需求                                                         |
| 传输协议      | 自建 TCP + protobuf 帧写死                                                                                                                                                           | gRPC 已调查并裁决不用（docs/archive/grpc-investigation.md）                                                      |
| 迁移机制      | goose 编号迁移链                                                                                                                                                                     | schema 演进唯一真值，迁移契约明文                                                                                |
| 审计链完整性  | hash 链写死                                                                                                                                                                          | 换链算法=历史链全部失效，没有「可换」的意义                                                                      |
| 站内通知      | 出口链首（plugin-mechanism §5.1）                                                                                                                                                    | 兜底触达不可被外发实现替换，只可被补充                                                                           |

## 6. 总表（功能 × 层级 × 可换 × 默认值）

设置页开关照此表出；「配置位」列是设置页字段与配置文件 section 的对照锚。

| 功能                            | 层级                      | 可换                 | 默认值             | 配置位                                 |
| ------------------------------- | ------------------------- | -------------------- | ------------------ | -------------------------------------- |
| 权限/RBAC/审计链                | L1                        | 否（引擎写死）       | 恒开               | ——                                     |
| 传输/注册表/调度                | L1                        | 否                   | 恒开               | `server.*` / `registry.*`              |
| execlog 审计                    | L1                        | 否（落库写死）       | **开**             | `executionLog.enabled`                 |
| 审批工作流                      | L1                        | 引擎否 / 通知渠道可  | 开                 | `approval.*`                           |
| 认证（JWT+local）               | L1                        | 身份源可换           | local 开           | `auth.*`                               |
| 多 game 隔离                    | L1                        | 否                   | 单库模式           | `database.multiGame`                   |
| 事故登记/报表                   | L1（域内入口随 ops flag） | 否                   | 开（ops flag 开）  | `featureFlags.ops`                     |
| 站内通知                        | L1                        | 否（链首）           | 开                 | ——                                     |
| CDC 风控                        | L1                        | 源可换（现仅 MySQL） | **关**             | `capture.enabled`                      |
| 五域软 flag                     | L2                        | 否                   | 全开               | `featureFlags.*`                       |
| 告警入站                        | L2                        | 来源可配             | 关                 | `alertInbound.sources.*`               |
| warehouse/SSE/metrics/telemetry | L2                        | 否                   | 关/按配置          | 各自 section                           |
| 告警出口                        | L3                        | 是                   | 站内（herald 关）  | `herald.*`                             |
| 服务器状态源                    | L3                        | 是                   | no-op              | `serverStatus.*`（设计态）             |
| CI/CD 供应商                    | L3                        | 是                   | 无实例             | DB `cicd_integrations`                 |
| 身份源                          | L3                        | 是                   | local              | `auth.<source>.*`                      |
| 外部平台 driver                 | L3                        | 是                   | 空注册表           | manifest `provider` 块                 |
| 存储/缓存/DB/LB                 | L3                        | 是                   | file/内存/mysql/RR | `storage.*` / `cache.*` / `database.*` |
| 通知渠道矩阵                    | L3                        | 是                   | 站内开、外发全关   | `notification.*`                       |

## 7. 设计 vs 代码差距（诚实清单，2026-10-10 巡检）

| 维度            | 设计文档口径                                                                       | 代码现状                                                        | 缺口归属                         |
| --------------- | ---------------------------------------------------------------------------------- | --------------------------------------------------------------- | -------------------------------- |
| Outlet 接口     | `Deliver(ctx, ev) (DeliveryOutcome, error)` + 降级链（plugin-mechanism §5.1/§6.6） | `Deliver(ctx, ev) error`，无回执、无序扇出（outlet.go:136-144） | M6 批次改造（事故报表批 4 前置） |
| Outlet 实现闭集 | 站内+herald+webhook+email+no-op                                                    | 仅 herald 一个                                                  | webhook/email 按需批             |
| 服务器状态源    | ServerStatusProvider + noop/atlas                                                  | 零代码                                                          | 批 S1-S3                         |
| CDC 可插拔      | source.Provider 多源                                                               | 单 MySQL 硬编码                                                 | 按需批                           |
| drivers 注册表  | openapi/webhook                                                                    | 空注册表零实现                                                  | #66 P1/P2                        |
| identity 注册表 | （无成文要求）                                                                     | 显式装配无注册表                                                | 维持现状（拍板「保留内置」）     |
| 报告分发        | incident-reports §6                                                                | 未实施                                                          | 事故报表批 4                     |

## 8. 后续动作

1. 本表作为 [config-organization.md](config-organization.md) 的分类输入（设置页分组导航 = §3 flag 域 + §4 服务商能力位）。
2. §7 差距项不new立项——全部归属既有批次（M6 / S1-S3 / #66 / 事故报表批 4），防止分层审计变成第二张路线图。
3. 设置页新开关出现时回填 §6 总表；语义变化（升层/降层）需在此留拍板记录。
