---
title: 配置开关效果
icon: toggle
order: 20
category:
  - 运维手册
tag:
  - 配置
  - 证据
---

# 配置开关效果

本页枚举本仓所有「开启后有效果」的配置开关——DOC-001 证据清单。每个开关的
表格记录配置键、默认状态、以及关闭/开启之间的可观察变化。GUI 可见的功能标志
已生成 off/on 截图 + GIF 证据（见下方）；server/agent 侧开关为配置级证据
（效果见表格行），因无操作控制台可视化变化。

约定：

- `off` / `on` 描述的是可观察行为变化，而非原始字节。
- 默认值标注 `(example)` 取自 `configs/*.yaml`；`(code)` 表示代码级默认
  （Go 零值，另有说明除外）。
- `featureFlags.*` 标志**缺省 = 启用**；只有显式 `false` 才禁用某域。控制面
  （API 路由 + 仪表盘菜单）由这些标志裁剪；数据面组件不受其管辖。

## 功能标志（仪表盘菜单 + API 路由域）

这些是最用户可见的开关：每个开关切换运维控制台某产品域的菜单项，以及其
API 路由的注册。

| 键                        | 默认         | 关闭效果                                                         | 开启效果                            |
| ------------------------- | ------------ | ---------------------------------------------------------------- | ----------------------------------- |
| `featureFlags.dev`        | 启用（缺省） | `dev` 菜单（研发）隐藏；路由不注册                               | `dev` 域（缺陷追踪/任务安排）可见   |
| `featureFlags.support`    | 启用（缺省） | `support` 菜单（客服系统）隐藏                                   | `support` 域（工单/FAQ/反馈）可见   |
| `featureFlags.analytics`  | 启用（缺省） | `analytics` 菜单（分析中心）隐藏                                 | `analytics` 域（数据分析）可见      |
| `featureFlags.ops`        | 启用（缺省） | `ops` 菜单（运维中心 + 风控）隐藏                                | `ops` 域（运维中心）可见            |
| `featureFlags.extensions` | 启用（缺省） | `extensions` 路由不注册；**菜单无变化**（扩展页均 `hideInMenu`） | `extensions` 域（扩展中心）路由注册 |

### 功能标志证据（关 → 开）

每个开关在一次性本地服务器上翻转，两个状态下用
`scripts/docs/config-switch-evidence.sh` 截取控制台侧边栏（无头 Chromium
登录 → 截图 → 加标签对比图 + 交叉淡化 GIF）。产物在
`docs/public/evidence/config-effects/<flag>/`。

**`featureFlags.ops`** — 运维中心与风控同时消失（两组路由同受 `canOpsRead`
访问键门控）：

![featureFlags.ops 关 vs 开](/evidence/config-effects/featureFlags.ops/ops-compare.png)

[切换 GIF](/evidence/config-effects/featureFlags.ops/ops.gif) ·
[关](/evidence/config-effects/featureFlags.ops/ops-off.png) ·
[开](/evidence/config-effects/featureFlags.ops/ops-on.png)

**`featureFlags.support`** — 客服系统菜单：

![featureFlags.support 关 vs 开](/evidence/config-effects/featureFlags.support/support-compare.png)

[切换 GIF](/evidence/config-effects/featureFlags.support/support.gif) ·
[关](/evidence/config-effects/featureFlags.support/support-off.png) ·
[开](/evidence/config-effects/featureFlags.support/support-on.png)

**`featureFlags.analytics`** — 分析中心菜单：

![featureFlags.analytics 关 vs 开](/evidence/config-effects/featureFlags.analytics/analytics-compare.png)

[切换 GIF](/evidence/config-effects/featureFlags.analytics/analytics.gif) ·
[关](/evidence/config-effects/featureFlags.analytics/analytics-off.png) ·
[开](/evidence/config-effects/featureFlags.analytics/analytics-on.png)

**`featureFlags.dev`** — 研发菜单：

![featureFlags.dev 关 vs 开](/evidence/config-effects/featureFlags.dev/dev-compare.png)

[切换 GIF](/evidence/config-effects/featureFlags.dev/dev.gif) ·
[关](/evidence/config-effects/featureFlags.dev/dev-off.png) ·
[开](/evidence/config-effects/featureFlags.dev/dev-on.png)

## 服务端配置开关

| 键                                               | 默认               | 关闭效果                                                         | 开启效果                                 |
| ------------------------------------------------ | ------------------ | ---------------------------------------------------------------- | ---------------------------------------- |
| `database.multiGame`                             | `false`（code）    | 单库；`game_id` 行级隔离                                         | 每个 `(game_id, env)` 独立物理库         |
| `cache.enabled`                                  | `false`（code）    | 缓存关闭；直连存储                                               | 缓存层生效                               |
| `cluster.enabled`                                | `false`（code）    | 集群特性关；单机运行                                             | 集群成员表 / owner 转发生效              |
| `cluster.insecureSkipTls`                        | `false`（code）    | 对等 TLS 校验                                                    | 对等 TLS 校验跳过                        |
| `cluster.allowStandaloneDegrade`                 | `false`（code）    | 集群必须；单机拒绝                                               | 集群不可用时允许单机降级                 |
| `cluster.allowPlaintextInterconnect`             | `false`（code）    | 互联 TLS fail-closed（证书缺失则拒绝启动）                       | 显式允许明文互联（仅受信隔离内网；降级） |
| `cluster.interconnectTls.enabled`                | `false`（code）    | 仅在 `allowPlaintextInterconnect: true` 时明文互联；否则拒绝启动 | mTLS 互联（对等证书身份）                |
| `cluster.interconnectTls.insecureSkipVerify`     | `false`（code）    | 对等证书校验                                                     | 对等证书校验跳过                         |
| `agentDispatch.enableHA`                         | `false`（code）    | HA 特性关                                                        | HA 多实例调度特性生效                    |
| `agentDispatch.reconnection.enableAutoReconnect` | `false`（code）    | 不自动重连                                                       | 掉线自动重连                             |
| `agentDispatch.toAgentTLS.enabled`               | `false`（code）    | 明文到 agent                                                     | TLS 到 agent                             |
| `agentDispatch.toAgentTLS.insecureSkipVerify`    | `false`（code）    | agent 证书校验                                                   | agent 证书校验跳过                       |
| `approval.allowSelfApprove`                      | `false`（example） | 拒绝自批                                                         | 允许审批人批自己的请求                   |
| `executionLog.enabled`                           | `false`（code）    | 不写执行日志                                                     | 每次执行写执行日志                       |
| `descriptors.schemaDiffWarn`                     | `false`（code）    | schema 漂移静默                                                  | schema 漂移记 warning                    |
| `storage.forcePathStyle`                         | `false`（code）    | 虚拟主机风格（S3）                                               | path-style 对象 URL                      |
| `metrics.perFunction`                            | `true`（example）  | 仅聚合指标                                                       | 逐函数指标输出                           |
| `metrics.perGameDenies`                          | `false`（example） | 不拒绝逐游戏指标                                                 | 逐游戏指标按 scope 拒绝                  |
| `serverStatus.enabled`                           | `false`（code）    | 维护窗口 gate 旁路                                               | agent 探活异常维护窗口 gate 生效         |
| `herald.enabled`                                 | `false`（code）    | 纯 croupier 告警页                                               | 告警经 herald 渠道投递                   |
| `provision.enabled`                              | `false`（code）    | 供给关闭                                                         | 供给/销毁工作流生效                      |
| `backup.schedule.enabled`                        | `false`（code）    | 无定时备份                                                       | 已注册数据源定时备份                     |
| `vault.enabled`                                  | `false`（code）    | 密钥内联在配置                                                   | 敏感凭据经 vault 提供者                  |

## 遥测开关

| 键                             | 默认    | 关闭效果            | 开启效果                |
| ------------------------------ | ------- | ------------------- | ----------------------- |
| `telemetry.enabled`            | `false` | OTel 关；零开销路径 | OTel SDK 生效           |
| `telemetry.enable_tracing`     | `false` | 不导出 trace        | trace span 导出         |
| `telemetry.enable_metrics`     | `false` | 不导出指标          | 指标导出                |
| `telemetry.use_tls`            | `false` | 明文 OTLP           | TLS 到 collector        |
| `telemetry.prometheus.enabled` | `false` | Prometheus 采集关   | `/metrics` 抓取端点生效 |

## 认证提供者开关

每个提供者的 `enabled` 门把该身份源加入/移出登录流。`register.enabled` 门
控自助注册。

| 键                                       | 默认            | 关闭效果      | 开启效果            |
| ---------------------------------------- | --------------- | ------------- | ------------------- |
| `auth.providers.local.enabled`           | `false`（code） | 本地登录关    | 本地账号密码登录开  |
| `auth.providers.ldap.enabled`            | `false`（code） | LDAP 登录关   | LDAP 目录登录开     |
| `auth.providers.ldap.startTls`           | `false`（code） | 明文 LDAP     | STARTTLS 升级       |
| `auth.providers.ldap.insecureSkipVerify` | `false`（code） | LDAP 证书校验 | LDAP 证书校验跳过   |
| `auth.providers.oidc.enabled`            | `false`（code） | OIDC 登录关   | OIDC 单点登录开     |
| `auth.providers.github.enabled`          | `false`（code） | GitHub 登录关 | GitHub OAuth 登录开 |
| `auth.providers.wechat.enabled`          | `false`（code） | 微信登录关    | 微信 OAuth 登录开   |
| `auth.providers.genericoauth.enabled`    | `false`（code） | 通用 OAuth 关 | 通用 OAuth 提供者开 |
| `auth.providers.register.enabled`        | `false`（code） | 自助注册关    | 自助注册开          |

## Agent 侧开关

这些位于 agent / sidecar / capture / devops / synthetics 配置
（`configs/*-agent.example.yaml`、`internal/app/agent/ops_config.go`）。

| 键                             | 默认               | 关闭效果              | 开启效果            |
| ------------------------------ | ------------------ | --------------------- | ------------------- |
| `ops.metricsEnabled`           | `false`（example） | ops 指标关            | 节点指标采集/上报   |
| `ops.allowRestart`             | `false`（example） | 拒绝重启 agent        | 经 ops API 允许重启 |
| `ops.allowExec`                | `false`（example） | 拒绝 exec（白名单空） | 白名单内允许 exec   |
| `ops.allowUpdate`              | `false`（example） | 拒绝更新 agent        | 允许更新 agent      |
| `ops.autoRestart`              | `false`（example） | 不自动重启            | 崩溃自动重启        |
| `capture.telemetry.enabled`    | `false`            | capture OTel 关       | capture 遥测导出    |
| `devops.telemetry.enabled`     | `false`            | devops OTel 关        | devops 遥测导出     |
| `synthetics.telemetry.enabled` | `false`            | synthetics OTel 关    | synthetics 遥测导出 |

> agent 侧遥测开关复用共享 `telemetry` 块（`enabled` + `enableMetrics` +
> `collectorUrl` + `useTls` + `serviceName`）；完整块与 `no_otel` 构建标签
> fail-closed 行为见 [config-agent.md](config-agent.md)。

## 已知边界

- 默认值是仓库示例 / 代码回退；部署可经 YAML / env / CLI 优先级覆盖任意项。
- 开关键遵循 canonical `lowerCamelCase` 配置契约；共享分析 `telemetry` 块
  保留其历史 `snake_case` 键（`enable_tracing`、`enable_metrics`、`use_tls`、
  `collector_url`、`service_name`）作为兼容面。
- `featureFlags.extensions` **仅影响路由**：扩展页全部 `hideInMenu`，切换它
  不改变任何侧边栏条目（实测验证——关/开菜单标签完全一致），只注册/注销
  扩展 API 路由。与其余四个各隐藏一个顶级菜单的标志不同。
- `featureFlags.ops` 同时隐藏运维中心与风控：`/risk/*` 路由与 `/ops/*` 同受
  `canOpsRead` 访问键门控，关闭 `ops` 会移除两个菜单。
- 上方 off/on 截图 + GIF 由 `scripts/docs/config-switch-evidence.sh` 对一次性
  本地 SQLite 服务器生成；流水线可对任意 `featureFlags.<name>` 重跑，产物写入
  `docs/public/evidence/config-effects/<flag>/`。
- 清单其余开关属 server/agent 侧，无操作控制台可视化变化；其证据是配置级
  （开关效果见表格行），非截图。
