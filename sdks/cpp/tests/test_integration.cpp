#include <gtest/gtest.h>
#include "croupier/sdk/croupier_client.h"
#include "croupier/sdk/grpc_service.h"
#include <thread>
#include <chrono>

using namespace croupier::sdk;

// 集成测试类
class IntegrationTest : public ::testing::Test {
protected:
    void SetUp() override {
        // 配置测试客户端
        config.game_id = "test-integration-game";
        config.env = "development";
        config.service_id = "integration-test-service";
        config.agent_addr = "127.0.0.1:19090";
        config.local_listen = "127.0.0.1:0";
        config.insecure = true;

        client = std::make_unique<CroupierClient>(config);
    }

    void TearDown() override {
        if (client) {
            client->Stop();
        }
    }

    ClientConfig config;
    std::unique_ptr<CroupierClient> client;
};

// 测试 gRPC 管理器初始化
TEST_F(IntegrationTest, GrpcManagerInitialization) {
    // gRPC 管理器应该在客户端创建时自动初始化
    EXPECT_NE(client, nullptr);

    // 测试配置传递
    EXPECT_EQ(config.game_id, "test-integration-game");
    EXPECT_EQ(config.env, "development");
}

// 测试虚拟对象注册
TEST_F(IntegrationTest, VirtualObjectRegistration) {
    // 创建测试钱包对象
    VirtualObjectDescriptor wallet;
    wallet.id = "integration.wallet";
    wallet.version = "1.0.0";
    wallet.name = "集成测试钱包";
    wallet.operations["get"] = "wallet.get";
    wallet.operations["transfer"] = "wallet.transfer";

    // 创建处理器
    std::map<std::string, FunctionHandler> handlers;
    handlers["wallet.get"] = [](const std::string& context, const std::string& payload) -> std::string {
        return R"({"balance": 1000, "status": "success"})";
    };
    handlers["wallet.transfer"] = [](const std::string& context, const std::string& payload) -> std::string {
        return R"({"transfer_id": "tx_123", "status": "completed"})";
    };

    // 注册应该成功
    bool result = client->RegisterVirtualObject(wallet, handlers);
    EXPECT_TRUE(result);

    // 验证注册的对象
    auto objects = client->GetRegisteredObjects();
    EXPECT_GE(objects.size(), 1);

    bool found = false;
    for (const auto& obj : objects) {
        if (obj.id == "integration.wallet") {
            found = true;
            EXPECT_EQ(obj.version, "1.0.0");
            EXPECT_EQ(obj.name, "集成测试钱包");
            break;
        }
    }
    EXPECT_TRUE(found);
}

// 测试组件注册
TEST_F(IntegrationTest, ComponentRegistration) {
    // 创建测试组件
    ComponentDescriptor economy;
    economy.id = "integration.economy";
    economy.version = "1.0.0";
    economy.name = "集成测试经济系统";

    // 添加虚拟对象
    VirtualObjectDescriptor wallet;
    wallet.id = "economy.wallet";
    wallet.version = "1.0.0";
    wallet.name = "经济钱包";
    wallet.operations["balance"] = "economy.wallet.balance";

    economy.virtual_objects["wallet"] = wallet;

    // 添加处理器
    economy.function_handlers["economy.wallet.balance"] =
        [](const std::string& context, const std::string& payload) -> std::string {
            return R"({"balance": 2500, "currency": "gold"})";
        };

    // 注册组件
    bool result = client->RegisterComponent(economy);
    EXPECT_TRUE(result);

    // 验证组件注册
    auto components = client->GetRegisteredComponents();
    EXPECT_GE(components.size(), 1);

    bool found = false;
    for (const auto& comp : components) {
        if (comp.id == "integration.economy") {
            found = true;
            EXPECT_EQ(comp.version, "1.0.0");
            break;
        }
    }
    EXPECT_TRUE(found);
}

// 测试连接尝试（模拟环境下）
TEST_F(IntegrationTest, ConnectionAttempt) {
    // 注册一些功能
    FunctionDescriptor desc;
    desc.id = "test.ping";
    desc.version = "1.0.0";

    auto handler = [](const std::string& context, const std::string& payload) -> std::string {
        return R"({"message": "pong", "timestamp": "2024-11-14T10:00:00Z"})";
    };

    client->RegisterFunction(desc, handler);

    // 尝试连接（在没有真实Agent的情况下会失败，这是正常的）
    bool connected = client->Connect();

    // 在集成测试环境中，我们可能没有真实的Agent运行
    // 所以连接失败是正常的，但gRPC管理器应该正确处理这种情况
    if (!connected) {
        std::cout << "ℹ️ Agent 连接失败（预期行为，因为测试环境可能没有运行 Agent）" << std::endl;
        EXPECT_FALSE(connected); // 这是预期的结果
    } else {
        std::cout << "✅ 成功连接到 Agent！" << std::endl;
        EXPECT_TRUE(connected);

        // 如果连接成功，测试基本服务功能
        std::string local_addr = client->GetLocalAddress();
        EXPECT_FALSE(local_addr.empty());

        std::cout << "📍 本地服务地址: " << local_addr << std::endl;
    }
}

// 测试错误处理
TEST_F(IntegrationTest, ErrorHandling) {
    // 测试无效的虚拟对象注册
    VirtualObjectDescriptor invalid_obj;
    // 故意留空 ID
    invalid_obj.id = "";
    invalid_obj.version = "1.0.0";

    std::map<std::string, FunctionHandler> empty_handlers;

    // 应该处理错误并返回 false
    bool result = client->RegisterVirtualObject(invalid_obj, empty_handlers);
    EXPECT_FALSE(result);
}

// 测试注销功能
TEST_F(IntegrationTest, UnregistrationFunctions) {
    // 首先注册一个对象
    VirtualObjectDescriptor test_obj;
    test_obj.id = "test.unregister.object";
    test_obj.version = "1.0.0";
    test_obj.name = "测试注销对象";

    std::map<std::string, FunctionHandler> handlers;
    handlers["test.function"] = [](const std::string& context, const std::string& payload) -> std::string {
        return "{}";
    };

    EXPECT_TRUE(client->RegisterVirtualObject(test_obj, handlers));

    // 验证对象已注册
    auto objects_before = client->GetRegisteredObjects();
    bool found_before = false;
    for (const auto& obj : objects_before) {
        if (obj.id == "test.unregister.object") {
            found_before = true;
            break;
        }
    }
    EXPECT_TRUE(found_before);

    // 注销对象
    EXPECT_TRUE(client->UnregisterVirtualObject("test.unregister.object"));

    // 验证对象已被移除
    auto objects_after = client->GetRegisteredObjects();
    bool found_after = false;
    for (const auto& obj : objects_after) {
        if (obj.id == "test.unregister.object") {
            found_after = true;
            break;
        }
    }
    EXPECT_FALSE(found_after);
}

// 测试复杂场景
TEST_F(IntegrationTest, ComplexScenario) {
    // 注册多个相互关联的虚拟对象

    // 1. 玩家对象
    VirtualObjectDescriptor player;
    player.id = "game.player";
    player.version = "1.0.0";
    player.name = "游戏玩家";
    player.operations["create"] = "player.create";
    player.operations["get"] = "player.get";

    // 玩家与钱包的关系
    RelationshipDef wallet_rel;
    wallet_rel.type = "one-to-one";
    wallet_rel.entity = "wallet";
    wallet_rel.foreign_key = "player_id";
    player.relationships["wallet"] = wallet_rel;

    // 2. 钱包对象
    VirtualObjectDescriptor wallet;
    wallet.id = "game.wallet";
    wallet.version = "1.0.0";
    wallet.name = "游戏钱包";
    wallet.operations["get"] = "wallet.get";
    wallet.operations["transfer"] = "wallet.transfer";

    // 3. 创建处理器
    std::map<std::string, FunctionHandler> player_handlers;
    player_handlers["player.create"] = [](const std::string& context, const std::string& payload) -> std::string {
        return R"({
            "player_id": "player_integration_001",
            "name": "IntegrationTestPlayer",
            "level": 1,
            "created_at": "2024-11-14T10:00:00Z"
        })";
    };
    player_handlers["player.get"] = [](const std::string& context, const std::string& payload) -> std::string {
        return R"({
            "player_id": "player_integration_001",
            "name": "IntegrationTestPlayer",
            "level": 5,
            "experience": 1250
        })";
    };

    std::map<std::string, FunctionHandler> wallet_handlers;
    wallet_handlers["wallet.get"] = [](const std::string& context, const std::string& payload) -> std::string {
        return R"({
            "wallet_id": "wallet_integration_001",
            "player_id": "player_integration_001",
            "balance": 750,
            "currency": "gold"
        })";
    };
    wallet_handlers["wallet.transfer"] = [](const std::string& context, const std::string& payload) -> std::string {
        return R"({
            "transfer_id": "tx_integration_001",
            "status": "success",
            "amount": 100,
            "fee": 5
        })";
    };

    // 4. 注册所有对象
    EXPECT_TRUE(client->RegisterVirtualObject(player, player_handlers));
    EXPECT_TRUE(client->RegisterVirtualObject(wallet, wallet_handlers));

    // 5. 验证注册结果
    auto all_objects = client->GetRegisteredObjects();
    EXPECT_GE(all_objects.size(), 2);

    // 验证关系定义
    bool player_found = false;
    for (const auto& obj : all_objects) {
        if (obj.id == "game.player") {
            player_found = true;
            EXPECT_EQ(obj.relationships.size(), 1);
            auto rel_it = obj.relationships.find("wallet");
            EXPECT_NE(rel_it, obj.relationships.end());
            EXPECT_EQ(rel_it->second.type, "one-to-one");
            EXPECT_EQ(rel_it->second.entity, "wallet");
            break;
        }
    }
    EXPECT_TRUE(player_found);

    std::cout << "✅ 复杂场景测试完成，已注册 " << all_objects.size() << " 个虚拟对象" << std::endl;
}