---
title: MCP Server (AI Agent Tools)
icon: robot
order: 7
category:
  - Integration Guides
tag:
  - MCP
  - AI agent
  - read-only
---

# MCP Server (AI Agent Tools)

> **Status**: Current (read-only batch) —— `croupier-server mcp` is the official Model Context Protocol entry point. This batch deliberately ships **read-only tools plus sync function invocation**; risky write tools stay out until the GAME-009/010 credential protocol lands.

`croupier-server mcp` runs a [Model Context Protocol](https://modelcontextprotocol.io) stdio server (Go official `modelcontextprotocol/go-sdk`). An AI host spawns the process, speaks MCP over stdin/stdout, and gets a scoped, audited view of the running control plane. The process is a **thin REST client**: every tool call is forwarded to the running croupier-server REST API with the operator's Bearer JWT and fixed game scope — the MCP layer adds no second source of truth.

```text
AI host ──MCP/stdio──► croupier-server mcp ──REST(Bearer + X-Game-ID/X-Env)──► croupier-server
                          │
                          ▼
              GET /api/v1/ops/agents · GET /api/v1/tasks · POST /api/v1/functions/{id}/invoke
```

## Tool list (complete and honest)

| Tool              | Surface                              | Notes                                                                                                                    |
| ----------------- | ------------------------------------ | ------------------------------------------------------------------------------------------------------------------------ |
| `list_sessions`   | `GET /api/v1/ops/agents`             | Registered agent sessions: connection status, version, served functions. No arguments — scope comes from the connection. |
| `query_jobs`      | `GET /api/v1/tasks`                  | Paged task records. Filters: `status`, `functionId`, `page`, `size`.                                                     |
| `invoke_function` | `POST /api/v1/functions/{id}/invoke` | **Sync mode only.** Args: `id` (required), `params` (object), `timeoutMs` (clamped to [1000,60000] server-side).         |

There are no hidden tools beyond these three.

### Deliberately absent: write tools

Game-side write operations (player bans, compensation, order mutations) are **not** exposed over MCP in this batch. They require the GAME-009/010 credential protocol (per-operation scoped credentials with explicit approval flows) before an AI agent may trigger them. The absence is intentional and will not be papered over with placeholder tools.

`invoke_function` is the one tool that can cause game-side effects: it executes a registered function through the existing registry dispatch chain, inherits the platform's RBAC/approval/audit rules, and is capped to sync mode — async task creation is not exposed to keep the AI surface free of fire-and-forget jobs.

## Configuration

The MCP process needs a running croupier-server, an operator JWT, and a fixed game scope:

| Flag           | Environment variable      | Meaning                                                |
| -------------- | ------------------------- | ------------------------------------------------------ |
| `--server-url` | `CROUPIER_MCP_SERVER_URL` | Base URL, e.g. `http://127.0.0.1:18780` (**required**) |
| `--token`      | `CROUPIER_MCP_TOKEN`      | Operator Bearer JWT (**required**)                     |
| `--game-id`    | `CROUPIER_MCP_GAME_ID`    | `X-Game-ID` scope header for every tool call           |
| `--env`        | `CROUPIER_MCP_ENV`        | `X-Env` scope header for every tool call               |

The scope is **fixed at process start**: the AI host cannot hop games or environments per call. Requests carry the operator's identity, so every invocation lands in the existing audit chain under that operator.

Example host configuration (`mcpServers` stanza):

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

## Semantics and boundaries

- **Passthrough, not a shadow contract**: tool outputs are the REST payloads verbatim (JSON text content). Response shapes live in one place — the REST API — and MCP consumers see exactly that.
- **Errors surface as tool errors**: non-2xx REST responses (including the unified error object `error`/`message`) are returned as `isError: true` tool results so the AI host can read the stable code and self-correct; they do not crash the protocol.
- **stdio discipline**: MCP frames go over stdin/stdout; all diagnostics go to stderr.
- **Known boundaries (honest)**: no write tools (see above); no async task tools; `list_sessions` has no per-call scope override; the tool surface is English (descriptions generated from the command source).
