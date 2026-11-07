---
title: 分层
---

# 分层

这是 2026-11-07、提交 `eec3592912d9c0156574495bb090caf3fd78c794` 时的实现分层模型。层是运行时职责，不是目录名。以下边界说明状态归属、使用的通道、配置入口，以及进程或会话如何启动和停止。

## 层清单

| #   | 层              | 职责                                                                       | 代表实现                                                                                                     |
| --- | --------------- | -------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------ |
| 1   | Web 控制台      | 完成操作员认证，在浏览器保存范围与令牌，渲染页面并发起 REST/SSE 调用。     | `web/src/app.tsx:113`; `web/src/services/core/http.ts:40`                                                    |
| 2   | Server HTTP API | 校验身份与范围，把 REST 请求路由到应用服务，并管理 HTTP 进程生命周期。     | `cmd/server/root.go:402`; `internal/handler/routes.go:89`                                                    |
| 3   | Server 应用服务 | 执行授权、审批、审计、契约、页面、注册表操作与派发用例。                   | `internal/svc/service_context.go:221`; `internal/platform/functioninvoke/govern.go:63`                       |
| 4   | 存储与协调      | 维护持久化记录、缓存、分析存储、Owner 选举和文件任务路由。                 | `internal/svc/migrations.go:186`; `internal/cache/helper.go:15`; `internal/cluster/owner_store.go:8`         |
| 5   | Agent 运行时    | 拨号 Server，维持注册与心跳，执行采集、探测、主机操作和托管进程。          | `internal/app/agent/app.go:184`; `agents/capture/runner/runner.go:201`; `agents/devops/session/broker.go:92` |
| 6   | 会话传输        | 对外向 Agent 会话做帧封装与多路复用，并定义控制、调用、任务和运维消息 ID。 | `core/transport/tcp/framing.go:25`; `pkg/protocol/message.go:51`                                             |
| 7   | Provider 会话   | 在本地终结 SDK provider 连接，按健康实例路由，调用处理器并适配插件协议。   | `internal/agent/local_handler.go:91`; `agents/sidecar/funcplugin/protocol/funcplugin.proto:24`               |
| 8   | 页面/控制台 DSL | 建模草稿、已发布页、菜单和选择器绑定，只执行发布时冻结的绑定。             | `internal/dashboard/spec/types.go:537`; `internal/api/console/service.go:313`                                |

## 术语表

| 术语              | 英文                   | 定义                                                                 | 证据                                                                                                                       |
| ----------------- | ---------------------- | -------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------- |
| 操作员范围        | Operator scope         | Web 请求使用的当前 game/env 选择。                                   | `web/src/services/core/scope.ts:56`                                                                                        |
| Provider 令牌     | Provider token         | 登录后保存在浏览器的认证令牌。                                       | `web/src/app.tsx:113`                                                                                                      |
| Scoped route      | Scoped route           | 通过 `GameDBMiddleware` 选择 game 数据库的 REST 路由组。             | `internal/handler/routes.go:150`                                                                                           |
| HTTP 生命周期     | HTTP lifecycle         | 由 SIGINT/SIGTERM 驱动的 REST 启动、排空、后台停止和连接池关闭序列。 | `cmd/server/root.go:417`; `cmd/server/root.go:440`                                                                         |
| Service context   | Service context        | Server 的组合根，管理数据库、缓存、注册表、审批、审计和后台任务。    | `internal/svc/service_context.go:221`                                                                                      |
| 执行治理          | Execution governance   | HTTP、任务和调度共用的禁用、分配、主体、ACL、政策、输入和审批链。    | `internal/platform/functioninvoke/govern.go:63`                                                                            |
| 函数契约          | Function contract      | 调用方式、schema、风险、权限和审批的持久化元数据。                   | `internal/model/function_contract.go:15`                                                                                   |
| 任务运行          | Task run               | 异步函数运行的持久化状态机。                                         | `internal/platform/dispatch/dispatcher.go:54`                                                                              |
| 派发              | Dispatch               | 在向 Agent 发送请求前选择会话并创建持久化运行的服务端流程。          | `internal/platform/dispatch/dispatcher.go:336`; `internal/platform/dispatch/dispatcher.go:775`                             |
| 关系模型存储      | Relational model store | 由 GORM 模型和迁移定义的 MySQL/PostgreSQL/SQLite 模型层。            | `internal/svc/migrations.go:186`; `internal/model/task.go:8`                                                               |
| 缓存存储          | Cache store            | 依据配置选择的 Redis 或进程内缓存。                                  | `internal/cache/helper.go:15`; `internal/cache/helper.go:41`                                                               |
| 分析落地          | Analytics sink         | 分析 worker 背后的 ClickHouse 批量写入。                             | `internal/analytics/worker/worker.go:394`; `internal/analytics/worker/worker.go:435`                                       |
| Cluster owner     | Cluster owner          | 用共享关系库或 Redis 把范围所有权分配给一个 Server 实例。            | `internal/cluster/owner_store.go:8`; `internal/cluster/owner_redis.go:27`                                                  |
| 文件任务路由      | File task routing      | 把 task ID 映射到 Agent 的 JSON 文件或内存适配器。                   | `internal/platform/dispatch/task_routing_store.go:28`; `internal/platform/dispatch/task_routing_store.go:162`              |
| Agent 运行时      | Agent runtime          | 包含注册、心跳、本地服务和角色 runner 的外向进程。                   | `internal/app/agent/app.go:138`; `internal/app/agent/app.go:184`                                                           |
| Agent 会话        | Agent session          | Agent 主动建立的 Server TCP 会话，承载控制和执行流量。               | `internal/app/agent/app.go:190`; `internal/server/control_handler.go:128`                                                  |
| 采集闸门          | Capture gate           | 采集运行时对数据源、过滤和持久投递的策略。                           | `agents/capture/runner/runner.go:201`; `agents/capture/runner/runner.go:272`                                               |
| DevOps 准入       | DevOps admission       | 打开主机会话前必须通过的审计和准入检查。                             | `agents/devops/session/broker.go:92`; `agents/devops/session/broker.go:126`                                                |
| 线上帧            | Wire frame             | 传输双方共享的 4 字节大端长度前缀帧。                                | `core/transport/tcp/framing.go:25`                                                                                         |
| 消息 ID           | Message ID             | 24 位线上操作标识，覆盖注册、调用、任务和运维族。                    | `pkg/protocol/message.go:51`; `pkg/protocol/message.go:71`                                                                 |
| Provider manager  | Provider manager       | sidecar 内保存 SDK provider 会话和健康窗口的注册表。                 | `internal/agent/local_handler.go:52`; `internal/agent/local_handler.go:34`                                                 |
| Provider 调用     | Provider invocation    | 本地请求选择健康 provider 并调用其注册处理器。                       | `internal/agent/local_handler.go:290`; `internal/agent/tcp_local_listener.go:28`                                           |
| Function plugin   | Function plugin        | sidecar 的 gRPC 插件契约，用于描述和调用插件函数。                   | `agents/sidecar/funcplugin/protocol/funcplugin.proto:24`; `agents/sidecar/funcplugin/protocol/funcplugin.proto:62`         |
| Probe plugin      | Probe plugin           | synthetics 的 go-plugin gRPC 契约，用于描述和运行探测。              | `agents/synthetics/pluginhost/protocol/probeplugin.proto:18`; `agents/synthetics/pluginhost/protocol/probeplugin.proto:30` |
| PageSpec          | PageSpec               | 强类型页面草稿模型，包含布局、本地化文案和绑定。                     | `internal/dashboard/spec/types.go:537`; `internal/dashboard/spec/types.go:18`                                              |
| Published page    | Published page         | 冻结范围、版本、绑定和契约摘要的不可变快照。                         | `internal/dashboard/spec/types.go:797`; `internal/dashboard/spec/types.go:823`                                             |
| Console menu      | Console menu           | 由已发布页和菜单记录生成的运行时导航模型。                           | `internal/dashboard/spec/types.go:850`; `internal/api/console/service.go:62`                                               |
| Binding execution | Binding execution      | 已发布快照的执行路径，映射输入、调用函数并包装唯一允许的响应形态。   | `internal/api/console/service.go:313`; `internal/api/console/service.go:413`                                               |

## 分层解说

### 1. Web 控制台

浏览器层负责登录、操作员选择的 `gameId + env` 以及 REST/SSE 访问，不打开 Agent 会话。UI 运行时从浏览器存储读取令牌（`web/src/app.tsx:113`），HTTP 服务注入范围头（`web/src/services/core/scope.ts:64`; `web/src/services/core/http.ts:40`）。配置来自 Umi 环境和构建配置；路由声明在 `web/config/routes.ts`。状态是浏览器内的瞬态状态。

### 2. Server HTTP API

`runServer` 读取 YAML 和 CLI 覆盖，校验集群 TLS，构建 service context，注册路由，启动调度器与 REST/TCP 进程，最后按序停机（`cmd/server/root.go:124`; `cmd/server/root.go:202`; `cmd/server/root.go:440`）。`RegisterHandlers` 是路由表（`internal/handler/routes.go:89`），scoped 组挂载 `GameDBMiddleware`（`internal/handler/routes.go:150`）。REST 同步调用应用服务；SSE 使用长写超时 HTTP 服务（`cmd/server/root.go:409`）。schema 在 `internal/config/config.go:13`，种子示例在 `configs/`。

### 3. Server 应用服务

`NewServiceContext` 组合关系模型、缓存、注册表、审批、审计、调度器和领域服务（`internal/svc/service_context.go:221`; 审批 `internal/svc/service_context.go:366`; 审计 `internal/svc/service_context.go:374`）。统一调用入口是 `functioninvoke.Invoke`（`internal/platform/functioninvoke/invoke.go:26`），其 `Govern` 链被 HTTP、任务启动、页面绑定和调度共用（`internal/platform/functioninvoke/govern.go:63`）。领域状态是关系状态，执行队列和缓存除外。启动时初始化分层配置（`cmd/server/root.go:205`），有效键在 `internal/platform/settings/layered.go:187`。

### 4. 存储与协调

该层区分权威关系状态、可选缓存、分析、协调和文件适配器。GORM 迁移定义关系 schema（`internal/svc/migrations.go:186`）。缓存按配置选择（`internal/cache/helper.go:15`）。分析 worker 批量写 ClickHouse（`internal/analytics/worker/worker.go:435`），进程在 `internal/analytics/worker/worker.go:106` 读环境输入，ingest 校验在 `cmd/ingest/cmd/root.go:152`。Owner 可用共享关系库或 Redis（`internal/cluster/owner_store.go:8`; `internal/cluster/owner_redis.go:27`）。任务路由可用 JSON 文件或内存（`internal/platform/dispatch/task_routing_store.go:34`; `internal/platform/dispatch/task_routing_store.go:162`）。

### 5. Agent 运行时

sidecar 运行时创建 Server 连接并启动本地 provider listener（`internal/app/agent/app.go:184`; `internal/app/agent/app.go:190`）。capture 构建闸门、应用闸门、读取源、消费持久记录并记录命中（`agents/capture/runner/runner.go:201`; `agents/capture/runner/runner.go:423`; `agents/capture/runner/runner.go:565`）。synthetics 有独立 runner 和插件宿主；DevOps 必须先通过审计准入再打开主机会话（`agents/devops/session/broker.go:126`; 会话上限在 `agents/devops/session/hub.go:98`）。YAML 示例在 `configs/agent.yaml`、`configs/capture-agent.example.yaml`、`configs/synthetics-agent.example.yaml` 和 `configs/devops-agent.example.yaml`。

### 6. 会话传输

传输双方共享长度前缀帧（`core/transport/tcp/framing.go:25`）、`core/transport/tcp/mux_conn.go` 中的请求多路复用，以及固定消息头（`pkg/protocol/message.go:33`）。消息族在 `pkg/protocol/message.go:51` 枚举，调用和任务 ID 在 `pkg/protocol/message.go:71`。protobuf 契约在 `proto/croupier/`，包括 `proto/croupier/sdk/v1/provider.proto`、`proto/croupier/agent/v1/register.proto`、`proto/croupier/agent/v1/task.proto`、`proto/croupier/ops/v1/ops.proto` 和 `proto/croupier/function/v1/metadata.proto`。`configs/` 下没有另一份复制式协议 schema。

### 7. Provider 会话

SDK provider 拨号 sidecar；sidecar 在本地终结并保存会话，选择健康实例，调用处理器（`internal/agent/tcp_local_listener.go:28`; `internal/agent/local_handler.go:290`）。插件后端使用不同契约：function plugin 在 `agents/sidecar/funcplugin/protocol/funcplugin.proto:24`，probe plugin 在 `agents/synthetics/pluginhost/protocol/probeplugin.proto:18`。provider 描述符及心跳/排空行为定义在 `proto/croupier/sdk/v1/provider.proto:11` 和 `proto/croupier/sdk/v1/provider.proto:69`。会话状态属于进程，除非显式导出到注册表。

### 8. 页面/控制台 DSL

`PageSpec` 是强类型草稿，`PublishedPageSpec` 冻结发布事实和绑定契约（`internal/dashboard/spec/types.go:537`; `internal/dashboard/spec/types.go:797`）。草稿持久化和发布是应用用例（`internal/api/page/service.go:319`; `internal/api/page/service.go:924`）。运行时菜单生成和绑定执行只消费已发布状态（`internal/api/console/service.go:62`; `internal/api/console/service.go:313`），幂等执行在 `internal/api/console/service.go:413`。该 DSL 没有独立文案包：`LocalizedText` 把 locale key 映射到字符串（`internal/dashboard/spec/types.go:18`）；generator 通过 `internal/dashboard/generator/generator.go:29` 消费 `configs/term_dictionary.json`。

## 为什么这样拆

1. **浏览器是信任与会话边界。** 令牌和范围保存在浏览器并随 HTTP 发送（`web/src/app.tsx:113`; `web/src/services/core/scope.ts:64`），服务端仍独立认证并挂载 scoped 数据库中间件（`internal/handler/routes.go:150`）。
2. **HTTP 路由不等于领域执行。** 路由层只注册端点和中间件（`internal/handler/routes.go:89`），而同一条治理链刻意被 HTTP、任务、调度和页面复用（`internal/platform/functioninvoke/govern.go:63`）；合并它们会造成授权和审批规则分叉。
3. **应用状态和执行不是同一件事。** 派发在发送 Agent 前创建持久化任务运行（`internal/platform/dispatch/dispatcher.go:775`），service context 管缓存和关系生命周期（`internal/svc/service_context.go:221`）。保持边界后，持久化和重试不会依赖路由或协议选择。
4. **持久化、缓存、分析和协调的语义不同。** GORM 迁移管理关系 schema（`internal/svc/migrations.go:186`），缓存可替换（`internal/cache/helper.go:15`），ClickHouse 批量写（`internal/analytics/worker/worker.go:435`），Owner 租约使用 TTL（`internal/cluster/owner_redis.go:27`）。只叫一个“存储层”会掩盖这些差异。
5. **Agent 是独立故障域。** capture 闸门、probe 插件和经审计的 DevOps 会话运行在独立进程，失败不会直接终止 REST 服务（`agents/capture/runner/runner.go:201`; `agents/synthetics/pluginhost/protocol/probeplugin.proto:18`; `agents/devops/session/broker.go:126`）。
6. **传输是共享机制，不是领域服务。** Agent 帧和请求 ID 被注册、调用、任务和运维流量复用（`core/transport/tcp/framing.go:25`; `pkg/protocol/message.go:51`），Server 领域处理器不必重复实现帧协议。
7. **Provider 连接是本地且按健康范围路由。** provider listener 拥有本地连接和健康窗口（`internal/agent/tcp_local_listener.go:28`; `internal/agent/local_handler.go:34`），SDK 与 synthetics 的插件协议不同（`agents/sidecar/funcplugin/protocol/funcplugin.proto:24`; `agents/synthetics/pluginhost/protocol/probeplugin.proto:18`）。
8. **页面草稿与可执行页面的保证不同。** `PageSpec` 支持编辑和校验，`PublishedPageSpec` 冻结版本、发布事实和绑定契约（`internal/dashboard/spec/types.go:537`; `internal/dashboard/spec/types.go:797`）；运行时基于冻结摘要拒绝过期绑定（`internal/dashboard/spec/types.go:823`）。
9. **本地化文案属于页面数据，不属于渲染器代码。** `LocalizedText` 是页面模型的一部分（`internal/dashboard/spec/types.go:18`），生成时可从术语字典取文案（`internal/dashboard/generator/generator.go:29`）；改语言是数据变更，不是渲染器部署。

## 相关页面

- [架构总览](/zh/architecture/)
- [运行拓扑](/zh/architecture/runtime-topology)
- [术语与分层](/zh/architecture/terms-and-layering)
