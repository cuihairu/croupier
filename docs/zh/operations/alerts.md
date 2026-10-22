---
title: 告警管理
icon: bell
order: 21
category:
  - 运维手册
tag:
  - 可观测性
  - 告警
---

# 告警管理

平台自带**内置阈值规则引擎**（不是 Alertmanager 转发器）：Agent 周期上报系统指标 → 规则求值器逐条判定 → 命中即产告警并分发到通知渠道。入口：「运营中心 → 告警中心」（需启用 `ops` 功能域）。

## 指标来源

Agent 上报的系统指标（路径式 key）：

| 指标路径                    | 含义                            |
| --------------------------- | ------------------------------- |
| `cpu.usagePercent`          | CPU 使用率                      |
| `memory.usagePercent`       | 内存使用率                      |
| `disk.<挂载点>.usedPercent` | 磁盘使用率（按挂载点）          |
| `custom.<key>`              | 自定义指标（业务经 Agent 注入） |

## 告警规则

规则字段（Dashboard Rules 页签或 `POST /api/v1/alerts/rules`）：

| 字段              | 说明                                                |
| ----------------- | --------------------------------------------------- |
| `metric`          | 指标路径，如 `cpu.usagePercent`                     |
| `operator`        | `gt` / `gte` / `lt` / `lte`                         |
| `threshold`       | 阈值                                                |
| `forCount`        | 连续命中 N 次才触发（模拟持续窗口；中断即重置计数） |
| `cooldownSeconds` | 触发后的冷却期（默认 300s；冷却期内同规则不再触发） |
| `level`           | `info` / `warning` / `critical`                     |
| `agentFilter`     | 空 = 全部 Agent；否则按 Agent ID 过滤               |
| `gameId` / `env`  | 归属范围                                            |

示例——CPU 超 90% 且连续 3 个周期才告警，冷却 10 分钟：

```json
{
  "metric": "cpu.usagePercent",
  "operator": "gt",
  "threshold": 90,
  "forCount": 3,
  "cooldownSeconds": 600,
  "level": "warning",
  "agentFilter": "",
  "gameId": "demo",
  "env": "prod"
}
```

## 告警生命周期

```
Rule hit → firing (write to the alerts table + dispatch to notification channels)
        → silenced (manual, minutes + reason) → no further prompts during silence
        → recovered (metric falls back; automatically resolved once the rule no longer hits)
```

- 列表可按 `level` / `status` 过滤；critical 默认置顶
- 静默到期自动解除；也可在「静默列表」手动删除
- 触发历史留痕可审计（谁静默的、为什么）

## 与通知渠道的联动

告警触发后按通知渠道配置分发：站内信（默认开启）、钉钉机器人、自定义 webhook、SMTP 邮件。分发失败只记日志，不阻塞告警落库。

另有 **herald 外部告警出口**（`herald.enabled`，默认关）：启用后监管熔断（supervisor breaker_tripped）等事件经 herald 投递——渠道/收件人都在 herald 侧配置，herald 不可达只记日志计数失败，不阻塞告警落库。

## 与外部 Alertmanager 的关系

已有 Alertmanager 体系时，有两种接入方式：

1. **入站 webhook（可落库）**：把 Alertmanager 的 webhook_configs 指到 `POST /api/v1/alerts/inbound/alertmanager`（HMAC-SHA256 签名 + 时间戳防重放；secret 经 `alertInbound.sources.alertmanager.secretEnv` 环境变量引用）；告警与本页及 capture/dbmon 告警同桶落库；通用入口（`POST /api/v1/alerts/inbound/generic`）允许任何第三方以统一信封推送。
2. **仅跳转链接（现状）**：`GET /api/v1/ops/config` 返回的 `alertmanagerUrl`（环境变量 `CROUPIER_ALERTMANAGER_URL`）仅作页面跳转。

croupier 不是跨系统告警去重/降噪引擎；inbound 是单向接收，不回写确认状态到源系统。
