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

平台内置**自建阈值规则引擎**（不是 Alertmanager 转发）：Agent 周期上报系统指标 → 规则评估器逐条判定 → 命中即产生告警并分发通知渠道。入口「运维中心 → 告警中心」（需 `ops` 功能域开启）。

## 指标来源

Agent 上报的系统指标（路径式 key）：

| 指标路径                    | 含义                            |
| --------------------------- | ------------------------------- |
| `cpu.usagePercent`          | CPU 使用率                      |
| `memory.usagePercent`       | 内存使用率                      |
| `disk.<挂载点>.usedPercent` | 磁盘使用率（按挂载点）          |
| `custom.<key>`              | 自定义指标（业务经 Agent 注入） |

## 告警规则

规则字段（Dashboard 规则 Tab 或 `POST /api/v1/alerts/rules`）：

| 字段              | 说明                                                |
| ----------------- | --------------------------------------------------- |
| `metric`          | 指标路径，如 `cpu.usagePercent`                     |
| `operator`        | `gt` / `gte` / `lt` / `lte`                         |
| `threshold`       | 阈值                                                |
| `forCount`        | 连续命中 N 次才触发（模拟持续窗口；中断即重置计数） |
| `cooldownSeconds` | 触发后冷却（默认 300s，冷却期内同规则不再触发）     |
| `level`           | `info` / `warning` / `critical`                     |
| `agentFilter`     | 空 = 全部 Agent；否则按 Agent ID 过滤               |
| `gameId` / `env`  | 作用域                                              |

示例——CPU 超 90% 持续 3 个周期才告警、冷却 10 分钟：

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
规则命中 → firing（写 alerts 表 + 分发通知渠道）
        → 静默（手动，minutes + reason）→ 静默期内不再提示
        → 恢复（指标回落，规则不再命中后自动 resolve）
```

- 列表按 `level` / `status` 过滤；critical 默认置顶
- 静默到期自动失效，也可在「静默列表」手动删除
- 触发历史审计留痕（谁静默的、为什么）

## 与通知渠道联动

告警触发时按[通知渠道](./notifications)配置分发：站内信（默认开）、钉钉机器人、自定义 webhook、SMTP 邮件。分发失败只记日志不阻塞告警落库。

## 与外部 Alertmanager 的关系

无集成。若已有 Alertmanager 体系，`GET /api/v1/ops/config` 返回的 `alertmanagerUrl`（环境变量 `CROUPIER_ALERTMANAGER_URL`）仅作为页面跳转链接——指标采集与告警判定都在平台内闭环，运维可自行选择只用其一。
