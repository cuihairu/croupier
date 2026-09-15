# OpenAPI Provider 接入示例

演示「游戏方 HTTP 服务」如何通过 Agent 的 `providers.yaml`（`type: openapi`）
把既有 REST API 接入 Croupier：一个进程同时扮演两个角色——

```
players HTTP API（/players CRUD + /openapi.json）
        ▲  HTTP
        │
Croupier Agent（providers.yaml: type=openapi, baseUrl, openapiSpec）
        │  注册 players.<operationId> 函数
        ▼
Croupier Server ──► 函数目录 / 控制台调用 / OpenAPI Source 绑定
```

## 快速开始

```bash
# 1. 启动 croupier-server（控制面 TCP 默认 19090）
make build && ./bin/croupier-server --config configs/server.yaml

# 2. 启动本 demo（HTTP API + 内嵌 Agent）
go run ./examples/openapi-provider -server 127.0.0.1:19090 -http 127.0.0.1:8091
```

启动后：

- demo 自动在临时目录生成 `providers.yaml`（路径见日志）并通过内嵌 Agent 注册；
- 控制台「函数目录」（game/env 与 `-game-id`/`-env` 一致，默认 `default/dev`）
  即可看到 `players.player.list`、`player.get`、`player.create`、
  `player.update`、`player.delete`、`player.kick`；
- 直接调用 `curl http://127.0.0.1:8091/openapi.json` 查看 Agent 拉取的契约。

### 只跑 HTTP API

生产 Agent 或独立 Agent 进程接入时，加 `-no-agent` 只启动 API：

```bash
go run ./examples/openapi-provider -no-agent -http 127.0.0.1:8091
```

然后在 Agent 配置目录写 `providers.yaml`：

```yaml
providers:
  players:
    enabled: true
    type: openapi
    game_id: default
    env: dev
    config:
      baseUrl: "http://127.0.0.1:8091"
      openapiSpec: "http://127.0.0.1:8091/openapi.json"
      timeout: "5s"
```

## 代码导览

| 文件             | 内容                                                                            |
| ---------------- | ------------------------------------------------------------------------------- |
| `main.go`        | demo 入口：HTTP API + 内嵌 Agent（`-no-agent` 可关）；自动生成 `providers.yaml` |
| `players_api.go` | 内存 players CRUD + `/openapi.json`；注释解释 REST 形状 → capability 推导规则   |

## 你的游戏服务如何接入

1. 保证 REST 形状标准（`GET/POST /资源`、`GET/PUT/DELETE /资源/{id}`），
   平台据此推导 collection_query / create / item_query / update / delete；
   非生命周期形状（如 `POST /players/{id}/kick`）归为 action。
2. 每个操作给稳定的 `operationId`——它就是函数名（`<provider>.<operationId>`）。
3. 导出与实现一致的 `/openapi.json`（OpenAPI 3.0）。
4. Agent `providers.yaml` 增加 provider 条目，重启 Agent（或通过扩展安装下发）。

完整字段参考与鉴权配置见
[Agent OpenAPI Provider 指南](../../docs/guide/integrations/agent-providers.md)；
与 Dashboard「OpenAPI Source」绑定的关系见
[OpenAPI 函数注册](../../docs/guide/integrations/openapi-registration.md)。
