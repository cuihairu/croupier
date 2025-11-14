# Croupier gRPC通信快速参考

## 1. 服务端口和通信方向一览表

| 组件 | 监听端口 | 角色 | 连接对象 | 协议 |
|------|--------|------|--------|------|
| **Server** | 8443 (gRPC) | Server | Agent, Edge | gRPC + JSON |
| **Server** | 8080 (HTTP) | Server | Web UI, 外部 | HTTP REST |
| **Agent** | 19090 (gRPC) | Server | Game Server | gRPC + JSON |
| **Edge** | 8443 (gRPC) | Server | Agent (outbound) | gRPC + JSON |
| **Game Server** | 动态 (gRPC) | Server | Agent | gRPC + JSON |

---

## 2. gRPC服务映射表

### ControlService (Server)
```
目标：Server:8443
服务端：Server
调用端：Agent

方法：
  Register(agent_id, game_id, functions) → session_id
  Heartbeat(agent_id, session_id) → void
  RegisterCapabilities(provider_meta, manifest) → void
```

### LocalControlService (Agent)
```
目标：Agent:19090
服务端：Agent
调用端：Game Server

方法：
  RegisterLocal(service_id, functions) → session_id
  Heartbeat(service_id, session_id) → void
  ListLocal() → functions[]
  GetJobResult(job_id) → {state, payload, error}
```

### FunctionService (全系统)
```
目标：多个节点上的FunctionService
服务端：Agent, Edge, Game Server (都可能实现)
调用端：Server, Agent, Game Server

方法：
  Invoke(function_id, payload, idempotency_key) → InvokeResponse
  StartJob(function_id, payload, idempotency_key) → job_id
  StreamJob(job_id) → stream JobEvent  [单向流]
  CancelJob(job_id) → void
```

### TunnelService (Edge/Server)
```
目标：Edge:8443 (或 Server:8443)
服务端：Edge/Server
调用端：Agent (outbound)

方法：
  Open(stream TunnelMessage) → stream TunnelMessage  [双向流]
  
消息类型：
  TunnelMessage.invoke → InvokeFrame
  TunnelMessage.start → StartJobFrame
  TunnelMessage.cancel → CancelJobFrame
  TunnelMessage.result → ResultFrame
  TunnelMessage.start_r → StartJobResult
  TunnelMessage.job_evt → JobEventFrame
```

---

## 3. 消息流向矩阵

```
          → Server  → Edge  → Agent  → GameServer
Web UI    ✓(HTTP)   ✗      ✗      ✗
Server    ✗         ✓(F.S) ✗      ✗
Edge      ◄(Tunnel) ✓(I)   ✓(F.S) ✗
Agent     ◄(Ctrl)   ◄      ✓(F.S) ✓(F.S)
GameServer✗         ✗      ✓(Reg) ✓(F.S)

F.S = FunctionService
I = Invoke (转发)
Ctrl = ControlService
Reg = RegisterLocal
◄ = 入站  → = 出站  ✓ = 支持  ✗ = 不支持
```

---

## 4. 数据流快速查询

### 同步调用 (Invoke)
```
Client
  ├─ HTTP POST /invoke
  ▼
Server (HTTP Handler)
  ├─ FunctionService.Invoke
  ▼
Edge (如果配置)
  ├─ FunctionService.Invoke
  ▼
Agent
  ├─ 查找 LocalStore[function_id] → game_server_addr
  ├─ FunctionService.Invoke
  ▼
Game Server
  ├─ Handler执行
  ▼
返回 InvokeResponse (payload)
```

### 异步任务 (StartJob + StreamJob)
```
Client
  ├─ HTTP POST /start-job
  ▼
Server
  ├─ FunctionService.StartJob → Edge → Agent → GameServer
  ▼
GameServer 启动后台任务，返回 job_id
Server 返回 job_id 给客户端
  │
  ├─ Client 轮询或订阅 /jobs/{job_id}/stream
  ▼
Server
  ├─ FunctionService.StreamJob
  ├─ Agent查询 jobIndex[job_id] → game_server_addr
  ├─ StreamJob → GameServer
  ▼
GameServer 推送 JobEvent (progress/log/done/error)
  │
  ├─ 通过 StreamJob 返回
  ├─ Agent → Edge → Server → HTTP(SSE)
  ▼
Client 接收事件流
```

### 取消任务 (CancelJob)
```
Client
  ├─ HTTP DELETE /jobs/{job_id}
  ▼
Server
  ├─ FunctionService.CancelJob
  ▼
Agent
  ├─ jobIndex.Get(job_id) → game_server_addr
  ├─ FunctionService.CancelJob
  ▼
GameServer
  ├─ 停止或中断任务
  ▼
返回确认
```

---

## 5. Agent 的双重身份速记

### Agent作为gRPC Server
```
监听：0.0.0.0:19090
提供：
  - LocalControlService (Game Server注册、心跳、查询)
  - FunctionService (Invoke/StartJob/StreamJob/CancelJob转发)
```

### Agent作为gRPC Client
```
连接目标：
  - Server:8443 (Register, Heartbeat, RegisterCapabilities)
  - GameServer:动态 (FunctionService调用转发)

转发机制：
  Agent.FunctionService.Invoke(request)
    → LocalStore.pickInstance(function_id)
    → grpc.Dial(gameserver_addr)
    → GameServer.FunctionService.Invoke(request)
    → 返回响应
```

---

## 6. 双向通信详解

### 只有 TunnelService 是真正双向的

#### TunnelService.Open (Bi-directional Stream)
```
Agent (Client)                    Edge/Server (Server)
  │                                   │
  ├─ Send TunnelMessage            │
  │  ├─ type: "invoke"             │
  │  ├─ invoke: InvokeFrame         │
  │                                 ├─ 处理
  │◄─ Recv TunnelMessage            │
  │  ├─ type: "result"              │
  │  ├─ result: ResultFrame         │
  │                                 │
  ├─ Send TunnelMessage            │
  │  ├─ type: "start_job"           │
  │  ├─ start: StartJobFrame        │
  │                                 ├─ 启动任务
  │◄─ Recv TunnelMessage            │
  │  ├─ type: "start_r"             │
  │  ├─ start_r: StartJobResult     │
  │                                 │
  │ (保活)                          │
  │                                 │
  │◄─ Recv TunnelMessage (推送)     │
  │  ├─ type: "job_evt"             │
  │  ├─ job_evt: JobEventFrame      │
  │  (进度、日志、完成)
```

#### StreamJob 只是单向Server Stream
```
Client                           Server
  │
  ├─ JobStreamRequest
  │  ├─ job_id
  │                               ├─ 查询任务
  │◄─ stream JobEvent
  │  ├─ {type: "progress", ...}
  │  ├─ {type: "log", ...}
  │  ├─ {type: "done", payload: ...}
  │
  ✗ 不支持Client → Server的推送
```

---

## 7. SDK快速使用

### C++ Provider (游戏服提供函数)
```cpp
#include "croupier/sdk/croupier_client.h"

// 配置
ClientConfig config{
    .agent_addr = "127.0.0.1:19090",
    .service_id = "my-game-server",
    .game_id = "game001",
    .env = "development"
};

// 注册函数
CroupierClient client(config);
client.RegisterFunction(
    {"player.ban", "1.0"},
    [](const std::string& ctx, const std::string& payload) {
        return banPlayer(payload);
    }
);

// 启动
client.Connect();
client.Serve();  // 阻塞，监听来自Agent的调用
```

### C++ Consumer (远程调用函数)
```cpp
#include "croupier/sdk/croupier_client.h"

InvokerConfig config{
    .address = "127.0.0.1:8443",  // Server or Edge
    .game_id = "game001",
    .env = "development"
};

CroupierInvoker invoker(config);
invoker.Connect();

// 同步调用
std::string result = invoker.Invoke("player.ban", 
    R"({"player_id": "123"})");

// 异步调用
std::string job_id = invoker.StartJob("long.task", "{}");
auto future = invoker.StreamJob(job_id);
// 等待结果...
invoker.CancelJob(job_id);  // 可取消
```

---

## 8. 关键配置参数

### ClientConfig (SDK端)
```cpp
agent_addr           = "127.0.0.1:19090"    // Agent地址
service_id           = "my-service"         // 此Game Server的ID
game_id              = "game001"            // 必需：游戏范围
env                  = "development"        // 可选：prod/stage
insecure             = true                 // 开发:true, 生产:false
cert_file            = "client.crt"         // mTLS证书
key_file             = "client.key"         // mTLS密钥
ca_file              = "ca.crt"             // CA证书
timeout_seconds      = 30                   // 超时
heartbeat_interval   = 60                   // 心跳间隔(秒)
```

### InvokeOptions (调用选项)
```cpp
idempotency_key      = "xxx"         // 幂等性
route                = "lb"          // lb/broadcast/targeted/hash
target_service_id    = "service-1"   // route=targeted时指定
hash_key             = "player:123"  // route=hash时使用
trace_id             = "trace-xxx"   // 链路追踪
metadata             = {}            // 自定义元数据
```

---

## 9. 故障排查快速表

| 现象 | 可能原因 | 排查步骤 |
|------|--------|--------|
| Game → Agent连接失败 | Agent未启动或端口错误 | 检查Agent监听19090，确认service_id/game_id配置 |
| Agent → Server连接失败 | Server未启动，或mTLS证书错误 | 检查Server监听8443，开发环境使用--insecure |
| 函数调用超时 | Game Server无此函数，或处理耗时 | 检查LocalStore中是否注册了此函数，看处理代码逻辑 |
| StreamJob无事件推送 | jobIndex未记录job_id | 检查Agent中StartJob时是否成功保存到jobIndex |
| 任务不隔离混乱 | game_id配置错误或为空 | 检查所有端的game_id配置是否一致 |
| gRPC连接断开频繁 | 心跳间隔太长或网络不稳定 | 减小heartbeat_interval或检查网络 |

---

## 10. 性能调优建议

| 参数 | 建议值 | 说明 |
|------|------|------|
| 同步Invoke超时 | 3-5秒 | Agent中默认3秒，超过则关闭连接 |
| StreamJob超时 | 根据业务 | 不设超时，由应用层手动cancel |
| 心跳间隔 | 30-60秒 | 越短越及时发现故障，但增加开销 |
| 连接复用 | 推荐 | gRPC内部自动连接池，无需手动管理 |
| 消息大小 | <10MB | 超大消息建议分片传输 |
| 并发数 | Agent可支持数百并发 | 取决于业务逻辑耗时 |

---

## 11. 完整通信示例

### 场景：远程禁封玩家

```
User在Web UI点击 "ban player 123"
  │
  ├─ HTTP POST /games/game001/functions/player.ban
  │  Body: {"player_id": "123", "reason": "cheating"}
  ▼
Server (httpserver)
  ├─ 认证检查
  ├─ 权限检查 (RBAC)
  ├─ 审计记录
  ├─ 调用 edgeForwarder.Invoke()
  ▼
Edge (gRPC Client)
  ├─ FunctionService.Invoke()
  │  request: {function_id: "player.ban", payload: "..."}
  ▼
Agent (gRPC Server → gRPC Client)
  ├─ FunctionService.Invoke()
  ├─ LocalStore.pickInstance("player.ban")
  │  → "gameserver1:50051"
  ├─ grpc.Dial("gameserver1:50051")
  ├─ FunctionService.Invoke()
  ▼
Game Server (gRPC Server)
  ├─ Handler执行 (player.ban)
  ├─ 更新数据库：UPDATE players SET banned=true WHERE id=123
  ├─ 返回 {status: "ok", message: "player banned"}
  ▼
返回响应链路 (逆向)
Agent → Edge → Server
  │
  ├─ HTTP 200 OK
  │  Body: {"status": "ok", "message": "player banned"}
  ▼
Web UI
  ├─ 显示成功提示
  ├─ 刷新玩家列表
```

### 场景：异步数据导出（长时间操作）

```
User在Web UI点击 "export player logs"
  │
  ├─ HTTP POST /games/game001/functions/export.logs
  │  Body: {"player_id": "123"}
  ▼
Server
  ├─ StartJob("export.logs", payload)
  │  → Edge → Agent → GameServer
  │  ← job_id: "job-abc123"
  ├─ 返回 HTTP 202 Accepted, location: /jobs/job-abc123
  ▼
Web UI
  ├─ 开始轮询 GET /jobs/job-abc123/stream
  ▼
Server (Streaming)
  ├─ FunctionService.StreamJob("job-abc123")
  │  → Edge → Agent (jobIndex.Get("job-abc123"))
  │  → GameServer
  ├─ GameServer后台导出操作：
  │  - 查询数据库
  │  - 生成CSV文件
  │  - 每进度变化 Emit JobEvent
  ▼
返回事件流
  ├─ {type: "progress", progress: 25}
  ├─ {type: "log", message: "reading records"}
  ├─ {type: "progress", progress: 50}
  ├─ {type: "log", message: "generating file"}
  ├─ {type: "progress", progress: 100}
  ├─ {type: "done", payload: {"file_url": "s3://..."}}
  ▼
Web UI
  ├─ 显示进度条
  ├─ 收到done事件后显示下载链接
```

---

## 12. Proto字段速查

### InvokeRequest
```protobuf
function_id         string              // "player.ban"
idempotency_key     string              // 幂等性key
payload             bytes               // JSON/Proto二进制
metadata            map<string,string>  // 元数据
```

### InvokeResponse
```protobuf
payload             bytes               // 返回数据
```

### StartJobResponse
```protobuf
job_id              string              // 后续用于StreamJob
```

### JobEvent
```protobuf
type                string              // "progress"|"log"|"done"|"error"
message             string              // 日志或错误信息
progress            int32               // 0-100
payload             bytes               // 最终结果
```

### TunnelMessage (复合)
```protobuf
type                string              // 消息类型标识
hello               Hello               // 握手信息
invoke              InvokeFrame         // 调用请求
result              ResultFrame         // 调用结果
start                StartJobFrame       // 启动任务
start_r             StartJobResult      // 启动结果
job_evt             JobEventFrame       // 任务事件
cancel              CancelJobFrame      // 取消请求
```

