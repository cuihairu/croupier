# 虚拟对象

虚拟对象是 Croupier C++ SDK 的核心概念，用于表示游戏中的业务实体。

## 什么是虚拟对象

虚拟对象（Virtual Object）是一种 ID 引用的实体抽象，包含：
- **元数据**: ID、名称、描述
- **操作**: 支持的方法映射
- **Schema**: 数据结构定义

## 基础用法

### 定义虚拟对象

```cpp
#include "croupier/sdk/croupier_client.h"

VirtualObjectDescriptor wallet_entity;
wallet_entity.id = "wallet.entity";
wallet_entity.version = "0.1.0";
wallet_entity.name = "玩家钱包";
wallet_entity.description = "管理玩家的游戏货币";

// 定义操作
wallet_entity.operations["read"] = "wallet.get";
wallet_entity.operations["transfer"] = "wallet.transfer";
wallet_entity.operations["deposit"] = "wallet.deposit";
```

### 注册处理器

```cpp
std::map<std::string, FunctionHandler> handlers;

// 读取钱包
handlers["wallet.get"] = [](const std::string& ctx, const std::string& payload) {
    auto data = utils::ParseJSON(payload);
    std::string player_id = data["player_id"];

    // 查询钱包数据...
    return utils::ToJSON({
        {"player_id", player_id},
        {"gold", 1000},
        {"diamond", 50}
    });
};

// 转账
handlers["wallet.transfer"] = [](const std::string& ctx, const std::string& payload) {
    auto data = utils::ParseJSON(payload);
    std::string from = data["from_player_id"];
    std::string to = data["to_player_id"];
    int amount = data["amount"];

    // 执行转账逻辑...
    return R"({"success": true})";
};

// 注册虚拟对象
client.RegisterVirtualObject(wallet_entity, handlers);
```

## Schema 定义

### 定义数据结构

```cpp
VirtualObjectDescriptor wallet_entity;
wallet_entity.id = "wallet.entity";

// 定义 Schema
wallet_entity.schema = R"({
    "type": "object",
    "properties": {
        "player_id": {"type": "string"},
        "gold": {"type": "integer", "minimum": 0},
        "diamond": {"type": "integer", "minimum": 0}
    },
    "required": ["player_id", "gold"]
})";
```

### 验证输入

```cpp
bool ValidateWallet(const std::string& payload) {
    auto data = utils::ParseJSON(payload);
    // 验证必需字段
    if (!data.contains("player_id") || !data.contains("amount")) {
        return false;
    }
    // 验证金额
    if (data["amount"] <= 0) {
        return false;
    }
    return true;
}
```

## 完整示例

### 钱包虚拟对象

```cpp
#include "croupier/sdk/croupier_client.h"
#include "croupier/sdk/utils/json_utils.h"

class WalletVirtualObject {
public:
    static void Register(CroupierClient& client) {
        VirtualObjectDescriptor desc;
        desc.id = "wallet.entity";
        desc.version = "0.1.0";
        desc.name = "玩家钱包";
        desc.description = "管理玩家游戏货币";

        // 操作映射
        desc.operations["get"] = "wallet.get";
        desc.operations["transfer"] = "wallet.transfer";
        desc.operations["deposit"] = "wallet.deposit";
        desc.operations["withdraw"] = "wallet.withdraw";

        // Schema
        desc.schema = GetSchema();

        // 处理器
        std::map<std::string, FunctionHandler> handlers = {
            {"wallet.get", GetWallet},
            {"wallet.transfer", Transfer},
            {"wallet.deposit", Deposit},
            {"wallet.withdraw", Withdraw}
        };

        client.RegisterVirtualObject(desc, handlers);
    }

private:
    static std::string GetSchema() {
        return R"({
            "type": "object",
            "properties": {
                "player_id": {"type": "string"},
                "gold": {"type": "integer", "minimum": 0},
                "diamond": {"type": "integer", "minimum": 0}
            },
            "required": ["player_id"]
        })";
    }

    static std::string GetWallet(const std::string& ctx, const std::string& payload) {
        auto data = utils::ParseJSON(payload);
        std::string player_id = data["player_id"];

        // 从数据库/缓存查询钱包...
        int gold = QueryGoldBalance(player_id);
        int diamond = QueryDiamondBalance(player_id);

        return utils::ToJSON({
            {"player_id", player_id},
            {"gold", gold},
            {"diamond", diamond}
        });
    }

    static std::string Transfer(const std::string& ctx, const std::string& payload) {
        auto data = utils::ParseJSON(payload);
        std::string from = data["from_player_id"];
        std::string to = data["to_player_id"];
        std::string currency = data.value("currency", "gold");
        int amount = data["amount"];

        // 验证余额
        if (!CheckBalance(from, currency, amount)) {
            return R"({"success": false, "error": "余额不足"})";
        }

        // 执行转账
        if (!ExecuteTransfer(from, to, currency, amount)) {
            return R"({"success": false, "error": "转账失败"})";
        }

        return utils::ToJSON({
            {"success", true},
            {"from", from},
            {"to", to},
            {"amount", amount}
        });
    }

    static std::string Deposit(const std::string& ctx, const std::string& payload) {
        // 存款实现...
        return R"({"success": true})";
    }

    static std::string Withdraw(const std::string& ctx, const std::string& payload) {
        // 取款实现...
        return R"({"success": true})";
    }

    // 辅助方法...
    static bool CheckBalance(const std::string& player_id, const std::string& currency, int amount);
    static bool ExecuteTransfer(const std::string& from, const std::string& to, const std::string& currency, int amount);
    static int QueryGoldBalance(const std::string& player_id);
    static int QueryDiamondBalance(const std::string& player_id);
};

int main() {
    ClientConfig config;
    config.game_id = "mmorpg-demo";
    config.env = "development";
    config.agent_addr = "127.0.0.1:19090";

    CroupierClient client(config);
    WalletVirtualObject::Register(client);

    client.Connect();
    client.Serve();

    return 0;
}
```

## 最佳实践

### 1. 使用类封装

```cpp
class WalletEntity {
public:
    static const std::string ID;
    static VirtualObjectDescriptor Descriptor();
    static std::map<std::string, FunctionHandler> Handlers();
};
```

### 2. 统一错误格式

```cpp
nlohmann::json ErrorResponse(const std::string& code, const std::string& message) {
    return {
        {"success", false},
        {"error", {{"code", code}, {"message", message}}}
    };
}
```

### 3. 输入验证

```cpp
bool ValidatePlayerId(const std::string& player_id) {
    return !player_id.empty() && player_id.length() <= 64;
}

bool ValidateAmount(int amount) {
    return amount > 0 && amount <= 1000000;
}
```

### 4. 日志记录

```cpp
#include <iostream>

std::string Transfer(const std::string& ctx, const std::string& payload) {
    std::cout << "[Transfer] Processing: " << payload << std::endl;

    auto data = utils::ParseJSON(payload);
    std::string from = data["from_player_id"];
    std::string to = data["to_player_id"];
    int amount = data["amount"];

    std::cout << "[Transfer] " << from << " -> " << to << ": " << amount << std::endl;

    // ... 业务逻辑

    std::cout << "[Transfer] Completed successfully" << std::endl;
    return R"({"success": true})";
}
```

## 高级特性

### 条件操作

```cpp
// 根据玩家等级限制转账
desc.conditions["transfer"] = R"({
    "min_level": 10
})";
```

### 权限控制

```cpp
// 需要特定权限才能执行
desc.permissions["withdraw"] = {"wallet:withdraw"};
```

### 生命周期钩子

```cpp
desc.on_create = "wallet.init";      // 创建时调用
desc.on_delete = "wallet.cleanup";   // 删除时调用
desc.on_query = "wallet.validate";   // 查询前调用
```
