---
title: 限速管理
icon: throttle
order: 42
category:
  - 运维手册
tag:
  - 安全
---

# 限速管理

「运维中心 → 限速管理」是**规则管理与影响预览**的工作台：定义函数/服务维度的 QPS 规则、灰度百分比，并用实时数据预览影响面，再落到执行侧。

## 模型（`rate_limits` 表）

| 字段                   | 说明                                         |
| ---------------------- | -------------------------------------------- |
| `rateLimitId` / `name` | 标识与展示名                                 |
| `resource`             | `function` / `api` / `user`                  |
| `limit` / `window`     | 窗口内请求数 / 窗口秒数（合成 QPS）          |
| `action`               | `reject`（超限拒绝）/ `throttle`（平滑降速） |
| `rules`                | 规则组 JSON：`{scope(function                | service), key, limitQps, match(labels), percent}` |
| `status`               | 启停                                         |

规则形态（`PUT /api/v1/rate-limits` 整组写入）：

```json
{
  "rateLimitId": "mail.send.guard",
  "name": "批量发信保护",
  "resource": "function",
  "limit": 100,
  "window": 1,
  "action": "reject",
  "rules": [
    {
      "scope": "function",
      "key": "mail.send",
      "limitQps": 50,
      "match": {},
      "percent": 100
    },
    {
      "scope": "service",
      "key": "game-node-1",
      "limitQps": 200,
      "match": { "env": "prod" },
      "percent": 30
    }
  ]
}
```

- `scope: function` 按函数 ID 限；`scope: service` 按 Agent/服务实例限
- `match` 按 Agent labels 匹配；`percent` 灰度比例（30 = 仅三成流量受此规则约束）

## 影响预览

`POST /api/v1/rate-limits/preview`：基于注册表实时数据返回规则命中面——匹配到的 Agent、当前 `qps` / `qps1m`，帮助上线前回答「这条规则会打到谁、现有流量是否已经超限」。

## 执行侧（重要边界）

**规则表本身不被 HTTP 中间件消费**——实际限速引擎在 `internal/platform/ratelimit`（令牌桶/分布式限速器），当前接入点是 OpenAPI 网关侧（`openapi` 配置的 `rateLimit.requestsPerMinute` / `burstSize`）。本页面的定位：

1. 规则的**登记、审计、预览**中心
2. 生效需在执行侧（OpenAPI provider 配置或游戏侧 SDK）引用对应规则

函数调用的硬保护另有通道：注册表/审批的高危函数治理与平台级 API 限流，勿混同本页配置。

## 运维建议

- 先 `preview` 后落规则：避免对高流量函数直接全量 `reject`
- 灰度：`percent` 从 10% 起步观察，再逐步放大
- `throttle` 适合可排队的批量操作；对外 API 用 `reject` 语义更清晰
