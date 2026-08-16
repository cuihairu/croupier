#include <gtest/gtest.h>
#include "croupier/sdk/config/client_config_loader.h"
#include "croupier/sdk/croupier_client.h"

using namespace croupier::sdk;
using namespace croupier::sdk::config;

// 测试夹具：默认配置测试
class ConfigDefaultsTest : public ::testing::Test {
protected:
    void SetUp() override {
        loader = std::make_unique<ClientConfigLoader>();
    }

    void TearDown() override {
        // 清理
    }

    std::unique_ptr<ClientConfigLoader> loader;
};

// ========== 测试用例 ==========

TEST_F(ConfigDefaultsTest, CreateDefaultConfig) {
    // RED: 测试创建默认配置对象
    ClientConfig config = loader->CreateDefaultConfig();

    // 验证配置对象成功创建
    EXPECT_FALSE(config.game_id.empty());
    EXPECT_FALSE(config.env.empty());
}

TEST_F(ConfigDefaultsTest, DefaultConfigValues) {
    // RED: 测试所有字段的默认值
    ClientConfig config = loader->CreateDefaultConfig();

    // 基本字段默认值
    EXPECT_EQ(config.env, "development");
    EXPECT_TRUE(config.insecure);

    // 网络配置默认值
    EXPECT_EQ(config.agent_addr, "127.0.0.1:19091");
    EXPECT_EQ(config.timeout_seconds, 30);
    EXPECT_EQ(config.heartbeat_interval, 60);

    // 重连配置默认值
    EXPECT_TRUE(config.auto_reconnect);
    EXPECT_EQ(config.reconnect_interval_seconds, 5);
    EXPECT_EQ(config.reconnect_max_attempts, 0);  // 0 表示无限重试

    // 提供者信息默认值
    EXPECT_EQ(config.provider_lang, "cpp");
    EXPECT_EQ(config.provider_sdk, "croupier-cpp-sdk");

    // 本地监听地址默认值
}

TEST_F(ConfigDefaultsTest, DefaultConfigWithRequiredFields) {
    // RED: 测试必填字段验证 - game_id 是必需的
    ClientConfig config = loader->CreateDefaultConfig();

    // 默认配置应该有 game_id
    EXPECT_FALSE(config.game_id.empty());

    // 验证配置有效性
    std::vector<std::string> errors = loader->ValidateConfig(config);

    // 默认配置应该是有效的
    EXPECT_TRUE(errors.empty()) << "Default config should be valid, but got errors: "
                                << (errors.empty() ? "" : errors[0]);
}

TEST_F(ConfigDefaultsTest, DefaultConfigOptionalFields) {
    // RED: 测试可选字段的默认值
    ClientConfig config = loader->CreateDefaultConfig();

    // 可选字段应该有合理的默认值
    EXPECT_FALSE(config.service_id.empty());  // service_id 有默认值
    EXPECT_FALSE(config.provider_lang.empty());
    EXPECT_FALSE(config.provider_sdk.empty());

    // headers 默认为空
    EXPECT_TRUE(config.headers.empty());

    // 认证 token 默认为空
    EXPECT_TRUE(config.auth_token.empty());

    // TLS 配置默认为空（development 模式）
    EXPECT_TRUE(config.cert_file.empty());
    EXPECT_TRUE(config.key_file.empty());
    EXPECT_TRUE(config.ca_file.empty());
    EXPECT_TRUE(config.server_name.empty());
}

TEST_F(ConfigDefaultsTest, DefaultConfigNetworkingDefaults) {
    // RED: 测试网络配置的默认值
    ClientConfig config = loader->CreateDefaultConfig();

    // Agent 地址
    EXPECT_EQ(config.agent_addr, "127.0.0.1:19091");

    // 超时配置
    EXPECT_EQ(config.timeout_seconds, 30);
    EXPECT_GT(config.timeout_seconds, 0);

    // 心跳间隔
    EXPECT_EQ(config.heartbeat_interval, 60);
    EXPECT_GT(config.heartbeat_interval, 0);

    // 本地监听地址
}

TEST_F(ConfigDefaultsTest, DefaultConfigSecurityDefaults) {
    // RED: 测试安全配置的默认值
    ClientConfig config = loader->CreateDefaultConfig();

    // 开发环境默认不使用 TLS
    EXPECT_TRUE(config.insecure);

    // TLS 文件默认为空
    EXPECT_TRUE(config.cert_file.empty());
    EXPECT_TRUE(config.key_file.empty());
    EXPECT_TRUE(config.ca_file.empty());
    EXPECT_TRUE(config.server_name.empty());

    // 认证 token 默认为空
    EXPECT_TRUE(config.auth_token.empty());

    // 验证开发环境配置应该是有效的
    std::vector<std::string> errors = loader->ValidateConfig(config);
    EXPECT_TRUE(errors.empty());
}

TEST_F(ConfigDefaultsTest, DefaultConfigReconnectDefaults) {
    // RED: 测试重连配置的默认值
    ClientConfig config = loader->CreateDefaultConfig();

    // 自动重连默认开启
    EXPECT_TRUE(config.auto_reconnect);

    // 重连间隔
    EXPECT_EQ(config.reconnect_interval_seconds, 5);
    EXPECT_GT(config.reconnect_interval_seconds, 0);

    // 最大重连次数（0 表示无限重试）
    EXPECT_EQ(config.reconnect_max_attempts, 0);
}

TEST_F(ConfigDefaultsTest, DefaultConfigEmptyToDefault) {
    // RED: 测试空配置使用默认值
    std::string empty_json = "{}";

    ClientConfig config = loader->LoadFromJson(empty_json);

    // 应该填充默认值
    EXPECT_EQ(config.env, "development");
    EXPECT_TRUE(config.insecure);
    EXPECT_EQ(config.provider_lang, "cpp");
    EXPECT_EQ(config.provider_sdk, "croupier-cpp-sdk");

    // game_id 应该有默认值（可能是一个生成的ID）
    EXPECT_FALSE(config.game_id.empty());
}

TEST_F(ConfigDefaultsTest, DefaultConfigPartialFields) {
    // RED: 测试部分字段使用默认值
    std::string partial_json = R"({
        "game_id": "test-game",
        "timeout_seconds": 90
    })";

    ClientConfig config = loader->LoadFromJson(partial_json);

    // 显式设置的字段
    EXPECT_EQ(config.game_id, "test-game");
    EXPECT_EQ(config.timeout_seconds, 90);

    // 其他字段应该使用默认值
    EXPECT_EQ(config.env, "development");
    EXPECT_TRUE(config.insecure);
    EXPECT_EQ(config.agent_addr, "127.0.0.1:19091");
    EXPECT_EQ(config.provider_lang, "cpp");

    // 重连配置默认值
    EXPECT_TRUE(config.auto_reconnect);
    EXPECT_EQ(config.reconnect_interval_seconds, 5);
    EXPECT_EQ(config.reconnect_max_attempts, 0);
}

TEST_F(ConfigDefaultsTest, ValidateConfigMissingRequiredField) {
    // RED: 测试缺少必填字段的验证
    std::string invalid_json = R"({
        "env": "development"
    })";  // 缺少 game_id

    ClientConfig config = loader->LoadFromJson(invalid_json);

    // 应该加载成功（game_id 可能有默认值）
    // 但如果 game_id 为空，验证应该失败
    if (config.game_id.empty()) {
        std::vector<std::string> errors = loader->ValidateConfig(config);
        EXPECT_FALSE(errors.empty());
        EXPECT_TRUE(std::any_of(errors.begin(), errors.end(),
            [](const std::string& err) {
                return err.find("game_id") != std::string::npos &&
                       (err.find("required") != std::string::npos ||
                        err.find("must not be empty") != std::string::npos);
            }));
    } else {
        // 如果有默认 game_id，验证应该通过
        std::vector<std::string> errors = loader->ValidateConfig(config);
        EXPECT_TRUE(errors.empty());
    }
}
