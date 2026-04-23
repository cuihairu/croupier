# 虚拟对象 API 详解

本文档详细说明虚拟对象注册和使用的 API。

## VirtualObjectDescriptor

虚拟对象描述符结构定义。

```cpp
struct VirtualObjectDescriptor {
    std::string id;                    // 必需: 对象唯一标识符
    std::string version;               // 必需: 版本号
    std::string name;                  // 可选: 显示名称
    std::string description;           // 可选: 描述信息
    std::map<std::string, std::string> operations;  // 必需: 操作映射
    nlohmann::json schema;             // 可选: 数据 Schema
    nlohmann::json metadata;           // 可选: 元数据
};
```

## 字段说明

### id

虚拟对象唯一标识符，建议使用 `.entity` 后缀。

```cpp
desc.id = "wallet.entity";
desc.id = "player.entity";
desc.id = "inventory.entity";
```

### operations

操作名到函数 ID 的映射。

```cpp
desc.operations = {
    {"read", "wallet.get"},              // 读取操作
    {"transfer", "wallet.transfer"},     // 转账操作
    {"deposit", "wallet.deposit"},       // 存款操作
    {"withdraw", "wallet.withdraw"}      // 取款操作
};
```

### schema

数据结构的 JSON Schema。

```cpp
desc.schema = R"({
    "type": "object",
    "properties": {
        "player_id": {"type": "string"},
        "gold": {"type": "integer", "minimum": 0},
        "diamond": {"type": "integer", "minimum": 0}
    },
    "required": ["player_id"]
})"_json;
```

## 注册虚拟对象

### 基础注册

```cpp
// 定义虚拟对象
VirtualObjectDescriptor wallet;
wallet.id = "wallet.entity";
wallet.version = "0.1.0";
wallet.name = "玩家钱包";
wallet.description = "管理玩家游戏货币";

// 定义操作
wallet.operations["get"] = "wallet.get";
wallet.operations["transfer"] = "wallet.transfer";

// 定义处理器
std::map<std::string, FunctionHandler> handlers = {
    {"wallet.get", GetWalletHandler},
    {"wallet.transfer", TransferHandler}
};

// 注册
client.RegisterVirtualObject(wallet, handlers);
```

### 完整示例

```cpp
class WalletEntity {
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
        desc.operations["history"] = "wallet.history";

        // 数据 Schema
        desc.schema = GetSchema();

        // 处理器
        std::map<std::string, FunctionHandler> handlers = {
            {"wallet.get", GetWallet},
            {"wallet.transfer", Transfer},
            {"wallet.deposit", Deposit},
            {"wallet.withdraw", Withdraw},
            {"wallet.history", GetHistory}
        };

        client.RegisterVirtualObject(desc, handlers);
    }

private:
    static nlohmann::json GetSchema() {
        return R"({
            "type": "object",
            "properties": {
                "player_id": {"type": "string"},
                "gold": {"type": "integer", "minimum": 0},
                "diamond": {"type": "integer", "minimum": 0},
                "created_at": {"type": "string", "format": "date-time"},
                "updated_at": {"type": "string", "format": "date-time"}
            },
            "required": ["player_id"]
        })"_json;
    }

    static std::string GetWallet(const std::string& ctx, const std::string& payload) {
        auto data = utils::ParseJSON(payload);
        std::string player_id = data["player_id"];

        auto wallet = database::GetWallet(player_id);
        if (!wallet) {
            return R"({"success": false, "error": {"code": "WALLET_NOT_FOUND"}})";
        }

        return utils::ToJSON({
            {"success", true},
            {"wallet", {
                {"player_id", wallet->player_id},
                {"gold", wallet->gold},
                {"diamond", wallet->diamond}
            }}
        });
    }

    static std::string Transfer(const std::string& ctx, const std::string& payload) {
        auto data = utils::ParseJSON(payload);
        std::string from = data["from_player_id"];
        std::string to = data["to_player_id"];
        std::string currency = data.value("currency", "gold");
        int amount = data["amount"];

        // 验证
        if (!ValidateTransfer(from, to, currency, amount)) {
            return R"({"success": false, "error": {"code": "INVALID_TRANSFER"}})";
        }

        // 检查余额
        if (!CheckBalance(from, currency, amount)) {
            return R"({"success": false, "error": {"code": "INSUFFICIENT_BALANCE"}})";
        }

        // 执行转账
        if (!ExecuteTransfer(from, to, currency, amount)) {
            return R"({"success": false, "error": {"code": "TRANSFER_FAILED"}})";
        }

        return utils::ToJSON({
            {"success", true},
            {"transaction_id", GenerateTransactionId()},
            {"from", from},
            {"to", to},
            {"currency", currency},
            {"amount", amount}
        });
    }

    static std::string Deposit(const std::string& ctx, const std::string& payload);
    static std::string Withdraw(const std::string& ctx, const std::string& payload);
    static std::string GetHistory(const std::string& ctx, const std::string& payload);
};
```

## 操作处理器

### CRUD 操作模式

```cpp
// Create
desc.operations["create"] = "entity.create";
handlers["entity.create"] = CreateEntity;

// Read
desc.operations["get"] = "entity.get";
handlers["entity.get"] = ReadEntity;

// Update
desc.operations["update"] = "entity.update";
handlers["entity.update"] = UpdateEntity;

// Delete
desc.operations["delete"] = "entity.delete";
handlers["entity.delete"] = DeleteEntity;

// List
desc.operations["list"] = "entity.list";
handlers["entity.list"] = ListEntities;
```

### 自定义操作

```cpp
desc.operations["custom_action"] = "entity.custom";
handlers["entity.custom"] = [](const std::string& ctx, const std::string& payload) {
    // 自定义操作逻辑
    return R"({"success": true})";
};
```

## Schema 验证

### 定义 Schema

```cpp
desc.schema = R"({
    "$schema": "http://json-schema.org/draft-07/schema#",
    "type": "object",
    "properties": {
        "player_id": {
            "type": "string",
            "minLength": 1,
            "maxLength": 64,
            "pattern": "^[a-zA-Z0-9_-]+$"
        },
        "balance": {
            "type": "object",
            "properties": {
                "gold": {"type": "integer", "minimum": 0},
                "diamond": {"type": "integer", "minimum": 0}
            },
            "required": ["gold"]
        },
        "settings": {
            "type": "object",
            "properties": {
                "notifications": {"type": "boolean"},
                "auto_transfer": {"type": "boolean"}
            }
        }
    },
    "required": ["player_id", "balance"]
})"_json;
```

### 验证数据

```cpp
#include "croupier/sdk/validation/schema_validator.h"

bool ValidateWallet(const nlohmann::json& data, const nlohmann::json& schema) {
    SchemaValidator validator;
    auto errors = validator.Validate(data, schema);

    if (!errors.empty()) {
        for (const auto& error : errors) {
            std::cerr << "Validation error: " << error << std::endl;
        }
        return false;
    }

    return true;
}
```

## 元数据

### 添加元数据

```cpp
desc.metadata = {
    {"category", "economy"},
    {"tags", {"wallet", "currency", "trade"}},
    {"ui", {
        {"icon", "wallet"},
        {"color", "#FFD700"}
    }},
    {"permissions", {
        {"read", "wallet:read"},
        {"write", "wallet:write"}
    }}
};
```

### 使用元数据

```cpp
std::string category = desc.metadata["category"];
std::vector<std::string> tags = desc.metadata["tags"];
```

## 关联关系

### 父子关系

```cpp
desc.metadata["parent"] = "player.entity";
desc.metadata["relationship"] = "child";
```

### 引用关系

```cpp
desc.metadata["references"] = {
    {"currency", "currency.entity"},
    {"player", "player.entity"}
};
```

## 最佳实践

### 1. 使用类封装

```cpp
class WalletEntity {
public:
    static const std::string ID;
    static VirtualObjectDescriptor Descriptor();
    static std::map<std::string, FunctionHandler> Handlers();

private:
    static std::string Get(const std::string& ctx, const std::string& payload);
    static std::string Transfer(const std::string& ctx, const std::string& payload);
};

// 使用
WalletEntity::Register(client);
```

### 2. 操作命名规范

```cpp
// CRUD 操作
"entity.get"       // 获取单个实体
"entity.list"      // 列出实体
"entity.create"    // 创建实体
"entity.update"    // 更新实体
"entity.delete"    // 删除实体

// 业务操作
"wallet.transfer"  // 转账
"wallet.deposit"   // 存款
"wallet.withdraw"  // 取款
```

### 3. 统一响应格式

```cpp
namespace Responses {
    nlohmann::json Entity(const nlohmann::json& entity) {
        return {
            {"success", true},
            {"entity", entity}
        };
    }

    nlohmann::json EntityList(const std::vector<nlohmann::json>& entities,
                              const nlohmann::json& paging) {
        return {
            {"success", true},
            {"entities", entities},
            {"paging", paging}
        };
    }
}
```

### 4. 权限检查

```cpp
std::string GetWallet(const std::string& ctx, const std::string& payload) {
    auto ctx_data = utils::ParseJSON(ctx);
    std::string caller_id = ctx_data["caller_id"];
    std::string player_id = utils::ParseJSON(payload)["player_id"];

    // 权限检查
    if (!HasPermission(caller_id, "wallet:read", player_id)) {
        return R"({"success": false, "error": {"code": "PERMISSION_DENIED"}})";
    }

    // 继续处理...
}
```
