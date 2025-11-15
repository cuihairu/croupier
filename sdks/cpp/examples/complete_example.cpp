#include "croupier/sdk/croupier_client.h"
#include <iostream>
#include <chrono>
#include <thread>
#include <signal.h>

using namespace croupier::sdk;

// 全局变量用于信号处理
std::unique_ptr<CroupierClient> g_client;

// 信号处理函数
void signalHandler(int /* signal */) {
    std::cout << "\n🛑 接收到停止信号，正在优雅关闭..." << std::endl;
    if (g_client) {
        g_client->Stop();
    }
    exit(0);
}

// 钱包相关的处理器
std::string WalletGetHandler(const std::string& /* context */, const std::string& payload) {
    std::cout << "💰 执行钱包查询: " << payload << std::endl;

    // 模拟业务逻辑
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
    std::cout << "💸 执行钱包转账: " << payload << std::endl;

    // 模拟转账处理
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

// 玩家管理处理器
std::string PlayerCreateHandler(const std::string& /* context */, const std::string& payload) {
    std::cout << "👤 创建新玩家: " << payload << std::endl;

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
    std::cout << "👤 查询玩家信息: " << payload << std::endl;

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

// 商店系统处理器
std::string ShopListItemsHandler(const std::string& /* context */, const std::string& payload) {
    std::cout << "🛒 查询商店物品: " << payload << std::endl;

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
    std::cout << "🎮 Croupier C++ SDK 完整示例" << std::endl;
    std::cout << "===============================================" << std::endl;

    // 设置信号处理
    signal(SIGINT, signalHandler);
    signal(SIGTERM, signalHandler);

    try {
        // 1. 配置客户端
        ClientConfig config;
        config.game_id = "example-mmorpg";
        config.env = "development";
        config.service_id = "game-backend";
        config.agent_addr = "127.0.0.1:19090";
        config.local_listen = "127.0.0.1:0";  // 自动分配端口
        config.insecure = true;  // 开发环境使用非安全连接

        std::cout << "🔧 配置客户端:" << std::endl;
        std::cout << "   - 游戏 ID: " << config.game_id << std::endl;
        std::cout << "   - 环境: " << config.env << std::endl;
        std::cout << "   - Agent 地址: " << config.agent_addr << std::endl;

        // 2. 创建客户端
        g_client = std::make_unique<CroupierClient>(config);

        // 3. 定义虚拟对象 - 钱包系统
        VirtualObjectDescriptor wallet;
        wallet.id = "economy.wallet";
        wallet.version = "1.0.0";
        wallet.name = "玩家钱包";
        wallet.description = "管理玩家虚拟货币和交易";
        wallet.operations["get"] = "wallet.get";
        wallet.operations["transfer"] = "wallet.transfer";

        // 钱包与玩家的关系
        RelationshipDef player_rel;
        player_rel.type = "many-to-one";
        player_rel.entity = "player";
        player_rel.foreign_key = "player_id";
        wallet.relationships["owner"] = player_rel;

        // 4. 定义虚拟对象 - 玩家系统
        VirtualObjectDescriptor player;
        player.id = "game.player";
        player.version = "1.0.0";
        player.name = "游戏玩家";
        player.description = "玩家账户和属性管理";
        player.operations["create"] = "player.create";
        player.operations["get"] = "player.get";

        // 5. 定义虚拟对象 - 商店系统
        VirtualObjectDescriptor shop;
        shop.id = "economy.shop";
        shop.version = "1.0.0";
        shop.name = "游戏商店";
        shop.description = "物品销售和购买系统";
        shop.operations["list"] = "shop.list_items";

        // 6. 注册钱包系统
        std::map<std::string, FunctionHandler> wallet_handlers;
        wallet_handlers["wallet.get"] = WalletGetHandler;
        wallet_handlers["wallet.transfer"] = WalletTransferHandler;

        std::cout << "\n💰 注册钱包系统..." << std::endl;
        if (!g_client->RegisterVirtualObject(wallet, wallet_handlers)) {
            std::cerr << "❌ 钱包系统注册失败!" << std::endl;
            return 1;
        }

        // 7. 注册玩家系统
        std::map<std::string, FunctionHandler> player_handlers;
        player_handlers["player.create"] = PlayerCreateHandler;
        player_handlers["player.get"] = PlayerGetHandler;

        std::cout << "👤 注册玩家系统..." << std::endl;
        if (!g_client->RegisterVirtualObject(player, player_handlers)) {
            std::cerr << "❌ 玩家系统注册失败!" << std::endl;
            return 1;
        }

        // 8. 注册商店系统
        std::map<std::string, FunctionHandler> shop_handlers;
        shop_handlers["shop.list_items"] = ShopListItemsHandler;

        std::cout << "🛒 注册商店系统..." << std::endl;
        if (!g_client->RegisterVirtualObject(shop, shop_handlers)) {
            std::cerr << "❌ 商店系统注册失败!" << std::endl;
            return 1;
        }

        // 9. 展示注册的系统
        auto registered_objects = g_client->GetRegisteredObjects();
        std::cout << "\n📋 已注册的虚拟对象 (" << registered_objects.size() << " 个):" << std::endl;
        for (const auto& obj : registered_objects) {
            std::cout << "   ✓ " << obj.id << " v" << obj.version
                      << " - " << obj.name << std::endl;
            std::cout << "     操作: ";
            for (const auto& op : obj.operations) {
                std::cout << op.first << " ";
            }
            std::cout << std::endl;
        }

        // 10. 连接到 Agent
        std::cout << "\n🔌 连接到 Croupier Agent..." << std::endl;
        if (!g_client->Connect()) {
            std::cerr << "❌ 无法连接到 Agent!" << std::endl;
            std::cerr << "💡 请确保 Croupier Agent 正在运行在: " << config.agent_addr << std::endl;
            return 1;
        }

        std::cout << "✅ 成功连接到 Agent!" << std::endl;

        // 11. 启动服务
        std::cout << "\n🚀 启动服务，等待函数调用..." << std::endl;
        std::cout << "💡 提示: 使用 Ctrl+C 优雅停止服务" << std::endl;
        std::cout << "===============================================" << std::endl;

        // 启动服务 (阻塞调用)
        g_client->Serve();

    } catch (const std::exception& e) {
        std::cerr << "💥 程序异常: " << e.what() << std::endl;
        return 1;
    }

    std::cout << "\n👋 示例程序已结束" << std::endl;
    return 0;
}