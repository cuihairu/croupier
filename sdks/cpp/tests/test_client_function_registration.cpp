#include <gtest/gtest.h>
#include "croupier/sdk/croupier_client.h"
#include "croupier/sdk/config/client_config_loader.h"

using namespace croupier::sdk;
using namespace croupier::sdk::config;

// 测试夹具：函数注册测试
class ClientFunctionRegistrationTest : public ::testing::Test {
protected:
    void SetUp() override {
        loader = std::make_unique<ClientConfigLoader>();
        config = loader->CreateDefaultConfig();
        client = std::make_unique<CroupierClient>(config);
    }

    void TearDown() override {
        // 清理
    }

    // 辅助函数：创建基本的函数描述符
    FunctionDescriptor CreateBasicFunctionDescriptor(const std::string& id) {
        FunctionDescriptor desc;
        desc.id = id;
        desc.version = "1.0.0";
        desc.category = "test";
        desc.risk = "low";
        desc.entity = "player";
        desc.operation = "create";
        desc.enabled = true;
        return desc;
    }

    // 辅助函数：创建简单的函数处理器
    FunctionHandler CreateSimpleHandler(const std::string& response) {
        return [response](const std::string& context, const std::string& payload) -> std::string {
            return response;
        };
    }

    std::unique_ptr<ClientConfigLoader> loader;
    ClientConfig config;
    std::unique_ptr<CroupierClient> client;
};

// ========== 测试用例 ==========

TEST_F(ClientFunctionRegistrationTest, RegisterSingleFunction) {
    // RED: 测试注册单个函数
    FunctionDescriptor desc = CreateBasicFunctionDescriptor("test.function");

    // 创建函数处理器
    FunctionHandler handler = CreateSimpleHandler("{\"result\":\"success\"}");

    // 注册函数
    bool success = client->RegisterFunction(desc, handler);

    // 注册应该成功
    EXPECT_TRUE(success);
}

TEST_F(ClientFunctionRegistrationTest, RegisterMultipleFunctions) {
    // RED: 测试注册多个不同函数
    // 创建第一个函数
    FunctionDescriptor func1 = CreateBasicFunctionDescriptor("test.function1");
    FunctionHandler handler1 = CreateSimpleHandler("{\"result\":\"function1\"}");

    bool success1 = client->RegisterFunction(func1, handler1);
    EXPECT_TRUE(success1);

    // 创建第二个函数
    FunctionDescriptor func2 = CreateBasicFunctionDescriptor("test.function2");
    FunctionHandler handler2 = CreateSimpleHandler("{\"result\":\"function2\"}");

    bool success2 = client->RegisterFunction(func2, handler2);
    EXPECT_TRUE(success2);

    // 创建第三个函数
    FunctionDescriptor func3 = CreateBasicFunctionDescriptor("test.function3");
    FunctionHandler handler3 = CreateSimpleHandler("{\"result\":\"function3\"}");

    bool success3 = client->RegisterFunction(func3, handler3);
    EXPECT_TRUE(success3);

    // 所有函数都应该成功注册
    EXPECT_TRUE(success1 && success2 && success3);
}

TEST_F(ClientFunctionRegistrationTest, FunctionDescriptorValidation) {
    // RED: 测试函数描述符验证
    FunctionDescriptor desc = CreateBasicFunctionDescriptor("validated.function");

    // 验证所有必填字段都已设置
    EXPECT_FALSE(desc.id.empty());
    EXPECT_FALSE(desc.version.empty());
    EXPECT_FALSE(desc.category.empty());
    EXPECT_FALSE(desc.risk.empty());
    EXPECT_FALSE(desc.entity.empty());
    EXPECT_FALSE(desc.operation.empty());

    // 验证字段值
    EXPECT_EQ(desc.id, "validated.function");
    EXPECT_EQ(desc.version, "1.0.0");
    EXPECT_EQ(desc.category, "test");
    EXPECT_EQ(desc.risk, "low");
    EXPECT_EQ(desc.entity, "player");
    EXPECT_EQ(desc.operation, "create");
    EXPECT_TRUE(desc.enabled);

    // 注册应该成功
    FunctionHandler handler = CreateSimpleHandler("{\"result\":\"ok\"}");
    bool success = client->RegisterFunction(desc, handler);
    EXPECT_TRUE(success);
}

TEST_F(ClientFunctionRegistrationTest, FunctionHandlerInvocation) {
    // RED: 测试函数处理器调用
    FunctionDescriptor desc = CreateBasicFunctionDescriptor("invokable.function");

    // 创建一个可以验证的处理器
    std::string received_context;
    std::string received_payload;
    bool handler_called = false;

    FunctionHandler handler = [&received_context, &received_payload, &handler_called](
        const std::string& context,
        const std::string& payload) -> std::string {
        received_context = context;
        received_payload = payload;
        handler_called = true;
        return "{\"result\":\"handler_executed\"}";
    };

    // 注册函数
    bool success = client->RegisterFunction(desc, handler);
    EXPECT_TRUE(success);

    // 注意：这里我们只测试处理器可以被创建和注册
    // 实际调用需要连接到 agent，这会在连接测试中覆盖
    SUCCEED();
}

TEST_F(ClientFunctionRegistrationTest, RegisterDuplicateFunction) {
    // RED: 测试注册相同函数ID（覆盖行为）
    FunctionDescriptor func1 = CreateBasicFunctionDescriptor("duplicate.function");
    func1.version = "1.0.0";
    FunctionHandler handler1 = CreateSimpleHandler("{\"version\":\"1.0.0\"}");

    // 第一次注册
    bool success1 = client->RegisterFunction(func1, handler1);
    EXPECT_TRUE(success1);

    // 使用相同ID注册第二个版本（应该覆盖）
    FunctionDescriptor func2 = CreateBasicFunctionDescriptor("duplicate.function");
    func2.version = "2.0.0";
    FunctionHandler handler2 = CreateSimpleHandler("{\"version\":\"2.0.0\"}");

    bool success2 = client->RegisterFunction(func2, handler2);
    EXPECT_TRUE(success2);

    // 第二次注册也应该成功（覆盖或更新）
    SUCCEED();
}

TEST_F(ClientFunctionRegistrationTest, RegisterInvalidFunctionMissingFields) {
    // RED: 测试无效函数（缺少必填字段）
    FunctionDescriptor desc;

    // 不设置 id（必填字段）
    desc.id = "";
    desc.version = "1.0.0";
    desc.category = "test";

    FunctionHandler handler = CreateSimpleHandler("{\"result\":\"test\"}");

    // 注册可能失败或拒绝
    bool success = client->RegisterFunction(desc, handler);

    // 期望注册失败或被拒绝
    // 具体行为取决于实现，这里我们只验证不会崩溃
    if (desc.id.empty()) {
        // 如果 ID 为空，应该返回 false
        EXPECT_FALSE(success);
    }
}

TEST_F(ClientFunctionRegistrationTest, FunctionEnabledDisabled) {
    // RED: 测试函数启用/禁用状态
    // 创建启用的函数
    FunctionDescriptor enabled_func = CreateBasicFunctionDescriptor("enabled.function");
    enabled_func.enabled = true;
    FunctionHandler handler1 = CreateSimpleHandler("{\"status\":\"enabled\"}");

    bool success1 = client->RegisterFunction(enabled_func, handler1);
    EXPECT_TRUE(success1);
    EXPECT_TRUE(enabled_func.enabled);

    // 创建禁用的函数
    FunctionDescriptor disabled_func = CreateBasicFunctionDescriptor("disabled.function");
    disabled_func.enabled = false;
    FunctionHandler handler2 = CreateSimpleHandler("{\"status\":\"disabled\"}");

    bool success2 = client->RegisterFunction(disabled_func, handler2);
    EXPECT_TRUE(success2);
    EXPECT_FALSE(disabled_func.enabled);

    // 两个函数都应该能注册成功
    EXPECT_TRUE(success1 && success2);
}

TEST_F(ClientFunctionRegistrationTest, FunctionMetadataCompleteness) {
    // RED: 测试函数元数据完整性
    FunctionDescriptor desc;
    desc.id = "complete.function";
    desc.version = "1.2.3";
    desc.category = "gameplay";
    desc.risk = "medium";
    desc.entity = "item";
    desc.operation = "update";
    desc.enabled = true;

    // 验证所有元数据字段
    EXPECT_EQ(desc.id, "complete.function");
    EXPECT_EQ(desc.version, "1.2.3");
    EXPECT_EQ(desc.category, "gameplay");
    EXPECT_EQ(desc.risk, "medium");
    EXPECT_EQ(desc.entity, "item");
    EXPECT_EQ(desc.operation, "update");
    EXPECT_TRUE(desc.enabled);

    // 验证风险等级是合法的
    bool valid_risk = (desc.risk == "low" || desc.risk == "medium" || desc.risk == "high");
    EXPECT_TRUE(valid_risk);

    // 注册函数
    FunctionHandler handler = CreateSimpleHandler("{\"metadata\":\"complete\"}");
    bool success = client->RegisterFunction(desc, handler);
    EXPECT_TRUE(success);
}
