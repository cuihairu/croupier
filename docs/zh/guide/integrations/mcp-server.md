---
title: MCP Server（AI Agent 工具）
icon: robot
order: 7
category:
  - 集成指南
tag:
  - MCP
  - AI agent
  - 只读
---

# MCP Server（AI Agent 工具）

> **状态**：Current（只读批）—— `croupier-server mcp` 是官方 Model Context Protocol 入口。本批**只交付只读工具 + 同步函数调用**；风险写工具等 GAME-009/010 凭据协议落地后再出。

`croupier-server mcp` 运行 [Model Context Protocol](https://modelcontextprotocol.io) stdio server（Go 官方 `modelcontextprotocol/go-sdk` 实现）。AI 宿主 spawn 本进程，经 stdin/stdout 说 MCP，即可获得一个带 scope、可审计的控制面视图。本进程是**薄 REST 转发客户端**：每次工具调用都以操作者 Bearer JWT 与固定游戏 scope 转发到运行中的 croupier-server REST API——MCP 层不另立第二真值。

```text
AI 宿主 ──MCP/stdio──► croupier-server mcp ──REST(Bearer + X-Game-ID/X-Env)──► croupier-server
                          │
                          ▼
              GET /api/v1/ops/agents · GET /api/v1/tasks · POST /api/v1/functions/{id}/invoke
```

## 工具清单（全量，如实缺位）

| 工具              | 对应面                               | 说明                                                                                                   |
| ----------------- | ------------------------------------ | ------------------------------------------------------------------------------------------------------ |
| `list_sessions`   | `GET /api/v1/ops/agents`             | 已注册 agent 会话：连接状态/版本/承载函数。无入参——scope 由连接参数决定。                              |
| `query_jobs`      | `GET /api/v1/tasks`                  | 分页任务记录。过滤：`status`、`functionId`、`page`、`size`。                                           |
| `invoke_function` | `POST /api/v1/functions/{id}/invoke` | **仅 sync 模式。** 入参：`id`（必填）、`params`（对象）、`timeoutMs`（服务端 clamp 到 [1000,60000]）。 |

除这三个工具外没有隐藏工具。

### 有意缺位：写工具

游戏侧写操作（封禁/补偿/订单变更）**不**在本批 MCP 面内。它们必须等 GAME-009/010 凭据协议（逐操作受控凭据 + 显式审批流）落地后才允许 AI agent 触发。缺位是有意的，不会以占位假工具粉饰。

`invoke_function` 是唯一可能造成游戏侧副作用的工具：它经既有注册调用链执行已注册函数，继承平台的 RBAC/审批/审计规则，且固定 sync 模式——不暴露异步任务创建，AI 面上不留 fire-and-forget 通道。

## 配置

MCP 进程需要一个运行中的 croupier-server、一个操作者 JWT 与固定游戏 scope：

| Flag           | 环境变量                  | 含义                                            |
| -------------- | ------------------------- | ----------------------------------------------- |
| `--server-url` | `CROUPIER_MCP_SERVER_URL` | 基地址，如 `http://127.0.0.1:18780`（**必填**） |
| `--token`      | `CROUPIER_MCP_TOKEN`      | 操作者 Bearer JWT（**必填**）                   |
| `--game-id`    | `CROUPIER_MCP_GAME_ID`    | 每次工具调用的 `X-Game-ID` scope 头             |
| `--env`        | `CROUPIER_MCP_ENV`        | 每次工具调用的 `X-Env` scope 头                 |

scope 在**进程启动时固定**：AI 宿主不能按调用跳游戏/环境。请求携带操作者身份，每次调用都落入该操作者名下的既有审计链。

宿主配置示例（`mcpServers` 段）：

```json
{
  "mcpServers": {
    "croupier": {
      "command": "croupier-server",
      "args": ["mcp"],
      "env": {
        "CROUPIER_MCP_SERVER_URL": "http://127.0.0.1:18780",
        "CROUPIER_MCP_TOKEN": "<operator jwt>",
        "CROUPIER_MCP_GAME_ID": "demo",
        "CROUPIER_MCP_ENV": "prod"
      }
    }
  }
}
```

## 语义与边界

- **透传而非影子契约**：工具输出即 REST payload 原文（JSON text content）。响应形状只有一处真值——REST API——MCP 消费方看到的就是它。
- **错误以工具错误呈现**：非 2xx 的 REST 响应（含统一错误对象 `error`/`message`）以 `isError: true` 工具结果返回，AI 宿主可读到稳定码并自纠；不会打断协议。
- **stdio 纪律**：MCP 帧走 stdin/stdout；诊断信息一律走 stderr。
- **已知边界（诚实列出）**：无写工具（见上）；无异步任务工具；`list_sessions` 无按调用 scope 覆盖；工具描述为英文（由命令源码生成）。
