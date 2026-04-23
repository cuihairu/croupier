#include <gtest/gtest.h>
#include "croupier/sdk/config/client_config_loader.h"
#include "croupier/sdk/croupier_client.h"
#include <fstream>
#include <cstdio>

using namespace croupier::sdk;
using namespace croupier::sdk::config;

// 测试夹具：环境变量覆盖测试
class ConfigEnvironmentTest : public ::testing::Test {
protected:
    void SetUp() override {
        loader = std::make_unique<ClientConfigLoader>();
    }

    void TearDown() override {
        // 清理所有测试设置的环境变量
        UnsetEnv("CROUPIER_GAME_ID");
        UnsetEnv("CROUPIER_ENV");
        UnsetEnv("CROUPIER_SERVICE_ID");
        UnsetEnv("CROUPIER_AGENT_ADDR");
        UnsetEnv("CROUPIER_AUTH_TOKEN");
        UnsetEnv("CROUPIER_INSECURE");
        UnsetEnv("CROUPIER_AUTO_RECONNECT");
        UnsetEnv("CROUPIER_TIMEOUT_SECONDS");
        UnsetEnv("CROUPIER_RECONNECT_INTERVAL_SECONDS");
        UnsetEnv("CROUPIER_RECONNECT_MAX_ATTEMPTS");
        UnsetEnv("CUSTOM_PREFIX_GAME_ID");

        // 清理测试文件
        for (const auto& file : test_files) {
            std::remove(file.c_str());
        }
        test_files.clear();
    }

    // 辅助函数：创建临时测试文件
    std::string CreateTempFile(const std::string& content) {
        std::string filename = "test_env_config_" + std::to_string(test_files.size()) + ".json";
        std::ofstream file(filename);
        file << content;
        file.close();
        test_files.push_back(filename);
        return filename;
    }

    // 辅助函数：设置环境变量
    void SetEnv(const std::string& name, const std::string& value) {
#ifdef _WIN32
        _putenv_s(name.c_str(), value.c_str());
#else
        setenv(name.c_str(), value.c_str(), 1);
#endif
    }

    // 辅助函数：取消设置环境变量
    void UnsetEnv(const std::string& name) {
#ifdef _WIN32
        _putenv_s(name.c_str(), "");
#else
        unsetenv(name.c_str());
#endif
    }

    std::unique_ptr<ClientConfigLoader> loader;
    std::vector<std::string> test_files;
};

// ========== 测试用例 ==========

TEST_F(ConfigEnvironmentTest, LoadWithEnvironmentOverrideGameId) {
    // RED: 测试环境变量覆盖 game_id
    std::string base_config = R"({
        "game_id": "base-game",
        "env": "development"
    })";

    std::string config_file = CreateTempFile(base_config);

    // 设置环境变量
    SetEnv("CROUPIER_GAME_ID", "env-game-override");

    // 加载配置，应该被环境变量覆盖
    ClientConfig config = loader->LoadWithEnvironmentOverrides(config_file, "CROUPIER_");

    EXPECT_EQ(config.game_id, "env-game-override");
    // 其他字段保持原值
    EXPECT_EQ(config.env, "development");
}

TEST_F(ConfigEnvironmentTest, LoadWithEnvironmentOverrideEnv) {
    // RED: 测试环境变量覆盖 env
    std::string base_config = R"({
        "game_id": "test-game",
        "env": "development"
    })";

    std::string config_file = CreateTempFile(base_config);

    // 设置环境变量
    SetEnv("CROUPIER_ENV", "production");

    ClientConfig config = loader->LoadWithEnvironmentOverrides(config_file, "CROUPIER_");

    EXPECT_EQ(config.env, "production");
    EXPECT_EQ(config.game_id, "test-game");
}

TEST_F(ConfigEnvironmentTest, LoadWithEnvironmentOverrideAgentAddr) {
    // RED: 测试环境变量覆盖 agent_addr
    std::string base_config = R"({
        "game_id": "test-game",
        "agent_addr": "127.0.0.1:19090"
    })";

    std::string config_file = CreateTempFile(base_config);

    // 设置环境变量
    SetEnv("CROUPIER_AGENT_ADDR", "192.168.1.100:8080");

    ClientConfig config = loader->LoadWithEnvironmentOverrides(config_file, "CROUPIER_");

    EXPECT_EQ(config.agent_addr, "192.168.1.100:8080");
}

TEST_F(ConfigEnvironmentTest, LoadWithEnvironmentOverrideAuthToken) {
    // RED: 测试环境变量覆盖 auth_token
    std::string base_config = R"({
        "game_id": "test-game",
        "auth_token": "base-token"
    })";

    std::string config_file = CreateTempFile(base_config);

    // 设置环境变量
    SetEnv("CROUPIER_AUTH_TOKEN", "Bearer env-token-xyz");

    ClientConfig config = loader->LoadWithEnvironmentOverrides(config_file, "CROUPIER_");

    EXPECT_EQ(config.auth_token, "Bearer env-token-xyz");
}

TEST_F(ConfigEnvironmentTest, LoadWithEnvironmentOverrideMultiple) {
    // RED: 测试多个环境变量同时覆盖
    std::string base_config = R"({
        "game_id": "base-game",
        "env": "development",
        "service_id": "base-service",
        "agent_addr": "127.0.0.1:19090"
    })";

    std::string config_file = CreateTempFile(base_config);

    // 设置多个环境变量
    SetEnv("CROUPIER_GAME_ID", "multi-override-game");
    SetEnv("CROUPIER_ENV", "staging");
    SetEnv("CROUPIER_AGENT_ADDR", "10.0.0.1:9999");

    ClientConfig config = loader->LoadWithEnvironmentOverrides(config_file, "CROUPIER_");

    EXPECT_EQ(config.game_id, "multi-override-game");
    EXPECT_EQ(config.env, "staging");
    EXPECT_EQ(config.agent_addr, "10.0.0.1:9999");
    // 未设置的保持原值
    EXPECT_EQ(config.service_id, "base-service");
}

TEST_F(ConfigEnvironmentTest, LoadWithEnvironmentOverrideCustomPrefix) {
    // RED: 测试自定义环境变量前缀
    std::string base_config = R"({
        "game_id": "base-game",
        "env": "development"
    })";

    std::string config_file = CreateTempFile(base_config);

    // 使用自定义前缀设置环境变量
    SetEnv("CUSTOM_PREFIX_GAME_ID", "custom-prefix-game");

    ClientConfig config = loader->LoadWithEnvironmentOverrides(config_file, "CUSTOM_PREFIX_");

    EXPECT_EQ(config.game_id, "custom-prefix-game");
}

TEST_F(ConfigEnvironmentTest, LoadWithEnvironmentOverrideEmpty) {
    // RED: 测试环境变量为空时使用配置文件值
    std::string base_config = R"({
        "game_id": "config-file-game",
        "env": "production"
    })";

    std::string config_file = CreateTempFile(base_config);

    // 不设置任何环境变量，应该使用配置文件的值
    ClientConfig config = loader->LoadWithEnvironmentOverrides(config_file, "CROUPIER_");

    EXPECT_EQ(config.game_id, "config-file-game");
    EXPECT_EQ(config.env, "production");
}

TEST_F(ConfigEnvironmentTest, LoadWithEnvironmentOverrideBooleanInsecure) {
    // RED: 测试布尔环境变量覆盖 - insecure
    std::string base_config = R"({
        "game_id": "test-game",
        "insecure": false
    })";

    std::string config_file = CreateTempFile(base_config);

    // 设置为 true
    SetEnv("CROUPIER_INSECURE", "true");

    ClientConfig config = loader->LoadWithEnvironmentOverrides(config_file, "CROUPIER_");

    EXPECT_TRUE(config.insecure);
}

TEST_F(ConfigEnvironmentTest, LoadWithEnvironmentOverrideBooleanAutoReconnect) {
    // RED: 测试布尔环境变量覆盖 - auto_reconnect
    std::string base_config = R"({
        "game_id": "test-game",
        "auto_reconnect": false
    })";

    std::string config_file = CreateTempFile(base_config);

    // 设置为 "1"
    SetEnv("CROUPIER_AUTO_RECONNECT", "1");

    ClientConfig config = loader->LoadWithEnvironmentOverrides(config_file, "CROUPIER_");

    EXPECT_TRUE(config.auto_reconnect);
}

TEST_F(ConfigEnvironmentTest, LoadWithEnvironmentOverrideNumericTimeout) {
    // RED: 测试数字环境变量覆盖 - timeout_seconds
    std::string base_config = R"({
        "game_id": "test-game",
        "timeout_seconds": 30
    })";

    std::string config_file = CreateTempFile(base_config);

    // 设置超时为 60 秒
    SetEnv("CROUPIER_TIMEOUT_SECONDS", "60");

    ClientConfig config = loader->LoadWithEnvironmentOverrides(config_file, "CROUPIER_");

    EXPECT_EQ(config.timeout_seconds, 60);
}

TEST_F(ConfigEnvironmentTest, LoadWithEnvironmentOverrideReconnectConfig) {
    // RED: 测试重连配置环境变量覆盖
    std::string base_config = R"({
        "game_id": "test-game",
        "reconnect_interval_seconds": 5,
        "reconnect_max_attempts": 0
    })";

    std::string config_file = CreateTempFile(base_config);

    // 设置重连配置
    SetEnv("CROUPIER_RECONNECT_INTERVAL_SECONDS", "10");
    SetEnv("CROUPIER_RECONNECT_MAX_ATTEMPTS", "5");

    ClientConfig config = loader->LoadWithEnvironmentOverrides(config_file, "CROUPIER_");

    EXPECT_EQ(config.reconnect_interval_seconds, 10);
    EXPECT_EQ(config.reconnect_max_attempts, 5);
}

TEST_F(ConfigEnvironmentTest, LoadWithEnvironmentOverrideBooleanVariants) {
    // RED: 测试布尔值的各种变体
    std::string base_config = R"({
        "game_id": "test-game",
        "insecure": false,
        "auto_reconnect": false
    })";

    std::string config_file = CreateTempFile(base_config);

    // 测试 "yes"
    SetEnv("CROUPIER_INSECURE", "yes");
    // 测试 "True" (大小写混合)
    SetEnv("CROUPIER_AUTO_RECONNECT", "True");

    ClientConfig config = loader->LoadWithEnvironmentOverrides(config_file, "CROUPIER_");

    EXPECT_TRUE(config.insecure);
    EXPECT_TRUE(config.auto_reconnect);
}

TEST_F(ConfigEnvironmentTest, LoadWithEnvironmentOverrideInvalidNumeric) {
    // RED: 测试无效数字环境变量时保持原值
    std::string base_config = R"({
        "game_id": "test-game",
        "timeout_seconds": 30
    })";

    std::string config_file = CreateTempFile(base_config);

    // 设置无效数字
    SetEnv("CROUPIER_TIMEOUT_SECONDS", "not-a-number");

    // 应该保持原值，并输出错误信息
    ClientConfig config = loader->LoadWithEnvironmentOverrides(config_file, "CROUPIER_");

    EXPECT_EQ(config.timeout_seconds, 30); // 保持原值
}

TEST_F(ConfigEnvironmentTest, LoadWithEnvironmentOverrideSecurityConfig) {
    // RED: 测试安全配置环境变量覆盖
    std::string base_config = R"({
        "game_id": "test-game",
        "cert_file": "/path/to/cert.pem",
        "key_file": "/path/to/key.pem"
    })";

    std::string config_file = CreateTempFile(base_config);

    // 设置安全配置环境变量
    SetEnv("CROUPIER_CERT_FILE", "/env/path/cert.pem");
    SetEnv("CROUPIER_KEY_FILE", "/env/path/key.pem");
    SetEnv("CROUPIER_CA_FILE", "/env/path/ca.pem");

    ClientConfig config = loader->LoadWithEnvironmentOverrides(config_file, "CROUPIER_");

    EXPECT_EQ(config.cert_file, "/env/path/cert.pem");
    EXPECT_EQ(config.key_file, "/env/path/key.pem");
    EXPECT_EQ(config.ca_file, "/env/path/ca.pem");
}
