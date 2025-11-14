# Croupier项目gRPC通信架构分析

## 概述

Croupier是一个分布式游戏GM后端系统，采用**三层分布式架构**，其gRPC通信架构设计精巧，支持多种通信模式。本文档详细分析SDK与Agent、Agent与Server的gRPC通信机制。

---

## 1. 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                      Web UI (HTTP)                          │
│                      (Umi Max + Ant Design)                 │
└────────────────────┬────────────────────────────────────────┘
                     │
        ┌────────────▼──────────────┐
        │      Server (HTTP + gRPC) │
        │ - Control Service         │
        │ - Function Service        │
        │ - Registry                │
        └────────────┬──────────────┘
                     │
        ┌────────────▼──────────────────┐
        │   Edge (gRPC, 可选)           │
        │   - Tunnel Service (双向)      │
        │   - Function Service (转发)    │
        │   - Job Service               │
        └────────────┬──────────────────┘
                     │
        ┌────────────▼──────────────────┐
        │      Agent (gRPC)             │
        │ - Function Service (转发)     │
        │ - Local Control Service       │
        │ - Job Index                   │
        └────────────┬──────────────────┘
                     │
        ┌────────────▼──────────────┐
        │   Game Server (SDK + gRPC)│
        │   - Local Function Service│
        │   - Function Handler实现   │
        └───────────────────────────┘
```

---

## 2. Proto文件分析

### 2.1 核心服务定义

#### **A. ControlService** (Server -> Agent)
文件：`proto/croupier/control/v1/control.proto`

```protobuf
service ControlService {
  rpc Register(RegisterRequest) returns (RegisterResponse);      // Agent注册
  rpc Heartbeat(HeartbeatRequest) returns (HeartbeatResponse);  // 心跳
  rpc RegisterCapabilities(RegisterCapabilitiesRequest) 
    returns (RegisterCapabilitiesResponse);                     // 能力注册
}
```

**消息结构：**
- `RegisterRequest`：Agent向Server注册自身
  - `agent_id`：Agent唯一标识
  - `game_id`：游戏范围隔离
  - `env`：环境标识
  - `functions`：支持的函数列表

**通信模式：** 单向RPC（Unary）

---

#### **B. FunctionService** (全系统函数调用)
文件：`proto/croupier/function/v1/function.proto`

```protobuf
service FunctionService {
  rpc Invoke(InvokeRequest) returns (InvokeResponse);           // 同步调用
  rpc StartJob(InvokeRequest) returns (StartJobResponse);       // 异步启动
  rpc StreamJob(JobStreamRequest) returns (stream JobEvent);    // 双向流式
  rpc CancelJob(CancelJobRequest) returns (StartJobResponse);   // 取消任务
}
```

**关键特性：**
- **Invoke**：同步RPC，等待立即响应
- **StartJob/StreamJob**：异步模式
  - StartJob启动后返回job_id
  - StreamJob订阅job_id的事件流（双向通信）
  - JobEvent包含：type (progress/log/done/error), message, progress%, payload

**通信模式：** 混合（Unary + Server-side Stream）

---

#### **C. LocalControlService** (Game Server -> Agent)
文件：`proto/croupier/agent/local/v1/local.proto`

```protobuf
service LocalControlService {
  rpc RegisterLocal(RegisterLocalRequest) 
    returns (RegisterLocalResponse);                            // 游戏服注册
  rpc Heartbeat(HeartbeatRequest) 
    returns (HeartbeatResponse);                                // 心跳
  rpc ListLocal(ListLocalRequest) 
    returns (ListLocalResponse);                                // 查询本地实例
  rpc GetJobResult(GetJobResultRequest) 
    returns (GetJobResultResponse);                             // 查询任务结果
}
```

**通信模式：** 单向RPC（Unary）

---

#### **D. TunnelService** (Agent -> Edge，出站mTLS)
文件：`proto/croupier/tunnel/v1/tunnel.proto`

```protobuf
service TunnelService {
  rpc Open(stream TunnelMessage) returns (stream TunnelMessage);  // 双向流
}
```

**关键特性：** **完全双向通信**
- Agent以客户端身份主动连接Server/Edge
- 发送：InvokeFrame, StartJobFrame, CancelJobFrame
- 接收：ResultFrame, StartJobResult, JobEventFrame

**消息多路复用：**
```protobuf
message TunnelMessage {
  string type = 1;                          // 消息类型标识
  Hello hello = 2;                          // 初始化握手
  ResultFrame result = 3;                   // 同步结果
  StartJobResult start_r = 4;               // 异步任务启动结果
  JobEventFrame job_evt = 5;                // 异步任务事件
  ListLocalRequest list_req = 6;            // 查询请求
  ListLocalResponse list_res = 7;           // 查询响应
  GetJobResultRequest job_res_req = 8;      // 任务结果查询请求
  GetJobResultResponse job_res_res = 9;     // 任务结果查询响应
  InvokeFrame invoke = 10;                  // 同步调用请求
  StartJobFrame start = 11;                 // 异步启动请求
  CancelJobFrame cancel = 12;               // 取消请求
}
```

---

### 2.2 数据流特性

| 特性 | 描述 |
|------|------|
| **幂等性** | 支持 `idempotency_key` 防止重复执行 |
| **游戏隔离** | 所有操作通过 `game_id`/`env` 作用域隔离 |
| **元数据传递** | `metadata` map传递跨层信息 |
| **请求追踪** | `request_id`, `trace_id` 用于链路追踪 |

---

## 3. SDK与Agent的gRPC通信

### 3.1 通信流程

```
Game Server (C++ SDK)
    │
    │ 1. Connect to Agent
    │    (gRPC Dial to agent_addr:19090)
    ▼
Agent (gRPC Server)
    │
    │ 2. RegisterLocal(service_id, functions)
    │    (LocalControlService::RegisterLocal)
    ▼
Agent maintains LocalStore
    │
    │ ┌─────────────────────────────┐
    │ │ 3. Function Invocation       │
    │ │    ┌───────────────────────┐ │
    │ │    │ Invoke               │ │
    │ │    │ (sync, direct)       │ │
    │ │    └───────────────────────┘ │
    │ │    ┌───────────────────────┐ │
    │ │    │ StartJob + StreamJob  │ │
    │ │    │ (async, streaming)    │ │
    │ │    └───────────────────────┘ │
    │ │    ┌───────────────────────┐ │
    │ │    │ CancelJob             │ │
    │ │    │ (cancel async)        │ │
    │ │    └───────────────────────┘ │
    │ └─────────────────────────────┘
    │
    │ 4. Heartbeat (定期)
    │    (LocalControlService::Heartbeat)
    ▼
Game Server
```

---

### 3.2 C++ SDK实现

文件：`sdks/cpp/include/croupier/sdk/croupier_client.h`

#### **SDK的两个核心类**

**1. CroupierClient（提供者/Provider）**
```cpp
class CroupierClient {
public:
    explicit CroupierClient(const ClientConfig& config);
    
    // 函数注册
    bool RegisterFunction(const FunctionDescriptor& desc, FunctionHandler handler);
    
    // 虚拟对象注册（新）
    bool RegisterVirtualObject(const VirtualObjectDescriptor& desc,
                               const std::map<std::string, FunctionHandler>& handlers);
    
    // 组件注册（推荐）
    bool RegisterComponent(const ComponentDescriptor& comp);
    
    // 核心操作
    bool Connect();        // 连接到Agent
    void Serve();          // 开始服务（阻塞）
    void Stop();           // 停止服务
    void Close();          // 关闭连接
    
    std::string GetLocalAddress() const;
};
```

**2. CroupierInvoker（消费者/Consumer）**
```cpp
class CroupierInvoker {
public:
    explicit CroupierInvoker(const InvokerConfig& config);
    
    bool Connect();
    
    // 同步调用
    std::string Invoke(const std::string& function_id, 
                      const std::string& payload,
                      const InvokeOptions& options = {});
    
    // 异步启动
    std::string StartJob(const std::string& function_id,
                        const std::string& payload,
                        const InvokeOptions& options = {});
    
    // 流式监听（双向通信）
    std::future<std::vector<JobEvent>> StreamJob(const std::string& job_id);
    
    bool CancelJob(const std::string& job_id);
    void Close();
};
```

#### **配置关键字段**

```cpp
struct ClientConfig {
    std::string agent_addr = "127.0.0.1:19090";      // Agent监听地址
    std::string service_id = "cpp-service";          // 服务标识
    std::string service_version = "1.0.0";
    
    std::string game_id;           // 必需：游戏隔离
    std::string env = "development"; // 环境标识
    
    bool insecure = true;          // 开发: insecure，生产: mTLS
    std::string cert_file;         // 客户端证书
    std::string key_file;          // 私钥
    std::string ca_file;           // CA证书
    
    int timeout_seconds = 30;
    int heartbeat_interval = 60;   // 心跳间隔
};
```

---

### 3.3 Agent处理流程

文件：`internal/app/agent/function_server.go`

#### **Agent作为双重身份**

```
┌─────────────────────────────────────┐
│         Agent (gRPC Server)          │
├─────────────────────────────────────┤
│ 1. LocalControlService              │
│    - RegisterLocal (Game -> Agent)   │
│    - Heartbeat (Game -> Agent)       │
│    - ListLocal (Game -> Agent)       │
│    - GetJobResult (Game -> Agent)    │
│                                      │
│ 2. FunctionService                  │
│    - Invoke (Game -> Agent -> Game)  │
│    - StartJob (async)                │
│    - StreamJob (双向)                │
│    - CancelJob                       │
│                                      │
│ 3. JobIndex                          │
│    - 维护job_id -> instance_addr的映射 │
└─────────────────────────────────────┘
         ↑
         │ (gRPC Client)
         │
    Game Servers
```

#### **FunctionServer实现关键代码**

```go
type FunctionServer struct{
    functionv1.UnimplementedFunctionServiceServer
    store *agentlocal.LocalStore    // 本地实例存储
    jobs  *jobIndex                 // job_id -> addr映射
}

func (s *FunctionServer) Invoke(ctx context.Context, 
                                in *functionv1.InvokeRequest) 
                                (*functionv1.InvokeResponse, error) {
    // 1. 从LocalStore查找function_id对应的实例
    addr, ok := s.pickInstance(in.GetFunctionId())
    if !ok { return nil, nil }
    
    // 2. 作为gRPC客户端连接到Game Server
    cc, cli, err := s.dial(addr)
    if err != nil { return nil, err }
    defer cc.Close()
    
    // 3. 转发调用到Game Server的FunctionService
    c2, cancel := context.WithTimeout(ctx, 3*time.Second)
    defer cancel()
    return cli.Invoke(c2, in)  // 同步调用转发
}

func (s *FunctionServer) StartJob(ctx context.Context, 
                                  in *functionv1.InvokeRequest) 
                                  (*functionv1.StartJobResponse, error) {
    addr, ok := s.pickInstance(in.GetFunctionId())
    if !ok { return nil, nil }
    
    cc, cli, err := s.dial(addr)
    if err != nil { return nil, err }
    defer cc.Close()
    
    resp, err := cli.StartJob(ctx, in)
    
    // 记录job_id -> addr映射，便于后续StreamJob查询
    if err == nil && resp != nil && resp.GetJobId() != "" && s.jobs != nil {
        s.jobs.Set(resp.GetJobId(), addr)
    }
    return resp, err
}

func (s *FunctionServer) CancelJob(ctx context.Context, 
                                   in *functionv1.CancelJobRequest) 
                                   (*functionv1.StartJobResponse, error) {
    // 从jobIndex查找job对应的实例
    if addr, ok := s.jobs.Get(in.GetJobId()); ok {
        cc, cli, err := s.dial(addr)
        if err == nil {
            defer cc.Close()
            c2, cancel := context.WithTimeout(ctx, 3*time.Second)
            defer cancel()
            resp, err2 := cli.CancelJob(c2, in)
            s.jobs.Delete(in.GetJobId())
            if err2 == nil { return resp, nil }
        }
    }
    // fallback: 确认取消
    return &functionv1.StartJobResponse{JobId: in.GetJobId()}, nil
}
```

**关键观察：**
- Agent同时是gRPC **Server**（接收Game Server和Server的调用）
- Agent同时是gRPC **Client**（转发调用到Game Server）
- LocalStore维护 `function_id -> []instance` 的映射
- jobIndex维护 `job_id -> instance_addr` 的映射，用于异步任务回源

---

## 4. Agent与Server的gRPC通信

### 4.1 通信模式

#### **模式1：单向注册（Server被动接收）**

```
Agent (Client)
    │
    │ grpc.Dial("server:8443")
    ▼
Server (Listener :8443)
    │
    │ ControlService::Register (Unary)
    │ ControlService::Heartbeat (Unary)
    │ ControlService::RegisterCapabilities (Unary)
    ▼
Server Registry (维护Agent会话)
```

实现：`internal/platform/control/server.go`

```go
type Server struct {
    controlv1.UnimplementedControlServiceServer
    reg *reg.Store
}

func (s *Server) Register(ctx context.Context, 
                         in *controlv1.RegisterRequest) 
                         (*controlv1.RegisterResponse, error) {
    sess := &reg.AgentSession{
        AgentID:  in.GetAgentId(),
        GameID:   in.GetGameId(),
        Env:      in.GetEnv(),
        RPCAddr:  in.GetRpcAddr(),
        ExpireAt: time.Now().Add(60 * time.Second),
        Functions: map[string]reg.FunctionMeta{},
    }
    s.reg.UpsertAgent(sess)
    return &controlv1.RegisterResponse{}, nil
}

func (s *Server) Heartbeat(ctx context.Context, 
                          in *controlv1.HeartbeatRequest) 
                          (*controlv1.HeartbeatResponse, error) {
    // 延长Agent会话有效期
    s.reg.Mu().Lock()
    if a := s.reg.AgentsUnsafe()[in.GetAgentId()]; a != nil {
        a.ExpireAt = time.Now().Add(60 * time.Second)
    }
    s.reg.Mu().Unlock()
    return &controlv1.HeartbeatResponse{}, nil
}
```

#### **模式2：双向隧道（Edge可选）**

```
Agent (Client)
    │
    │ TunnelService::Open(stream TunnelMessage)
    │ (gRPC 双向流)
    ▼
Edge/Server (TunnelService Server)
    │
    ├─► 接收：InvokeFrame, StartJobFrame, CancelJobFrame
    │
    └─► 发送：ResultFrame, JobEventFrame, StartJobResult
```

实现：`internal/app/edge/app.go`

```go
type TunnelServer struct{ 
    tunnelv1.UnimplementedTunnelServiceServer 
}

// Open(stream TunnelMessage) returns (stream TunnelMessage)
// - Agent作为客户端发起连接
// - 双向流：Agent可随时发送消息，Server也可随时推送
```

---

### 4.2 Server中的gRPC客户端

文件：`cmd/server/main.go`

Server可以配置转发到Edge进行函数调用：

```go
// Server连接到Edge（如果配置了）
type edgeForwarder struct {
    cc    *grpc.ClientConn
    cli   functionv1.FunctionServiceClient
    mu    sync.Mutex
    stats httpserver.AgentStats
}

func newEdgeForwarder(addr string, creds credentials.TransportCredentials) 
                     (*edgeForwarder, error) {
    cc, err := grpc.Dial(addr,
        grpc.WithTransportCredentials(creds),
        grpc.WithKeepaliveParams(keepalive.ClientParameters{}),
        grpc.WithDefaultCallOptions(grpc.CallContentSubtype("json")),
    )
    if err != nil { return nil, err }
    return &edgeForwarder{cc: cc, cli: functionv1.NewFunctionServiceClient(cc)}, nil
}

func (e *edgeForwarder) Invoke(ctx context.Context, 
                               req *functionv1.InvokeRequest) 
                               (*functionv1.InvokeResponse, error) {
    return e.cli.Invoke(ctx, req)  // 转发到Edge
}

func (e *edgeForwarder) StreamJob(ctx context.Context, 
                                  req *functionv1.JobStreamRequest) 
                                  (functionv1.FunctionService_StreamJobClient, error) {
    return e.cli.StreamJob(ctx, req)  // 转发流式任务
}
```

---

## 5. 双向通信支持分析

### 5.1 完全双向的服务

#### **1. TunnelService（最典型的双向）**

```protobuf
service TunnelService {
  rpc Open(stream TunnelMessage) returns (stream TunnelMessage);
}
```

**特点：**
- Agent可随时向Server发送消息（InvokeFrame, StartJobFrame, CancelJobFrame）
- Server可随时向Agent推送事件（JobEventFrame, StartJobResult）
- 使用单一长连接复用多种类型的消息
- 支持多路复用（通过TunnelMessage.type字段）

**数据流：**
```
Agent                          Edge/Server
  │                                 │
  ├─ TunnelMessage(invoke) ───────►│
  │                                 │
  │                                 ├─ 处理调用
  │                                 │
  │◄─ TunnelMessage(result) ────────┤
  │                                 │
  ├─ TunnelMessage(start_job) ────►│
  │                                 │
  │◄─ TunnelMessage(start_r) ──────┤
  │                                 │
  │                                 ├─ 异步执行
  │                                 │
  │◄─ TunnelMessage(job_evt) ───────┤ (推送进度)
  │◄─ TunnelMessage(job_evt) ───────┤ (推送日志)
  │◄─ TunnelMessage(job_evt) ───────┤ (推送完成)
  │                                 │
  ├─ TunnelMessage(cancel) ───────►│
  │                                 │
  └─────────────────────────────────┘
  (连接保活)
```

#### **2. StreamJob（单向Server Stream）**

```protobuf
rpc StreamJob(JobStreamRequest) returns (stream JobEvent);
```

**特点：**
- 客户端发送一次请求（包含job_id）
- Server持续推送JobEvent流
- 客户端可接收进度、日志、错误、最终结果

**数据流：**
```
Client                       Server
  │
  ├─ JobStreamRequest ───────►│
  │                           │
  │◄─ stream JobEvent ────────┤
  │◄─ {type: progress, progress: 25}
  │◄─ {type: log, message: "..."}
  │◄─ {type: progress, progress: 50}
  │◄─ {type: done, payload: result}
  │
  └─ (连接关闭)
```

---

### 5.2 支持情况总结

| 服务 | 通信模式 | 双向支持 | 说明 |
|------|--------|--------|------|
| ControlService | Unary RPC | ✗ | Agent -> Server, 单向 |
| LocalControlService | Unary RPC | ✗ | Game -> Agent, 单向 |
| FunctionService (Invoke) | Unary RPC | ✗ | 同步调用 |
| FunctionService (StreamJob) | Server Stream | ⚠️ | 单向流，Server推送 |
| TunnelService (Open) | Bi-directional Stream | ✓ | 完全双向 |

---

## 6. 通信链路细节

### 6.1 同步函数调用链

```
Web UI (HTTP)
    │
    │ POST /invoke
    ▼
Server HTTP Handler
    │
    │ 1. 认证、授权、审计
    │
    ▼
edgeForwarder (gRPC Client)
    │
    │ FunctionService::Invoke(gRPC)
    │ Content-Subtype: "json"
    ▼
Edge/Server (gRPC Server)
    │
    │ FunctionService::Invoke
    ▼
Agent (gRPC Client)
    │
    │ FunctionService::Invoke
    │ Dial to game_server_addr
    ▼
Game Server (gRPC Server)
    │
    │ FunctionService::Invoke
    ▼
Handler 执行 → 响应
    │
    │ InvokeResponse.payload (JSON/Proto)
    ▼
Game Server (Client)
    │
    │ Response
    ▼
Agent (Server/Client)
    │
    │ Response
    ▼
Edge (Server/Client)
    │
    │ Response
    ▼
Server HTTP Handler
    │
    │ 2. 响应序列化 (JSON)
    │
    ▼
Web UI
```

### 6.2 异步任务执行链

```
Web UI
    │
    │ POST /start-job
    ▼
Server HTTP
    │
    ├─ FunctionService::StartJob → Edge → Agent → Game Server
    │  返回 job_id
    │
    ▼
Server HTTP (返回 job_id 给客户端)
    │
    │ 客户端轮询或订阅
    │ GET /jobs/{job_id}/stream
    ▼
Server HTTP (SSE/WebSocket)
    │
    ├─ FunctionService::StreamJob → Edge → Agent → Game Server
    │
    ▼
Game Server
    │
    ├─ 执行长时间操作
    │
    ├─ Emit JobEvent (progress/log)
    │  通过 Agent -> Edge -> Server -> HTTP(SSE)
    │
    └─ Emit JobEvent (done/error)
       返回最终结果
```

**关键机制：**
- Agent通过 jobIndex 维护 job_id -> instance_addr 映射
- StreamJob需要知道job在哪个实例运行
- 多个客户端可同时订阅同一个job的事件流

---

## 7. mTLS与安全

### 7.1 证书层次

```
Development (--insecure)
    │
    ├─ Server-Agent: 无证书
    ├─ Agent-GameServer: 无证书
    └─ Web-Server: HTTP

Production (--tls)
    │
    ├─ Server-Agent: mTLS (server.crt/key, ca.crt)
    ├─ Agent-GameServer: mTLS
    └─ Web-Server: HTTPS
```

### 7.2 实现参考

```go
// Server 配置 TLS
creds, _ := credentials.NewServerTLSFromFile(
    "server.crt", "server.key")
s := grpc.NewServer(grpc.Creds(creds))

// Client 连接
tlsConfig, _ := tlsutil.NewClientConfig(
    "client.crt", "client.key", "ca.crt", "server.example.com")
cc, _ := grpc.Dial(addr,
    grpc.WithTransportCredentials(
        credentials.NewTLS(tlsConfig)))
```

---

## 8. JSON编码支持

gRPC默认使用Protocol Buffers二进制编码，但Croupier支持JSON：

```go
// 注册JSON编码器
import _ "github.com/cuihairu/croupier/internal/transport/jsoncodec"

// 客户端指定使用JSON
cc, _ := grpc.Dial(addr,
    grpc.WithDefaultCallOptions(
        grpc.CallContentSubtype("json")))

// Server自动处理JSON和Protocol Buffers
```

**优势：**
- HTTP和gRPC可共用相同的消息格式
- 便于调试（可读的JSON而非二进制）
- 跨语言兼容性更好

---

## 9. SDK架构模式

### 9.1 两种使用模式

**模式A：提供者（Provider）**
```cpp
// 游戏服务器在本地实现函数，注册到Agent
CroupierClient client(config);
client.RegisterFunction({"player.ban", "1.0"}, 
                       [](const std::string& ctx, const std::string& payload) {
                           return processPlayerBan(payload);
                       });
client.Connect();
client.Serve();  // 开始监听
```

**模式B：消费者（Consumer）**
```cpp
// 远程调用游戏服务上的函数
CroupierInvoker invoker(config);
invoker.Connect();
std::string result = invoker.Invoke("player.ban", 
                                    R"({"player_id": "123"})");
```

### 9.2 虚拟对象管理

```cpp
// 新特性：虚拟对象（Virtual Objects）
VirtualObjectDescriptor wallet_obj = {
    .id = "wallet.entity",
    .version = "1.0.0",
    .operations = {
        {"read", "wallet.get"},
        {"update", "wallet.add"},
        {"delete", "wallet.clear"}
    }
};

client.RegisterVirtualObject(wallet_obj, handlers);
```

---

## 10. 总结

### 10.1 核心要点

1. **Agent双重身份**
   - 作为gRPC Server接收来自Game Server的函数调用
   - 作为gRPC Client转发调用到具体的Game Server实例
   - 维护LocalStore和jobIndex供路由使用

2. **完全双向通信**
   - TunnelService提供Agent-Server之间的双向流
   - 支持多种消息类型的复用
   - 出站连接模式（Agent主动连接Server）

3. **多层转发**
   - Web UI → Server (HTTP) → Edge (gRPC) → Agent (gRPC) → Game Server (gRPC)
   - 每层都可以进行功能转发和监控

4. **异步任务管理**
   - StartJob返回job_id
   - jobIndex记录job_id -> 执行实例的映射
   - StreamJob通过job_id查询执行进度

5. **游戏隔离**
   - game_id和env作为所有操作的范围限定
   - 支持多游戏、多环境共存

### 10.2 关键文件汇总

```
Proto定义:
  - proto/croupier/control/v1/control.proto (Agent注册)
  - proto/croupier/agent/local/v1/local.proto (GameServer注册)
  - proto/croupier/function/v1/function.proto (函数调用)
  - proto/croupier/tunnel/v1/tunnel.proto (双向隧道)

实现:
  - internal/app/agent/function_server.go (Agent转发)
  - internal/app/agent/app.go (Agent启动)
  - internal/platform/control/server.go (ControlService)
  - internal/platform/agentlocal/local_control.go (LocalControlService)
  - cmd/agent/main.go (Agent监听)
  - cmd/server/main.go (Server+Edge)

SDK:
  - sdks/cpp/include/croupier/sdk/croupier_client.h (C++ SDK头文件)
  - sdks/cpp/src/croupier_client.cpp (C++ SDK实现)
```

---

## 附录：通信时序图

### A. 初始化序列

```
Game Server          Agent          Server
    │                 │              │
    ├─ RegisterLocal ─►│              │
    │                 │              │
    │                 ├─ Register ─► │
    │                 │              │
    │◄────────────────┤ (heartbeat)  │
    │  (HeartbeatReq) │              │
    │                 ├─ Heartbeat ─►│
    │                 │              │
```

### B. 同步调用序列

```
Client          Server         Agent       GameServer
  │              │              │              │
  ├─Invoke ─────►│              │              │
  │              │              │              │
  │              ├─ Invoke ────►│              │
  │              │              │              │
  │              │              ├─ Invoke ───►│
  │              │              │              │
  │              │              │◄─ Response ─┤
  │              │◄─ Response ───┤              │
  │              │              │              │
  │◄─Response ────┤              │              │
  │              │              │              │
```

### C. 异步任务序列

```
Client          Server         Agent       GameServer
  │              │              │              │
  ├─StartJob ───►│              │              │
  │              ├─StartJob ───►│              │
  │              │              ├─StartJob ──►│
  │              │              │◄─job_id ────┤
  │              │◄─job_id ───────┤              │
  │◄─job_id ──────┤              │              │
  │              │              │              │
  │ (客户端保存job_id)           │              │
  │              │              │              │
  ├─StreamJob(job_id) ────────┐ │              │
  │                           │ │              │
  │                    ┌──────┘ │              │
  │◄─ JobEvent ────────┤┌─────────────────────┐│
  │  (progress)        ││                     ││
  │◄─ JobEvent ────────┤│ 长时间操作执行      ││
  │  (log)             ││                     ││
  │◄─ JobEvent ────────┤└─────────────────────┘│
  │  (done, result)    │              │        │
  │                    │              │        │
```

