#include <gtest/gtest.h>
#include "croupier/sdk/croupier_client.h"
#include "croupier/sdk/config/client_config_loader.h"
#include <fstream>

using namespace croupier::sdk;
using namespace croupier::sdk::config;

// 测试夹具：客户端初始化测试
class ClientInitializationTest : public ::testing::Test {
protected:
    void SetUp() override {
        loader = std::make_unique<ClientConfigLoader>();
    }

    void TearDown() override {
        // 清理测试文件
        for (const auto& file : test_files) {
            std::remove(file.c_str());
        }
        test_files.clear();
    }

    // 辅助函数：创建临时测试文件
    std::string CreateTempFile(const std::string& filename, const std::string& content) {
        std::ofstream file(filename);
        file << content;
        file.close();
        test_files.push_back(filename);
        return filename;
    }

    // 辅助函数：创建默认配置
    ClientConfig CreateDefaultConfig() {
        return loader->CreateDefaultConfig();
    }

    // 辅助函数：创建自定义配置
    ClientConfig CreateCustomConfig() {
        ClientConfig config;
        config.game_id = "test-game";
        config.env = "testing";
        config.service_id = "test-service";
        config.agent_addr = "127.0.0.1:19090";
        config.timeout_seconds = 30;
        return config;
    }

    std::unique_ptr<ClientConfigLoader> loader;
    std::vector<std::string> test_files;
};

// ========== 测试用例 ==========

TEST_F(ClientInitializationTest, InitializeClientDefaultConfig) {
    // RED: 测试使用默认配置初始化客户端
    ClientConfig config = CreateDefaultConfig();

    // 验证默认配置已创建
    EXPECT_FALSE(config.game_id.empty());
    EXPECT_EQ(config.env, "development");
    EXPECT_EQ(config.agent_addr, "127.0.0.1:19090");

    // 创建客户端实例（不连接，仅测试构造）
    EXPECT_NO_THROW({
        CroupierClient client(config);
    });

    // 客户端应该能够成功创建
    CroupierClient client(config);

    // 初始状态应该是未连接
    EXPECT_FALSE(client.IsConnected());
}

TEST_F(ClientInitializationTest, InitializeClientCustomConfig) {
    // RED: 测试使用自定义配置初始化客户端
    ClientConfig config = CreateCustomConfig();

    // 验证自定义配置
    EXPECT_EQ(config.game_id, "test-game");
    EXPECT_EQ(config.env, "testing");
    EXPECT_EQ(config.service_id, "test-service");

    // 创建客户端实例
    EXPECT_NO_THROW({
        CroupierClient client(config);
    });

    CroupierClient client(config);

    // 初始状态应该是未连接
    EXPECT_FALSE(client.IsConnected());
}

TEST_F(ClientInitializationTest, InitializeClientConfigFile) {
    // RED: 测试从配置文件初始化客户端
    std::string config_content = R"({
        "game_id": "file-game",
        "env": "development",
        "service_id": "file-service",
        "agent_addr": "127.0.0.1:19090",
        "timeout_seconds": 45
    })";

    std::string config_file = CreateTempFile("test_client_config.json", config_content);

    // 从文件加载配置
    ClientConfig config = loader->LoadFromFile(config_file);

    // 验证配置已正确加载
    EXPECT_EQ(config.game_id, "file-game");
    EXPECT_EQ(config.service_id, "file-service");
    EXPECT_EQ(config.timeout_seconds, 45);

    // 创建客户端实例
    EXPECT_NO_THROW({
        CroupierClient client(config);
    });

    CroupierClient client(config);
    EXPECT_FALSE(client.IsConnected());
}

TEST_F(ClientInitializationTest, InitializeClientMultipleTimesProtection) {
    // RED: 测试多次初始化客户端的保护
    ClientConfig config = CreateCustomConfig();

    // 创建多个客户端实例（每个实例独立）
    CroupierClient client1(config);
    CroupierClient client2(config);

    // 两个客户端应该都是独立的实例
    EXPECT_FALSE(client1.IsConnected());
    EXPECT_FALSE(client2.IsConnected());

    // 每个客户端可以独立管理
    // 这里只是测试不会因为多个实例而崩溃
    SUCCEED();
}

TEST_F(ClientInitializationTest, InitializeClientInvalidConfig) {
    // RED: 测试无效配置初始化失败
    ClientConfig config;

    // 设置无效的 agent 地址（缺少端口）
    config.agent_addr = "invalid-address";
    config.game_id = "test-game";

    // 配置验证应该失败
    std::vector<std::string> errors = loader->ValidateConfig(config);
    EXPECT_FALSE(errors.empty());

    // 客户端仍然可以创建（验证在使用时才执行）
    CroupierClient client(config);

    // 但是连接时会失败（不实际连接，只验证逻辑）
    // 这里我们只测试客户端可以被创建
    EXPECT_FALSE(client.IsConnected());
}

TEST_F(ClientInitializationTest, InitializeClientMissingRequiredField) {
    // RED: 测试缺少必填字段
    ClientConfig config;

    // game_id 是必填字段
    config.game_id = "";
    config.env = "development";

    // 加载空 JSON
    std::string empty_json = "{}";
    config = loader->LoadFromJson(empty_json);

    // 如果 game_id 为空，验证应该失败
    if (config.game_id.empty()) {
        std::vector<std::string> errors = loader->ValidateConfig(config);
        EXPECT_FALSE(errors.empty());
        EXPECT_TRUE(std::any_of(errors.begin(), errors.end(),
            [](const std::string& err) {
                return err.find("game_id") != std::string::npos;
            }));
    } else {
        // 如果有默认 game_id，验证应该通过
        std::vector<std::string> errors = loader->ValidateConfig(config);
        EXPECT_TRUE(errors.empty());
    }

    // 客户端可以创建
    CroupierClient client(config);
    EXPECT_FALSE(client.IsConnected());
}

TEST_F(ClientInitializationTest, GetInitializationState) {
    // RED: 测试获取初始化状态
    ClientConfig config = CreateCustomConfig();

    // 创建客户端
    CroupierClient client(config);

    // 初始状态应该是未连接
    EXPECT_FALSE(client.IsConnected());

    // 获取本地地址（未连接时可能为空）
    EXPECT_FALSE(client.IsConnected());
}

TEST_F(ClientInitializationTest, IsInitializedAfterSuccess) {
    // RED: 测试成功创建后的状态
    ClientConfig config = CreateCustomConfig();

    // 创建客户端（构造成功）
    CroupierClient client(config);

    // 验证客户端已正确初始化
    EXPECT_FALSE(client.IsConnected());

    // 客户端对象存在且可用
    SUCCEED();
}

TEST_F(ClientInitializationTest, IsInitializedAfterFailure) {
    // RED: 测试失败后的状态处理
    ClientConfig config;

    // 使用无效配置
    config.game_id = "";
    config.agent_addr = "";

    // 客户端仍然可以创建（验证是延迟的）
    CroupierClient client(config);

    // 状态应该是未连接
    EXPECT_FALSE(client.IsConnected());

    // 客户端对象存在但可能处于无效状态
    SUCCEED();
}

TEST_F(ClientInitializationTest, CleanupAfterInitialization) {
    // RED: 测试初始化后的清理
    ClientConfig config = CreateCustomConfig();

    {
        // 创建客户端实例（作用域内）
        CroupierClient client(config);
        EXPECT_FALSE(client.IsConnected());

        // 显式关闭
        client.Close();
    }

    // 离开作用域，析构函数被调用
    // 应该没有内存泄漏或崩溃
    SUCCEED();
}

TEST_F(ClientInitializationTest, ReinitializeAfterCleanup) {
    // RED: 测试清理后重新初始化
    ClientConfig config = CreateCustomConfig();

    {
        // 第一个客户端实例
        CroupierClient client1(config);
        EXPECT_FALSE(client1.IsConnected());
        client1.Close();
    }

    {
        // 第二个客户端实例（重新初始化）
        CroupierClient client2(config);
        EXPECT_FALSE(client2.IsConnected());
        client2.Close();
    }

    // 两个实例应该都能正常创建和销毁
    SUCCEED();
}
