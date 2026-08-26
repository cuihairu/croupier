---
title: 功能开关（Feature Flags）
---

# 功能开关（Feature Flags）

## 状态

Accepted（P1 已落地：配置驱动，控制面全域覆盖）。

## 语义

- **权限 vs 开关**：权限（access）回答"**谁**能用"；功能开关回答"**这套部署有没有**这个域"。开关关闭时菜单隐藏、路由拦截（403）、后端 API 不注册（404）——比"藏入口"更强的"直接关闭"。
- **控制面 vs 数据面**：P1 开关只治理 server 控制面（REST API + 仪表盘）。数据面组件（agent、analytics worker、ingest、clickhouse）不读 server 配置，不受影响。例如关闭 `analytics` 后仪表盘与分析 API 消失，但数据管道照常写入（历史数据不丢，重新开启即恢复）。
- **默认开启（fail-open）**：未配置的域全部开启；配置 typo 不会锁死整个域。后端 `FeatureFlagsConfig.Enabled` 与前端 `access.ts` 保持相同语义；前端 meta 拉取失败时同样全开，**后端路由 gating 是权威闸门**。

## 配置

```yaml
featureFlags:
  dev: false # 研发域（缺陷追踪 /dev/bugs、/api/v1/bugs）
  support: false # 客服域（工单/FAQ/反馈 + 对应 API）
  analytics: false # 数据分析域（/analytics/*）
  ops: false # 运维中心域（/ops/*、告警/备份/证书 API）
  extensions: false # 扩展中心域（/extensions、/platforms、agents 扩展兼容）
```

## 受控域与影响面

| Flag         | 前端菜单                  | 后端路由组                                     |
| ------------ | ------------------------- | ---------------------------------------------- |
| `dev`        | 研发（缺陷追踪）          | `/api/v1/bugs`                                 |
| `support`    | 客服系统（工单/FAQ/反馈） | `/tickets` `/faqs` `/feedback`                 |
| `analytics`  | 数据分析                  | `/analytics`                                   |
| `ops`        | 运维中心                  | `/ops` `/alerts` `/backups` `/certificates`    |
| `extensions` | 系统管理-扩展中心         | `/extensions` `/platforms` `/agents`(扩展兼容) |

常驻域（不受开关控制）：认证/权限/账号、游戏环境管理、函数与页面、运行控制台、审计、消息、监控健康——它们是平台运行的基础设施。

## 数据流

```
server.yaml featureFlags
   ├─ server 启动：受控路由组条件注册（关闭 = 404）
   └─ GET /api/v1 (public) → features[]（仅含启用域）
        └─ 前端 getInitialState → initialState.features
             └─ access.ts：各域 canXxxRead/Manage 复合 featureOn(domain)
                  ├─ 菜单自动隐藏（layout 按 access 过滤）
                  └─ 直接访问 URL → 403
```

## 实施注意

- 后端新增域时：`config.FlagXxx` 常量 + routes.go 条件注册 + meta `enabledFeatures` 列表 + 前端 `ServerFeatures` 类型与 `access.ts` 同步（四处，见 Review Checklist）。
- flag 名即前后端契约（小写单词），保持与 `config.Flag*` 常量一致。
- 环境变量覆盖遵循既有 `CROUPIER_SERVER_*` 规则时同样适用于嵌套段。

## 后续阶段

- **P2 运行时开关**：flags 落库（系统配置表）+ 管理界面 + 热更新（无需重启）；当前 P1 为部署级（改配置重启生效），满足"不需要的域直接关掉"的主诉求。
- **P3 灰度**：按角色/游戏维度的 flag 覆盖（类似 LaunchDarkly 的 targeting），仅当多团队共用部署时才需要。

## Review Checklist

新增受控域时四处同步，缺一即 review failure：

1. `internal/config/config.go`：`FlagXxx` 常量
2. `internal/handler/routes.go`：条件注册
3. `internal/api/meta/service.go`：`enabledFeatures` 域列表
4. `web/src/services/api/features.ts` + `web/src/access.ts`：类型与 `featureOn` 复合
