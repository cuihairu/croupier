---
title: 数据流
icon: route
order: 4
category:
  - 系统架构
tag:
  - 架构
  - 数据流
  - 调用流程
---

# 数据流

> **状态**：Current — 当前实现/规范，可作为实现依据。

本文档只描述当前目标架构下的主要调用流，不再使用历史 `gRPC/旧传输 回拨` 模型。

## 1. 页面发布与受控执行（vNext 主路径）

运营页面的执行主路径经过发布快照，而不是直接调用函数目录：

```mermaid
sequenceDiagram
    participant SDK as SDK / OpenAPI Provider
    participant Server as Server
    participant Admin as 管理员（Proposal Inbox / Page Studio）
    participant UI as Console 运行时

    SDK->>Server: 注册 FunctionContract（无 UI 字段）
    Server->>Server: 聚合 CapabilitySemantics、生成 PageProposal
    Admin->>Server: 预览 / accept-and-publish（冻结 BindingContractSnapshot）
    Server-->>UI: GET /api/v1/console/menu（菜单树 menu_items + 挂载的 active PublishedPageSpec）
    UI->>Server: POST /api/v1/console/pages/:pageKey/bindings/:bindingId/execute
    Server->>Server: binding/snapshot/stale/permission/approval 校验
    Note over Server: 合同变化未重新发布时返回 409 binding_stale
    Server->>Server: 按 selector 组装 payload 并 dispatch（同第 2 节）
    Server-->>UI: PageExecutionResult（sync data / taskId / approvalId）
```

要点：

- 浏览器只提交 `bindingId` 与 selector context（form/row/selection/page state），不传 functionId、target、gameId、env。
- 发布快照冻结函数版本、schema digest、risk、permission、approval 与 renderer 版本；函数重注册只产生新 Proposal 与 stale 诊断，绝不静默改写已发布页面。
- 合同变化后的恢复路径：`GET /api/v1/versioning/pages/:pageKey/diff` → 三方合并（展示字段自动合并、执行字段人工决策）→ 重新发布。
- 函数直调 `POST /api/v1/functions/:id/invoke` 仍存在，但定位是调试/管理面路径，不是运营页面主路径。

## 2. Server 到业务函数（dispatch）

```mermaid
sequenceDiagram
    participant Server as Server
    participant Agent as Agent
    participant App as Game Server / SDK

    Server->>Server: 鉴权 / RBAC / 审批 / 路由
    Server->>Agent: InvokeRequest over agent-server session
    Agent->>App: InvokeRequest over local session
    App-->>Agent: InvokeResponse
    Agent-->>Server: InvokeResponse
```

要点：

- `Server -> Agent` 不再新开一条回拨连接
- `Agent -> App` 由本地 session 边界处理
- 两段链路都遵循相同的 session runtime 思维

### invoke 路由：Provider/Instance 双索引

当一个 Agent 后挂多个游戏服务（service）时，Agent 内部按 **Nacos 风格的双层索引**选择目标实例：

- **注册期**：SDK 的 `ProviderConnectRequest` 携带 `service_id` 与 `Metadata`（proto 字段 `sdk_language`/`sdk_version`/`game_id`/`env` 等）。Agent 以 `functionId → serviceId → []Instance` 双层索引登记，实例带 `LastSeen` 用于健康判断（`internal/platform/agentlocal/store.go`）。
- **调用期**：`pickInstance`（`internal/agent/local_handler.go`）先按 `functionId` 取 service 索引；若 invoke metadata 带 `serviceId` 则精确落到该 service 的实例集合，否则合并该函数下所有 service 的实例；再按 `LastSeen` 过滤健康实例并做负载均衡。
- `Instance.Metadata` 负责透传 SDK 元信息（键为 `sdkLanguage`/`sdkVersion`/`gameId` 等小驼峰），最终经 `AgentProcess → ProviderSession` 暴露到 opsNodes。

## 3. SDK 注册到 Agent

```mermaid
sequenceDiagram
    participant SDK as Embedded SDK
    participant Agent as Agent

    SDK->>Agent: ProviderConnectRequest
    Agent-->>SDK: ProviderConnectResponse(session_id)
    SDK->>Agent: ProviderHeartbeatRequest
    Agent-->>SDK: ProviderHeartbeatResponse
```

## 4. Agent 注册到 Server

```mermaid
sequenceDiagram
    participant Agent as Agent
    participant Server as Server

    Agent->>Server: RegisterRequest（演进命名 AgentConnectRequest）
    Server-->>Agent: AgentConnectResponse / RegisterResponse(session_id)
    Agent->>Server: Heartbeat
    Server-->>Agent: HeartbeatResponse
```

### SDK 滑动版本门槛（函数注册）

`RegisterRequest.processes` 各进程自报 `sdk_language`/`sdk_version`。Server 按 `(game_id, env, sdk_language)` 记录历史最高 SDK 版本（**高水位**，落各 game 库 `sdk_version_highwatermarks` 表，编号迁移 0028），注册时三档判定（`internal/platform/registry/sdkversion`）：

| 判定        | 条件                            | 行为                                                                                              |
| ----------- | ------------------------------- | ------------------------------------------------------------------------------------------------- |
| 放行        | 版本 ≥ 高水位                   | 注册成功后抬升高水位                                                                              |
| 放行 + 警告 | 同 major 且落后 1 个 minor      | 函数照常注册，写 `sdk_version_behind` 注册警告                                                    |
| 拒绝        | 落后 ≥2 个 minor 或 ≥1 个 major | 该进程独占声明的函数不进本次注册（会话函数表/契约均不物化），写 `sdk_version_floor_rejected` 警告 |

细节语义：

- **拒绝粒度是函数不是连接**：被拒进程的 provider 记录仍进会话（进程存在是事实；调度候选第一道门是 `agent.Functions`，不会误路由）。多进程交叉提供同一函数时，只要任一放行进程也声明了该函数就不剔除，拒绝不制造可用性缺口。
- **版本解析失败（`"unknown"`/空/非数字段）一律放行**，且不抬升高水位——门槛只在两端都是可解析的语义化版本时生效。
- 无 `sdk_language`/`sdk_version` 自报的进程（自定义游戏服直连）不参与门槛。
- 拒绝与落后警告同时随 `RegisterResponse.warnings` 返回，agent 侧日志可见；连接保持不断开。

**已知边界**：

- 高水位是单调观测值：高版本 SDK 永久下线后，低版本会持续被拒，重置需手动 `DELETE FROM sdk_version_highwatermarks WHERE ...`。
- 内存 registry（无 DB）退化为进程内 map，重启丢失，重新从首次注册版本开始累积。
- 落后警告（`sdk_version_behind`）在 SDK 追赶后不自动清除（多语言 provider 共用 agent+code 维度无法精确按语言清理），人工删除兜底。
- 抬升高水位发生在 `UpsertAgent` 成功之后：注册链中途失败不会留下「未注册但已抬升」的状态；反之注册成功后进程崩溃在抬升之前，高水位少记一次，下次注册补记，无正确性影响。

## 5. 作业流

```mermaid
sequenceDiagram
    participant UI as Dashboard
    participant Server as Server
    participant Agent as Agent
    participant App as Game Server / SDK

    UI->>Server: POST /api/v1/tasks
    Server-->>UI: task_id
    Server->>Agent: StartTaskRequest
    Agent->>App: StartTaskRequest
    App-->>Agent: TaskEvent / StartTaskResponse
    Agent-->>Server: TaskEvent / StartTaskResponse
    Server-->>UI: SSE / WebSocket / Polling
```

运营侧的 TaskPage 也走第 1 节的 binding execute：`task_status/task_events/task_result/task_cancel` 各自对应一个 binding，页面模型见 [Dashboard Resource/Page 模型](./dashboard-page-model.md)。

## 6. 核心原则

当前数据流遵循以下原则：

1. 控制面和调用面统一收敛到 session 复用，不再拆成“注册链路 + 回拨链路”。
2. `Agent` 本地监听只面向本地接入方。
3. `Server` 通过 session ownership 路由到对应 `Agent`，而不是直接拨 `Agent`。
4. 业务 payload 保持 JSON，平台控制字段保持 protobuf。
