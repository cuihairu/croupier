#include <gtest/gtest.h>
#include "croupier/sdk/croupier_client.h"
#include <memory>

using namespace croupier::sdk;

class VirtualObjectTest : public ::testing::Test {
protected:
    void SetUp() override {
        ClientConfig config;
        config.game_id = "test-game";
        config.env = "development";
        config.service_id = "test-service";
        config.agent_addr = "127.0.0.1:19090";
        config.insecure = true;

        client = std::make_unique<CroupierClient>(config);
    }

    void TearDown() override {
        if (client) {
            client->Close();
        }
    }

    std::unique_ptr<CroupierClient> client;
};

// 测试虚拟对象描述符的创建
TEST_F(VirtualObjectTest, CreateVirtualObjectDescriptor) {
    VirtualObjectDescriptor wallet;
    wallet.id = "wallet.entity";
    wallet.version = "1.0.0";
    wallet.name = "玩家钱包";
    wallet.description = "管理玩家虚拟货币的钱包系统";

    // 添加操作映射
    wallet.operations["read"] = "wallet.get";
    wallet.operations["transfer"] = "wallet.transfer";
    wallet.operations["update"] = "wallet.update";

    // 添加关系
    RelationshipDef currency_rel;
    currency_rel.type = "many-to-one";
    currency_rel.entity = "currency";
    currency_rel.foreign_key = "currency_id";
    wallet.relationships["currency"] = currency_rel;

    EXPECT_EQ(wallet.id, "wallet.entity");
    EXPECT_EQ(wallet.version, "1.0.0");
    EXPECT_EQ(wallet.operations.size(), 3);
    EXPECT_EQ(wallet.relationships.size(), 1);
}

// 测试函数处理器
TEST_F(VirtualObjectTest, FunctionHandler) {
    // 创建一个简单的处理器
    auto handler = [](const std::string& context, const std::string& payload) -> std::string {
        return R"({"status": "success", "message": "handler executed"})";
    };

    // 执行处理器
    std::string result = handler("test-context", R"({"player_id": "123"})");

    EXPECT_FALSE(result.empty());
    EXPECT_NE(result.find("success"), std::string::npos);
}

// 测试虚拟对象注册
TEST_F(VirtualObjectTest, RegisterVirtualObject) {
    // 创建虚拟对象描述符
    VirtualObjectDescriptor wallet;
    wallet.id = "wallet.entity";
    wallet.version = "1.0.0";
    wallet.name = "钱包系统";
    wallet.operations["get"] = "wallet.get";
    wallet.operations["transfer"] = "wallet.transfer";

    // 创建处理器映射
    std::map<std::string, FunctionHandler> handlers;

    handlers["wallet.get"] = [](const std::string& context, const std::string& payload) -> std::string {
        return R"({"balance": 1000, "currency": "gold"})";
    };

    handlers["wallet.transfer"] = [](const std::string& context, const std::string& payload) -> std::string {
        return R"({"transfer_id": "tx_123", "status": "success"})";
    };

    // 注册虚拟对象
    bool result = client->RegisterVirtualObject(wallet, handlers);
    EXPECT_TRUE(result);
}

// 测试获取已注册的虚拟对象
TEST_F(VirtualObjectTest, GetRegisteredObjects) {
    // 首先注册一个虚拟对象
    VirtualObjectDescriptor wallet;
    wallet.id = "test.wallet";
    wallet.version = "1.0.0";
    wallet.name = "测试钱包";

    std::map<std::string, FunctionHandler> handlers;
    handlers["wallet.get"] = [](const std::string& context, const std::string& payload) -> std::string {
        return R"({"test": "data"})";
    };

    client->RegisterVirtualObject(wallet, handlers);

    // 获取已注册对象列表
    auto objects = client->GetRegisteredObjects();

    // 检查是否包含我们注册的对象
    bool found = false;
    for (const auto& obj : objects) {
        if (obj.id == "test.wallet") {
            found = true;
            break;
        }
    }
    EXPECT_TRUE(found);
}

// 测试组件注册
TEST_F(VirtualObjectTest, RegisterComponent) {
    ComponentDescriptor comp;
    comp.id = "economy-system";
    comp.version = "1.0.0";
    comp.name = "经济系统";
    comp.description = "完整的游戏经济管理系统";

    // 添加虚拟对象
    VirtualObjectDescriptor wallet;
    wallet.id = "economy.wallet";
    wallet.version = "1.0.0";
    wallet.name = "钱包";
    comp.virtual_objects["wallet"] = wallet;

    // 添加函数映射
    comp.function_handlers["economy.wallet.get"] = [](const std::string& context, const std::string& payload) -> std::string {
        return R"({"component": "economy", "function": "wallet.get"})";
    };

    bool result = client->RegisterComponent(comp);
    EXPECT_TRUE(result);
}

// 测试注销虚拟对象
TEST_F(VirtualObjectTest, UnregisterVirtualObject) {
    // 首先注册一个虚拟对象
    VirtualObjectDescriptor test_obj;
    test_obj.id = "test.unregister";
    test_obj.version = "1.0.0";

    std::map<std::string, FunctionHandler> handlers;
    handlers["test.function"] = [](const std::string& context, const std::string& payload) -> std::string {
        return "{}";
    };

    client->RegisterVirtualObject(test_obj, handlers);

    // 注销虚拟对象
    bool result = client->UnregisterVirtualObject("test.unregister");
    EXPECT_TRUE(result);
}

// 测试关系定义
TEST_F(VirtualObjectTest, RelationshipDefinition) {
    RelationshipDef rel;
    rel.type = "one-to-many";
    rel.entity = "transaction";
    rel.foreign_key = "wallet_id";

    EXPECT_EQ(rel.type, "one-to-many");
    EXPECT_EQ(rel.entity, "transaction");
    EXPECT_EQ(rel.foreign_key, "wallet_id");
}

// 测试复杂的虚拟对象系统
TEST_F(VirtualObjectTest, ComplexVirtualObjectSystem) {
    // 创建玩家虚拟对象
    VirtualObjectDescriptor player;
    player.id = "player.entity";
    player.version = "1.0.0";
    player.name = "游戏玩家";
    player.operations["create"] = "player.create";
    player.operations["get"] = "player.get";
    player.operations["update"] = "player.update";

    // 玩家与钱包的关系
    RelationshipDef wallet_rel;
    wallet_rel.type = "one-to-one";
    wallet_rel.entity = "wallet";
    wallet_rel.foreign_key = "player_id";
    player.relationships["wallet"] = wallet_rel;

    // 创建钱包虚拟对象
    VirtualObjectDescriptor wallet;
    wallet.id = "wallet.entity";
    wallet.version = "1.0.0";
    wallet.name = "玩家钱包";
    wallet.operations["get"] = "wallet.get";
    wallet.operations["transfer"] = "wallet.transfer";

    // 钱包与交易的关系
    RelationshipDef transaction_rel;
    transaction_rel.type = "one-to-many";
    transaction_rel.entity = "transaction";
    transaction_rel.foreign_key = "wallet_id";
    wallet.relationships["transactions"] = transaction_rel;

    // 创建处理器
    std::map<std::string, FunctionHandler> player_handlers;
    player_handlers["player.create"] = [](const std::string& context, const std::string& payload) -> std::string {
        return R"({"player_id": "p123", "status": "created"})";
    };
    player_handlers["player.get"] = [](const std::string& context, const std::string& payload) -> std::string {
        return R"({"player_id": "p123", "name": "TestPlayer", "level": 10})";
    };
    player_handlers["player.update"] = [](const std::string& context, const std::string& payload) -> std::string {
        return R"({"player_id": "p123", "status": "updated"})";
    };

    std::map<std::string, FunctionHandler> wallet_handlers;
    wallet_handlers["wallet.get"] = [](const std::string& context, const std::string& payload) -> std::string {
        return R"({"wallet_id": "w123", "balance": 1500, "currency": "gold"})";
    };
    wallet_handlers["wallet.transfer"] = [](const std::string& context, const std::string& payload) -> std::string {
        return R"({"transfer_id": "tx456", "status": "completed", "amount": 100})";
    };

    // 注册两个虚拟对象
    bool player_result = client->RegisterVirtualObject(player, player_handlers);
    bool wallet_result = client->RegisterVirtualObject(wallet, wallet_handlers);

    EXPECT_TRUE(player_result);
    EXPECT_TRUE(wallet_result);

    // 验证注册的对象数量
    auto objects = client->GetRegisteredObjects();
    EXPECT_GE(objects.size(), 2);
}