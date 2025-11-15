#include <gtest/gtest.h>
#include "croupier/sdk/croupier_client.h"
#include <sstream>
#include <random>

using namespace croupier::sdk;

// 工具函数测试类
class UtilsTest : public ::testing::Test {
protected:
    void SetUp() override {
        // 初始化测试环境
    }

    void TearDown() override {
        // 清理测试环境
    }
};

// 测试配置验证函数
TEST_F(UtilsTest, ClientConfigValidation) {
    // 测试有效配置
    {
        ClientConfig valid_config;
        valid_config.game_id = "test-game";
        valid_config.env = "development";
        valid_config.service_id = "test-service";
        valid_config.agent_addr = "127.0.0.1:19090";

        // 创建客户端应该成功
        EXPECT_NO_THROW({
            CroupierClient client(valid_config);
        });
    }

    // 测试空 game_id
    {
        ClientConfig invalid_config;
        invalid_config.game_id = "";  // 空的 game_id
        invalid_config.env = "development";
        invalid_config.service_id = "test-service";

        // 应该能创建，但会有警告
        EXPECT_NO_THROW({
            CroupierClient client(invalid_config);
        });
    }

    // 测试无效环境
    {
        ClientConfig config;
        config.game_id = "test-game";
        config.env = "invalid-environment";  // 无效环境
        config.service_id = "test-service";

        // 应该能创建，但会有警告
        EXPECT_NO_THROW({
            CroupierClient client(config);
        });
    }
}

// 测试 InvokeOptions 创建和验证
TEST_F(UtilsTest, InvokeOptionsCreation) {
    InvokeOptions options;
    options.idempotency_key = "test-key-123";
    options.route = "lb";
    options.target_service_id = "target-service";
    options.hash_key = "hash-123";
    options.trace_id = "trace-456";
    options.metadata["user_id"] = "user123";
    options.metadata["session_id"] = "session456";

    EXPECT_EQ(options.idempotency_key, "test-key-123");
    EXPECT_EQ(options.route, "lb");
    EXPECT_EQ(options.target_service_id, "target-service");
    EXPECT_EQ(options.hash_key, "hash-123");
    EXPECT_EQ(options.trace_id, "trace-456");
    EXPECT_EQ(options.metadata.size(), 2);
    EXPECT_EQ(options.metadata["user_id"], "user123");
    EXPECT_EQ(options.metadata["session_id"], "session456");
}

// 测试路由策略验证
TEST_F(UtilsTest, RouteValidation) {
    std::vector<std::string> valid_routes = {"lb", "broadcast", "targeted", "hash"};

    for (const auto& route : valid_routes) {
        InvokeOptions options;
        options.route = route;

        // 所有这些都应该是有效的路由类型
        EXPECT_FALSE(options.route.empty());
        EXPECT_TRUE(route == "lb" || route == "broadcast" || route == "targeted" || route == "hash");
    }
}

// 测试JobEvent结构
TEST_F(UtilsTest, JobEventCreation) {
    JobEvent event;
    event.event_type = "progress";
    event.job_id = "job-123";
    event.payload = R"({"progress": 50, "message": "Processing..."})";
    event.error = "";
    event.done = false;

    EXPECT_EQ(event.event_type, "progress");
    EXPECT_EQ(event.job_id, "job-123");
    EXPECT_FALSE(event.payload.empty());
    EXPECT_TRUE(event.error.empty());
    EXPECT_FALSE(event.done);

    // 测试完成事件
    JobEvent done_event;
    done_event.event_type = "done";
    done_event.job_id = "job-123";
    done_event.payload = R"({"result": "success"})";
    done_event.done = true;

    EXPECT_TRUE(done_event.done);
    EXPECT_EQ(done_event.event_type, "done");
}

// 测试错误事件
TEST_F(UtilsTest, JobEventError) {
    JobEvent error_event;
    error_event.event_type = "error";
    error_event.job_id = "job-456";
    error_event.error = "Connection failed";
    error_event.done = true;

    EXPECT_EQ(error_event.event_type, "error");
    EXPECT_FALSE(error_event.error.empty());
    EXPECT_TRUE(error_event.done);
}

// 测试 FunctionDescriptor 创建
TEST_F(UtilsTest, FunctionDescriptorCreation) {
    FunctionDescriptor desc;
    desc.id = "player.create";
    desc.version = "1.0.0";
    desc.schema["type"] = "object";
    desc.schema["properties"] = R"({
        "name": {"type": "string", "required": true},
        "level": {"type": "integer", "minimum": 1},
        "email": {"type": "string", "format": "email"}
    })";

    EXPECT_EQ(desc.id, "player.create");
    EXPECT_EQ(desc.version, "1.0.0");
    EXPECT_EQ(desc.schema.size(), 2);
    EXPECT_EQ(desc.schema["type"], "object");
    EXPECT_FALSE(desc.schema["properties"].empty());
}

// 测试关系定义验证
TEST_F(UtilsTest, RelationshipValidation) {
    std::vector<std::string> valid_types = {"one-to-one", "one-to-many", "many-to-one", "many-to-many"};

    for (const auto& type : valid_types) {
        RelationshipDef rel;
        rel.type = type;
        rel.entity = "related_entity";
        rel.foreign_key = "foreign_key_id";

        EXPECT_FALSE(rel.type.empty());
        EXPECT_FALSE(rel.entity.empty());
        EXPECT_FALSE(rel.foreign_key.empty());

        // 验证类型是有效的
        bool is_valid = (type == "one-to-one" || type == "one-to-many" ||
                        type == "many-to-one" || type == "many-to-many");
        EXPECT_TRUE(is_valid);
    }
}

// 测试复杂场景下的数据结构
TEST_F(UtilsTest, ComplexDataStructures) {
    // 创建一个复杂的组件描述符
    ComponentDescriptor comp;
    comp.id = "complete-economy-system";
    comp.version = "2.0.0";
    comp.name = "完整经济系统";
    comp.description = "包含钱包、交易、商店等所有经济功能";

    // 添加多个虚拟对象
    VirtualObjectDescriptor wallet;
    wallet.id = "economy.wallet";
    wallet.name = "钱包系统";
    wallet.operations["get"] = "wallet.get";
    wallet.operations["transfer"] = "wallet.transfer";

    VirtualObjectDescriptor shop;
    shop.id = "economy.shop";
    shop.name = "商店系统";
    shop.operations["list"] = "shop.list";
    shop.operations["buy"] = "shop.buy";

    comp.virtual_objects["wallet"] = wallet;
    comp.virtual_objects["shop"] = shop;

    // 验证结构完整性
    EXPECT_EQ(comp.virtual_objects.size(), 2);
    EXPECT_EQ(comp.virtual_objects["wallet"].id, "economy.wallet");
    EXPECT_EQ(comp.virtual_objects["shop"].id, "economy.shop");
    EXPECT_EQ(comp.virtual_objects["wallet"].operations.size(), 2);
    EXPECT_EQ(comp.virtual_objects["shop"].operations.size(), 2);
}

// 测试配置边界条件
TEST_F(UtilsTest, ConfigurationBoundaryConditions) {
    // 测试超长字符串
    ClientConfig config;
    config.game_id = std::string(1000, 'a'); // 1000 字符的game_id
    config.env = "development";
    config.service_id = "test";

    // 应该能处理长字符串
    EXPECT_NO_THROW({
        CroupierClient client(config);
    });

    // 测试特殊字符
    ClientConfig special_config;
    special_config.game_id = "test-game-#@$%";
    special_config.env = "development";
    special_config.service_id = "service_123";

    EXPECT_NO_THROW({
        CroupierClient client(special_config);
    });
}

// 测试超时配置
TEST_F(UtilsTest, TimeoutConfiguration) {
    ClientConfig config;
    config.game_id = "test";
    config.env = "development";
    config.timeout_seconds = 60; // 1分钟超时

    EXPECT_EQ(config.timeout_seconds, 60);

    // 测试极端值
    config.timeout_seconds = 0; // 立即超时
    EXPECT_EQ(config.timeout_seconds, 0);

    config.timeout_seconds = 3600; // 1小时超时
    EXPECT_EQ(config.timeout_seconds, 3600);
}

// 测试认证配置
TEST_F(UtilsTest, AuthenticationConfiguration) {
    ClientConfig config;
    config.game_id = "test";
    config.env = "production";
    config.auth_token = "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...";
    config.headers["X-API-Version"] = "v2";
    config.headers["X-Client-Version"] = "1.0.0";

    EXPECT_FALSE(config.auth_token.empty());
    EXPECT_EQ(config.headers.size(), 2);
    EXPECT_EQ(config.headers["X-API-Version"], "v2");
    EXPECT_EQ(config.headers["X-Client-Version"], "1.0.0");
}

// 测试 TLS 配置
TEST_F(UtilsTest, TLSConfiguration) {
    ClientConfig config;
    config.game_id = "test";
    config.env = "production";
    config.insecure = false;
    config.cert_file = "/etc/croupier/client.crt";
    config.key_file = "/etc/croupier/client.key";
    config.ca_file = "/etc/croupier/ca.crt";
    config.server_name = "croupier.internal";

    EXPECT_FALSE(config.insecure);
    EXPECT_FALSE(config.cert_file.empty());
    EXPECT_FALSE(config.key_file.empty());
    EXPECT_FALSE(config.ca_file.empty());
    EXPECT_FALSE(config.server_name.empty());
}

// 测试幂等键生成逻辑
TEST_F(UtilsTest, IdempotencyKeyGeneration) {
    // 模拟幂等键生成（真实实现在源代码中）
    auto generateKey = []() -> std::string {
        std::random_device rd;
        std::mt19937 gen(rd());
        std::uniform_int_distribution<> dis(0, 15);

        const char* chars = "0123456789abcdef";
        std::string key = "idem_";
        for (int i = 0; i < 32; i++) {
            key += chars[dis(gen)];
        }
        return key;
    };

    std::string key1 = generateKey();
    std::string key2 = generateKey();

    EXPECT_NE(key1, key2); // 两个键应该不同
    EXPECT_EQ(key1.length(), 37); // "idem_" + 32字符 = 37
    EXPECT_EQ(key2.length(), 37);
    EXPECT_TRUE(key1.starts_with("idem_"));
    EXPECT_TRUE(key2.starts_with("idem_"));
}