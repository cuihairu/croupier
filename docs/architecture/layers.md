---
title: 分层设计
icon: layers
order: 2
category:
  - 系统架构
tag:
  - 架构
  - 分层
---

# 分层设计

Croupier 采用**三层分布式架构**，实现权限控制、函数路由和 UI 展示的完全分离。

## 目录

[[toc]]

## 三层架构概览

```
┌────────────────────────────────────────────────────────────┐
│                    展示层 (Display Layer)                  │
│  ┌──────────────────────────────────────────────────────┐ │
│  │  Web Dashboard (React + Ant Design + X-Render)      │ │
│  │  - 描述符驱动 UI 自动生成                            │ │
│  │  - 实时进度与日志流式展示                            │ │
│  │  - 审批流程可视化                                    │ │
│  └──────────────────────────────────────────────────────┘ │
└────────────────────────────────────────────────────────────┘
                              ▼
┌────────────────────────────────────────────────────────────┐
│                    控制层 (Control Layer)                   │
│  ┌──────────────────────────────────────────────────────┐ │
│  │  Server (HTTP + gRPC)                                │ │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐ │ │
│  │  │ RBAC/ABAC   │  │ 函数路由    │  │ 审计日志    │ │ │
│  │  │ 权限引擎    │  │ 负载均衡    │  │ 审批流程    │ │ │
│  │  └─────────────┘  └─────────────┘  └─────────────┘ │ │
│  └──────────────────────────────────────────────────────┘ │
└────────────────────────────────────────────────────────────┘
                              ▼
┌────────────────────────────────────────────────────────────┐
│                    接入层 (Access Layer)                    │
│  ┌──────────────────────────────────────────────────────┐ │
│  │  Edge Proxy (可选)                                   │ │
│  │  - DMZ 部署                                          │ │
│  │  - 隧道复用                                          │ │
│  │  - 流量转发                                          │ │
│  └──────────────────────────────────────────────────────┘ │
└────────────────────────────────────────────────────────────┘
                              ▼
┌────────────────────────────────────────────────────────────┐
│                    代理层 (Agent Layer)                     │
│  ┌──────────────────────────────────────────────────────┐ │
│  │  Agent (游戏内网部署)                                 │ │
│  │  - 函数注册                                          │ │
│  │  - 调用转发                                          │ │
│  │  - 作业执行                                          │ │
│  └──────────────────────────────────────────────────────┘ │
└────────────────────────────────────────────────────────────┘
                              ▼
┌────────────────────────────────────────────────────────────┐
│                    业务层 (Service Layer)                   │
│  ┌──────────────────────────────────────────────────────┐ │
│  │  Game Server + SDK                                   │ │
│  │  - 函数实现                                          │ │
│  │  - 业务逻辑                                          │ │
│  └──────────────────────────────────────────────────────┘ │
└────────────────────────────────────────────────────────────┘
```

## 第一层：展示层

### 职责

- 用户界面呈现
- 表单自动生成
- 实时数据展示
- 审批流程交互

### 组件

| 组件 | 技术栈 | 职责 |
|------|--------|------|
| **Dashboard** | React + Ant Design | Web 管理界面 |
| **X-Render** | FormRender | 表单自动生成 |
| **ProTable** | Ant Design Pro | 列表自动生成 |

### 描述符驱动 UI

```json
// 函数描述符 → 自动生成 UI
{
  "id": "player.ban",
  "params": {
    "type": "object",
    "properties": {
      "player_id": {"type": "string", "title": "玩家ID"},
      "duration": {"type": "integer", "title": "封禁时长"}
    }
  }
}
```

自动生成的 UI：
- 表单字段
- 验证规则
- 风险提示
- 提交确认

## 第二层：控制层

### 职责

- 权限控制 (RBAC/ABAC)
- 函数路由与负载均衡
- 审批工作流
- 审计日志记录

### 核心模块

#### 1. 权限引擎

```go
// 权限检查流程
func (e *Engine) Check(ctx context.Context, req *CheckRequest) *CheckResponse {
    // 1. RBAC 检查
    if !e.rbac.HasPermission(ctx.User, req.Permission) {
        return Denied("missing permission")
    }

    // 2. ABAC 检查
    if !e.abac.Evaluate(ctx.User, req.Attributes) {
        return Denied("abac denied")
    }

    // 3. 审批检查
    if e.approval.Required(req) {
        return PendingApproval()
    }

    return Approved()
}
```

#### 2. 函数路由

```go
// 路由策略
type Router interface {
    SelectAgent(functionID string, agents []Agent) (Agent, error)
}

type RoundRobinRouter struct{}
type ConsistentHashRouter struct{}
type LeastConnectionRouter struct{}
```

#### 3. 审计日志

```go
type AuditLog struct {
    AuditID   string
    Timestamp time.Time
    User      UserInfo
    Action    string
    Target    string
    Result    string
    Hash      string  // 防篡改
}
```

### Server 组件

```
Server
├── HTTP API Server    :8080
│   ├── REST API
│   ├── WebSocket
│   └── SSE Events
│
├── gRPC Server        :8443
│   ├── ControlService  (Agent 管理)
│   ├── FunctionService (函数调用)
│   └── RegistryService (注册中心)
│
├── Auth Engine
│   ├── RBAC
│   ├── ABAC
│   └── JWT/OIDC
│
├── Approval Engine
│   ├── Two-person rule
│   └── Workflow
│
└── Audit Log
    ├── Events
    ├── Chain
    └── Sensitive masking
```

## 第三层：接入层（可选）

### Edge Proxy

当 Server 在内网，需要通过 Edge 暴露到公网时使用。

```
公网 ──TLS──> Edge ──mTLS──> Server ──mTLS──> Agent
```

### Edge 职责

| 功能 | 说明 |
|------|------|
| **隧道复用** | 多个连接共享隧道 |
| **流量转发** | 请求/响应转发 |
| **连接管理** | 保持长连接池 |
| **安全隔离** | DMZ 部署 |

## 第四层：代理层

### Agent 架构

```
Agent
├── gRPC Client (出站)
│   └── 连接 Server
│
├── gRPC Server (本地)
│   └── :19090
│
├── Function Registry
│   ├── 注册函数
│   ├── 心跳保活
│   └── 更新通知
│
├── Job Executor
│   ├── 同步调用
│   ├── 异步作业
│   └── 事件流
│
└── Downloader
    └── 函数包下载
```

### Agent 特性

| 特性 | 说明 |
|------|------|
| **出站连接** | 主动连接 Server，穿透内网 |
| **函数注册** | 向 Server 注册本地函数 |
| **调用转发** | 转发 Server 调用到 Game Server |
| **作业执行** | 支持长时间运行的异步作业 |
| **热重载** | 函数更新无需重启 |

## 第五层：业务层

### Game Server + SDK

```
Game Server
└── Croupier SDK
    ├── 函数定义
    ├── 函数实现
    ├── gRPC 客户端
    └── 类型安全
```

### SDK 职责

1. **函数定义**：定义函数接口
2. **函数实现**：实现业务逻辑
3. **类型安全**：编译时类型检查
4. **自动重连**：连接断开自动重连

## 层间通信

### 通信协议

| 层级 | 协议 | 端口 | 安全 |
|------|------|------|------|
| Dashboard → Server | HTTPS | 8080 | TLS |
| Server → Agent | gRPC | 8443 | mTLS |
| Server → Edge | gRPC | 8443 | mTLS |
| Edge → Agent | gRPC | 8443 | mTLS |
| Agent → Game Server | gRPC | 19090 | 可选 mTLS |

### 数据格式

```protobuf
// 函数调用请求
message InvokeFunctionRequest {
  string game_id = 1;
  string env = 2;
  string function_id = 3;
  google.protobuf.Struct payload = 4;
  InvokeOptions options = 5;
}

// 函数调用响应
message InvokeFunctionResponse {
  bool success = 1;
  google.protobuf.Struct result = 2;
  string error = 3;
}
```

## 分层优势

### 1. 关注点分离

- 展示层专注 UI
- 控制层专注安全和路由
- 业务层专注游戏逻辑

### 2. 独立部署

- Dashboard 可独立部署
- Server 可水平扩展
- Agent 随游戏服务器部署

### 3. 安全隔离

- 层间强制认证
- 最小权限原则
- 审计完整追溯

### 4. 技术栈灵活

- 各层可使用不同技术
- 接口通过 Protobuf 定义
- 语言无关的集成

## 相关文档

- [组件说明](./components.md)
- [数据流](./data-flow.md)
- [设计模式](./design-patterns.md)
