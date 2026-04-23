# API 参考

本文档提供 Croupier C++ SDK 的完整 API 参考。

## 命名空间

```cpp
namespace croupier::sdk {
    // 核心类
    class CroupierClient;
    struct ClientConfig;

    // 描述符
    struct FunctionDescriptor;
    struct VirtualObjectDescriptor;
    struct ComponentDescriptor;

    // 工具
    namespace utils {
        nlohmann::json ParseJSON(const std::string& str);
        std::string ToJSON(const nlohmann::json& json);
    }
}
```

## 核心类型

### FunctionHandler

函数处理器类型定义。

```cpp
using FunctionHandler = std::function<std::string(const std::string& context, const std::string& payload)>;
```

**参数:**
- `context`: 调用上下文（JSON 字符串）
- `payload`: 函数参数（JSON 字符串）

**返回值:** 函数执行结果（JSON 字符串）

---

### ClientConfig

客户端配置结构。

```cpp
struct ClientConfig {
    std::string agent_addr = "127.0.0.1:19090";  // Agent 地址
    std::string game_id;                          // 游戏 ID
    std::string env = "development";              // 环境
    std::string service_id = "cpp-service";       // 服务 ID
    bool insecure = true;                         // 是否跳过 TLS 验证
    int timeout_seconds = 30;                     // 超时时间

    // 重连配置
    bool auto_reconnect = true;
    int reconnect_interval_seconds = 5;
    int reconnect_max_attempts = 0;  // 0 = 无限

    // 安全配置
    struct {
        std::string cert_file;
        std::string key_file;
        std::string ca_file;
        std::string server_name;
    } security;

    // 认证配置
    struct {
        std::string token;
        std::map<std::string, std::string> headers;
    } auth;
};
```

**环境变量覆盖:**

| 环境变量 | 说明 |
|----------|------|
| `CROUPIER_GAME_ID` | 游戏 ID |
| `CROUPIER_ENV` | 环境 |
| `CROUPIER_AGENT_ADDR` | Agent 地址 |
| `CROUPIER_INSECURE` | 是否跳过 TLS |
| `CROUPIER_AUTO_RECONNECT` | 自动重连 |
| `CROUPIER_RECONNECT_INTERVAL_SECONDS` | 重连间隔 |
| `CROUPIER_RECONNECT_MAX_ATTEMPTS` | 最大重连次数 |

---

### FunctionDescriptor

函数描述符。

```cpp
struct FunctionDescriptor {
    std::string id;                    // 函数 ID，如 "wallet.transfer"
    std::string version;               // 版本号，如 "0.1.0"
    std::string name;                  // 显示名称
    std::string description;           // 描述
    nlohmann::json input_schema;       // 输入 Schema
    nlohmann::json output_schema;      // 输出 Schema
    std::vector<std::string> tags;     // 标签
    int timeout_ms = 30000;            // 超时时间（毫秒）
};
```

---

### VirtualObjectDescriptor

虚拟对象描述符。

```cpp
struct VirtualObjectDescriptor {
    std::string id;                    // 对象 ID
    std::string version;               // 版本号
    std::string name;                  // 显示名称
    std::string description;           // 描述
    std::map<std::string, std::string> operations;  // 操作映射
    nlohmann::json schema;             // 数据 Schema
    nlohmann::json metadata;           // 元数据
};
```

---

## 类: CroupierClient

主客户端类，管理与 Croupier Agent 的连接和函数注册。

### 构造函数

```cpp
CroupierClient(const ClientConfig& config);
```

### 公共方法

#### RegisterFunction

注册单个函数。

```cpp
bool RegisterFunction(const FunctionDescriptor& desc, FunctionHandler handler);
```

**参数:**
- `desc`: 函数描述符
- `handler`: 函数处理器

**返回值:** 成功返回 `true`，失败返回 `false`

---

#### RegisterVirtualObject

注册虚拟对象。

```cpp
bool RegisterVirtualObject(
    const VirtualObjectDescriptor& desc,
    const std::map<std::string, FunctionHandler>& handlers
);
```

**参数:**
- `desc`: 虚拟对象描述符
- `handlers`: 操作处理器映射

**返回值:** 成功返回 `true`，失败返回 `false`

---

#### RegisterComponent

注册组件。

```cpp
bool RegisterComponent(const ComponentDescriptor& comp);
```

**参数:**
- `comp`: 组件描述符

**返回值:** 成功返回 `true`，失败返回 `false`

---

#### Connect

连接到 Agent。

```cpp
bool Connect();
```

**返回值:** 成功返回 `true`，失败返回 `false`

---

#### Serve

开始服务（阻塞调用）。

```cpp
void Serve();
```

此方法会阻塞当前线程，处理来自 Agent 的函数调用请求。

---

#### Stop

停止服务。

```cpp
void Stop();
```

---

#### Close

关闭连接并释放资源。

```cpp
void Close();
```

---

#### IsConnected

检查连接状态。

```cpp
bool IsConnected() const;
```

**返回值:** 已连接返回 `true`

---

## 工具函数

### utils::ParseJSON

解析 JSON 字符串。

```cpp
nlohmann::json ParseJSON(const std::string& str);
```

**异常:** 如果 JSON 格式无效，抛出 `std::runtime_error`

---

### utils::ToJSON

将对象转换为 JSON 字符串。

```cpp
std::string ToJSON(const nlohmann::json& json);
```

---

### utils::ValidateJSON

根据 Schema 验证 JSON。

```cpp
bool ValidateJSON(const nlohmann::json& data, const nlohmann::json& schema);
```

---

## 配置加载

### ClientConfigLoader

配置加载器类。

```cpp
class ClientConfigLoader {
public:
    // 从文件加载
    ClientConfig LoadFromFile(const std::string& file_path);

    // 从 JSON 字符串加载
    ClientConfig LoadFromJson(const std::string& json_content);

    // 带环境变量覆盖加载
    ClientConfig LoadWithEnvironmentOverrides(
        const std::string& file_path,
        const std::string& env_prefix = "CROUPIER_"
    );

    // 验证配置
    std::vector<std::string> ValidateConfig(const ClientConfig& config);

    // 生成示例配置
    std::string GenerateExampleConfig(const std::string& env = "development");
};
```

## 错误处理

### CroupierException

基础异常类。

```cpp
class CroupierException : public std::runtime_error {
public:
    explicit CroupierException(const std::string& message);
    explicit CroupierException(const std::string& code, const std::string& message);
};
```

### 异常类型

| 异常类 | 说明 |
|--------|------|
| `ConnectionException` | 连接失败 |
| `RegistrationException` | 注册失败 |
| `ConfigException` | 配置错误 |
| `TimeoutException` | 操作超时 |

## 回调类型

### ConnectionCallback

连接状态变化回调。

```cpp
using ConnectionCallback = std::function<void(bool connected)>;
```

### ErrorCallback

错误回调。

```cpp
using ErrorCallback = std::function<void(const std::string& error)>;
```

### LogCallback

日志回调。

```cpp
using LogCallback = std::function<void(const std::string& level, const std::string& message)>;
```
