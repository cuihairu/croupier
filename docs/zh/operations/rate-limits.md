---
title: 限流管理
icon: throttle
order: 42
category:
  - 运维手册
tag:
  - 安全
---

# 限流管理

「运营中心 → 限流管理」是**规则管理与影响预览**的工作台：定义函数/服务维度的 QPS 规则与放量百分比，用实时数据预览影响面，再落到执行侧生效。

## 模型（`rate_limits` 表）

| 字段                   | 说明                                                                               |
| ---------------------- | ---------------------------------------------------------------------------------- |
| `rateLimitId` / `name` | 标识与显示名                                                                       |
| `resource`             | `function` / `api` / `user`                                                        |
| `limit` / `window`     | 窗口内请求数 / 窗口秒数（隐含 QPS）                                                |
| `action`               | `reject`（超限拒绝）/ `throttle`（平滑降速）                                       |
| `rules`                | 规则组 JSON：`{scope(function 或 service), key, limitQps, match(labels), percent}` |
| `status`               | 启用/停用                                                                          |

规则形态（`PUT /api/v1/rate-limits` 整组写入）：

```json
{
  "rateLimitId": "mail.send.guard",
  "name": "Bulk mail sending guard",
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

- `scope: function` 按函数 ID 限流；`scope: service` 按 Agent/服务实例限流
- `match` 按 Agent 标签匹配；`percent` 是放量百分比（30 = 只对 30% 流量套用该规则）

## 影响预览

`POST /api/v1/rate-limits/preview`：基于注册中心实时数据，返回规则的命中面——匹配到哪些 Agent 及其当前 `qps` / `qps1m`，帮助你在放量前回答「这条规则会打到谁、当前流量是否已经超限」。

## 执行侧（重要边界）

**规则表本身不被 HTTP 中间件消费**——真正的限流引擎在 `internal/platform/ratelimit`（token bucket / 分布式限流器），当前接线点在 OpenAPI 网关侧（`openapi` 配置的 `rateLimit.requestsPerMinute` / `burstSize`）。本页的角色是：

1. 规则**登记、审计与预览**的集中地
2. 要生效需要执行侧（OpenAPI provider 配置或游戏侧 SDK）引用对应规则

函数调用的硬防护另有通道：注册中心/审批流的高风险函数治理与平台级 API 限流，不要与本页配置混为一谈。

## 运营建议

- 落规则前先预览：避免对高流量函数直接上全量 `reject`
- 放量：`percent` 从 10% 起步，观察后再逐步放大
- `throttle` 适合可排队的批处理操作；`reject` 对外公开 API 语义更清晰
