# CroupierClient API 详解

`CroupierClient` 是 SDK 的核心类，负责管理与 Croupier Agent 的连接和函数注册。

## 类定义

```cpp
namespace croupier::sdk {

class CroupierClient {
public:
    // 构造函数
    explicit CroupierClient(const ClientConfig& config);

    // 析构函数
    ~CroupierClient();

    // 函数注册
    bool RegisterFunction(const FunctionDescriptor& desc, FunctionHandler handler);
    bool RegisterFunctions(const std::vector<FunctionDescriptor>& handlers);

    // 虚拟对象注册
    bool RegisterVirtualObject(
        const VirtualObjectDescriptor& desc,
        const std::map<std::string, FunctionHandler>& handlers
    );

    // 组件注册
    bool RegisterComponent(const ComponentDescriptor& comp);

    // 连接管理
    bool Connect();
    void Disconnect();
    bool IsConnected() const;

    // 服务管理
    void Serve();
    void Stop();

    // 回调设置
    void SetConnectionCallback(ConnectionCallback callback);
    void SetErrorCallback(ErrorCallback callback);
    void SetLogCallback(LogCallback callback);

    // 配置
    const ClientConfig& GetConfig() const;

private:
    // 内部实现...
};

} // namespace croupier::sdk
```

## 方法详解

### 构造函数

```cpp
explicit CroupierClient(const ClientConfig& config);
```

创建客户端实例。

**参数:**
- `config`: 客户端配置

**示例:**
```cpp
ClientConfig config;
config.game_id = "my-game";
config.env = "production";
config.agent_addr = "croupier-agent:19090";

CroupierClient client(config);
```

---

### RegisterFunction

```cpp
bool RegisterFunction(const FunctionDescriptor& desc, FunctionHandler handler);
```

注册单个函数到 Agent。

**参数:**
- `desc`: 函数描述符
- `handler`: 函数处理器

**返回值:** 成功返回 `true`

**示例:**
```cpp
FunctionDescriptor desc;
desc.id = "math.add";
desc.version = "0.1.0";
desc.name = "Add Numbers";
desc.description = "Adds two numbers together";

client.RegisterFunction(desc, [](const std::string& ctx, const std::string& payload) {
    auto data = utils::ParseJSON(payload);
    int result = data["a"] + data["b"];
    return utils::ToJSON({{"result", result}});
});
```

---

### RegisterVirtualObject

```cpp
bool RegisterVirtualObject(
    const VirtualObjectDescriptor& desc,
    const std::map<std::string, FunctionHandler>& handlers
);
```

注册虚拟对象及其操作处理器。

**参数:**
- `desc`: 虚拟对象描述符
- `handlers`: 操作名到处理器的映射

**返回值:** 成功返回 `true`

**示例:**
```cpp
VirtualObjectDescriptor wallet;
wallet.id = "wallet.entity";
wallet.operations["get"] = "wallet.get";
wallet.operations["transfer"] = "wallet.transfer";

std::map<std::string, FunctionHandler> handlers = {
    {"wallet.get", GetWalletHandler},
    {"wallet.transfer", TransferHandler}
};

client.RegisterVirtualObject(wallet, handlers);
```

---

### Connect

```cpp
bool Connect();
```

连接到 Croupier Agent。

**返回值:** 成功返回 `true`

**连接流程:**
1. 解析 Agent 地址
2. 建立 gRPC 连接
3. 发送注册信息
4. 启动心跳

**示例:**
```cpp
if (!client.Connect()) {
    std::cerr << "Failed to connect to agent" << std::endl;
    return 1;
}
```

---

### Serve

```cpp
void Serve();
```

开始处理请求（阻塞调用）。

**行为:**
- 阻塞当前线程
- 处理来自 Agent 的函数调用请求
- 自动重连（如果启用）
- 直到调用 `Stop()` 或连接关闭

**示例:**
```cpp
// 在单独的线程中运行
std::thread server_thread([&client]() {
    client.Serve();
});

// 主线程继续其他工作...
// ...

// 需要停止时
client.Stop();
server_thread.join();
```

---

### Stop

```cpp
void Stop();
```

停止服务并退出 `Serve()` 循环。

**示例:**
```cpp
// 信号处理
void SignalHandler(int signal) {
    client.Stop();
}

std::signal(SIGINT, SignalHandler);
client.Serve();
```

---

### SetConnectionCallback

```cpp
void SetConnectionCallback(ConnectionCallback callback);
```

设置连接状态变化回调。

**回调签名:**
```cpp
using ConnectionCallback = std::function<void(bool connected)>;
```

**示例:**
```cpp
client.SetConnectionCallback([](bool connected) {
    if (connected) {
        std::cout << "Connected to agent" << std::endl;
    } else {
        std::cout << "Disconnected from agent" << std::endl;
    }
});
```

---

### SetErrorCallback

```cpp
void SetErrorCallback(ErrorCallback callback);
```

设置错误回调。

**回调签名:**
```cpp
using ErrorCallback = std::function<void(const std::string& error)>;
```

**示例:**
```cpp
client.SetErrorCallback([](const std::string& error) {
    std::cerr << "Error: " << error << std::endl;
});
```

---

### SetLogCallback

```cpp
void SetLogCallback(LogCallback callback);
```

设置日志回调。

**回调签名:**
```cpp
using LogCallback = std::function<void(const std::string& level, const std::string& message)>;
```

**示例:**
```cpp
client.SetLogCallback([](const std::string& level, const std::string& message) {
    std::cout << "[" << level << "] " << message << std::endl;
});
```

## 使用模式

### 基础模式

```cpp
int main() {
    ClientConfig config;
    config.game_id = "demo-game";
    config.env = "development";
    config.agent_addr = "127.0.0.1:19090";

    CroupierClient client(config);

    // 注册函数
    client.RegisterFunction(desc, handler);

    // 连接并服务
    if (client.Connect()) {
        client.Serve();  // 阻塞
    }

    return 0;
}
```

### 多线程模式

```cpp
int main() {
    CroupierClient client(config);

    // 后台线程服务
    std::thread service_thread([&client]() {
        if (client.Connect()) {
            client.Serve();
        }
    });

    // 主线程做其他工作
    while (running) {
        // ...
        std::this_thread::sleep_for(std::chrono::seconds(1));
    }

    // 清理
    client.Stop();
    service_thread.join();

    return 0;
}
```

### 优雅退出模式

```cpp
std::atomic<bool> shutdown(false);

void SignalHandler(int signal) {
    shutdown = true;
}

int main() {
    std::signal(SIGINT, SignalHandler);
    std::signal(SIGTERM, SignalHandler);

    CroupierClient client(config);
    client.RegisterFunction(desc, handler);

    std::thread service_thread([&client]() {
        while (!shutdown) {
            if (!client.IsConnected()) {
                client.Connect();
            }
            client.Serve();
        }
    });

    while (!shutdown) {
        std::this_thread::sleep_for(std::chrono::seconds(1));
    }

    client.Stop();
    service_thread.join();

    return 0;
}
```
