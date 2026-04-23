# Croupier C++ SDK 约定规范

本文档详细说明使用 Croupier C++ SDK 时需要遵守的约定和规范。

## 目录

- [命名约定](#命名约定)
- [函数注册约定](#函数注册约定)
- [虚拟对象设计约定](#虚拟对象设计约定)
- [错误处理约定](#错误处理约定)
- [版本管理约定](#版本管理约定)
- [安全约定](#安全约定)
- [避让规则](#避让规则)

---

## 命名约定

### 函数 ID 命名

函数 ID 采用 `entity.operation` 格式，描述函数所属实体和执行的操作。

#### 格式规则

```
[namespace.]entity.operation
```

| 部分 | 说明 | 示例 |
|------|------|------|
| `namespace` (可选) | 命名空间，用于模块分组 | `game`, `inventory`, `chat` |
| `entity` | 实体名称，小写 | `player`, `item`, `guild` |
| `operation` | 操作名称，小写动词 | `get`, `create`, `update`, `delete`, `ban` |

#### 命名示例

```cpp
// ✅ 正确的函数 ID
"player.get"              // 获取玩家信息
"player.ban"              // 封禁玩家
"player.update_profile"   // 更新玩家资料
"item.create"             // 创建道具
"item.delete"             // 删除道具
"wallet.transfer"         // 钱包转账
"game.player.ban"         // 带命名空间
"inventory.item.add"      // 带命名空间

// ❌ 错误的函数 ID
"PlayerGet"               // 不要使用驼峰命名
"player-get"              // 不要使用连字符
"player_get"              // 不要使用下划线
"get_player"              // 实体应该在前
"player"                  // 缺少操作
""                        // 不能为空
```

### 实体命名

实体代表业务领域的对象，使用**小写单数名词**。

```cpp
// ✅ 推荐的实体名称
"player"      // 玩家
"item"        // 道具
"guild"       // 公会
"wallet"      // 钱包
"match"       // 比赛
"chat"        // 聊天

// ❌ 避免使用
"players"     // 不要使用复数
"Player"      // 不要使用大写
"player_data" // 不要使用下划线
```

### 操作命名

操作使用**小写动词**，表示对实体执行的动作。

#### CRUD 标准操作

| 操作 | 说明 | 适用场景 |
|------|------|----------|
| `create` | 创建新实体 | 新建玩家、创建道具 |
| `get` | 获取实体信息 | 查询玩家、查询道具 |
| `update` | 更新实体信息 | 修改玩家属性、更新道具状态 |
| `delete` | 删除实体 | 删除玩家、删除道具 |
| `list` | 列出实体集合 | 查询玩家列表、查询道具列表 |

#### 业务操作

| 操作 | 说明 | 示例 |
|------|------|------|
| `ban` | 封禁/禁用 | `player.ban` |
| `unban` | 解封/启用 | `player.unban` |
| `transfer` | 转移 | `wallet.transfer`, `item.transfer` |
| `add` | 添加 | `inventory.item.add`, `guild.member.add` |
| `remove` | 移除 | `inventory.item.remove`, `guild.member.remove` |
| `start` | 启动 | `match.start` |
| `end` | 结束 | `match.end` |
| `join` | 加入 | `match.join` |
| `leave` | 离开 | `match.leave` |

---

## 函数注册约定

### 注册时机

所有函数必须在调用 `Connect()` **之前**完成注册。

```cpp
CroupierClient client(config);

// ✅ 正确：先注册，后连接
client.RegisterFunction(descriptor1, handler1);
client.RegisterFunction(descriptor2, handler2);
client.Connect();  // 连接时会将所有注册的函数上传到 Agent

// ❌ 错误：连接后不能再注册
client.Connect();
client.RegisterFunction(descriptor3, handler3);  // 无效，会失败
```

### 函数唯一性

同一个服务实例内，函数 ID 必须唯一。

```cpp
// ❌ 错误：重复注册相同 ID
FunctionDescriptor desc1;
desc1.id = "player.get";
desc1.version = "1.0.0";

FunctionDescriptor desc2;
desc2.id = "player.get";  // 与 desc1 相同
desc2.version = "1.0.0";

client.RegisterFunction(desc1, handler1);
client.RegisterFunction(desc2, handler2);  // 会覆盖或报错
```

### 函数描述符必填字段

```cpp
FunctionDescriptor desc;

// ✅ 必填字段
desc.id = "player.get";      // 函数 ID，不能为空
desc.version = "1.0.0";       // 版本号，不能为空

// ⭐ 推荐字段
desc.category = "player";     // 业务分类
desc.risk = "low";            // 风险等级: low, medium, high
desc.entity = "player";       // 关联实体
desc.operation = "read";      // 操作类型: create, read, update, delete
desc.enabled = true;         // 是否启用
```

### 风险等级 (Risk)

风险等级用于标识函数操作的影响范围和重要性。

| 等级 | 说明 | 示例 |
|------|------|------|
| `low` | 只读操作，无副作用 | `player.get`, `item.list` |
| `medium` | 有副作用但可逆 | `player.update`, `item.add` |
| `high` | 有重大影响或不可逆 | `player.delete`, `player.ban`, `wallet.transfer` |

```cpp
FunctionDescriptor desc;
desc.id = "player.ban";
desc.risk = "high";  // 封禁是高风险操作
```

---

## 虚拟对象设计约定

### 虚拟对象结构

虚拟对象将相关的 CRUD 操作组合在一起，形成完整的实体管理单元。

```cpp
VirtualObjectDescriptor player;

// 基础信息
player.id = "player";                    // 对象 ID
player.version = "1.0.0";               // 版本号
player.name = "游戏玩家";               // 显示名称
player.description = "管理玩家信息";     // 描述

// Schema 定义（可选）
player.schema["type"] = "object";
player.schema["properties"] = R"({
    "id": {"type": "string"},
    "name": {"type": "string"},
    "level": {"type": "integer"}
})";

// 操作映射
player.operations["create"] = "player.create";
player.operations["read"] = "player.get";
player.operations["update"] = "player.update";
player.operations["delete"] = "player.delete";
player.operations["ban"] = "player.ban";
player.operations["unban"] = "player.unban";

// 关系定义
player.relationships["inventory"] = {"one-to-many", "inventory", "player_id"};
player.relationships["guild"] = {"many-to-one", "guild", "guild_id"};
```

### 标准操作映射

虚拟对象应尽可能支持标准 CRUD 操作：

| 操作 | 函数 ID | 说明 |
|------|---------|------|
| `create` | `{entity}.create` | 创建新实体 |
| `read` | `{entity}.get` | 读取实体信息 |
| `update` | `{entity}.update` | 更新实体信息 |
| `delete` | `{entity}.delete` | 删除实体 |

### 关系类型

| 关系类型 | 说明 | 示例 |
|----------|------|------|
| `one-to-one` | 一对一 | 玩家与玩家资料 |
| `one-to-many` | 一对多 | 玩家与道具 |
| `many-to-one` | 多对一 | 玩家与公会 |
| `many-to-many` | 多对多 | 玩家与比赛 |

```cpp
// 关系定义格式
player.relationships["items"] = {"one-to-many", "item", "owner_id"};
//                             关系类型        目标实体  外键
```

---

## 错误处理约定

### 函数处理器错误处理

函数处理器应该捕获所有异常并返回标准错误响应。

```cpp
std::string SafeHandler(const std::string& context, const std::string& payload) {
    try {
        // 1. 解析输入
        auto data = nlohmann::json::parse(payload);

        // 2. 验证参数
        if (!data.contains("player_id") || !data["player_id"].is_string()) {
            return R"({"status":"error","code":"INVALID_PARAM","message":"player_id is required"})";
        }

        // 3. 执行业务逻辑
        std::string result = ProcessPlayer(data["player_id"]);

        // 4. 返回成功响应
        return R"({"status":"success","data":)" + result + R"(})";

    } catch (const nlohmann::json::exception& e) {
        // JSON 解析错误
        return R"({"status":"error","code":"INVALID_JSON","message":")" +
               std::string(e.what()) + R"("})";

    } catch (const std::exception& e) {
        // 其他错误
        return R"({"status":"error","code":"INTERNAL_ERROR","message":")" +
               std::string(e.what()) + R"("})";
    }
}
```

### 标准错误响应格式

所有错误响应应遵循统一格式：

```json
{
  "status": "error",
  "code": "ERROR_CODE",
  "message": "Human readable error message",
  "details": {}
}
```

### 常见错误码

| 错误码 | 说明 | HTTP 等价 |
|--------|------|----------|
| `INVALID_PARAM` | 参数错误 | 400 |
| `INVALID_JSON` | JSON 格式错误 | 400 |
| `UNAUTHORIZED` | 未授权 | 401 |
| `FORBIDDEN` | 无权限 | 403 |
| `NOT_FOUND` | 资源不存在 | 404 |
| `ALREADY_EXISTS` | 资源已存在 | 409 |
| `INTERNAL_ERROR` | 内部错误 | 500 |
| `SERVICE_UNAVAILABLE` | 服务不可用 | 503 |

---

## 版本管理约定

### 语义化版本

SDK 遵循语义化版本规范 (Semver)：`MAJOR.MINOR.PATCH`

| 部分 | 说明 | 示例 |
|------|------|------|
| `MAJOR` | 主版本号，不兼容的 API 变更 | 1.0.0 → 2.0.0 |
| `MINOR` | 次版本号，向后兼容的功能新增 | 1.0.0 → 1.1.0 |
| `PATCH` | 修订号，向后兼容的问题修正 | 1.0.0 → 1.0.1 |

### 版本兼容性

```cpp
// ✅ 兼容变更：增加 PATCH
// 1.0.0 → 1.0.1
desc.version = "1.0.1";  // 修复 bug

// ✅ 兼容变更：增加 MINOR
// 1.0.0 → 1.1.0
desc.version = "1.1.0";  // 新增可选参数

// ❌ 不兼容变更：增加 MAJOR
// 1.0.0 → 2.0.0
desc.version = "2.0.0";  // 修改参数结构
```

---

## 安全约定

### 输入验证

所有函数处理器必须验证输入参数：

```cpp
std::string SecureHandler(const std::string& context, const std::string& payload) {
    // 1. 检查 payload 是否为空
    if (payload.empty()) {
        return R"({"status":"error","code":"EMPTY_PAYLOAD"})";
    }

    // 2. 验证 JSON 格式
    nlohmann::json data;
    try {
        data = nlohmann::json::parse(payload);
    } catch (...) {
        return R"({"status":"error","code":"INVALID_JSON"})";
    }

    // 3. 验证必需字段
    if (!data.contains("player_id")) {
        return R"({"status":"error","code":"MISSING_PLAYER_ID"})";
    }

    // 4. 类型检查
    if (!data["player_id"].is_string()) {
        return R"({"status":"error","code":"INVALID_PLAYER_ID_TYPE"})";
    }

    // 5. 值范围检查
    std::string player_id = data["player_id"];
    if (player_id.empty() || player_id.length() > 64) {
        return R"({"status":"error","code":"INVALID_PLAYER_ID_VALUE"})";
    }

    // 6. 业务逻辑
    return ProcessRequest(data);
}
```

### TLS 配置

生产环境必须启用 TLS：

```cpp
ClientConfig config;
config.agent_addr = "agent.croupier.io:443";
config.insecure = false;  // 生产环境必须为 false
config.cert_file = "/etc/tls/client.crt";
config.key_file = "/etc/tls/client.key";
config.ca_file = "/etc/tls/ca.crt";
config.server_name = "agent.croupier.io";
```

---

## 避让规则

### 函数注册避让

当多个服务实例注册相同函数时，Agent 会根据以下规则路由请求：

1. **版本优先级**：优先路由到高版本的函数
   ```
   player.get v2.0.0 > player.get v1.1.0 > player.get v1.0.0
   ```

2. **负载均衡**：同版本函数之间进行负载分配

3. **健康检查**：不健康的实例不会被路由

```cpp
// 设置合适的版本号
desc.version = "2.0.0";  // 新版本会优先处理请求
```

### 服务优先级

通过 `risk` 字段影响路由优先级：

```cpp
// 高风险函数会被更谨慎地路由
high_risk_desc.risk = "high";

// 低风险函数可以更自由地负载均衡
low_risk_desc.risk = "low";
```

### 优雅降级

当服务不可用时，返回降级响应而非错误：

```cpp
std::string GracefulDegradeHandler(const std::string& context, const std::string& payload) {
    try {
        return ProcessRequest(payload);
    } catch (const std::exception& e) {
        // 返回降级响应
        return R"({"status":"degraded","message":"Service temporarily unavailable","cached":true})";
    }
}
```

---

## 完整示例

### 符合约定的完整实现

```cpp
#include "croupier/sdk/croupier_client.h"
#include <nlohmann/json.hpp>
#include <iostream>

using namespace croupier::sdk;
using json = nlohmann::json;

// 玩家管理处理器 - 符合所有约定
class PlayerHandlers {
public:
    // 获取玩家信息 (low risk, read operation)
    static std::string Get(const std::string& context, const std::string& payload) {
        try {
            json data = json::parse(payload);

            // 参数验证
            if (!data.contains("player_id")) {
                return Error("MISSING_PARAM", "player_id is required");
            }

            std::string player_id = data["player_id"];

            // 业务逻辑
            json result = {
                {"status", "success"},
                {"data", {
                    {"id", player_id},
                    {"name", "Player One"},
                    {"level", 50}
                }}
            };

            return result.dump();

        } catch (const json::exception& e) {
            return Error("INVALID_JSON", e.what());
        } catch (const std::exception& e) {
            return Error("INTERNAL_ERROR", e.what());
        }
    }

    // 更新玩家信息 (medium risk, update operation)
    static std::string Update(const std::string& context, const std::string& payload) {
        try {
            json data = json::parse(payload);

            if (!data.contains("player_id")) {
                return Error("MISSING_PARAM", "player_id is required");
            }

            // 业务逻辑
            json result = {
                {"status", "success"},
                {"updated_fields", {"level", "exp"}}
            };

            return result.dump();

        } catch (const json::exception& e) {
            return Error("INVALID_JSON", e.what());
        } catch (const std::exception& e) {
            return Error("INTERNAL_ERROR", e.what());
        }
    }

    // 封禁玩家 (high risk, sensitive operation)
    static std::string Ban(const std::string& context, const std::string& payload) {
        try {
            json data = json::parse(payload);

            // 验证操作者权限（从 context 获取）
            // 高风险操作需要额外验证

            json result = {
                {"status", "success"},
                {"action", "ban"},
                {"player_id", data["player_id"]}
            };

            return result.dump();

        } catch (const json::exception& e) {
            return Error("INVALID_JSON", e.what());
        } catch (const std::exception& e) {
            return Error("INTERNAL_ERROR", e.what());
        }
    }

private:
    static std::string Error(const std::string& code, const std::string& message) {
        json err = {
            {"status", "error"},
            {"code", code},
            {"message", message}
        };
        return err.dump();
    }
};

int main() {
    // 配置
    ClientConfig config;
    config.game_id = "my-game";
    config.env = "production";
    config.service_id = "player-service";
    config.agent_addr = "agent.croupier.io:443";
    config.insecure = false;  // 生产环境启用 TLS

    CroupierClient client(config);

    // 注册函数 - 遵循命名约定
    FunctionDescriptor getDesc;
    getDesc.id = "player.get";
    getDesc.version = "1.0.0";
    getDesc.category = "player";
    getDesc.risk = "low";
    getDesc.entity = "player";
    getDesc.operation = "read";

    FunctionDescriptor updateDesc;
    updateDesc.id = "player.update";
    updateDesc.version = "1.0.0";
    updateDesc.category = "player";
    updateDesc.risk = "medium";
    updateDesc.entity = "player";
    updateDesc.operation = "update";

    FunctionDescriptor banDesc;
    banDesc.id = "player.ban";
    banDesc.version = "1.0.0";
    banDesc.category = "player";
    banDesc.risk = "high";
    banDesc.entity = "player";
    banDesc.operation = "update";

    // 在连接前注册所有函数
    client.RegisterFunction(getDesc, PlayerHandlers::Get);
    client.RegisterFunction(updateDesc, PlayerHandlers::Update);
    client.RegisterFunction(banDesc, PlayerHandlers::Ban);

    // 连接并启动服务
    if (client.Connect()) {
        std::cout << "服务已启动\n";
        client.Serve();
    }

    return 0;
}
```
