---
title: 配置整理——现状盘点与分层归类（设置页 × 配置文件）
---

# 配置整理（config organization）

## 状态

- 状态: Proposed（2026-10-10，用户令整理单 2；**本文落盘过目后再动设置页/配置结构**，涉及改键的项均附兼容读入方案）
- 输入：[feature-tiers.md](feature-tiers.md) 分层结论（L1 核心 / L2 可选 / L3 可切换服务商 / 不可换）+ 2026-10-10 全量盘点（`internal/platform/settings/layered.go` L3 白名单、`internal/config/config.go` 顶层段、`web/src/pages/System/SiteSettings/` 10 Tab）。

## 1. 现状盘点

### 1.1 配置面有三层真值

```
L1 代码默认（layered.go 缺省值）
  ↑
L2 配置文件 / env（configs/server.yaml，CROUPIER_SERVER_* 前缀）
  ↑
L3 数据库覆盖（settings 表，设置页读写，108 键白名单）
```

### 1.2 配置文件顶层段（27 段，`internal/config/config.go`）

| 分组       | 段                                                        |
| ---------- | --------------------------------------------------------- |
| 传输与服务 | `server` `cluster` `region` `zone` `labels`               |
| 数据       | `database` `storage` `cache` `bootstrapData`              |
| 注册与调度 | `control` `registry` `agentDispatch` `profiles`           |
| 安全与合规 | `auth` `approval` `executionLog` `taskLog` `log`          |
| 业务域开关 | `featureFlags`                                            |
| 外部服务   | `herald` `alertInbound` `openapi` `descriptors` `schemas` |
| 可观测     | `metrics` `telemetry` `sse` `pages`                       |

文件侧问题不大：段已按域聚合、lowerCamelCase 契约（CLAUDE.md 强制）。遗留少量兼容解析（`yaml:"Enabled,omitempty"` 大写键 compat，config.go:555/1145）——只容忍不传播。

### 1.3 设置页 L3 白名单（108 键，按前缀）

| 前缀             | 键数 | 内容                                                                                                                      |
| ---------------- | ---- | ------------------------------------------------------------------------------------------------------------------------- |
| `site.*`         | 11   | name/description/defaultLocale/docsUrl/faviconUrl/homeContent/logoUrl/privacyPolicy/serverUrl/taskPublicUrl/userAgreement |
| `footer.*`       | 3    | copyright/icp/links                                                                                                       |
| `features.*`     | 5    | dev/support/analytics/ops/extensions                                                                                      |
| `auth.*`         | ≈50  | local.enabled + email 注册策略 3 + register 2 + ldap 9 + oidc 7 + github 6 + wechat 7 + genericoauth 12                   |
| `notification.*` | 17   | inAppEnabled + emailEnabled/smtp×8 + dingtalk×2 + feishu×2 + wecom×1 + webhook×2                                          |
| `obs.*`          | 3    | alertmanagerUrl/grafanaExploreUrl/jaegerUrl                                                                               |
| `perf.*`         | 6    | cacheSize/maxConcurrent/maxCpuPct/maxDiskPct/maxMemoryPct/maxThreadCount                                                  |
| `log.*`          | 4    | cleanupCron/copierDir/copierKeep/retentionDays                                                                            |
| `net.*`          | 3    | maxRetries/requestTimeoutMs/retryBackoffMs                                                                                |
| `system.*`       | 1    | updateCheckUrl                                                                                                            |

### 1.4 设置页现状（10 Tab）

`site / appearance / features / auth / notification / security / observability / maintenance / performance / logs`

### 1.5 现状四大乱源（本整理要解的）

1. **通知 tab 一层平铺**：站内信（核心、默认开、不可关死）与 5 个外发渠道（Provider、默认关、含凭据）混在同一层，无分级（info/warn/critical）与可见范围概念——批 4 通知强化要加 level/scope/ref，现在没有落点。
2. **auth tab 平铺**：local（核心默认开）与 5 个外接身份源（Provider，各自 enabled+凭据+回调 URL）同层，找不到「哪些是外部服务」。
3. **外部服务散落三处**：`observability` tab 实为外部系统链接（alertmanager/grafana/jaeger URL）；通知外发在 `notification`；身份源在 `auth`；herald/alertInbound/serverStatus 只有文件配置没有页面归属——「外部服务配置与启用开关放一起」不成立。
4. **默认值不可见**：设置页只显示已存 L3 覆盖值，代码默认（L1）与文件覆盖（L2）对管理员不可见，改了不知道生效值从哪来。

## 2. 分层归类（改版结构）

按 feature-tiers 四类重排：**核心配置 / 可选功能配置 / 外部服务配置（Provider 开关+凭据）/ 通知分级与可见范围**。

### 2.1 设置页分组导航（改版后 7 组）

| #   | 组                 | 内容（键前缀）                                                                                                                                                                                                                                                 | 层        | 危险标注                                                                                                                                              |
| --- | ------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------- | ----------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | **站点**           | `site.*` + `footer.*`（合并现 site/appearance 两 tab）                                                                                                                                                                                                         | L1        | ——                                                                                                                                                    |
| 2   | **功能开关**       | `features.*` 五域；warehouse/SSE/metrics/telemetry 各自 enabled 集中展示（只读展示文件侧值，页内可翻转项落 L3）                                                                                                                                                | L2        | 关 analytics/ops 藏整域入口（二次确认）                                                                                                               |
| 3   | **认证与安全**     | `auth.local` + `auth.email.*` + `auth.register.*` + 现 security tab（密码策略/MFA/IP 约束）                                                                                                                                                                    | L1        | 关 local 前须有可用身份源（守卫提示）                                                                                                                 |
| 4   | **外部服务**（新） | 身份源：`auth.ldap/oidc/github/wechat/genericoauth`；告警出口：`herald.*`；告警入站：`alertInbound.sources.*`；通知外发渠道：smtp/dingtalk/wecom/feishu/webhook；外部系统链接：`obs.*`、`system.updateCheckUrl`；状态源：`serverStatus.*`（批 S1-S3 落地后进） | L3        | **外发**（herald/通知渠道/telemetry）；**暴露面**（alertInbound 公网 webhook）；凭据回显一律掩码（现 `smtpPasswordSet`/`dingtalkSecretSet` 模式推广） |
| 5   | **通知**           | 站内开关 `notification.inAppEnabled`；分级与可见范围（**设计态，批 4**：level info/warn/critical、按类别 scope、leader 分片/管理员聚合、升级去重）                                                                                                             | L1+设计态 | 分级改错=漏报（critical 静音需二次确认）                                                                                                              |
| 6   | **性能与资源**     | `perf.*` + `net.*`                                                                                                                                                                                                                                             | L1 调优   | 阈值调低=限流自杀（显示当前水位）                                                                                                                     |
| 7   | **日志与维护**     | `log.*` + 现 maintenance tab                                                                                                                                                                                                                                   | L1 调优   | **破坏性**：cleanupCron/retentionDays（删数据）、维护动作（重建/清理）全部二次确认                                                                    |

> tab 数 10→7：appearance 并入站点、security 并入认证与安全、performance+logs+maintenance 收敛为两组、外部服务独立成组。旧 tab 路径 301 到新组（或路由别名）防书签断链。

### 2.2 配置文件 section（改版原则：段名不动，给分组视图）

文件侧 27 段**全部保留现名**——改段名=全量部署配置迁移，收益低风险高（违反「同类相聚」的性价比判断）。改版产出是**文档分组视图**（configs/server.yaml 注释分节 + docs 分组索引），按 §2.1 同名七组编排，运维按组找段。

可选后续（不承诺）：`herald`/`alertInbound`/`serverStatus` 未来统一挪入 `externalServices:` 大段——需 compat 双读一个发布周期，仅在 Provider 数量 ≥5 使顶层段拥挤时立项。

### 2.3 通知分级与可见范围（设计态，键位预留）

批 4 落地时的键形态（进 L3 白名单需同步 layered.go）：

```
notification.inAppEnabled          # 站内（现键，保留）
notification.level.default         # info|warn|critical 缺省 info
notification.scope.<categoryId>    # leader 账号分片；空=管理员聚合
notification.escalate.minutes      # 升级去重窗口（0=不升级）
```

外发渠道键统一收进 `notification.outbound.<channel>.*` 的**改键方案见 §3.1 对照**——批 4 动手前先过目本表。

## 3. 改版前后对照示例

### 3.1 通知（键改：需兼容读入）

```
改前（notification tab 平铺）           改后
notification.inAppEnabled          →   通知组 · 站内：notification.inAppEnabled（键不动）
notification.emailEnabled          →   外部服务组 · 通知渠道/邮件：notification.outbound.email.enabled
notification.smtpHost              →                        notification.outbound.email.host
notification.smtpPort              →                        notification.outbound.email.port
notification.smtpUser              →                        notification.outbound.email.user
notification.smtpPassword          →                        notification.outbound.email.password（掩码回显）
notification.smtpEncryption        →                        notification.outbound.email.encryption
notification.smtpAuthType          →                        notification.outbound.email.authType
notification.smtpFrom              →                        notification.outbound.email.from
notification.smtpInsecureSkipVerify→                        notification.outbound.email.skipVerify
notification.dingtalkUrl           →   外部服务组 · 通知渠道/钉钉：notification.outbound.dingtalk.url
notification.dingtalkSecret        →                        notification.outbound.dingtalk.secret（掩码）
notification.feishuUrl/Secret      →                        notification.outbound.feishu.url/secret
notification.wecomUrl              →                        notification.outbound.wecom.url
notification.webhookUrl/Secret     →                        notification.outbound.webhook.url/secret
（无）                             →   通知组 · 分级：notification.level.default / scope.<categoryId> / escalate.minutes（批 4 新增）
```

兼容读入：layered.go 读 `notification.outbound.email.enabled` 时 miss 则回落旧键 `notification.emailEnabled`（单代兼容，写回一律新键；下个发布周期删回落）。

### 3.2 认证（键不改，仅 UI 归属迁移：零迁移）

```
改前（auth tab 平铺）                  改后
auth.local.enabled                 →   认证与安全组 · 本地口令
auth.email.* / auth.register.*     →   认证与安全组 · 注册与邮箱策略
auth.ldap.*（9 键）                →   外部服务组 · 身份源/LDAP（enabled+addr+bindDn+bindPassword 同卡片）
auth.oidc.* / github.* / wechat.* /
  genericoauth.*                   →   外部服务组 · 身份源/各卡（enabled 开关与凭据同卡片=「外部服务配置与启用开关放一起」）
```

### 3.3 功能开关（默认值集中展示）

```
改前（FeatureFlagsTab 只有五域翻转）   改后
features.dev/support/analytics/
  ops/extensions                    →   功能开关组 · 五域卡（显示 生效值=L1默认←L2文件←L3覆盖 的来源徽标）
（散在文件配置，页面不可见）        →   功能开关组 · 附加能力卡：warehouse(SDN 未设=关)/SSE/metrics/telemetry 各 enabled+默认值只读展示
```

「生效值来源徽标」（default/file/db）是解乱源 4 的通用控件：设置页所有字段标注当前值来自哪层。

## 4. 实施顺序与边界

1. **本批（文档）**：落盘过目，不动代码。
2. **批 A（页面重排，零迁移）**：3.2/3.3 两例——UI 归属调整 + 生效值来源徽标 + 危险项标注；键不动，无迁移。
3. **批 B（通知改键，批 4 前置）**：3.1 对照表落地 + 旧键兼容读入一个发布周期 + layered.go 白名单同步。
4. **批 C（批 4 联动）**：通知分级/可见范围键 + serverStatus 卡进外部服务组。

边界诚实：

- 本整理不改任何配置语义与默认值，只动归类与可见性。
- `auth.*` 前缀跨两个页面组（local 在组 3、外接源在组 4）——键前缀与页面分组不必一一对应，这是有意为之（键=部署域，页面=管理动线）。
- 文件侧 27 段名冻结；§2.2 的 `externalServices:` 归并仅为远期选项，未立项。
- 旧 tab 书签兼容（路由别名/301）属批 A 交付项，遗漏即 review failure。
