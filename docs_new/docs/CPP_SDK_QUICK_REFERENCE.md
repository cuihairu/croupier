# Croupier C++ SDK 快速参考

## 🎯 四种使用模式速查表

### 模式1：基础函数注册
```cpp
CroupierClient client(config);
FunctionDescriptor desc{"wallet.transfer", "1.0.0"};
client.RegisterFunction(desc, [](const std::string& ctx, const std::string& payload) {
    // 处理函数逻辑
    return "{\"status\":\"ok\"}";
});
```

### 模式2：虚拟对象注册
```cpp
VirtualObjectDescriptor wallet;
wallet.id = "wallet.entity";
wallet.operations["transfer"] = "wallet.transfer";

std::map<std::string, FunctionHandler> handlers;
handlers["wallet.transfer"] = [](auto ctx, auto payload) { /* ... */ };

client.RegisterVirtualObject(wallet, handlers);
```

### 模式3：组件注册
```cpp
ComponentDescriptor comp;
comp.id = "economy-system";
comp.entities = {wallet, currency};
comp.functions = {market_trade};

client.RegisterComponent(comp);
```

### 模式4：配置文件驱动
```cpp
client.LoadComponentFromFile("economy-system.json");
```

---

## 🎮 game_id + env 配置表

| game_id 示例 | env | insecure | 用途 |
|-----------|-----|---------|------|
| `game-dev` | development | true | 本地开发 |
| `game-staging` | staging | true/false | 预发布测试 |
| `game-prod` | production | false | 生产环境 |

**关键代码**：
```cpp
config.game_id = "my-game";              // 🎮 必需
config.env = "production";               // 🔧 必需
config.insecure = false;                 // 🔐 生产需要关闭
config.auth_token = "Bearer token...";   // 🔑 认证
```

---

## 📡 与 Agent 交互流程

```
SDK 启动
  ↓
RegisterFunction/RegisterVirtualObject/RegisterComponent
  ↓
Connect()  ← 连接到 Agent (127.0.0.1:19090)
  ↓
Serve()    ← 启动本地 gRPC 服务器，等待调用
```

**Agent 消息协议**：
- **RegisterLocal**: 向 Agent 注册服务
- **Heartbeat**: 定期保活 (60秒)
- **FunctionService**: 接收来自 Server 的调用

---

## 🔐 权限和认证

### 开发环境
```cpp
ClientConfig config;
config.insecure = true;  // 允许不安全连接
```

### 生产环境
```cpp
ClientConfig config;
config.insecure = false;
config.cert_file = "/etc/certs/client.crt";
config.key_file = "/etc/certs/client.key";
config.ca_file = "/etc/certs/ca.crt";
config.auth_token = "Bearer <JWT>";
config.headers["X-Request-Id"] = "trace_123";
```

### 调用时的权限
```cpp
InvokeOptions opts;
opts.idempotency_key = croupier::sdk::utils::NewIdempotencyKey();
opts.trace_id = "trace_xyz";  // 审计
opts.metadata["user_id"] = "admin_1";

invoker.Invoke("player.ban", payload, opts);
```

---

## 📂 目录导航

| 文件 | 用途 |
|------|------|
| `include/croupier/sdk/croupier_client.h` | 核心公开接口（SPI 定义） |
| `src/croupier_client.cpp` | 实现细节（Handler 存储、验证） |
| `examples/virtual_object_demo.cpp` | 6 个完整示例 |
| `proto/croupier/control/v1/control.proto` | 后台消息协议 |
| `proto/croupier/agent/local/v1/local.proto` | Agent 协议 |

---

## ⚙️ Handler 签名

```cpp
// 标准签名
std::string HandlerFunction(
    const std::string& context,   // 调用上下文（目前未用）
    const std::string& payload    // JSON 字符串
);

// 实现模板
std::string MyHandler(const std::string& ctx, const std::string& payload) {
    // 1. 解析
    auto data = utils::ParseJSON(payload);
    std::string param = data["key"];
    
    // 2. 处理
    std::string result = DoSomething(param);
    
    // 3. 返回
    std::map<std::string, std::string> resp;
    resp["result"] = result;
    return utils::ToJSON(resp);
}
```

---

## 🔍 调试和诊断

### 获取注册信息
```cpp
auto objects = client.GetRegisteredObjects();
auto components = client.GetRegisteredComponents();

for (const auto& obj : objects) {
    std::cout << "Object: " << obj.id << " with " 
              << obj.operations.size() << " operations" << std::endl;
}
```

### 日志和错误处理
```cpp
try {
    client.RegisterVirtualObject(desc, handlers);
    client.Connect();
} catch (const std::exception& e) {
    std::cerr << "Error: " << e.what() << std::endl;
    // 实现重连逻辑
}
```

---

## 📊 性能考虑

### ✅ 最优实践
- 使用 ID 引用模式（只传递 ID 字符串）
- Handler 保持无状态
- 使用对象缓存而非重复序列化
- 定期检查心跳状态

### ❌ 避免的做法
- 传递序列化的大对象
- Handler 中阻塞操作（或异步化）
- 频繁重新连接
- 忽视幂等性检查

---

## 🚀 构建和依赖

### vcpkg 依赖
```
✓ gRPC      (gRPC 通信)
✓ Protobuf  (消息编码)
✓ nlohmann/json (JSON 处理)
✓ gtest     (可选测试)
```

### 构建命令
```bash
# 快速构建
./scripts/build.sh

# 启用测试和示例
./scripts/build.sh --tests ON --examples ON

# 清理重建
./scripts/build.sh --clean
```

---

## 📋 关键类和方法速查

### CroupierClient
```cpp
bool RegisterFunction(const FunctionDescriptor&, FunctionHandler);
bool RegisterVirtualObject(const VirtualObjectDescriptor&, const std::map<...>&);
bool RegisterComponent(const ComponentDescriptor&);
bool LoadComponentFromFile(const std::string&);
bool Connect();
void Serve();
void Stop();
std::vector<VirtualObjectDescriptor> GetRegisteredObjects() const;
std::vector<ComponentDescriptor> GetRegisteredComponents() const;
```

### CroupierInvoker
```cpp
bool Connect();
std::string Invoke(const std::string& func_id, const std::string& payload, const InvokeOptions&);
std::string StartJob(const std::string& func_id, const std::string& payload, const InvokeOptions&);
std::future<std::vector<JobEvent>> StreamJob(const std::string& job_id);
bool CancelJob(const std::string& job_id);
```

### 工具函数 (utils)
```cpp
std::string NewIdempotencyKey();                                    // 生成唯一 ID
bool ValidateJSON(const std::string&, const std::map<...>&);       // 验证 JSON
std::map<std::string, std::string> ParseJSON(const std::string&);  // 解析 JSON
std::string ToJSON(const std::map<...>&);                          // 转换为 JSON
bool ValidateObjectDescriptor(const VirtualObjectDescriptor&);     // 验证对象
bool ValidateComponentDescriptor(const ComponentDescriptor&);      // 验证组件
std::string GenerateObjectTemplate(const std::string& id);         // 生成模板
```

---

## 🔗 相关资源

- **完整分析**: `docs/CPP_SDK_DEEP_ANALYSIS.md`
- **README**: `sdks/cpp/README.md`
- **架构文档**: `sdks/cpp/VIRTUAL_OBJECT_REGISTRATION.md`
- **示例代码**: `sdks/cpp/examples/virtual_object_demo.cpp`
- **Proto 定义**:
  - `proto/croupier/control/v1/control.proto`
  - `proto/croupier/agent/local/v1/local.proto`
  - `proto/croupier/function/v1/function.proto`

