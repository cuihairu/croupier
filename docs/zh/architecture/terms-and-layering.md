---
title: 术语与分层
---

# 术语与分层

本术语表是 [分层](/zh/architecture/layers) 的抽象层源码索引补充，审阅基准为 2026-11-07、提交 `eec3592912d9c0156574495bb090caf3fd78c794`。术语按运行时职责排列，不按包名排列。

## 术语

| 术语              | 英文                   | 定义                                                          | file:line                                                                                      |
| ----------------- | ---------------------- | ------------------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| 操作员范围        | Operator scope         | 浏览器选择的 game/env，随 Web 请求发送。                      | `web/src/services/core/scope.ts:56`                                                            |
| Provider 令牌     | Provider token         | 登录后保存在浏览器的认证令牌。                                | `web/src/app.tsx:113`                                                                          |
| HTTP 生命周期     | HTTP lifecycle         | 信号驱动的 REST 启动、排空、后台停止和连接池关闭。            | `cmd/server/root.go:417`; `cmd/server/root.go:440`                                             |
| Scoped route      | Scoped route           | 由 `GameDBMiddleware` 选择 game 数据库的 REST 路由组。        | `internal/handler/routes.go:150`                                                               |
| Service context   | Service context        | Server 组合根，管理关系模型、缓存、注册表、审批、审计和任务。 | `internal/svc/service_context.go:221`                                                          |
| 执行治理          | Execution governance   | 所有执行入口共用的禁用/分配/主体/ACL/政策/输入/审批链。       | `internal/platform/functioninvoke/govern.go:63`                                                |
| 函数契约          | Function contract      | 持久化的调用、schema、风险、权限和审批元数据。                | `internal/model/function_contract.go:15`                                                       |
| 任务运行          | Task run               | 一次异步函数运行的持久化状态机。                              | `internal/platform/dispatch/dispatcher.go:54`                                                  |
| 派发              | Dispatch               | 向 Agent 请求前完成 Agent 选择和持久化运行创建。              | `internal/platform/dispatch/dispatcher.go:336`; `internal/platform/dispatch/dispatcher.go:775` |
| 关系模型存储      | Relational model store | 由 GORM 模型和迁移定义的 MySQL/PostgreSQL/SQLite 状态。       | `internal/svc/migrations.go:186`; `internal/model/task.go:8`                                   |
| 缓存存储          | Cache store            | 依据 Server 配置选择的 Redis 或进程内缓存。                   | `internal/cache/helper.go:15`                                                                  |
| 分析落地          | Analytics sink         | 分析 worker 背后的 ClickHouse 批量写入。                      | `internal/analytics/worker/worker.go:435`                                                      |
| Cluster owner     | Cluster owner          | 用共享数据库或 Redis 租约把范围所有权分配给一个实例。         | `internal/cluster/owner_store.go:8`; `internal/cluster/owner_redis.go:27`                      |
| 文件任务路由      | File task routing      | 把 task ID 映射到 Agent 的 JSON 文件或内存适配器。            | `internal/platform/dispatch/task_routing_store.go:28`                                          |
| Agent 运行时      | Agent runtime          | 拥有注册、心跳、本地服务和角色 runner 的外向进程。            | `internal/app/agent/app.go:184`                                                                |
| Agent 会话        | Agent session          | Agent 主动建立的 Server TCP 会话，承载控制和执行流量。        | `internal/app/agent/app.go:190`; `internal/server/control_handler.go:128`                      |
| 采集闸门          | Capture gate           | capture 对数据源、过滤和持久投递的策略。                      | `agents/capture/runner/runner.go:201`                                                          |
| DevOps 准入       | DevOps admission       | 打开主机会话前必须通过的审计/准入闸门。                       | `agents/devops/session/broker.go:126`                                                          |
| 线上帧            | Wire frame             | 共享的 4 字节大端长度前缀帧。                                 | `core/transport/tcp/framing.go:25`                                                             |
| 消息 ID           | Message ID             | 覆盖注册、调用、任务和运维族的 24 位操作 ID。                 | `pkg/protocol/message.go:51`; `pkg/protocol/message.go:71`                                     |
| Provider manager  | Provider manager       | sidecar 保存 SDK provider 会话和健康窗口的注册表。            | `internal/agent/local_handler.go:52`                                                           |
| Provider 调用     | Provider invocation    | 先选择健康 provider，再调用其处理器。                         | `internal/agent/local_handler.go:290`                                                          |
| Function plugin   | Function plugin        | sidecar 的 gRPC 契约，用于描述和调用插件函数。                | `agents/sidecar/funcplugin/protocol/funcplugin.proto:24`                                       |
| Probe plugin      | Probe plugin           | synthetics 的 go-plugin 契约，用于描述和运行探测。            | `agents/synthetics/pluginhost/protocol/probeplugin.proto:18`                                   |
| PageSpec          | PageSpec               | 强类型页面草稿模型，包含布局、本地化文案和绑定。              | `internal/dashboard/spec/types.go:537`; `internal/dashboard/spec/types.go:18`                  |
| Published page    | Published page         | 冻结范围、版本、绑定和契约摘要的不可变快照。                  | `internal/dashboard/spec/types.go:797`; `internal/dashboard/spec/types.go:823`                 |
| Console menu      | Console menu           | 由已发布页和菜单记录生成的运行时导航。                        | `internal/dashboard/spec/types.go:850`; `internal/api/console/service.go:62`                   |
| Binding execution | Binding execution      | 已发布快照的执行路径，映射输入并调用函数。                    | `internal/api/console/service.go:313`; `internal/api/console/service.go:413`                   |

## 保留边界

- `gameId + env` 是逻辑范围；物理位置属于 target、agent 和 node 记录。
- Agent 拥有到 Server 的外向会话；SDK provider 拥有到 sidecar 的本地连接。
- 函数是可执行元数据，不是页面或权限；页面只执行已冻结的绑定契约。
- 已发布页不可变；草稿不会进入运行时导航。

## 相关页面

- [分层](/zh/architecture/layers)
- [架构总览](/zh/architecture/)
- [运行拓扑](/zh/architecture/runtime-topology)
