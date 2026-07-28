#include <gtest/gtest.h>
#include "croupier/sdk/croupier_client.h"
#include <algorithm>
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
    EXPECT_EQ(options.metadata.size(), 2U);
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

// 测试TaskEvent结构
TEST_F(UtilsTest, TaskEventCreation) {
    TaskEvent event;
    event.event_type = "progress";
    event.task_id = "job-123";
    event.payload = R"({"progress": 50, "message": "Processing..."})";
    event.error = "";
    event.done = false;

    EXPECT_EQ(event.event_type, "progress");
    EXPECT_EQ(event.task_id, "job-123");
    EXPECT_FALSE(event.payload.empty());
    EXPECT_TRUE(event.error.empty());
    EXPECT_FALSE(event.done);

    // 测试完成事件
    TaskEvent done_event;
    done_event.event_type = "done";
    done_event.task_id = "job-123";
    done_event.payload = R"({"result": "success"})";
    done_event.done = true;

    EXPECT_TRUE(done_event.done);
    EXPECT_EQ(done_event.event_type, "done");
}

// 测试错误事件
TEST_F(UtilsTest, TaskEventError) {
    TaskEvent error_event;
    error_event.event_type = "error";
    error_event.task_id = "job-456";
    error_event.error = "Connection failed";
    error_event.done = true;

    EXPECT_EQ(error_event.event_type, "error");
    EXPECT_FALSE(error_event.error.empty());
    EXPECT_TRUE(error_event.done);
}

// 测试 FunctionDescriptor 创建
TEST_F(UtilsTest, FunctionDescriptorCreation) {
    FunctionDescriptor desc;
    desc.id = "player.ban";
    desc.version = "1.0.0";
    desc.resource = "player";
    desc.operation = "ban";
    desc.risk = "danger";
    desc.enabled = true;
    desc.permission = "player.ban";

    EXPECT_EQ(desc.id, "player.ban");
    EXPECT_EQ(desc.version, "1.0.0");
    EXPECT_EQ(desc.resource, "player");
    EXPECT_EQ(desc.operation, "ban");
    EXPECT_EQ(desc.risk, "danger");
    EXPECT_TRUE(desc.enabled);
    EXPECT_EQ(desc.permission, "player.ban");
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
    EXPECT_EQ(config.headers.size(), 2U);
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
    std::string key1 = utils::NewIdempotencyKey();
    std::string key2 = utils::NewIdempotencyKey();

    auto is_hex = [](unsigned char c) {
        return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f');
    };

    EXPECT_NE(key1, key2);
    EXPECT_EQ(key1.size(), 32U);
    EXPECT_EQ(key2.size(), 32U);
    EXPECT_TRUE(std::all_of(key1.begin(), key1.end(), is_hex));
    EXPECT_TRUE(std::all_of(key2.begin(), key2.end(), is_hex));
}

TEST_F(UtilsTest, InvokerRequiresAddress) {
    InvokerConfig config;
    config.address = "";
    config.retry.enabled = false;

    CroupierInvoker invoker(config);
    EXPECT_FALSE(invoker.Connect());
    invoker.Close();
}

TEST_F(UtilsTest, InvokerStartTaskStreamsCompletedEvent) {
    InvokerConfig config;
    config.address = "http://127.0.0.1:8080";
    config.retry.enabled = false;

    CroupierInvoker invoker(config);
    ASSERT_TRUE(invoker.Connect());

    const std::string task_id = invoker.StartTask("wallet.get", R"({"player_id":"u1"})");
    EXPECT_FALSE(task_id.empty());

    const auto events = invoker.StreamTask(task_id).get();
    ASSERT_GE(events.size(), 3U);
    EXPECT_EQ(events.front().event_type, "started");
    EXPECT_EQ(events[1].event_type, "progress");
    EXPECT_EQ(events.back().event_type, "completed");
    EXPECT_TRUE(events.back().done);
    EXPECT_NE(events.back().payload.find("\"function_id\":\"wallet.get\""), std::string::npos);

    invoker.Close();
}

TEST_F(UtilsTest, InvokerCancelTaskEmitsCancelledEvent) {
    InvokerConfig config;
    config.address = "http://127.0.0.1:8080";
    config.retry.enabled = false;

    CroupierInvoker invoker(config);
    ASSERT_TRUE(invoker.Connect());

    const std::string task_id = invoker.StartTask("wallet.transfer", R"({"amount":10})");
    ASSERT_FALSE(task_id.empty());
    EXPECT_TRUE(invoker.CancelTask(task_id));

    const auto events = invoker.StreamTask(task_id).get();
    ASSERT_FALSE(events.empty());
    EXPECT_EQ(events.back().event_type, "cancelled");
    EXPECT_TRUE(events.back().done);
    EXPECT_FALSE(invoker.CancelTask(task_id));

    invoker.Close();
}
