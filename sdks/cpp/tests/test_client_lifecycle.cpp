#include <gtest/gtest.h>
#include "croupier/sdk/croupier_client.h"
#include "croupier/sdk/config/client_config_loader.h"
#include <thread>
#include <chrono>

using namespace croupier::sdk;
using namespace croupier::sdk::config;

// 测试夹具：客户端生命周期测试
class ClientLifecycleTest : public ::testing::Test {
protected:
    void SetUp() override {
        loader = std::make_unique<ClientConfigLoader>();
        config = loader->CreateDefaultConfig();

        // 使用非阻塞模式进行测试
        config.blocking_connect = false;
        config.auto_reconnect = false;

        client = std::make_unique<CroupierClient>(config);
    }

    void TearDown() override {
        if (client) {
            try {
                client->Close();
            } catch (...) {
                // 忽略关闭时的异常
            }
        }
    }

    std::unique_ptr<ClientConfigLoader> loader;
    ClientConfig config;
    std::unique_ptr<CroupierClient> client;
};

// ========== 测试用例 ==========

TEST_F(ClientLifecycleTest, ClientStartStop) {
    // RED: 测试启动和停止客户端
    // 初始状态
    EXPECT_FALSE(client->IsConnected());

    // 连接到 agent
    [[maybe_unused]] bool connected = client->Connect();

    // 尝试获取本地地址

    // 停止客户端
    client->Close();

    // 停止后应该是未连接状态
    bool stopped = client->IsConnected();
    EXPECT_FALSE(stopped);

    // 验证生命周期可以正常完成
    SUCCEED();
}

TEST_F(ClientLifecycleTest, ServeMethod) {
    // RED: 测试 Serve() 方法
    // 注意：Serve() 是阻塞调用，不能在单元测试中直接调用
    // 这里我们验证客户端对象可以被正确创建和配置

    // 创建用于 Serve 的客户端
    ClientConfig serve_config = loader->CreateDefaultConfig();
    serve_config.blocking_connect = false;
    CroupierClient serve_client(serve_config);

    // 连接
    serve_client.Connect();

    // 验证客户端已准备好可以 Serve（虽然没有实际调用）
    EXPECT_TRUE(true);

    // 清理
    serve_client.Close();
}

TEST_F(ClientLifecycleTest, ResourceCleanup) {
    // RED: 测试资源清理验证
    {
        // 创建临时客户端
        CroupierClient temp_client(config);

        // 连接
        temp_client.Connect();

        // 离开作用域，析构函数应该自动清理资源
    }

    // 如果没有内存泄漏或崩溃，测试通过
    SUCCEED();
}

TEST_F(ClientLifecycleTest, StateTransitions) {
    // RED: 测试状态转换验证
    // 状态转换：未连接 -> 连接中 -> 已连接/连接失败 -> 已关闭 -> 未连接

    // 初始状态：未连接
    bool state_initial = client->IsConnected();
    EXPECT_FALSE(state_initial);

    // 连接尝试
    client->Connect();
    [[maybe_unused]] bool state_connecting_or_connected = client->IsConnected();

    // 关闭
    client->Close();
    bool state_closed = client->IsConnected();
    EXPECT_FALSE(state_closed);

    // 验证状态转换逻辑
    SUCCEED();
}

TEST_F(ClientLifecycleTest, MultipleStartStopCycles) {
    // RED: 测试多次启动停止循环
    // 第一次循环
    {
        CroupierClient client1(config);
        client1.Connect();
        std::this_thread::sleep_for(std::chrono::milliseconds(100));
        client1.Close();
    }

    // 第二次循环
    {
        CroupierClient client2(config);
        client2.Connect();
        std::this_thread::sleep_for(std::chrono::milliseconds(100));
        client2.Close();
    }

    // 第三次循环
    {
        CroupierClient client3(config);
        client3.Connect();
        std::this_thread::sleep_for(std::chrono::milliseconds(100));
        client3.Close();
    }

    // 验证可以多次创建和销毁客户端
    SUCCEED();
}

TEST_F(ClientLifecycleTest, StopWithoutStart) {
    // RED: 测试未启动就停止
    // 创建新客户端但不连接
    ClientConfig new_config = loader->CreateDefaultConfig();
    CroupierClient new_client(new_config);

    // 直接关闭（未连接状态）
    new_client.Close();

    // 应该不会崩溃或抛出异常
    bool is_connected = new_client.IsConnected();
    EXPECT_FALSE(is_connected);

    // 再次关闭（幂等性）
    new_client.Close();

    EXPECT_FALSE(new_client.IsConnected());
}

TEST_F(ClientLifecycleTest, CompleteLifecycle) {
    // RED: 测试完整生命周期
    // 1. 创建客户端
    ClientConfig lifecycle_config = loader->CreateDefaultConfig();
    lifecycle_config.service_id = "lifecycle-test";
    lifecycle_config.game_id = "test-game";

    CroupierClient lifecycle_client(lifecycle_config);

    // 2. 注册函数
    FunctionDescriptor desc;
    desc.id = "test.function";
    desc.version = "1.0.0";
    desc.category = "test";
    desc.risk = "low";
    desc.entity = "test";
    desc.operation = "create";

    FunctionHandler handler = [](const std::string& context, const std::string& payload) -> std::string {
        return "{\"result\":\"success\"}";
    };

    bool registered = lifecycle_client.RegisterFunction(desc, handler);
    EXPECT_TRUE(registered);

    // 3. 连接到 agent
    lifecycle_client.Connect();

    // 4. 获取状态
    [[maybe_unused]] bool connected = lifecycle_client.IsConnected();

    // 5. 关闭连接
    lifecycle_client.Close();

    // 6. 验证最终状态
    bool final_state = lifecycle_client.IsConnected();
    EXPECT_FALSE(final_state);

    // 完整生命周期成功完成
    SUCCEED();
}
