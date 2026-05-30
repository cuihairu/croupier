#include <gtest/gtest.h>
#include "croupier/sdk/config/client_config_loader.h"
#include "croupier/sdk/croupier_client.h"
#include <fstream>

using namespace croupier::sdk;
using namespace croupier::sdk::config;

// 测试夹具：配置文件和profile测试
class ConfigProfileTest : public ::testing::Test {
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

    // 辅助函数：创建基础配置文件
    std::string CreateBaseConfigFile() {
        std::string content = R"({
            "game_id": "base-game",
            "env": "development",
            "service_id": "base-service",
            "agent_addr": "127.0.0.1:19090",
            "insecure": true,
            "timeout_seconds": 30
        })";
        return CreateTempFile("base_config.json", content);
    }

    // 辅助函数：创建profile配置文件
    std::string CreateProfileConfigFile(const std::string& env) {
        std::string content;
        if (env == "development") {
            content = R"({
                "game_id": "dev-game",
                "env": "development",
                "insecure": true,
                "agent_addr": "localhost:19090"
            })";
        } else if (env == "staging") {
            content = R"({
                "game_id": "staging-game",
                "env": "staging",
                "insecure": false,
                "cert_file": "/path/to/staging/cert.pem",
                "key_file": "/path/to/staging/key.pem",
                "ca_file": "/path/to/staging/ca.pem",
                "server_name": "staging.example.com"
            })";
        } else if (env == "production") {
            content = R"({
                "game_id": "prod-game",
                "env": "production",
                "insecure": false,
                "cert_file": "/path/to/prod/cert.pem",
                "key_file": "/path/to/prod/key.pem",
                "ca_file": "/path/to/prod/ca.pem",
                "server_name": "prod.example.com",
                "auth_token": "Bearer prod-token",
                "timeout_seconds": 60
            })";
        }
        return CreateTempFile("profile_" + env + ".json", content);
    }

    std::unique_ptr<ClientConfigLoader> loader;
    std::vector<std::string> test_files;
};

// ========== 测试用例 ==========

TEST_F(ConfigProfileTest, LoadProfileDevelopment) {
    // RED: 测试加载开发环境profile
    std::string profile_file = CreateProfileConfigFile("development");

    ClientConfig config = loader->LoadFromFile(profile_file);

    EXPECT_EQ(config.game_id, "dev-game");
    EXPECT_EQ(config.env, "development");
    EXPECT_TRUE(config.insecure);
    EXPECT_EQ(config.agent_addr, "localhost:19090");
}

TEST_F(ConfigProfileTest, LoadProfileStaging) {
    // RED: 测试加载预发布环境profile
    std::string profile_file = CreateProfileConfigFile("staging");

    ClientConfig config = loader->LoadFromFile(profile_file);

    EXPECT_EQ(config.game_id, "staging-game");
    EXPECT_EQ(config.env, "staging");
    EXPECT_FALSE(config.insecure);
    EXPECT_EQ(config.cert_file, "/path/to/staging/cert.pem");
    EXPECT_EQ(config.key_file, "/path/to/staging/key.pem");
    EXPECT_EQ(config.ca_file, "/path/to/staging/ca.pem");
    EXPECT_EQ(config.server_name, "staging.example.com");
}

TEST_F(ConfigProfileTest, LoadProfileProduction) {
    // RED: 测试加载生产环境profile
    std::string profile_file = CreateProfileConfigFile("production");

    ClientConfig config = loader->LoadFromFile(profile_file);

    EXPECT_EQ(config.game_id, "prod-game");
    EXPECT_EQ(config.env, "production");
    EXPECT_FALSE(config.insecure);
    EXPECT_EQ(config.cert_file, "/path/to/prod/cert.pem");
    EXPECT_EQ(config.key_file, "/path/to/prod/key.pem");
    EXPECT_EQ(config.ca_file, "/path/to/prod/ca.pem");
    EXPECT_EQ(config.server_name, "prod.example.com");
    EXPECT_EQ(config.auth_token, "Bearer prod-token");
    EXPECT_EQ(config.timeout_seconds, 60);
}

TEST_F(ConfigProfileTest, LoadProfileWithBaseMerge) {
    // RED: 测试 base.json + profile.json 合并
    std::string base_file = CreateBaseConfigFile();
    std::string profile_file = CreateProfileConfigFile("staging");

    // 加载base配置
    ClientConfig base = loader->LoadFromFile(base_file);
    EXPECT_EQ(base.game_id, "base-game");
    EXPECT_EQ(base.env, "development");

    // 加载profile配置
    ClientConfig profile = loader->LoadFromFile(profile_file);

    // 合并配置（profile覆盖base）
    ClientConfig merged = loader->MergeConfigs(base, profile);

    // profile的值应该覆盖base
    EXPECT_EQ(merged.game_id, "staging-game");
    EXPECT_EQ(merged.env, "staging");
    EXPECT_FALSE(merged.insecure);

    // base中独有的值应该保留
    EXPECT_EQ(merged.service_id, "base-service");

    // profile中的安全配置应该应用
    EXPECT_EQ(merged.cert_file, "/path/to/staging/cert.pem");
    EXPECT_EQ(merged.server_name, "staging.example.com");
}

TEST_F(ConfigProfileTest, LoadProfileCustomPath) {
    // RED: 测试自定义配置文件路径
    std::string custom_path = "custom_profile.json";
    std::string content = R"({
        "game_id": "custom-game",
        "env": "custom",
        "agent_addr": "192.168.1.1:8080"
    })";

    CreateTempFile(custom_path, content);

    ClientConfig config = loader->LoadFromFile(custom_path);

    EXPECT_EQ(config.game_id, "custom-game");
    EXPECT_EQ(config.env, "custom");
    EXPECT_EQ(config.agent_addr, "192.168.1.1:8080");
}

TEST_F(ConfigProfileTest, LoadProfileNotFound) {
    // RED: 测试profile文件不存在时抛出异常
    std::string nonexistent_file = "nonexistent_profile.json";

    EXPECT_THROW(
        loader->LoadFromFile(nonexistent_file),
        std::runtime_error
    );
}

TEST_F(ConfigProfileTest, LoadProfileInvalidJson) {
    // RED: 测试无效JSON格式
    std::string invalid_file = "invalid_profile.json";
    std::string invalid_content = R"({
        "game_id": "test-game",
        "env": "development",
        "invalid_json": [
    })";  // 缺少闭合括号

    CreateTempFile(invalid_file, invalid_content);

    EXPECT_THROW(
        loader->LoadFromFile(invalid_file),
        std::runtime_error
    );
}

TEST_F(ConfigProfileTest, LoadProfileOverrideDefaults) {
    // RED: 测试profile覆盖默认配置值
    // 先创建一个带默认值的配置
    std::string default_content = R"({
        "game_id": "default-game",
        "env": "development",
        "timeout_seconds": 30,
        "auto_reconnect": true,
        "reconnect_interval_seconds": 5,
        "reconnect_max_attempts": 0
    })";
    std::string default_file = CreateTempFile("default_config.json", default_content);

    // 加载默认配置
    ClientConfig default_config = loader->LoadFromFile(default_file);
    EXPECT_EQ(default_config.timeout_seconds, 30);
    EXPECT_TRUE(default_config.auto_reconnect);

    // 创建profile配置覆盖部分默认值
    std::string profile_content = R"({
        "game_id": "profile-game",
        "timeout_seconds": 90,
        "auto_reconnect": false,
        "reconnect_max_attempts": 10
    })";
    std::string profile_file = CreateTempFile("override_profile.json", profile_content);

    // 加载profile配置
    ClientConfig profile_config = loader->LoadFromFile(profile_file);

    // 合并配置
    ClientConfig merged = loader->MergeConfigs(default_config, profile_config);

    // 验证profile覆盖了默认值
    EXPECT_EQ(merged.game_id, "profile-game");
    EXPECT_EQ(merged.timeout_seconds, 90);  // 被覆盖
    EXPECT_FALSE(merged.auto_reconnect);  // 布尔值被覆盖
    EXPECT_EQ(merged.reconnect_interval_seconds, 5);  // 保留默认值
    EXPECT_EQ(merged.reconnect_max_attempts, 10);  // 被覆盖
}
