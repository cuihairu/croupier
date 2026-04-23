# 函数 API 详解

本文档详细说明函数注册和使用的 API。

## FunctionDescriptor

函数描述符结构定义。

```cpp
struct FunctionDescriptor {
    std::string id;                    // 必需: 函数唯一标识符
    std::string version;               // 必需: 版本号
    std::string name;                  // 可选: 显示名称
    std::string description;           // 可选: 描述信息
    nlohmann::json input_schema;       // 可选: 输入参数的 JSON Schema
    nlohmann::json output_schema;      // 可选: 输出结果的 JSON Schema
    std::vector<std::string> tags;     // 可选: 标签
    int timeout_ms = 30000;            // 可选: 超时时间（毫秒）
};
```

## 字段说明

### id

函数唯一标识符，使用点分层命名。

```cpp
desc.id = "wallet.transfer";        // 钱包转账
desc.id = "player.ban";             // 封禁玩家
desc.id = "inventory.add_item";     // 添加物品
```

### version

语义化版本号。

```cpp
desc.version = "0.1.0";   // 主版本.次版本.补丁
desc.version = "2.1.3";   // 具体版本
```

### name & description

人类可读的名称和描述。

```cpp
desc.name = "钱包转账";
desc.description = "在两个玩家之间转账游戏货币";
```

### input_schema

输入参数的 JSON Schema。

```cpp
desc.input_schema = R"({
    "type": "object",
    "properties": {
        "from_player_id": {"type": "string"},
        "to_player_id": {"type": "string"},
        "amount": {"type": "integer", "minimum": 1},
        "currency": {"type": "string", "enum": ["gold", "diamond"]}
    },
    "required": ["from_player_id", "to_player_id", "amount"]
})"_json;
```

### output_schema

输出结果的 JSON Schema。

```cpp
desc.output_schema = R"({
    "type": "object",
    "properties": {
        "success": {"type": "boolean"},
        "transaction_id": {"type": "string"},
        "error": {
            "type": "object",
            "properties": {
                "code": {"type": "string"},
                "message": {"type": "string"}
            }
        }
    },
    "required": ["success"]
})"_json;
```

## FunctionHandler

函数处理器类型。

```cpp
using FunctionHandler = std::function<std::string(const std::string& context, const std::string& payload)>;
```

### 参数说明

#### context

调用上下文（JSON 字符串）：

```json
{
  "game_id": "my-game",
  "env": "production",
  "caller_id": "web-ui",
  "call_id": "uuid-1234-5678",
  "timestamp": "2024-01-01T00:00:00Z",
  "auth": {
    "user_id": "admin",
    "permissions": ["wallet:*"]
  }
}
```

#### payload

函数参数（JSON 字符串），根据 `input_schema` 定义。

### 返回值

函数执行结果（JSON 字符串），应符合 `output_schema` 定义。

```cpp
return R"({"success": true, "transaction_id": "tx-123"})";
return R"({"success": false, "error": {"code": "INSUFFICIENT_FUNDS"}})";
```

## 注册函数

### 单个注册

```cpp
client.RegisterFunction(
    FunctionDescriptor{"math.add", "0.1.0"},
    [](const std::string& ctx, const std::string& payload) {
        auto data = utils::ParseJSON(payload);
        int result = data["a"] + data["b"];
        return utils::ToJSON({{"result", result}});
    }
);
```

### 完整描述符注册

```cpp
FunctionDescriptor desc;
desc.id = "wallet.transfer";
desc.version = "0.1.0";
desc.name = "转账";
desc.description = "在玩家之间转账";
desc.input_schema = /* ... */;
desc.output_schema = /* ... */;

client.RegisterFunction(desc, TransferHandler);
```

### 批量注册

```cpp
std::vector<std::pair<FunctionDescriptor, FunctionHandler>> functions = {
    {{"math.add", "0.1.0"}, AddHandler},
    {{"math.subtract", "0.1.0"}, SubtractHandler},
    {{"math.multiply", "0.1.0"}, MultiplyHandler},
};

for (const auto& [desc, handler] : functions) {
    client.RegisterFunction(desc, handler);
}
```

## 函数模式

### 简单函数

```cpp
std::string GetServerTime(const std::string& ctx, const std::string& payload) {
    auto now = std::chrono::system_clock::now();
    auto timestamp = std::chrono::duration_cast<std::chrono::milliseconds>(
        now.time_since_epoch()
    ).count();
    return utils::ToJSON({{"timestamp", timestamp}});
}
```

### 业务逻辑函数

```cpp
std::string BanPlayer(const std::string& ctx, const std::string& payload) {
    auto data = utils::ParseJSON(payload);
    std::string player_id = data["player_id"];
    std::string reason = data["reason"];
    int duration_hours = data["duration_hours"];

    // 执行封禁逻辑
    bool success = database::BanPlayer(player_id, reason, duration_hours);

    if (success) {
        // 发送通知
        notification::Send(player_id, "你已被封禁: " + reason);

        return utils::ToJSON({
            {"success", true},
            {"player_id", player_id},
            {"banned_until", GetBanExpiry(duration_hours)}
        });
    } else {
        return utils::ToJSON({
            {"success", false},
            {"error", {{"code", "BAN_FAILED"}, {"message", "封禁操作失败"}}}
        });
    }
}
```

### 类成员函数

```cpp
class GameService {
public:
    void Register(CroupierClient& client) {
        client.RegisterFunction(
            FunctionDescriptor{"game.start", "0.1.0"},
            [this](const std::string& ctx, const std::string& payload) {
                return this->StartGame(ctx, payload);
            }
        );
    }

private:
    std::string StartGame(const std::string& ctx, const std::string& payload) {
        auto data = utils::ParseJSON(payload);
        std::string player_id = data["player_id"];

        // 创建游戏实例...
        std::string game_id = CreateGame(player_id);

        return utils::ToJSON({
            {"success", true},
            {"game_id", game_id}
        });
    }

    std::string CreateGame(const std::string& player_id);
};
```

### 异步处理函数

```cpp
std::string AsyncOperation(const std::string& ctx, const std::string& payload) {
    auto data = utils::ParseJSON(payload);
    std::string operation_id = GenerateID();

    // 提交到任务队列
    task_queue.Push({operation_id, payload});

    // 立即返回操作 ID
    return utils::ToJSON({
        {"success", true},
        {"operation_id", operation_id},
        {"status", "pending"}
    });
}
```

## 错误处理

### 标准错误响应

```cpp
namespace Errors {
    inline nlohmann::json Success(const nlohmann::json& data = nullptr) {
        nlohmann::json response = {{"success", true}};
        if (data != nullptr) response["data"] = data;
        return response;
    }

    inline nlohmann::json Error(const std::string& code, const std::string& message) {
        return {
            {"success", false},
            {"error", {{"code", code}, {"message", message}}}
        };
    }

    inline nlohmann::json ValidationError(const std::string& field) {
        return Error("VALIDATION_ERROR", "Invalid field: " + field);
    }

    inline nlohmann::json NotFound(const std::string& resource) {
        return Error("NOT_FOUND", resource + " not found");
    }
};

// 使用
std::string GetPlayer(const std::string& ctx, const std::string& payload) {
    auto data = utils::ParseJSON(payload);
    if (!data.contains("player_id")) {
        return Errors::ValidationError("player_id").dump();
    }

    std::string player_id = data["player_id"];
    auto player = database::FindPlayer(player_id);

    if (!player) {
        return Errors::NotFound("Player").dump();
    }

    return Errors::Success(player->ToJSON()).dump();
}
```

## 最佳实践

### 1. 输入验证

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
    if (payload["from_player_id"] == payload["to_player_id"])
        return false;
    return true;
}
```

### 2. 超时控制

```cpp
desc.timeout_ms = 5000;  // 5 秒超时
desc.timeout_ms = 60000; // 1 分钟超时（长操作）
```

### 3. 版本管理

```cpp
// 支持多版本同时存在
client.RegisterFunction({"wallet.transfer", "0.1.0"}, TransferV1);
client.RegisterFunction({"wallet.transfer", "2.0.0"}, TransferV2);

// V2 增加了新功能
std::string TransferV2(const std::string& ctx, const std::string& payload) {
    auto data = utils::ParseJSON(payload);
    // 支持多货币转账
    std::string currency = data.value("currency", "gold");
    // ...
}
```

### 4. 日志记录

```cpp
std::string Transfer(const std::string& ctx, const std::string& payload) {
    auto ctx_data = utils::ParseJSON(ctx);
    std::string call_id = ctx_data["call_id"];

    LOG_INFO("Call {}: Transfer started", call_id);

    auto data = utils::ParseJSON(payload);
    LOG_INFO("Call {}: {} -> {}", call_id, data["from_player_id"], data["to_player_id"]);

    // 执行转账...

    LOG_INFO("Call {}: Transfer completed", call_id);
    return utils::ToJSON({{"success", true}});
}
```
