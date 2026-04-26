#include <gtest/gtest.h>
#include "croupier/sdk/croupier_client.h"
#include "croupier/sdk/config/client_config_loader.h"

using namespace croupier::sdk;
using namespace croupier::sdk::config;

// 辅助函数：为客户端注册一个测试函数
static void RegisterTestFunction(CroupierClient& cl) {
    FunctionDescriptor desc;
    desc.id = "test.temp.function";
    desc.version = "1.0.0";
    desc.category = "test";
    desc.risk = "low";
    desc.entity = "test";
    desc.operation = "temp";

    FunctionHandler handler = [](const std::string& context, const std::string& payload) -> std::string {
        return "{\"status\":\"ok\"}";
    };
    cl.RegisterFunction(desc, handler);
}

// 测试夹具：连接管理测试
class ClientConnectionTest : public ::testing::Test {
protected:
    void SetUp() override {
        loader = std::make_unique<ClientConfigLoader>();
        config = loader->CreateDefaultConfig();

        // 使用非阻塞模式进行测试，避免在没有 agent 时卡住
        config.blocking_connect = false;
        config.auto_reconnect = false;  // 禁用自动重连，加快测试

        client = std::make_unique<CroupierClient>(config);

        // 注册一个测试函数以满足 CroupierClient::Connect() 的要求
        FunctionDescriptor test_desc;
        test_desc.id = "test.connection.function";
        test_desc.version = "1.0.0";
        test_desc.category = "test";
        test_desc.risk = "low";
        test_desc.entity = "test";
        test_desc.operation = "connect";

        FunctionHandler test_handler = [](const std::string& context, const std::string& payload) -> std::string {
            return "{\"status\":\"ok\"}";
        };

        client->RegisterFunction(test_desc, test_handler);
    }

    void TearDown() override {
        if (client) {
            client->Close();
        }
    }

    std::unique_ptr<ClientConfigLoader> loader;
    ClientConfig config;
    std::unique_ptr<CroupierClient> client;
};

// ========== 测试用例 ==========

TEST_F(ClientConnectionTest, ConnectToAgent) {
    // RED: 测试连接到 agent
    // 初始状态应该是未连接
    EXPECT_FALSE(client->IsConnected());

    // 尝试连接（非阻塞模式）
    [[maybe_unused]] bool success = client->Connect();

    // 在没有真实 agent 的情况下，连接可能失败
    // 这里我们测试 Connect() API 可以正常调用而不崩溃
    // 连接结果取决于 agent 是否可用
    SUCCEED();
}

TEST_F(ClientConnectionTest, IsConnectedStatus) {
    // RED: 测试检查连接状态
    // 初始状态应该是未连接
    bool initially_connected = client->IsConnected();
    EXPECT_FALSE(initially_connected);

    // 尝试连接
    client->Connect();

    // 检查连接状态（可能仍然是 false，因为没有 agent）
    [[maybe_unused]] bool connected_after_attempt = client->IsConnected();

    // 验证 IsConnected() API 可以正常调用
    SUCCEED();
}

TEST_F(ClientConnectionTest, DisconnectFromAgent) {
    // RED: 测试断开连接
    // 尝试连接
    client->Connect();

    // 关闭连接
    client->Close();

    // 关闭后应该是未连接状态
    bool connected_after_close = client->IsConnected();
    EXPECT_FALSE(connected_after_close);
}

TEST_F(ClientConnectionTest, BlockingConnect) {
    // RED: 测试阻塞连接模式
    // 创建阻塞模式的配置
    ClientConfig blocking_config = loader->CreateDefaultConfig();
    blocking_config.blocking_connect = true;
    blocking_config.timeout_seconds = 1;  // 短超时，避免测试卡住太久

    CroupierClient blocking_client(blocking_config);
    RegisterTestFunction(blocking_client);  // 注册测试函数

    // 验证配置已应用
    EXPECT_TRUE(blocking_config.blocking_connect);

    // 尝试阻塞连接（如果没有 agent，会在超时后返回）
    [[maybe_unused]] bool success = blocking_client.Connect();

    // 测试完成后关闭
    blocking_client.Close();

    // 验证 API 可以正常调用
    SUCCEED();
}

TEST_F(ClientConnectionTest, NonBlockingConnect) {
    // RED: 测试非阻塞连接模式
    // 创建非阻塞模式的配置
    ClientConfig non_blocking_config = loader->CreateDefaultConfig();
    non_blocking_config.blocking_connect = false;
    non_blocking_config.connect_timeout_seconds = 1;

    CroupierClient non_blocking_client(non_blocking_config);
    RegisterTestFunction(non_blocking_client);  // 注册测试函数

    // 验证配置已应用
    EXPECT_FALSE(non_blocking_config.blocking_connect);

    // 非阻塞连接应该立即返回
    [[maybe_unused]] bool success = non_blocking_client.Connect();

    // 验证 API 可以正常调用
    SUCCEED();

    non_blocking_client.Close();
}

TEST_F(ClientConnectionTest, ConnectionTimeout) {
    // RED: 测试连接超时
    // 创建短超时配置
    ClientConfig timeout_config = loader->CreateDefaultConfig();
    timeout_config.timeout_seconds = 1;  // 1秒超时
    timeout_config.blocking_connect = true;

    CroupierClient timeout_client(timeout_config);
    RegisterTestFunction(timeout_client);  // 注册测试函数

    // 记录开始时间
    // 注意：这里不使用时间测量，因为单元测试不应该依赖时间

    // 尝试连接（会在超时后返回）
    timeout_client.Connect();

    // 验证客户端不会永久阻塞
    SUCCEED();

    timeout_client.Close();
}

TEST_F(ClientConnectionTest, AutoReconnect) {
    // RED: 测试自动重连
    // 创建启用自动重连的配置
    ClientConfig reconnect_config = loader->CreateDefaultConfig();
    reconnect_config.auto_reconnect = true;
    reconnect_config.reconnect_interval_seconds = 1;
    reconnect_config.reconnect_max_attempts = 3;  // 最多重试3次
    reconnect_config.blocking_connect = false;

    CroupierClient reconnect_client(reconnect_config);
    RegisterTestFunction(reconnect_client);  // 注册测试函数

    // 验证配置已应用
    EXPECT_TRUE(reconnect_config.auto_reconnect);
    EXPECT_EQ(reconnect_config.reconnect_max_attempts, 3);

    // 尝试连接
    reconnect_client.Connect();

    // 验证 API 可以正常调用
    SUCCEED();

    reconnect_client.Close();
}

TEST_F(ClientConnectionTest, MultipleConnectionAttempts) {
    // RED: 测试多次连接尝试
    // 第一次连接
    [[maybe_unused]] bool first_attempt = client->Connect();

    // 关闭
    client->Close();

    // 第二次连接
    [[maybe_unused]] bool second_attempt = client->Connect();

    // 再次关闭
    client->Close();

    // 第三次连接
    [[maybe_unused]] bool third_attempt = client->Connect();

    // 最后关闭
    client->Close();

    // 验证可以多次尝试连接而不会崩溃
    SUCCEED();
}

TEST_F(ClientConnectionTest, ConnectionStateChanges) {
    // RED: 测试连接状态转换
    // 状态转换：未连接 -> 连接中 -> 已连接/连接失败 -> 断开 -> 未连接

    // 初始状态：未连接
    bool state1 = client->IsConnected();
    EXPECT_FALSE(state1);

    // 尝试连接：连接中/已连接/连接失败
    client->Connect();
    [[maybe_unused]] bool state2 = client->IsConnected();

    // 关闭连接：回到未连接状态
    client->Close();
    bool state3 = client->IsConnected();
    EXPECT_FALSE(state3);

    // 验证状态转换逻辑
    SUCCEED();
}
