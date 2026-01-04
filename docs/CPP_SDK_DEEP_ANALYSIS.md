# Croupier C++ SDK 深度分析

**创建日期**: 2025-11-13  
**SDK版本**: 1.0.0  
**C++ 标准**: C++17  

---

## 📌 概述

Croupier C++ SDK 是一个高性能的游戏后端虚拟对象注册系统，提供：
- **虚拟对象管理** - 四层架构 (Function → Entity → Resource → Component)
- **与后台Agent交互** - 通过 gRPC LocalControlService 注册和通信
- **多游戏环境隔离** - 通过 game_id + env 实现租户隔离
- **权限和控制** - RBAC 和描述符验证机制

---

## 1️⃣ SPI (Service Provider Interface) 实现方式

### 1.1 核心 SPI 设计

**Handler 回调模式** (Service Provider Interface):
```cpp
// 类型定义：函数处理器
using FunctionHandler = std::function<std::string(
    const std::string& context,  // 请求上下文
    const std::string& payload   // JSON 序列化的参数
)>;
```

**文件位置**: `/Users/cui/Workspaces/croupier/sdks/cpp/include/croupier/sdk/croupier_client.h` (第 18 行)

### 1.2 SPI 注册机制

#### **方式1：基础函数注册 (向后兼容)**
```cpp
bool RegisterFunction(
    const FunctionDescriptor& desc,
    FunctionHandler handler
);
```
- **说明**: 注册单个原子操作函数
- **参数**: 函数描述符 + 处理器回调
- **用途**: 简单的函数导出

#### **方式2：虚拟对象注册 (推荐)**
```cpp
bool RegisterVirtualObject(
    const VirtualObjectDescriptor& desc,
    const std::map<std::string, FunctionHandler>& handlers
);
```
- **说明**: 将相关函数组织为业务对象
- **参数**: 对象描述符 + 操作函数映射
- **优势**: 关系明确，易于管理

#### **方式3：组件级注册 (生产推荐)**
```cpp
bool RegisterComponent(const ComponentDescriptor& comp);
bool LoadComponentFromFile(const std::string& config_file);
```
- **说明**: 整个子系统一次性注册
- **参数**: 组件描述符或配置文件路径
- **特点**: 支持声明式配置驱动

### 1.3 Handler 签名规范

```cpp
// 实现示例
std::string WalletTransferHandler(
    const std::string& context,  // 调用上下文
    const std::string& payload   // JSON: {"from_player_id":"p1", "to_player_id":"p2", "amount":"100"}
) {
    // 1. 解析 payload
    auto data = utils::ParseJSON(payload);
    std::string from_player = data["from_player_id"];
    std::string to_player = data["to_player_id"];
    std::string amount = data["amount"];
    
    // 2. 执行业务逻辑
    TransferResult result = WalletService::Transfer(from_player, to_player, amount);
    
    // 3. 返回 JSON 响应
    std::map<std::string, std::string> response;
    response["transfer_id"] = result.transfer_id;
    response["status"] = result.status;
    return utils::ToJSON(response);
}
```

**调用时机**：
- 后台 Agent 通过 gRPC 调用 → 转发到本地 Server → 查找对应 Handler → 同步执行
- 支持幂等性 (idempotency_key)

**文件位置**: 
- 实现: `/Users/cui/Workspaces/croupier/sdks/cpp/src/croupier_client.cpp` (第 102-407 行)
- 示例: `/Users/cui/Workspaces/croupier/sdks/cpp/examples/virtual_object_demo.cpp` (第 8-93 行)

---

## 2️⃣ game_id 和 env 相关代码

### 2.1 配置结构体

**ClientConfig** (客户端配置):
```cpp
struct ClientConfig {
    // ========== Game Environment Configuration ==========
    std::string game_id = "";              // 🎮 必需：游戏标识符
    std::string env = "development";       // 🔧 必需：环境隔离
    
    // 其他配置项
    std::string agent_addr = "127.0.0.1:19090";
    std::string local_listen = "127.0.0.1:0";
    std::string service_id = "cpp-service";
    
    // ... 认证、TLS 等
};
```

**InvokerConfig** (调用者配置):
```cpp
struct InvokerConfig {
    std::string address;
    std::string game_id;                   // 🎮 必需
    std::string env = "development";       // 🔧 必需
    // ... 其他配置
};
```

**文件位置**: `/Users/cui/Workspaces/croupier/sdks/cpp/include/croupier/sdk/croupier_client.h` (第 57-106 行)

### 2.2 game_id/env 的验证和使用

**初始化时验证**:
```cpp
explicit Impl(const ClientConfig& config) : config_(config) {
    // Validate required configuration
    if (config_.game_id.empty()) {
        std::cerr << "Warning: game_id is required for proper backend separation" << std::endl;
    }
    
    // Validate environment
    if (config_.env != "development" && config_.env != "staging" && config_.env != "production") {
        std::cerr << "Warning: Unknown environment '" << config_.env
                  << "'. Valid values: development, staging, production" << std::endl;
    }
    
    std::cout << "Initialized CroupierClient for game '" << config_.game_id
              << "' in '" << config_.env << "' environment" << std::endl;
}
```

**文件位置**: `/Users/cui/Workspaces/croupier/sdks/cpp/src/croupier_client.cpp` (第 117-131 行)

### 2.3 后台交互中的传递

**在 Proto 中的定义** (`control.proto`):
```protobuf
message RegisterRequest {
    string agent_id = 1;
    string version = 2;
    repeated FunctionDescriptor functions = 3;
    string rpc_addr = 4;
    string game_id = 5;           // ← 关键字段
    string env = 6;               // ← 关键字段
}
```

**文件位置**: `/Users/cui/Workspaces/croupier/proto/croupier/control/v1/control.proto` (第 17-24 行)

### 2.4 环境隔离策略

| 环境 | 用途 | 特点 |
|------|------|------|
| **development** | 本地开发 | 允许不安全连接 (insecure=true) |
| **staging** | 预发布测试 | 需要 TLS 但可能使用自签名证书 |
| **production** | 生产环境 | 强制 TLS + 证书验证 + 认证 Token |

**租户隔离机制**:
- Backend 按 (game_id, env) 元组索引所有资源
- 不同游戏的函数注册表完全隔离
- 调用时必须传递 game_id，后台验证租户权限

**示例配置**:
```cpp
// 游戏A开发环境
ClientConfig config_a;
config_a.game_id = "game-a";
config_a.env = "development";

// 游戏B生产环境
ClientConfig config_b;
config_b.game_id = "game-b";
config_b.env = "production";
config_b.insecure = false;
config_b.cert_file = "/etc/croupier/client.crt";
```

**文件位置**: `/Users/cui/Workspaces/croupier/sdks/cpp/include/croupier/sdk/croupier_client.h` (第 58-83 行)

---

## 3️⃣ 与后台 Agent 的注册交互机制

### 3.1 整体交互流程

```
┌─────────────────────────────────────────────────────────┐
│  游戏服务器 (C++ SDK)                                      │
└──────────────────────┬──────────────────────────────────┘
                       │
                       │ 1. LocalControlService::RegisterLocal()
                       │    (service_id, rpc_addr, functions)
                       ↓
┌──────────────────────────────────────────────────────────┐
│  Agent (19090 LocalControlService)                       │
├──────────────────────────────────────────────────────────┤
│  • 接收函数注册                                             │
│  • 返回 session_id                                         │
│  • 建立反向 Tunnel                                        │
└──────────────────────┬──────────────────────────────────┘
                       │
                       │ 2. 定期 Heartbeat 保持活跃
                       │    (service_id, session_id)
                       │
                       │ 3. Agent 负载均衡向后台 Server 转发
                       │    ControlService::Register()
                       ↓
┌──────────────────────────────────────────────────────────┐
│  Server (8443 ControlService)                           │
├──────────────────────────────────────────────────────────┤
│  • game_id + env 隔离维护                                  │
│  • RBAC 权限检查                                           │
│  • 函数注册表管理                                           │
└──────────────────────────────────────────────────────────┘
```

### 3.2 注册流程详解

#### **步骤1：连接到 Agent**

```cpp
bool Connect() {
    if (connected_) return true;

    std::cout << "Connecting to agent at: " << config_.agent_addr << std::endl;

    // NOTE: 当前为模拟实现，真实 gRPC 连接待实现
    // 预期实现：
    // 1. 建立 gRPC stub 到 LocalControlService
    // 2. 调用 RegisterLocal RPC
    // 3. 接收 session_id

    // Start local gRPC server
    if (!StartLocalServer()) {
        std::cerr << "Failed to start local server" << std::endl;
        return false;
    }

    // NOTE: Agent 注册功能待实现
    std::cout << "Registered " << handlers_.size() << " functions with agent" << std::endl;

    connected_ = true;
    return true;
}
```

**文件位置**: `/Users/cui/Workspaces/croupier/sdks/cpp/src/croupier_client.cpp` (第 317-337 行)

#### **步骤2：本地服务启动**

```cpp
bool StartLocalServer() {
    // Parse listen address
    std::string host, port_str;
    auto colon_pos = config_.local_listen.find(':');
    if (colon_pos != std::string::npos) {
        host = config_.local_listen.substr(0, colon_pos);
        port_str = config_.local_listen.substr(colon_pos + 1);
    } else {
        host = config_.local_listen;
        port_str = "0";
    }
    
    // Simulate port allocation
    int port = std::stoi(port_str);
    if (port == 0) {
        // Allocate random port
        std::random_device rd;
        std::mt19937 gen(rd());
        std::uniform_int_distribution<> dis(20000, 30000);
        port = dis(gen);
    }
    
    local_address_ = host + ":" + std::to_string(port);
    
    std::cout << "Local server listening on: " << local_address_ << std::endl;
    return true;
}
```

**文件位置**: `/Users/cui/Workspaces/croupier/sdks/cpp/src/croupier_client.cpp` (第 377-406 行)

#### **步骤3：gRPC Proto 消息定义**

**LocalControlService** (agent/local/v1/local.proto):
```protobuf
// 客户端 → Agent 注册请求
message RegisterLocalRequest {
    string service_id = 1;                        // e.g. "game-server-1"
    string version = 2;                           // e.g. "1.0.0"
    string rpc_addr = 3;                          // e.g. "127.0.0.1:20001"
    repeated LocalFunctionDescriptor functions = 4;  // 函数列表
}

// Agent 返回 session_id
message RegisterLocalResponse {
    string session_id = 1;  // 后续用于识别连接
}

// 定期心跳
message HeartbeatRequest {
    string service_id = 1;
    string session_id = 2;
}

// 获取本地函数列表（用于调试）
message ListLocalRequest {}
message ListLocalResponse {
    repeated LocalFunction functions = 1;  // 已注册函数
}
```

**文件位置**: `/Users/cui/Workspaces/croupier/proto/croupier/agent/local/v1/local.proto`

### 3.3 注册消息结构

**完整注册流程消息**:

1. **初始化阶段**：
```
C++ SDK                          Agent (19090)
   |                              |
   | RegisterLocal(                |
   |   service_id="game-1",       |
   |   version="1.0.0",           |
   |   rpc_addr="127.0.0.1:20001",|
   |   functions=[                |
   |     {id:"wallet.transfer"},  |
   |     {id:"wallet.get"}        |
   |   ]                          |
   | )                            |
   |----------------------------->|
   |                              | 存储注册信息
   |        RegisterLocalResponse  | 转发到 Server
   |        {session_id:"sess_abc"}|
   |<-----------------------------|
   |                              |
```

2. **心跳阶段**（定期，如 60 秒一次）：
```
C++ SDK                          Agent (19090)
   |                              |
   | Heartbeat(                   |
   |   service_id="game-1",       |
   |   session_id="sess_abc"      |
   | )                            |
   |----------------------------->|
   |        HeartbeatResponse     |
   |<-----------------------------|
   |                              |
```

3. **调用阶段**（来自后台）：
```
Server                           Agent                    C++ SDK
  |                               |                         |
  | FunctionService::Invoke()     |                         |
  | (wallet.transfer, game="x")   |                         |
  |------------------------------>|                         |
  |                               | 根据 game_id             |
  |                               | 查找 service_id="game-1" |
  |                               |                         |
  |                               | 转发 RPC 到本地服务      |
  |                               | (或反向隧道)            |
  |                               |----------------------->|
  |                               |                         | 执行 Handler
  |                               |                         | 返回结果
  |                               |<------------------------|
  |                    结果         |                         |
  |<------------------------------|                         |
```

### 3.4 实现关键要点

#### **重点：建立本地 gRPC 服务器**

C++ SDK 需要实现一个本地 gRPC 服务器来接收来自 Agent 的函数调用。这涉及：

```cpp
// 伪代码：实现思路
class LocalGameServer : public croupier::agent::local::v1::LocalControlService::Service {
public:
    ::grpc::Status InvokeFunction(
        ::grpc::ServerContext* context,
        const croupier::function::v1::InvokeRequest* request,
        croupier::function::v1::InvokeResponse* response
    ) override {
        // 1. 查找 function_id 对应的 handler
        auto handler = handlers_[request->function_id()];
        
        // 2. 执行 handler，获得 response payload
        std::string result = handler("", std::string(request->payload().begin(), request->payload().end()));
        
        // 3. 返回结果
        response->set_payload(result);
        return ::grpc::Status::OK;
    }
};
```

#### **重点：函数表维护**

```cpp
private:
    std::map<std::string, FunctionHandler> handlers_;      // function_id → handler
    std::map<std::string, FunctionDescriptor> descriptors_; // 元数据
    std::map<std::string, VirtualObjectDescriptor> objects_; // 对象描述
    std::map<std::string, ComponentDescriptor> components_;  // 组件描述
```

**文件位置**: `/Users/cui/Workspaces/croupier/sdks/cpp/src/croupier_client.cpp` (第 102-115 行)

### 3.5 连接参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `agent_addr` | `127.0.0.1:19090` | Agent 本地服务地址 |
| `local_listen` | `127.0.0.1:0` | 本地服务监听地址（0=自动分配端口） |
| `service_id` | `cpp-service` | 服务标识，用于 Agent 识别 |
| `timeout_seconds` | `30` | 连接超时（秒） |
| `heartbeat_interval` | `60` | 心跳间隔（秒） |

---

## 4️⃣ 权限相关的接口设计

### 4.1 权限模型概览

**多层权限架构**:
```
┌─────────────────────────────────────┐
│  Backend RBAC/ABAC (Server 层)       │
├─────────────────────────────────────┤
│  • 角色权限 (Role-Based)              │
│  • 属性权限 (Attribute-Based)         │
│  • 二人规则 (Two-Person Rule)        │
│  • 审计链 (Audit Chain)              │
└─────────────────┬───────────────────┘
                  │ 验证
┌─────────────────────────────────────┐
│  Agent 权限验证层                     │
├─────────────────────────────────────┤
│  • game_id 租户隔离                    │
│  • env 环境隔离                        │
│  • session 会话管理                   │
└─────────────────┬───────────────────┘
                  │ 授权
┌─────────────────────────────────────┐
│  C++ SDK（应用层）                    │
├─────────────────────────────────────┤
│  • Handler 执行                       │
│  • 本地业务逻辑                        │
│  • 结果返回                            │
└─────────────────────────────────────┘
```

### 4.2 SDK 端权限接口

#### **1. 认证配置**

```cpp
struct ClientConfig {
    // ========== Authentication ==========
    std::string auth_token;                    // Bearer token
    std::map<std::string, std::string> headers; // 自定义 HTTP 头
};

struct InvokerConfig {
    // ========== Authentication & Headers ==========
    std::string auth_token;                    // Bearer token
    std::map<std::string, std::string> headers; // 额外的请求头
};
```

**使用示例**:
```cpp
ClientConfig config;
config.game_id = "my-game";
config.env = "production";
config.auth_token = "Bearer eyJhbGc...";  // JWT Token
config.headers["X-Custom-Header"] = "value";
```

**文件位置**: `/Users/cui/Workspaces/croupier/sdks/cpp/include/croupier/sdk/croupier_client.h` (第 76-78, 100-102 行)

#### **2. TLS/mTLS 配置**

```cpp
struct ClientConfig {
    // ========== Optional TLS Configuration ==========
    bool insecure = true;              // 开发：true，生产：false
    std::string cert_file;             // 客户端证书
    std::string key_file;              // 私钥
    std::string ca_file;               // CA 证书
    std::string server_name;           // SNI 验证
};
```

**生产配置示例**:
```cpp
ClientConfig production_config;
production_config.game_id = "my-production-game";
production_config.env = "production";
production_config.insecure = false;
production_config.cert_file = "/etc/croupier/client.crt";
production_config.key_file = "/etc/croupier/client.key";
production_config.ca_file = "/etc/croupier/ca.crt";
production_config.server_name = "croupier.internal";
```

**文件位置**: `/Users/cui/Workspaces/croupier/sdks/cpp/include/croupier/sdk/croupier_client.h` (第 70-74 行)

### 4.3 后台权限协议

#### **Proto 定义**：

**control.proto** - 权限在后台处理：
```protobuf
message FunctionDescriptor {
    string id = 1;        // "player.ban"
    string version = 2;
    string category = 3;  // 权限分类：e.g. "player_management"
    string risk = 4;      // 风险等级："low" | "medium" | "high"
    string entity = 5;    // e.g. "player"
    string operation = 6; // "create" | "read" | "update" | "delete"
    bool enabled = 7;     // 是否启用（权限控制）
}

message RegisterRequest {
    string agent_id = 1;
    string version = 2;
    repeated FunctionDescriptor functions = 3;
    string rpc_addr = 4;
    string game_id = 5;          // ← 租户隔离
    string env = 6;              // ← 环境隔离
}
```

**文件位置**: `/Users/cui/Workspaces/croupier/proto/croupier/control/v1/control.proto` (第 7-24 行)

### 4.4 SDK 调用时的权限选项

#### **InvokeOptions 中的权限相关字段**

```cpp
struct InvokeOptions {
    std::string idempotency_key;        // 幂等性（防重复）
    std::string route;                  // 路由策略
    std::string target_service_id;      // 目标服务（权限受限）
    std::string hash_key;               // 一致性哈希
    std::string trace_id;               // 追踪 ID（审计）
    std::map<std::string, std::string> metadata; // 请求元数据（可用于权限信息）
};
```

**权限应用场景**:
```cpp
InvokeOptions options;
options.idempotency_key = croupier::sdk::utils::NewIdempotencyKey();
options.trace_id = "trace_123456";  // 用于审计日志追踪
options.metadata["user_id"] = "admin_user_1";  // 可在后台进行权限检查
options.metadata["approval_id"] = "approval_xyz";  // 审批流水号

std::string result = invoker.Invoke("player.ban", payload, options);
```

**文件位置**: `/Users/cui/Workspaces/croupier/sdks/cpp/include/croupier/sdk/croupier_client.h` (第 108-116 行)

### 4.5 权限验证流程

```
C++ SDK 调用
    ↓
┌─────────────────────────────────────┐
│ 1. 客户端验证 (SDK)                  │
│  • 检查认证 token                    │
│  • 验证 TLS 证书                     │
└──────────────┬──────────────────────┘
               ↓
┌─────────────────────────────────────┐
│ 2. Agent 层验证                      │
│  • 检查 session 有效性               │
│  • 验证 game_id 权限                 │
│  • 验证 env 访问权限                 │
└──────────────┬──────────────────────┘
               ↓
┌─────────────────────────────────────┐
│ 3. Server 层验证 (RBAC/ABAC)        │
│  • 检查用户角色                      │
│  • 检查函数访问权限                  │
│  • 检查属性权限 (ABAC)              │
│  • 触发审批流 (如果需要)             │
└──────────────┬──────────────────────┘
               ↓
┌─────────────────────────────────────┐
│ 4. 执行函数                          │
│  • 调用本地 handler                 │
│  • 生成审计日志                      │
└─────────────────────────────────────┘
```

### 4.6 权限相关的数据结构

#### **函数描述符中的权限信息**

```cpp
struct FunctionDescriptor {
    std::string id;         // "player.ban"
    std::string version;    // "1.0.0"
    std::map<std::string, std::string> schema;  // 参数 schema（可包含权限需求）
};

// 扩展提案（后续版本）：
struct FunctionDescriptorExtended {
    std::string category;       // "player_management"
    std::string risk_level;     // "high" - 需要更严格审批
    std::string required_role;  // "admin" - 所需角色
    bool requires_approval;     // true - 需要二人规则
};
```

**文件位置**: `/Users/cui/Workspaces/croupier/sdks/cpp/include/croupier/sdk/croupier_client.h` (第 20-25 行)

### 4.7 审计和追踪

#### **追踪 ID 机制**

```cpp
// 生成唯一追踪 ID
std::string trace_id = croupier::sdk::utils::NewIdempotencyKey();

InvokeOptions options;
options.trace_id = trace_id;
options.idempotency_key = croupier::sdk::utils::NewIdempotencyKey();

// 后台会在审计日志中记录：
// {
//   "trace_id": "abc123...",
//   "idempotency_key": "def456...",
//   "function_id": "player.ban",
//   "game_id": "game_x",
//   "timestamp": "2025-11-13T10:30:00Z",
//   "user": "admin_1",
//   "result": "success"
// }
```

**文件位置**: `/Users/cui/Workspaces/croupier/sdks/cpp/src/croupier_client.cpp` (第 17-27 行)

---

## 📂 目录结构详解

```
/Users/cui/Workspaces/croupier/sdks/cpp/
├── CMakeLists.txt                    # 构建系统配置
│                                    # - gRPC/Protobuf 集成
│                                    # - 多平台支持 (Windows/Linux/macOS)
│                                    # - vcpkg 依赖管理
│
├── include/croupier/sdk/
│   └── croupier_client.h            # 【核心公开接口】
│                                    # - CroupierClient (SPI 实现)
│                                    # - CroupierInvoker (调用者)
│                                    # - ClientConfig/InvokerConfig (game_id/env)
│                                    # - 虚拟对象相关数据结构
│
├── src/
│   └── croupier_client.cpp          # 【核心实现】
│                                    # - Impl class (PImpl 模式)
│                                    # - 本地 gRPC 服务器启动
│                                    # - Handler 映射和调用
│                                    # - game_id/env 验证逻辑
│
├── examples/
│   └── virtual_object_demo.cpp      # 【使用示例】
│                                    # - 6 个演示场景
│                                    # - 虚拟对象注册流程
│                                    # - 完整的 handler 实现
│
├── .github/workflows/
│   └── cpp-sdk-build.yml            # 【CI/CD 自动化】
│                                    # - 每日构建 (nightly)
│                                    # - 多平台矩阵编译
│                                    # - 自动发布 releases
│
├── vcpkg.json                       # 【依赖描述】
│                                    # - gRPC, Protobuf, nlohmann-json
│
├── README.md                        # 【用户文档】
│                                    # - 快速开始指南
│                                    # - API 参考
│                                    # - 部署说明
│
└── VIRTUAL_OBJECT_REGISTRATION.md  # 【架构文档】
                                    # - 四层设计
                                    # - ID 引用模式
                                    # - 实现指南
```

### 关键文件功能对应

| 功能 | 主要文件 | 行号范围 |
|------|--------|---------|
| **SPI 定义** | croupier_client.h | 10-220 |
| **game_id/env** | croupier_client.h | 57-106 |
| **虚拟对象结构** | croupier_client.h | 20-55 |
| **权限配置** | croupier_client.h | 70-102 |
| **Handler 实现** | croupier_client.cpp | 102-407 |
| **本地服务器** | croupier_client.cpp | 317-406 |
| **示例代码** | virtual_object_demo.cpp | 1-334 |

---

## 🔌 集成示例

### 完整的游戏经济系统集成

```cpp
#include "croupier/sdk/croupier_client.h"
using namespace croupier::sdk;

// 1. 定义钱包实体的操作处理器
std::string WalletGetHandler(const std::string& ctx, const std::string& payload) {
    auto data = utils::ParseJSON(payload);
    std::string wallet_id = data["wallet_id"];
    // 业务逻辑：从数据库获取钱包信息
    return "{\"wallet_id\":\"" + wallet_id + "\",\"balance\":\"1000\"}";
}

std::string WalletTransferHandler(const std::string& ctx, const std::string& payload) {
    // 业务逻辑：转账操作
    return "{\"status\":\"success\"}";
}

int main() {
    // 2. 配置客户端
    ClientConfig config;
    config.game_id = "mmorpg-game";        // 🎮 游戏标识
    config.env = "production";              // 🔧 环境隔离
    config.service_id = "economy-service";
    config.agent_addr = "127.0.0.1:19090";
    config.insecure = false;
    config.cert_file = "/etc/croupier/client.crt";
    
    CroupierClient client(config);
    
    // 3. 定义虚拟对象
    VirtualObjectDescriptor wallet;
    wallet.id = "wallet.entity";
    wallet.version = "1.0.0";
    wallet.name = "玩家钱包";
    wallet.operations["read"] = "wallet.get";
    wallet.operations["transfer"] = "wallet.transfer";
    
    RelationshipDef currency_rel;
    currency_rel.type = "many-to-one";
    currency_rel.entity = "currency";
    wallet.relationships["currency"] = currency_rel;
    
    // 4. 关联处理器
    std::map<std::string, FunctionHandler> handlers;
    handlers["wallet.get"] = WalletGetHandler;
    handlers["wallet.transfer"] = WalletTransferHandler;
    
    // 5. 注册虚拟对象
    if (!client.RegisterVirtualObject(wallet, handlers)) {
        std::cerr << "Failed to register wallet" << std::endl;
        return 1;
    }
    
    // 6. 连接并服务
    if (!client.Connect()) {
        std::cerr << "Failed to connect to agent" << std::endl;
        return 1;
    }
    
    // 7. 启动阻塞服务
    client.Serve();  // 接收来自后台的函数调用
    
    return 0;
}
```

---

## 📚 总结

| 方面 | 关键设计 |
|------|--------|
| **SPI** | Handler 回调 + 描述符驱动 |
| **game_id/env** | 客户端配置必需字段，实现租户隔离 |
| **Agent 交互** | LocalControlService gRPC，注册+心跳模式 |
| **权限** | 分层验证：认证 → Agent 授权 → Server RBAC/ABAC |
| **架构** | 四层：Function → Entity → Resource → Component |

**核心优势**：
- ✅ 高性能（ID 引用模式，无重对象序列化）
- ✅ 易扩展（声明式配置，模块化组件）
- ✅ 安全（多层权限验证，审计追踪）
- ✅ 多环境（game_id + env 隔离）

