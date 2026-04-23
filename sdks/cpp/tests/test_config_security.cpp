#include <gtest/gtest.h>
#include "croupier/sdk/config/client_config_loader.h"
#include "croupier/sdk/croupier_client.h"
#include <fstream>

using namespace croupier::sdk;
using namespace croupier::sdk::config;

// 测试夹具：安全配置验证测试
class ConfigSecurityTest : public ::testing::Test {
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

        // 清理 TLS 测试文件
        std::remove("test_cert.pem");
        std::remove("test_key.pem");
        std::remove("test_ca.pem");
    }

    // 辅助函数：创建临时测试文件
    std::string CreateTempFile(const std::string& content) {
        std::string filename = "test_security_config_" + std::to_string(test_files.size()) + ".json";
        std::ofstream file(filename);
        file << content;
        file.close();
        test_files.push_back(filename);
        return filename;
    }

    // 辅助函数：创建测试 TLS 证书文件
    void CreateTestTLSFiles() {
        std::ofstream cert("test_cert.pem");
        cert << "-----BEGIN CERTIFICATE-----\n";
        cert << "MIIC9jCCAd4CCQD2rKLXz0BqczANBgkqhkiG9w0BAQsFADANBgkqhkiG9w0BAQsF\n";
        cert << "-----END CERTIFICATE-----\n";
        cert.close();

        std::ofstream key("test_key.pem");
        key << "-----BEGIN PRIVATE KEY-----\n";
        key << "MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBC0wggKJAgEAAoIBAQD\n";
        key << "-----END PRIVATE KEY-----\n";
        key.close();

        std::ofstream ca("test_ca.pem");
        ca << "-----BEGIN CERTIFICATE-----\n";
        ca << "MIIC9jCCAd4CCQD2rKLXz0BqczANBgkqhkiG9w0BAQsFADANBgkqhkiG9w0BAQsF\n";
        ca << "-----END CERTIFICATE-----\n";
        ca.close();
    }

    std::unique_ptr<ClientConfigLoader> loader;
    std::vector<std::string> test_files;
};

// ========== 测试用例 ==========

TEST_F(ConfigSecurityTest, ValidateTlsConfigValid) {
    // RED: 测试有效的 TLS 配置（文件存在）
    CreateTestTLSFiles();

    std::string config_json = R"({
        "game_id": "test-game",
        "insecure": false,
        "cert_file": "test_cert.pem",
        "key_file": "test_key.pem",
        "ca_file": "test_ca.pem",
        "server_name": "example.com"
    })";

    ClientConfig config = loader->LoadFromJson(config_json);
    std::vector<std::string> errors = loader->ValidateConfig(config);

    // 应该没有错误（文件都存在）
    EXPECT_TRUE(errors.empty()) << "Expected no errors, but got: " << (errors.empty() ? "" : errors[0]);
}

TEST_F(ConfigSecurityTest, ValidateTlsConfigMissingCertFile) {
    // RED: 测试缺少证书文件
    std::string config_json = R"({
        "game_id": "test-game",
        "insecure": false,
        "cert_file": "",
        "key_file": "test_key.pem"
    })";

    ClientConfig config = loader->LoadFromJson(config_json);
    std::vector<std::string> errors = loader->ValidateConfig(config);

    // 应该有错误：cert_file is required
    EXPECT_FALSE(errors.empty());
    EXPECT_TRUE(std::any_of(errors.begin(), errors.end(),
        [](const std::string& err) { return err.find("cert_file is required") != std::string::npos; }));
}

TEST_F(ConfigSecurityTest, ValidateTlsConfigCertFileNotFound) {
    // RED: 测试证书文件不存在
    std::string config_json = R"({
        "game_id": "test-game",
        "insecure": false,
        "cert_file": "nonexistent_cert.pem",
        "key_file": "test_key.pem",
        "ca_file": "test_ca.pem"
    })";

    ClientConfig config = loader->LoadFromJson(config_json);
    std::vector<std::string> errors = loader->ValidateConfig(config);

    // 应该有错误：cert_file does not exist
    EXPECT_FALSE(errors.empty());
    EXPECT_TRUE(std::any_of(errors.begin(), errors.end(),
        [](const std::string& err) { return err.find("cert_file does not exist") != std::string::npos; }));
}

TEST_F(ConfigSecurityTest, ValidateTlsConfigMissingKeyFile) {
    // RED: 测试缺少密钥文件
    CreateTestTLSFiles();

    std::string config_json = R"({
        "game_id": "test-game",
        "insecure": false,
        "cert_file": "test_cert.pem",
        "key_file": "",
        "ca_file": "test_ca.pem"
    })";

    ClientConfig config = loader->LoadFromJson(config_json);
    std::vector<std::string> errors = loader->ValidateConfig(config);

    // 应该有错误：key_file is required
    EXPECT_FALSE(errors.empty());
    EXPECT_TRUE(std::any_of(errors.begin(), errors.end(),
        [](const std::string& err) { return err.find("key_file is required") != std::string::npos; }));
}

TEST_F(ConfigSecurityTest, ValidateTlsConfigKeyFileNotFound) {
    // RED: 测试密钥文件不存在
    CreateTestTLSFiles();

    std::string config_json = R"({
        "game_id": "test-game",
        "insecure": false,
        "cert_file": "test_cert.pem",
        "key_file": "nonexistent_key.pem",
        "ca_file": "test_ca.pem"
    })";

    ClientConfig config = loader->LoadFromJson(config_json);
    std::vector<std::string> errors = loader->ValidateConfig(config);

    // 应该有错误：key_file does not exist
    EXPECT_FALSE(errors.empty());
    EXPECT_TRUE(std::any_of(errors.begin(), errors.end(),
        [](const std::string& err) { return err.find("key_file does not exist") != std::string::npos; }));
}

TEST_F(ConfigSecurityTest, ValidateTlsConfigMissingCaFile) {
    // RED: 测试缺少 CA 文件
    CreateTestTLSFiles();

    std::string config_json = R"({
        "game_id": "test-game",
        "insecure": false,
        "cert_file": "test_cert.pem",
        "key_file": "test_key.pem",
        "ca_file": ""
    })";

    ClientConfig config = loader->LoadFromJson(config_json);
    std::vector<std::string> errors = loader->ValidateConfig(config);

    // 应该有错误：ca_file is required
    EXPECT_FALSE(errors.empty());
    EXPECT_TRUE(std::any_of(errors.begin(), errors.end(),
        [](const std::string& err) { return err.find("ca_file is required") != std::string::npos; }));
}

TEST_F(ConfigSecurityTest, ValidateTlsConfigCaFileNotFound) {
    // RED: 测试 CA 文件不存在
    CreateTestTLSFiles();

    std::string config_json = R"({
        "game_id": "test-game",
        "insecure": false,
        "cert_file": "test_cert.pem",
        "key_file": "test_key.pem",
        "ca_file": "nonexistent_ca.pem"
    })";

    ClientConfig config = loader->LoadFromJson(config_json);
    std::vector<std::string> errors = loader->ValidateConfig(config);

    // 应该有错误：ca_file does not exist
    EXPECT_FALSE(errors.empty());
    EXPECT_TRUE(std::any_of(errors.begin(), errors.end(),
        [](const std::string& err) { return err.find("ca_file does not exist") != std::string::npos; }));
}

TEST_F(ConfigSecurityTest, ValidateTlsConfigWithServerName) {
    // RED: 测试 TLS 配置带 server_name
    CreateTestTLSFiles();

    std::string config_json = R"({
        "game_id": "test-game",
        "insecure": false,
        "cert_file": "test_cert.pem",
        "key_file": "test_key.pem",
        "ca_file": "test_ca.pem",
        "server_name": "croupier.example.com"
    })";

    ClientConfig config = loader->LoadFromJson(config_json);
    std::vector<std::string> errors = loader->ValidateConfig(config);

    // 应该没有错误
    EXPECT_TRUE(errors.empty());
}

TEST_F(ConfigSecurityTest, ValidateTlsConfigMissingServerName) {
    // RED: 测试缺少 server_name
    CreateTestTLSFiles();

    std::string config_json = R"({
        "game_id": "test-game",
        "insecure": false,
        "cert_file": "test_cert.pem",
        "key_file": "test_key.pem",
        "ca_file": "test_ca.pem",
        "server_name": ""
    })";

    ClientConfig config = loader->LoadFromJson(config_json);
    std::vector<std::string> errors = loader->ValidateConfig(config);

    // 应该有错误：server_name is required
    EXPECT_FALSE(errors.empty());
    EXPECT_TRUE(std::any_of(errors.begin(), errors.end(),
        [](const std::string& err) { return err.find("server_name is required") != std::string::npos; }));
}

TEST_F(ConfigSecurityTest, ValidateTlsConfigInsecureFlag) {
    // RED: 测试 insecure=true 时不需要 TLS 配置
    std::string config_json = R"({
        "game_id": "test-game",
        "insecure": true
    })";

    ClientConfig config = loader->LoadFromJson(config_json);
    std::vector<std::string> errors = loader->ValidateConfig(config);

    // insecure=true 时，不需要 TLS 配置，应该没有错误
    EXPECT_TRUE(errors.empty());
}

TEST_F(ConfigSecurityTest, ValidateAuthConfigValid) {
    // RED: 测试有效的认证配置
    std::string config_json = R"({
        "game_id": "test-game",
        "auth_token": "Bearer my-jwt-token-12345"
    })";

    ClientConfig config = loader->LoadFromJson(config_json);
    std::vector<std::string> errors = loader->ValidateConfig(config);

    // 应该没有错误
    EXPECT_TRUE(errors.empty());
}

TEST_F(ConfigSecurityTest, ValidateAuthTokenWithBearerPrefix) {
    // RED: 测试 auth_token 带 Bearer 前缀
    std::string config_json = R"({
        "game_id": "test-game",
        "auth_token": "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"
    })";

    ClientConfig config = loader->LoadFromJson(config_json);
    std::vector<std::string> errors = loader->ValidateConfig(config);

    // 应该没有错误
    EXPECT_TRUE(errors.empty());
    EXPECT_EQ(config.auth_token, "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9");
}

TEST_F(ConfigSecurityTest, ValidateAuthTokenWithoutBearerPrefix) {
    // RED: 测试 auth_token 不带 Bearer 前缀
    std::string config_json = R"({
        "game_id": "test-game",
        "auth_token": "my-token-without-bearer"
    })";

    ClientConfig config = loader->LoadFromJson(config_json);
    std::vector<std::string> errors = loader->ValidateConfig(config);

    // 应该有错误：auth_token should start with 'Bearer '
    EXPECT_FALSE(errors.empty());
    EXPECT_TRUE(std::any_of(errors.begin(), errors.end(),
        [](const std::string& err) { return err.find("auth_token should start with 'Bearer '") != std::string::npos; }));
}

TEST_F(ConfigSecurityTest, ValidateAuthTokenEmpty) {
    // RED: 测试空 auth_token（允许）
    std::string config_json = R"({
        "game_id": "test-game",
        "auth_token": ""
    })";

    ClientConfig config = loader->LoadFromJson(config_json);
    std::vector<std::string> errors = loader->ValidateConfig(config);

    // 空 auth_token 是允许的，不应该有错误
    EXPECT_TRUE(errors.empty());
}

TEST_F(ConfigSecurityTest, ValidateAuthConfigWithHeaders) {
    // RED: 测试认证配置带 headers
    std::string config_json = R"({
        "game_id": "test-game",
        "auth_token": "Bearer token123",
        "headers": {
            "X-Custom-Header": "custom-value",
            "X-Game-Version": "1.0.0"
        }
    })";

    ClientConfig config = loader->LoadFromJson(config_json);
    std::vector<std::string> errors = loader->ValidateConfig(config);

    // 应该没有错误
    EXPECT_TRUE(errors.empty());
    EXPECT_EQ(config.headers.size(), 2U);
}

TEST_F(ConfigSecurityTest, ValidateSecurityConfigProduction) {
    // RED: 测试生产环境安全配置
    CreateTestTLSFiles();

    std::string config_json = R"({
        "game_id": "prod-game",
        "env": "production",
        "insecure": false,
        "cert_file": "test_cert.pem",
        "key_file": "test_key.pem",
        "ca_file": "test_ca.pem",
        "server_name": "prod.croupier.io",
        "auth_token": "Bearer prod-token-xyz"
    })";

    ClientConfig config = loader->LoadFromJson(config_json);
    std::vector<std::string> errors = loader->ValidateConfig(config);

    // 生产环境应该有完整的安全配置
    EXPECT_TRUE(errors.empty());
    EXPECT_FALSE(config.insecure);
    EXPECT_FALSE(config.auth_token.empty());
}

TEST_F(ConfigSecurityTest, ValidateSecurityConfigDevelopment) {
    // RED: 测试开发环境可以使用 insecure=true
    std::string config_json = R"({
        "game_id": "dev-game",
        "env": "development",
        "insecure": true
    })";

    ClientConfig config = loader->LoadFromJson(config_json);
    std::vector<std::string> errors = loader->ValidateConfig(config);

    // 开发环境允许 insecure=true
    EXPECT_TRUE(errors.empty());
    EXPECT_TRUE(config.insecure);
}

TEST_F(ConfigSecurityTest, ValidateSecurityConfigStaging) {
    // RED: 测试预发布环境应该使用 TLS
    CreateTestTLSFiles();

    std::string config_json = R"({
        "game_id": "staging-game",
        "env": "staging",
        "insecure": false,
        "cert_file": "test_cert.pem",
        "key_file": "test_key.pem",
        "ca_file": "test_ca.pem",
        "server_name": "staging.croupier.io"
    })";

    ClientConfig config = loader->LoadFromJson(config_json);
    std::vector<std::string> errors = loader->ValidateConfig(config);

    // 预发布环境应该有 TLS
    EXPECT_TRUE(errors.empty());
    EXPECT_EQ(config.env, "staging");
}

TEST_F(ConfigSecurityTest, ValidateSecurityConfigPartial) {
    // RED: 测试部分 TLS 配置（缺少某些文件）
    CreateTestTLSFiles();

    // 有 cert 和 key，但缺少 ca
    std::string config_json = R"({
        "game_id": "test-game",
        "insecure": false,
        "cert_file": "test_cert.pem",
        "key_file": "test_key.pem",
        "ca_file": "",
        "server_name": "example.com"
    })";

    ClientConfig config = loader->LoadFromJson(config_json);
    std::vector<std::string> errors = loader->ValidateConfig(config);

    // 应该有错误：缺少 ca_file
    EXPECT_FALSE(errors.empty());
    EXPECT_TRUE(std::any_of(errors.begin(), errors.end(),
        [](const std::string& err) { return err.find("ca_file") != std::string::npos; }));
}
