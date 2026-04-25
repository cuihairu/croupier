#include "croupier/sdk/croupier_client.h"

#include <chrono>
#include <iostream>
#include <signal.h>
#include <thread>

using namespace croupier::sdk;

// 全局变量用于信号Handler
std::unique_ptr<CroupierClient> g_client;

// 信号HandlerFunction
void signalHandler(int /* signal */) {
    std::cout << "\n🛑 接收到Stop信号，正在优雅Close..." << std::endl;
    if (g_client) {
        g_client->Stop();
    }
    exit(0);
}

// 钱包相关的Handler器
std::string WalletGetHandler(const std::string& /* context */, const std::string& payload) {
    std::cout << "💰 Execute钱包Query: " << payload << std::endl;

    // Simulate业务逻辑
    std::this_thread::sleep_for(std::chrono::milliseconds(100));

    // 返回JSON结果
    return R"({
        "wallet_id": "wallet_12345",
        "player_id": "player_67890",
        "balance": 1500,
        "currency": "gold",
        "last_updated": "2024-11-14T10:30:00Z"
    })";
}

std::string WalletTransferHandler(const std::string& /* context */, const std::string& payload) {
    std::cout << "💸 Execute钱包转账: " << payload << std::endl;

    // Simulate转账Handler
    std::this_thread::sleep_for(std::chrono::milliseconds(200));

    return R"({
        "transfer_id": "tx_abcdef123456",
        "status": "success",
        "from_wallet": "wallet_12345",
        "to_wallet": "wallet_67890",
        "amount": 100,
        "currency": "gold",
        "timestamp": "2024-11-14T10:35:00Z",
        "fee": 5
    })";
}

// PlayerManagementHandler器
std::string PlayerCreateHandler(const std::string& /* context */, const std::string& payload) {
    std::cout << "👤 Create新Player: " << payload << std::endl;

    std::this_thread::sleep_for(std::chrono::milliseconds(150));

    return R"({
        "player_id": "player_new_001",
        "status": "created",
        "name": "NewPlayer",
        "level": 1,
        "created_at": "2024-11-14T10:40:00Z",
        "wallet_id": "wallet_new_001"
    })";
}

std::string PlayerGetHandler(const std::string& /* context */, const std::string& payload) {
    std::cout << "👤 QueryPlayerInfo: " << payload << std::endl;

    return R"({
        "player_id": "player_67890",
        "name": "TestPlayer",
        "level": 25,
        "experience": 12500,
        "guild": "AwesomeGuild",
        "last_login": "2024-11-14T09:15:00Z",
        "wallet_id": "wallet_67890",
        "achievements": ["first_kill", "level_10", "treasure_hunter"]
    })";
}

// 商店系统Handler器
std::string ShopListItemsHandler(const std::string& /* context */, const std::string& payload) {
    std::cout << "🛒 Query商店物品: " << payload << std::endl;

    return R"({
        "items": [
            {
                "item_id": "sword_001",
                "name": "钢铁长剑",
                "price": 150,
                "currency": "gold",
                "category": "weapon",
                "in_stock": 5
            },
            {
                "item_id": "potion_hp_001",
                "name": "生命药水",
                "price": 20,
                "currency": "gold",
                "category": "consumable",
                "in_stock": 50
            },
            {
                "item_id": "armor_001",
                "name": "皮革护甲",
                "price": 80,
                "currency": "gold",
                "category": "armor",
                "in_stock": 3
            }
        ],
        "shop_id": "main_shop",
        "last_updated": "2024-11-14T10:00:00Z"
    })";
}

int main(int /* argc */, char* /* argv */[]) {
    std::cout << "🎮 Croupier C++ SDK 完整Sample" << std::endl;
    std::cout << "===============================================" << std::endl;

    // Set信号Handler
    signal(SIGINT, signalHandler);
    signal(SIGTERM, signalHandler);

    try {
        // 1. ConfigurationClient
        ClientConfig config;
        config.game_id = "example-mmorpg";
        config.env = "development";
        config.service_id = "game-backend";
        config.agent_addr = "127.0.0.1:19090";
        config.insecure = true;  // 开发环境使用非安全Connect

        std::cout << "🔧 ConfigurationClient:" << std::endl;
        std::cout << "   - 游戏 ID: " << config.game_id << std::endl;
        std::cout << "   - 环境: " << config.env << std::endl;
        std::cout << "   - Agent Address: " << config.agent_addr << std::endl;

        // 2. CreateClient
        g_client = std::make_unique<CroupierClient>(config);

        // 3. DefinitionVirtual Object - 钱包系统
        VirtualObjectDescriptor wallet;
        wallet.id = "economy.wallet";
        wallet.version = "1.0.0";
        wallet.name = "Player钱包";
        wallet.description = "ManagementPlayer虚拟货币和交易";
        wallet.operations["get"] = "wallet.get";
        wallet.operations["transfer"] = "wallet.transfer";

        // 钱包与Player的关系
        RelationshipDef player_rel;
        player_rel.type = "many-to-one";
        player_rel.entity = "player";
        player_rel.foreign_key = "player_id";
        wallet.relationships["owner"] = player_rel;

        // 4. DefinitionVirtual Object - Player系统
        VirtualObjectDescriptor player;
        player.id = "game.player";
        player.version = "1.0.0";
        player.name = "游戏Player";
        player.description = "Player账户和属性Management";
        player.operations["create"] = "player.create";
        player.operations["get"] = "player.get";

        // 5. DefinitionVirtual Object - 商店系统
        VirtualObjectDescriptor shop;
        shop.id = "economy.shop";
        shop.version = "1.0.0";
        shop.name = "游戏商店";
        shop.description = "物品销售和购买系统";
        shop.operations["list"] = "shop.list_items";

        // 6. Register钱包系统
        std::map<std::string, FunctionHandler> wallet_handlers;
        wallet_handlers["wallet.get"] = WalletGetHandler;
        wallet_handlers["wallet.transfer"] = WalletTransferHandler;

        std::cout << "\n💰 Register钱包系统..." << std::endl;
        if (!g_client->RegisterVirtualObject(wallet, wallet_handlers)) {
            std::cerr << "❌ 钱包系统RegisterFailed!" << std::endl;
            return 1;
        }

        // 7. RegisterPlayer系统
        std::map<std::string, FunctionHandler> player_handlers;
        player_handlers["player.create"] = PlayerCreateHandler;
        player_handlers["player.get"] = PlayerGetHandler;

        std::cout << "👤 RegisterPlayer系统..." << std::endl;
        if (!g_client->RegisterVirtualObject(player, player_handlers)) {
            std::cerr << "❌ Player系统RegisterFailed!" << std::endl;
            return 1;
        }

        // 8. Register商店系统
        std::map<std::string, FunctionHandler> shop_handlers;
        shop_handlers["shop.list_items"] = ShopListItemsHandler;

        std::cout << "🛒 Register商店系统..." << std::endl;
        if (!g_client->RegisterVirtualObject(shop, shop_handlers)) {
            std::cerr << "❌ 商店系统RegisterFailed!" << std::endl;
            return 1;
        }

        // 9. 展示Register的系统
        auto registered_objects = g_client->GetRegisteredObjects();
        std::cout << "\n📋 已Register的Virtual Object (" << registered_objects.size() << " 个):" << std::endl;
        for (const auto& obj : registered_objects) {
            std::cout << "   ✓ " << obj.id << " v" << obj.version << " - " << obj.name << std::endl;
            std::cout << "     Operation: ";
            for (const auto& op : obj.operations) {
                std::cout << op.first << " ";
            }
            std::cout << std::endl;
        }

        // 10. Connect到 Agent
        std::cout << "\n🔌 Connect到 Croupier Agent..." << std::endl;
        if (!g_client->Connect()) {
            std::cerr << "❌ 无法Connect到 Agent!" << std::endl;
            std::cerr << "💡 请确保 Croupier Agent 正在运行在: " << config.agent_addr << std::endl;
            return 1;
        }

        std::cout << "✅ SuccessConnect到 Agent!" << std::endl;

        // 11. StartService
        std::cout << "\n🚀 StartService，等待FunctionInvoke..." << std::endl;
        std::cout << "💡 提示: 使用 Ctrl+C 优雅StopService" << std::endl;
        std::cout << "===============================================" << std::endl;

        // StartService (阻塞Invoke)
        g_client->Serve();

    } catch (const std::exception& e) {
        std::cerr << "💥 程序Exception: " << e.what() << std::endl;
        return 1;
    }

    std::cout << "\n👋 Sample程序已结束" << std::endl;
    return 0;
}