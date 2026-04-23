# 综合示例

本示例展示一个完整的游戏服务端，包含多个虚拟对象和功能模块。

## 架构概览

```
Game Server
├── Player Entity    (玩家管理)
├── Wallet Entity    (钱包系统)
├── Inventory Entity (背包系统)
└── Guild Entity     (公会系统)
```

## 完整代码

```cpp
#include "croupier/sdk/croupier_client.h"
#include "croupier/sdk/utils/json_utils.h"
#include <iostream>
#include <map>
#include <mutex>
#include <memory>

using namespace croupier::sdk;

// ==================== 数据存储 ====================

class Database {
public:
    // 玩家数据
    bool CreatePlayer(const std::string& player_id, const std::string& name) {
        std::lock_guard<std::mutex> lock(mutex_);
        if (players_.find(player_id) != players_.end()) return false;
        players_[player_id] = {{"name", name}, {"level", 1}, {"exp", 0}};
        return true;
    }

    std::optional<nlohmann::json> GetPlayer(const std::string& player_id) {
        std::lock_guard<std::mutex> lock(mutex_);
        auto it = players_.find(player_id);
        if (it == players_.end()) return std::nullopt;
        return nlohmann::json{{"player_id", player_id}, {"data", it->second}};
    }

    // 钱包数据
    bool CreateWallet(const std::string& player_id) {
        std::lock_guard<std::mutex> lock(mutex_);
        if (wallets_.find(player_id) != wallets_.end()) return false;
        wallets_[player_id] = {{"gold", 1000}, {"diamond", 50}};
        return true;
    }

    std::optional<nlohmann::json> GetWallet(const std::string& player_id) {
        std::lock_guard<std::mutex> lock(mutex_);
        auto it = wallets_.find(player_id);
        if (it == wallets_.end()) return std::nullopt;
        return nlohmann::json{{"player_id", player_id}, {"balance", it->second}};
    }

    bool Transfer(const std::string& from, const std::string& to,
                  const std::string& currency, int amount) {
        std::lock_guard<std::mutex> lock(mutex_);
        auto from_it = wallets_.find(from);
        auto to_it = wallets_.find(to);
        if (from_it == wallets_.end() || to_it == wallets_.end()) return false;
        if (from_it->second[currency].get<int>() < amount) return false;
        from_it->second[currency] = from_it->second[currency].get<int>() - amount;
        to_it->second[currency] = to_it->second[currency].get<int>() + amount;
        return true;
    }

private:
    std::map<std::string, nlohmann::json> players_;
    std::map<std::string, nlohmann::json> wallets_;
    std::mutex mutex_;
};

Database db;

// ==================== 响应工具 ====================

namespace Responses {
    inline std::string Success(const nlohmann::json& data = nullptr) {
        nlohmann::json resp = {{"success", true}};
        if (data != nullptr) resp["data"] = data;
        return resp.dump();
    }

    inline std::string Error(const std::string& code, const std::string& message) {
        return nlohmann::json{
            {"success", false},
            {"error", {{"code", code}, {"message", message}}}
        }.dump();
    }
}

// ==================== 玩家实体 ====================

std::string PlayerCreate(const std::string& ctx, const std::string& payload) {
    auto data = utils::ParseJSON(payload);
    std::string player_id = data["player_id"];
    std::string name = data["name"];

    if (db.CreatePlayer(player_id, name)) {
        return Responses::Success(nlohmann::json{{"player_id", player_id}});
    }
    return Responses::Error("PLAYER_EXISTS", "玩家已存在");
}

std::string PlayerGet(const std::string& ctx, const std::string& payload) {
    auto data = utils::ParseJSON(payload);
    std::string player_id = data["player_id"];

    auto player = db.GetPlayer(player_id);
    if (player) {
        return Responses::Success(*player);
    }
    return Responses::Error("PLAYER_NOT_FOUND", "玩家不存在");
}

void RegisterPlayerEntity(CroupierClient& client) {
    VirtualObjectDescriptor player_entity;
    player_entity.id = "player.entity";
    player_entity.version = "0.1.0";
    player_entity.name = "玩家";
    player_entity.description = "玩家管理";

    player_entity.operations["create"] = "player.create";
    player_entity.operations["get"] = "player.get";

    std::map<std::string, FunctionHandler> handlers = {
        {"player.create", PlayerCreate},
        {"player.get", PlayerGet}
    };

    client.RegisterVirtualObject(player_entity, handlers);
}

// ==================== 钱包实体 ====================

std::string WalletCreate(const std::string& ctx, const std::string& payload) {
    auto data = utils::ParseJSON(payload);
    std::string player_id = data["player_id"];

    if (db.CreateWallet(player_id)) {
        return Responses::Success(nlohmann::json{{"player_id", player_id}});
    }
    return Responses::Error("WALLET_EXISTS", "钱包已存在");
}

std::string WalletGet(const std::string& ctx, const std::string& payload) {
    auto data = utils::ParseJSON(payload);
    std::string player_id = data["player_id"];

    auto wallet = db.GetWallet(player_id);
    if (wallet) {
        return Responses::Success(*wallet);
    }
    return Responses::Error("WALLET_NOT_FOUND", "钱包不存在");
}

std::string WalletTransfer(const std::string& ctx, const std::string& payload) {
    auto data = utils::ParseJSON(payload);
    std::string from = data["from_player_id"];
    std::string to = data["to_player_id"];
    std::string currency = data.value("currency", "gold");
    int amount = data["amount"];

    if (from == to) {
        return Responses::Error("INVALID_TRANSFER", "不能转账给自己");
    }
    if (amount <= 0) {
        return Responses::Error("INVALID_AMOUNT", "金额必须大于0");
    }

    if (db.Transfer(from, to, currency, amount)) {
        return Responses::Success(nlohmann::json{
            {"from", from},
            {"to", to},
            {"currency", currency},
            {"amount", amount}
        });
    }
    return Responses::Error("TRANSFER_FAILED", "转账失败");
}

void RegisterWalletEntity(CroupierClient& client) {
    VirtualObjectDescriptor wallet_entity;
    wallet_entity.id = "wallet.entity";
    wallet_entity.version = "0.1.0";
    wallet_entity.name = "钱包";
    wallet_entity.description = "游戏货币管理";

    wallet_entity.operations["create"] = "wallet.create";
    wallet_entity.operations["get"] = "wallet.get";
    wallet_entity.operations["transfer"] = "wallet.transfer";

    wallet_entity.schema = R"({
        "type": "object",
        "properties": {
            "player_id": {"type": "string"},
            "balance": {
                "type": "object",
                "properties": {
                    "gold": {"type": "integer", "minimum": 0},
                    "diamond": {"type": "integer", "minimum": 0}
                }
            }
        }
    })"_json;

    std::map<std::string, FunctionHandler> handlers = {
        {"wallet.create", WalletCreate},
        {"wallet.get", WalletGet},
        {"wallet.transfer", WalletTransfer}
    };

    client.RegisterVirtualObject(wallet_entity, handlers);
}

// ==================== 工具函数 ====================

std::string GetServerInfo(const std::string& ctx, const std::string& payload) {
    return Responses::Success(nlohmann::json{
        {"version", "0.1.0"},
        {"name", "Croupier Game Server"},
        {"uptime", std::chrono::duration_cast<std::chrono::seconds>(
            std::chrono::steady_clock::now() - start_time).count()}
    });
}

// ==================== 主程序 ====================

std::chrono::steady_clock::time_point start_time;

int main() {
    start_time = std::chrono::steady_clock::now();

    ClientConfig config;
    config.game_id = "demo-game";
    config.env = "development";
    config.agent_addr = "127.0.0.1:19090";
    config.insecure = true;

    CroupierClient client(config);

    // 注册实体
    RegisterPlayerEntity(client);
    RegisterWalletEntity(client);

    // 注册工具函数
    client.RegisterFunction(
        FunctionDescriptor{"server.info", "0.1.0", "Server Info", "获取服务器信息"},
        GetServerInfo
    );

    std::cout << "========================================" << std::endl;
    std::cout << "Croupier Game Server" << std::endl;
    std::cout << "Version: 0.1.0" << std::endl;
    std::cout << "========================================" << std::endl;
    std::cout << "Registered entities:" << std::endl;
    std::cout << "  - player.entity" << std::endl;
    std::cout << "  - wallet.entity" << std::endl;
    std::cout << "========================================" << std::endl;

    if (client.Connect()) {
        std::cout << "Connected to agent! Serving..." << std::endl;
        client.Serve();
    }

    return 0;
}
```

## 编译

### CMakeLists.txt

```cmake
cmake_minimum_required(VERSION 3.20)
project(comprehensive_demo)

set(CMAKE_CXX_STANDARD 17)

find_package(croupier-sdk REQUIRED)

add_executable(comprehensive_demo main.cpp)
target_link_libraries(comprehensive_demo PRIVATE croupier-sdk::croupier-sdk)
```

## 运行测试

### 创建玩家

```bash
curl -X POST http://localhost:18780/api/v1/functions/player.create/invoke \
  -H "Content-Type: application/json" \
  -H "X-Game-ID: demo-game" \
  -d '{"player_id": "player123", "name": "Alice"}'
```

### 创建钱包

```bash
curl -X POST http://localhost:18780/api/v1/functions/wallet.create/invoke \
  -H "Content-Type: application/json" \
  -H "X-Game-ID: demo-game" \
  -d '{"player_id": "player123"}'
```

### 查询玩家

```bash
curl -X POST http://localhost:18780/api/v1/functions/player.get/invoke \
  -H "Content-Type: application/json" \
  -H "X-Game-ID: demo-game" \
  -d '{"player_id": "player123"}'
```

### 查询钱包

```bash
curl -X POST http://localhost:18780/api/v1/functions/wallet.get/invoke \
  -H "Content-Type: application/json" \
  -H "X-Game-ID: demo-game" \
  -d '{"player_id": "player123"}'
```

### 转账

```bash
# 先创建第二个玩家和钱包
curl -X POST http://localhost:18780/api/v1/functions/player.create/invoke \
  -H "Content-Type: application/json" \
  -H "X-Game-ID: demo-game" \
  -d '{"player_id": "player456", "name": "Bob"}'

curl -X POST http://localhost:18780/api/v1/functions/wallet.create/invoke \
  -H "Content-Type: application/json" \
  -H "X-Game-ID: demo-game" \
  -d '{"player_id": "player456"}'

# 转账
curl -X POST http://localhost:18780/api/v1/functions/wallet.transfer/invoke \
  -H "Content-Type: application/json" \
  -H "X-Game-ID: demo-game" \
  -d '{
    "from_player_id": "player123",
    "to_player_id": "player456",
    "currency": "gold",
    "amount": 100
  }'
```

### 获取服务器信息

```bash
curl -X POST http://localhost:18780/api/v1/functions/server.info/invoke \
  -H "Content-Type: application/json" \
  -H "X-Game-ID: demo-game" \
  -d '{}'
```
