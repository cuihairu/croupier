# 虚拟对象示例

本示例展示如何创建和注册一个完整的钱包虚拟对象。

## 完整代码

```cpp
#include "croupier/sdk/croupier_client.h"
#include "croupier/sdk/utils/json_utils.h"
#include <iostream>
#include <map>
#include <mutex>

using namespace croupier::sdk;

// 简单的内存存储
class WalletStorage {
public:
    bool CreateWallet(const std::string& player_id) {
        std::lock_guard<std::mutex> lock(mutex_);
        if (wallets_.find(player_id) != wallets_.end()) {
            return false;
        }
        wallets_[player_id] = {{"gold", 1000}, {"diamond", 50}};
        return true;
    }

    std::optional<nlohmann::json> GetWallet(const std::string& player_id) {
        std::lock_guard<std::mutex> lock(mutex_);
        auto it = wallets_.find(player_id);
        if (it == wallets_.end()) {
            return std::nullopt;
        }
        return nlohmann::json{
            {"player_id", player_id},
            {"balance", it->second}
        };
    }

    bool Transfer(const std::string& from, const std::string& to,
                  const std::string& currency, int amount) {
        std::lock_guard<std::mutex> lock(mutex_);

        auto from_it = wallets_.find(from);
        auto to_it = wallets_.find(to);

        if (from_it == wallets_.end() || to_it == wallets_.end()) {
            return false;
        }

        if (from_it->second[currency].get<int>() < amount) {
            return false;  // 余额不足
        }

        from_it->second[currency] = from_it->second[currency].get<int>() - amount;
        to_it->second[currency] = to_it->second[currency].get<int>() + amount;

        return true;
    }

private:
    std::map<std::string, nlohmann::json> wallets_;
    std::mutex mutex_;
};

WalletStorage storage;

// 处理器函数
std::string WalletCreate(const std::string& ctx, const std::string& payload) {
    auto data = utils::ParseJSON(payload);
    std::string player_id = data["player_id"];

    if (storage.CreateWallet(player_id)) {
        return utils::ToJSON({
            {"success", true},
            {"player_id", player_id},
            {"message", "钱包创建成功"}
        });
    }

    return utils::ToJSON({
        {"success", false},
        {"error", {{"code", "WALLET_EXISTS"}, {"message", "钱包已存在"}}}
    });
}

std::string WalletGet(const std::string& ctx, const std::string& payload) {
    auto data = utils::ParseJSON(payload);
    std::string player_id = data["player_id"];

    auto wallet = storage.GetWallet(player_id);
    if (wallet) {
        return utils::ToJSON({
            {"success", true},
            {"wallet", *wallet}
        });
    }

    return utils::ToJSON({
        {"success", false},
        {"error", {{"code", "WALLET_NOT_FOUND"}, {"message", "钱包不存在"}}}
    });
}

std::string WalletTransfer(const std::string& ctx, const std::string& payload) {
    auto data = utils::ParseJSON(payload);

    std::string from = data["from_player_id"];
    std::string to = data["to_player_id"];
    std::string currency = data.value("currency", "gold");
    int amount = data["amount"];

    // 验证
    if (from == to) {
        return R"({"success": false, "error": {"code": "INVALID_TRANSFER", "message": "不能转账给自己"}})";
    }
    if (amount <= 0) {
        return R"({"success": false, "error": {"code": "INVALID_AMOUNT", "message": "金额必须大于0"}})";
    }

    // 执行转账
    if (storage.Transfer(from, to, currency, amount)) {
        return utils::ToJSON({
            {"success", true},
            {"from", from},
            {"to", to},
            {"currency", currency},
            {"amount", amount}
        });
    }

    return utils::ToJSON({
        {"success", false},
        {"error", {{"code", "TRANSFER_FAILED"}, {"message", "转账失败（可能是余额不足）"}}}
    });
}

int main() {
    ClientConfig config;
    config.game_id = "demo-game";
    config.env = "development";
    config.agent_addr = "127.0.0.1:19090";

    CroupierClient client(config);

    // 定义虚拟对象
    VirtualObjectDescriptor wallet_entity;
    wallet_entity.id = "wallet.entity";
    wallet_entity.version = "0.1.0";
    wallet_entity.name = "玩家钱包";
    wallet_entity.description = "管理玩家游戏货币";

    // 定义操作
    wallet_entity.operations["create"] = "wallet.create";
    wallet_entity.operations["get"] = "wallet.get";
    wallet_entity.operations["transfer"] = "wallet.transfer";

    // 定义 Schema
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

    // 注册处理器
    std::map<std::string, FunctionHandler> handlers = {
        {"wallet.create", WalletCreate},
        {"wallet.get", WalletGet},
        {"wallet.transfer", WalletTransfer}
    };

    // 注册虚拟对象
    if (client.RegisterVirtualObject(wallet_entity, handlers)) {
        std::cout << "Wallet entity registered!" << std::endl;
    }

    // 连接并服务
    if (client.Connect()) {
        std::cout << "Connected! Serving..." << std::endl;
        client.Serve();
    }

    return 0;
}
```

## 测试

### 创建钱包

```bash
curl -X POST http://localhost:18780/api/v1/functions/wallet.create/invoke \
  -H "Content-Type: application/json" \
  -H "X-Game-ID: demo-game" \
  -d '{"player_id": "player123"}'
```

响应：
```json
{
  "success": true,
  "player_id": "player123",
  "message": "钱包创建成功"
}
```

### 查询钱包

```bash
curl -X POST http://localhost:18780/api/v1/functions/wallet.get/invoke \
  -H "Content-Type: application/json" \
  -H "X-Game-ID: demo-game" \
  -d '{"player_id": "player123"}'
```

响应：
```json
{
  "success": true,
  "wallet": {
    "player_id": "player123",
    "balance": {
      "gold": 1000,
      "diamond": 50
    }
  }
}
```

### 转账

```bash
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

响应：
```json
{
  "success": true,
  "from": "player123",
  "to": "player456",
  "currency": "gold",
  "amount": 100
}
```

## 扩展

### 添加新操作

```cpp
// 存款
std::string WalletDeposit(const std::string& ctx, const std::string& payload) {
    auto data = utils::ParseJSON(payload);
    std::string player_id = data["player_id"];
    std::string currency = data["currency"];
    int amount = data["amount"];

    // 实现存款逻辑...

    return utils::ToJSON({{"success", true}});
}

// 在 main 中添加
wallet_entity.operations["deposit"] = "wallet.deposit";
handlers["wallet.deposit"] = WalletDeposit;
```

### 添加权限检查

```cpp
std::string WalletGet(const std::string& ctx, const std::string& payload) {
    auto ctx_data = utils::ParseJSON(ctx);
    std::string caller_id = ctx_data.value("caller_id", "");

    auto data = utils::ParseJSON(payload);
    std::string player_id = data["player_id"];

    // 权限检查：只能查看自己的钱包
    if (caller_id != player_id && !IsAdmin(caller_id)) {
        return R"({"success": false, "error": {"code": "PERMISSION_DENIED"}})";
    }

    // 继续处理...
}
```
