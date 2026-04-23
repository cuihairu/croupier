# 函数注册

函数是 Croupier C++ SDK 中最基础的执行单元。

## 函数处理器

### 基础签名

```cpp
using FunctionHandler = std::function<std::string(const std::string& context, const std::string& payload)>;
```

- `context`: 调用上下文信息（元数据）
- `payload`: 函数输入参数（JSON 字符串）
- 返回值: 函数执行结果（JSON 字符串）

### 简单函数示例

```cpp
std::string HelloWorld(const std::string& context, const std::string& payload) {
    return R"({"message": "Hello, World!"})";
}

std::string Echo(const std::string& context, const std::string& payload) {
    return payload;  // 直接返回输入
}

std::string Add(const std::string& context, const std::string& payload) {
    auto data = utils::ParseJSON(payload);
    int a = data["a"];
    int b = data["b"];
    return utils::ToJSON({{"result", a + b}});
}
```

## 注册函数

### 单个函数注册

```cpp
CroupierClient client(config);

FunctionDescriptor desc;
desc.id = "hello.world";
desc.version = "0.1.0";
desc.name = "Hello World";
desc.description = "A simple greeting function";

client.RegisterFunction(desc, HelloWorld);
```

### 批量函数注册

```cpp
std::vector<std::pair<FunctionDescriptor, FunctionHandler>> functions = {
    {{"math.add", "0.1.0", "Add", "Add two numbers"}, Add},
    {{"math.subtract", "0.1.0", "Subtract", "Subtract two numbers"}, Subtract},
    {{"math.multiply", "0.1.0", "Multiply", "Multiply two numbers"}, Multiply},
};

for (const auto& [desc, handler] : functions) {
    client.RegisterFunction(desc, handler);
}
```

### 使用 Lambda 函数

```cpp
client.RegisterFunction(
    FunctionDescriptor{"utils.timestamp", "0.1.0"},
    [](const std::string& ctx, const std::string& payload) {
        auto now = std::chrono::system_clock::now();
        auto timestamp = std::chrono::duration_cast<std::chrono::milliseconds>(
            now.time_since_epoch()
        ).count();
        return utils::ToJSON({{"timestamp", timestamp}});
    }
);
```

### 使用类成员函数

```cpp
class GameService {
public:
    void RegisterFunctions(CroupierClient& client) {
        client.RegisterFunction(
            FunctionDescriptor{"game.start", "0.1.0"},
            [this](const std::string& ctx, const std::string& payload) {
                return this->StartGame(ctx, payload);
            }
        );
    }

private:
    std::string StartGame(const std::string& context, const std::string& payload) {
        // 游戏启动逻辑
        return R"({"status": "started"})";
    }
};
```

## 函数描述符

### 完整描述符

```cpp
struct FunctionDescriptor {
    std::string id;                    // 唯一标识符，如 "wallet.transfer"
    std::string version;               // 版本号，如 "0.1.0"
    std::string name;                  // 显示名称
    std::string description;           // 描述信息
    nlohmann::json input_schema;       // 输入参数的 JSON Schema
    nlohmann::json output_schema;      // 输出结果的 JSON Schema
    std::vector<std::string> tags;     // 标签
    int timeout_ms = 30000;            // 超时时间（毫秒）
};
```

### 带输入输出 Schema 的函数

```cpp
FunctionDescriptor transfer_desc;
transfer_desc.id = "wallet.transfer";
transfer_desc.version = "0.1.0";
transfer_desc.name = "转账";
transfer_desc.description = "在两个玩家之间转账";

// 输入 Schema
transfer_desc.input_schema = R"({
    "type": "object",
    "properties": {
        "from_player_id": {"type": "string"},
        "to_player_id": {"type": "string"},
        "currency": {"type": "string", "enum": ["gold", "diamond"]},
        "amount": {"type": "integer", "minimum": 1}
    },
    "required": ["from_player_id", "to_player_id", "amount"]
})"_json;

// 输出 Schema
transfer_desc.output_schema = R"({
    "type": "object",
    "properties": {
        "success": {"type": "boolean"},
        "transaction_id": {"type": "string"}
    }
})"_json;

client.RegisterFunction(transfer_desc, TransferHandler);
```

## 上下文信息

### 解析上下文

```cpp
std::string HandlerWithContext(const std::string& context, const std::string& payload) {
    auto ctx = utils::ParseJSON(context);

    std::string game_id = ctx["game_id"];
    std::string env = ctx["env"];
    std::string caller_id = ctx["caller_id"];

    std::cout << "Called from: " << caller_id << " in " << game_id << "/" << env << std::endl;

    // 处理业务逻辑...
    return R"({"success": true})";
}
```

### 上下文字段

| 字段 | 说明 |
|------|------|
| `game_id` | 游戏标识符 |
| `env` | 环境（development/staging/production） |
| `caller_id` | 调用者标识符 |
| `call_id` | 调用唯一标识符 |
| `timestamp` | 调用时间戳 |
| `auth` | 认证信息 |

## 错误处理

### 返回错误

```cpp
std::string SafeTransfer(const std::string& context, const std::string& payload) {
    try {
        auto data = utils::ParseJSON(payload);

        // 验证输入
        if (!data.contains("from_player_id") || !data.contains("to_player_id")) {
            return R"({
                "success": false,
                "error": {
                    "code": "INVALID_INPUT",
                    "message": "缺少必需的玩家 ID"
                }
            })";
        }

        // 业务逻辑...
        return R"({"success": true})";

    } catch (const std::exception& e) {
        return utils::ToJSON({
            {"success", false},
            {"error", {{"code", "INTERNAL_ERROR"}, {"message", e.what()}}}
        });
    }
}
```

### 统一错误响应

```cpp
namespace Responses {
    nlohmann::json Success(const nlohmann::json& data = nullptr) {
        nlohmann::json response = {{"success", true}};
        if (data != nullptr) {
            response["data"] = data;
        }
        return response;
    }

    nlohmann::json Error(const std::string& code, const std::string& message) {
        return {
            {"success", false},
            {"error", {{"code", code}, {"message", message}}}
        };
    }

    nlohmann::json ValidationError(const std::string& field) {
        return Error("VALIDATION_ERROR", "字段验证失败: " + field);
    }
};

// 使用
std::string Handler(const std::string& ctx, const std::string& payload) {
    if (!ValidateInput(payload)) {
        return Responses::ValidationError("player_id");
    }
    return Responses::Success({{"processed", true}});
}
```

## 异步处理

### 使用线程池

```cpp
#include <thread>
#include <queue>
#include <condition_variable>

class AsyncFunctionServer {
public:
    void RegisterAsyncFunction(CroupierClient& client) {
        client.RegisterFunction(
            FunctionDescriptor{"async.process", "0.1.0"},
            [this](const std::string& ctx, const std::string& payload) {
                // 提交到任务队列
                std::unique_lock lock(queue_mutex_);
                tasks_.push({ctx, payload});
                condition_.notify_one();

                // 立即返回任务 ID
                return utils::ToJSON({
                    {"task_id", GenerateTaskId()},
                    {"status", "pending"}
                });
            }
        );
    }

    void StartWorkers(int num_threads) {
        for (int i = 0; i < num_threads; ++i) {
            workers_.emplace_back([this] { WorkerLoop(); });
        }
    }

private:
    void WorkerLoop() {
        while (running_) {
            std::unique_lock lock(queue_mutex_);
            condition_.wait(lock, [this] { return !tasks_.empty() || !running_; });

            if (!tasks_.empty()) {
                auto [ctx, payload] = tasks_.front();
                tasks_.pop();
                lock.unlock();

                // 处理任务
                ProcessTask(ctx, payload);
            }
        }
    }

    std::queue<std::pair<std::string, std::string>> tasks_;
    std::vector<std::thread> workers_;
    std::mutex queue_mutex_;
    std::condition_variable condition_;
    bool running_ = true;
};
```

## 最佳实践

### 1. 函数命名规范

```cpp
// 使用点分层命名
"player.get"           // 获取玩家信息
"player.ban"           // 封禁玩家
"wallet.transfer"      // 钱包转账
"inventory.add_item"   // 添加物品
```

### 2. 版本管理

```cpp
// 支持多版本并存
client.RegisterFunction({"wallet.transfer", "0.1.0"}, TransferV1);
client.RegisterFunction({"wallet.transfer", "2.0.0"}, TransferV2);
```

### 3. 参数验证

```cpp
bool ValidateTransfer(const nlohmann::json& payload) {
    if (!payload.contains("from_player_id") || !payload["from_player_id"].is_string())
        return false;
    if (!payload.contains("to_player_id") || !payload["to_player_id"].is_string())
        return false;
    if (!payload.contains("amount") || !payload["amount"].is_number_integer())
        return false;
    if (payload["amount"] <= 0)
        return false;
    return true;
}
```

### 4. 超时处理

```cpp
// 设置合理的超时
FunctionDescriptor desc;
desc.id = "complex.process";
desc.timeout_ms = 60000;  // 60 秒
```
