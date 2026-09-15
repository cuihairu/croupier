---
title: Agent Providers（OpenAPI Provider）
icon: plug
order: 6
category:
  - 集成指南
tag:
  - Agent
  - providers.yaml
  - OpenAPI
---

# Agent Providers（OpenAPI Provider）

> **状态**：Current —— `providers.yaml` 是 Agent 侧把外部 HTTP API 接入 Croupier 的一等配置；本文是其完整参考。

Agent 的 `providers.yaml`（位于 Agent 配置目录，可用 `CROUPIER_CONFIG_DIR` 指定）把第三方 HTTP API 声明为 **openapi provider**：Agent 启动时拉取 OpenAPI 文档，把每个 operation 注册为平台函数，控制台目录/调用/OpenAPI Source 绑定均按普通函数使用，无需游戏方编写任何 Croupier 专有代码。

```text
providers.yaml ──► Agent openapi provider ──► 拉取 openapiSpec
                       │
                       ▼
        注册函数 <provider>.<operationId>（service: provider:<provider>）
                       │
                       ▼
     调用链：控制台/页面 ─► Server ─► Agent ─► baseUrl（真实 HTTP API）
```

## 配置结构

```yaml
providers:
  players: # provider 名；函数 ID 前缀（players.player.list）
    enabled: true
    type: openapi # 目前唯一支持的类型
    game_id: default # 归属游戏（与请求 scope 一致才可调用）
    env: dev # 归属环境
    config:
      baseUrl: "http://127.0.0.1:8091" # 上游 API 根地址
      openapiSpec: "http://127.0.0.1:8091/openapi.json" # OpenAPI 3.0 文档 URL
      timeout: "5s" # 可选；默认 30s（"500ms"/"1m" 均可）
      headers: # 可选；全部请求附带的默认 header
        X-Request-Source: croupier
      auth: # 可选；鉴权见下表
        type: bearer
        token: ${PLAYERS_API_TOKEN}
```

### 字段参考

| 字段                       | 必填                     | 说明                                         |
| -------------------------- | ------------------------ | -------------------------------------------- |
| `providers.<name>.enabled` | 是                       | `false` 时跳过加载                           |
| `providers.<name>.type`    | 是                       | 仅支持 `openapi`                             |
| `providers.<name>.game_id` | 建议                     | 归属游戏 ID；为空时跟随 Agent 注册 scope     |
| `providers.<name>.env`     | 建议                     | 归属环境（prod/stage/test/dev…）             |
| `config.baseUrl`           | 是                       | 上游 API 根地址，路径直接拼接                |
| `config.openapiSpec`       | 与 `openapiSpecs` 二选一 | OpenAPI 文档 URL                             |
| `config.openapiSpecs`      | 与 `openapiSpec` 二选一  | 多文档 URL 列表（合并注册）                  |
| `config.timeout`           | 否                       | 上游调用超时，Go duration 字符串，默认 `30s` |
| `config.headers`           | 否                       | 全部请求附带的默认 header                    |
| `config.auth`              | 否                       | 鉴权配置，见下                               |

### 鉴权（`config.auth`）

| `type`         | 字段                                                                   | 行为                            |
| -------------- | ---------------------------------------------------------------------- | ------------------------------- |
| `none`（默认） | —                                                                      | 不附加鉴权                      |
| `bearer`       | `token`                                                                | `Authorization: Bearer <token>` |
| `basic`        | `username` / `password`                                                | HTTP Basic                      |
| `api_key`      | `api_key.name` / `api_key.value` / `api_key.in`（`header` 或 `query`） | 按位置附加 API key              |
| `custom`       | `custom_headers`                                                       | 附加任意自定义 header 集合      |

配置值支持 `${ENV_VAR}` 环境变量展开（如上例 `token`），secret 不必写死在文件里。

## 函数注册规则

- **函数 ID**：`<provider名>.<operationId>`，例如 provider `players` + `operationId: player.list` → `players.player.list`。`operationId` 必须稳定——改名等于换函数。
- **契约来源**：summary / description / inputSchema（request body）/ outputSchema（response）全部来自 OpenAPI 文档。
- **capability 推导**（method + path 形状，高置信度）：

  | 形状                                    | capability           |
  | --------------------------------------- | -------------------- |
  | `GET /{resource}`                       | `collection_query`   |
  | `GET /{resource}/{id}`                  | `item_query`         |
  | `POST /{resource}`                      | `create`             |
  | `PUT/PATCH /{resource}/{id}`            | `update`             |
  | `DELETE /{resource}/{id}`               | `delete`             |
  | 其他（如 `POST /{resource}/{id}/kick`） | `action`（低置信度） |

  可用 `x-capability` / `x-resource` / `x-operation` 显式覆盖；`x-execution: task` 声明异步任务执行；`x-risk` / `x-permission` 声明治理字段；`x-approval: required` 声明审批要求。

- **分页**：collection 接口的请求带 `page`/`page_size` 参数、响应为 `{items, total}` 形状时，生成的资源页自动带分页绑定。
- **注册时机**：Agent 启动加载 `providers.yaml`；通过扩展安装下发的 provider 走同一注册路径（`SyncExtensionProviders`），与静态文件同名冲突时默认静态优先（`CROUPIER_EXTENSION_PROVIDER_OVERRIDE_STATIC=1` 可反转）。`CROUPIER_EXTENSION_PROVIDERS_ONLY=1` 时跳过静态文件。

## 与 OpenAPI Source 的关系

Agent provider 注册的是**运行时函数**；Dashboard 的「OpenAPI Source」导入的是**契约候选**。两者通过函数 ID 关联：Source 绑定 `kind: provider`、`functionId: players.player.list` 后，契约物化为 FunctionContract（source=`openapi`）并可生成页面 Proposal。详见 [OpenAPI 函数注册](openapi-registration.md)。

Dashboard「OpenAPI Sources」页的**运行时导入**区块读取当前 scope 下的 provider 会话（含来源 Agent、函数清单与最近心跳），其数据来自：

```http
GET /api/v1/openapi/runtime-sources   # 认证 + X-Game-ID/X-Env scope
```

绑定弹窗的函数候选同样包含这些运行时函数（标注导入 Agent）。

## 本地验证

仓库自带可运行的端到端示例：

```bash
go run ./examples/openapi-provider -server 127.0.0.1:19090 -http 127.0.0.1:8091
```

启动后控制台函数目录（default/dev）应出现 `players.player.list` 等 6 个函数。示例说明见仓库 `examples/openapi-provider/README.md`。

自托管部署（docker-compose.deploy.yml）默认随 `sdk-examples` profile 常驻该示例的容器版（镜像 `croupier-openapi-provider-demo`，上游走 `haproxy:19090`，scope 由 `CROUPIER_SDK_EXAMPLE_GAME_ID/ENV` 注入）——Dashboard「OpenAPI Sources」页的运行时导入区块开箱即有数据，无需手工拉起。

## 边界

- OpenAPI 文档只描述 API 契约与能力语义；页面 schema、菜单、多语言、按钮位置等 UI 信息一律不允许进入，导入器遇到会报 diagnostics（见 [OpenAPI 输入边界](openapi-registration.md#openapi-输入边界)）。
- 文档支持外部 URL 拉取（provider 场景）；Dashboard OpenAPI Source 上传则必须内联（≤2MiB，拒绝外部 `$ref`）——两条路径的边界不同，注意区分。
