---
title: 当前架构总览
---

# 当前架构总览

Croupier 是面向单公司多游戏、多环境的运营控制面。当前实现采用 **模块化 Server + 按角色部署的出站 Agent + 游戏 SDK**，另有独立的数据分析管道。本页按 2026-10-28 源码装配描述当前态；历史设计中的目标能力不能视为默认已启用。

## 系统图

箭头表示标注的数据流或建连方向。Agent 主动连接 Server，SDK 主动连接 sidecar；建立后的 session 双向承载调用、结果与控制消息。虚线是可选集群协调关系。L7/L4 入口是多实例部署选项，单实例可直接连接。

<ArchitectureDiagram lang="zh" />

## 进程和职责

| 进程/模块                 | 实际职责                                                                       | 边界                                                                |
| ------------------------- | ------------------------------------------------------------------------------ | ------------------------------------------------------------------- |
| Web 控制台                | Umi Max/React/Ant Design；资源目录、Page Studio、发布页面和运营操作            | HTTP API/SSE，不直接持有 Agent 连接                                 |
| Server                    | Gin API、身份权限、scope、政策、审批、审计、契约和页面、registry/dispatch、Ops | 主部署单元仍是一个模块化服务                                        |
| sidecar-agent             | SDK Provider 接入、函数转发、任务及受管程序/插件装配                           | Makefile 输出名仍为 croupier-agent；部分装配仍在 internal/app/agent |
| capture-agent             | 数据库变更采集、位点、过滤和规则告警                                           | 独立角色，不是 SDK 函数网关                                         |
| synthetics-agent          | 外部 HTTP 探测、结果与探测插件                                                 | 复用公共运行时，探测业务独立                                        |
| devops-agent              | 注册、白名单同步、Broker/Hub 会话基础能力                                      | 面板→Server→Agent 的完整中继未接；RDP/VNC 尚不可用                  |
| ingest / analytics-worker | 接收事件、队列消费、聚合并写 ClickHouse                                        | 内置 worker 消费 Redis Streams；Kafka 发布适配不代表消费端已支持    |

## 请求与发布链路

### 运营执行

1. Web 经 HTTP 入口进入 Server；认证后，scope 中间件解析并授权 `gameId + env`。
2. 页面执行读取对应 PageSpec 快照并校验绑定；普通函数调用进入统一 `platform/functioninvoke` 执行链。
3. 执行链检查禁用/分配、政策及输入契约；需要审批时创建审批记录，批准后续跑。
4. Dispatcher 选择符合 scope 的 Agent。连接在本实例时直接投递；多实例模式下可转发至持有连接的 owner。
5. sidecar 通过已有 SDK provider session 调用游戏函数。异步任务通过 task_runs/task_events 留存状态，查询与 SSE 向控制台返回进展。
6. 审计按相应路径/政策记录；执行日志是另一条留痕机制，不能等同审计事务保证。

### 注册到页面

`SDK 注册 / OpenAPI 契约 → FunctionContract → 资源语义与生成器 → 页面提案/草稿 → 校验发布 → Console 绑定执行`

Server 保留契约、页面快照与发布历史。正式 Console 使用发布态；Page Studio 另有受控草稿执行路径。页面执行使用数据库幂等占位和结果回写，但不代表所有外部副作用都具有 exactly-once 保证。

## 状态归属与高可用

| 状态                                            | 当前存放位置                               | 多实例含义                                           |
| ----------------------------------------------- | ------------------------------------------ | ---------------------------------------------------- |
| Agent/SDK 活连接                                | 对应进程内存                               | 不能共享，通过 owner 转发访问                        |
| 注册信息                                        | 本地 registry + 数据库 agent_sessions 快照 | 快照用于发现，不能代替活连接                         |
| 成员与 Agent owner 租约                         | 共享关系库，或配置 Redis                   | 一跳转发与 epoch fencing 防止循环和旧任期请求        |
| 业务、页面、task_runs/task_events、页面幂等记录 | 关系库；部分模型可通过 dbctx 分库          | 需按具体模型确认 scope/分库行为                      |
| task→Agent 路由                                 | 本地缓存/文件 + task_runs 查询兜底         | 本地丢失不等于跨实例取消失效                         |
| 函数审批                                        | 当前 ServiceContext 装配 MemStore          | SQLStore 存在，但当前装配尚未提供跨实例/重启持久保证 |
| 审计                                            | SQLAuditStore + AuditService               | 有哈希链校验；默认未装配 signer，不能宣称不可篡改    |
| 分析事件                                        | Redis Streams → worker → ClickHouse        | 独立于控制面数据库                                   |

**HA 是有边界的能力**：owner 转发已经实现，但不意味着每个业务模块的状态都已经共享。集群初始化的部分错误会回退 standalone，现有 healthz 不完整表达集群就绪状态。

## 传输和信任边界

- Server HTTP 默认端口 `18780`；控制台 HTTPS 通常在反向代理终止。
- Agent 主动连接 Server control `19090`，SDK 连接 sidecar 本地 provider `19091`；端口以配置为准。
- session 使用长度前缀分帧与 protobuf 平台消息，函数业务 payload 通常是 UTF-8 JSON。
- 公共 `core/transport/tcp` 提供 MuxConn；注册、重连、配置同步等由角色组合公共包，不是只引入 transport 就自动拥有完整生命周期。
- Agent↔Server 支持 TLS，**仓库开发模板未启用**：Server cert/key 为空，Agent insecure=true。生产应显式配置证书，不能依赖“默认 TLS”表述。
- 当前 Server↔Server 互联为内网明文 TCP，hello 不是对端身份认证；互联端口须保持可信网络隔离。mTLS 是待补能力，不是当前图中的已实现保证。

## 业务 scope 与存储

`gameId + env` 是逻辑授权边界；target、Agent、node 表达执行位置。一个 Agent 实例声明一个 game/env，同机多环境使用多个实例。

默认单库不取消 scope 校验。开启 multiGame 后，meta 库与部分游戏模型数据库分开，`dbctx` 为模型选择连接。当前 binding 表检查与 Router 计算物理库名仍是两条路径；迁移旧库名或变更前缀前必须核对实际映射。不要把逻辑 env 自动理解为已经拥有独立机器、集群或物理数据库。

## 源码导航

路径相对仓库根目录，便于核对本页：

| 关注点                 | 入口                                                                 |
| ---------------------- | -------------------------------------------------------------------- |
| Server 装配与 listener | `cmd/server/root.go`、`cmd/server/cluster.go`                        |
| 依赖装配与 scope       | `internal/svc/service_context.go`、`internal/svc/game_middleware.go` |
| HTTP 入口              | `internal/handler/routes.go`、`internal/api/`                        |
| 函数执行和路由         | `internal/platform/functioninvoke/`、`internal/platform/dispatch/`   |
| 集群和注册表           | `internal/cluster/`、`internal/platform/registry/`                   |
| 页面生成与执行         | `internal/dashboard/`、`internal/api/page/`、`internal/api/console/` |
| 审批与审计             | `internal/platform/approvals/`、`internal/audit/`                    |
| 数据库路由             | `internal/db/router/`、`internal/db/dbctx/`                          |
| Agent 与公共能力       | `cmd/*-agent/`、`agents/`、`core/`、`internal/app/agent/`            |
| 分析管道               | `cmd/ingest/`、`cmd/analytics-worker/`、`internal/analytics/`        |

## 阅读路径

- [SDK wire 协议](./sdk-wire-protocol)
- [会话生命周期](./session-lifecycle)
- [多实例 HA 与当前限制](./server-ha-multi-instance)
- [Agent 选型](../operations/agent-comparison)
- [分析管道](../analytics/)
- [快速开始](../guide/quick-start)
