#include <gtest/gtest.h>
#include "croupier/sdk/config/client_config_loader.h"
#include "croupier/sdk/croupier_client.h"
#include <fstream>
#include <cstdio>

using namespace croupier::sdk;
using namespace croupier::sdk::config;

// 测试夹具：Config Loading Tests
class ConfigLoadingTest : public ::testing::Test {
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
    std::string CreateTempFile(const std::string& content) {
        std::string filename = "test_config_" + std::to_string(test_files.size()) + ".json";
        std::ofstream file(filename);
        file << content;
        file.close();
        test_files.push_back(filename);
        return filename;
    }

    std::unique_ptr<ClientConfigLoader> loader;
    std::vector<std::string> test_files;
};

// ========== 测试用例 ==========

TEST_F(ConfigLoadingTest, LoadFromFileValid) {
    // RED: 测试加载有效的配置文件
    std::string valid_config = R"({
        "game_id": "test-game",
        "env": "development",
        "service_id": "test-service",
        "agent_addr": "127.0.0.1:19090"
    })";

    std::string config_file = CreateTempFile(valid_config);

    // 应该成功加载，不抛出异常
    EXPECT_NO_THROW({
        ClientConfig config = loader->LoadFromFile(config_file);
        EXPECT_EQ(config.game_id, "test-game");
        EXPECT_EQ(config.env, "development");
        EXPECT_EQ(config.service_id, "test-service");
        EXPECT_EQ(config.agent_addr, "127.0.0.1:19090");
    });
}

TEST_F(ConfigLoadingTest, LoadFromFileNotFound) {
    // RED: 测试文件不存在的情况
    std::string non_existent_file = "non_existent_config_12345.json";

    // 应该抛出异常
    EXPECT_THROW({
        loader->LoadFromFile(non_existent_file);
    }, std::runtime_error);
}

TEST_F(ConfigLoadingTest, LoadFromFileInvalidJson) {
    // RED: 测试无效的 JSON 格式
    std::string invalid_json = "{ this is not valid json }";
    std::string config_file = CreateTempFile(invalid_json);

    // 应该抛出异常
    EXPECT_THROW({
        loader->LoadFromFile(config_file);
    }, std::runtime_error);
}

TEST_F(ConfigLoadingTest, LoadFromFileEmptyJson) {
    // RED: 测试空的 JSON 对象
    std::string empty_json = "{}";
    std::string config_file = CreateTempFile(empty_json);

    // 应该成功加载（使用默认值）
    EXPECT_NO_THROW({
        ClientConfig config = loader->LoadFromFile(config_file);
        // 验证默认值
        EXPECT_EQ(config.agent_addr, "127.0.0.1:19090");
        EXPECT_EQ(config.service_id, "cpp-service");
        EXPECT_EQ(config.env, "development");
    });
}

TEST_F(ConfigLoadingTest, LoadFromFilePartialConfig) {
    // RED: 测试部分配置
    std::string partial_config = R"({
        "game_id": "partial-game",
        "env": "staging"
    })";

    std::string config_file = CreateTempFile(partial_config);

    EXPECT_NO_THROW({
        ClientConfig config = loader->LoadFromFile(config_file);
        EXPECT_EQ(config.game_id, "partial-game");
        EXPECT_EQ(config.env, "staging");
        // 其他字段应该使用默认值
        EXPECT_EQ(config.service_id, "cpp-service");
    });
}

TEST_F(ConfigLoadingTest, LoadFromJsonStringValid) {
    // RED: 测试从 JSON 字符串加载
    std::string json_content = R"({
        "game_id": "json-game",
        "service_id": "json-service",
        "agent_addr": "192.168.1.1:8080"
    })";

    EXPECT_NO_THROW({
        ClientConfig config = loader->LoadFromJson(json_content);
        EXPECT_EQ(config.game_id, "json-game");
        EXPECT_EQ(config.service_id, "json-service");
        EXPECT_EQ(config.agent_addr, "192.168.1.1:8080");
    });
}

TEST_F(ConfigLoadingTest, LoadFromJsonStringInvalid) {
    // RED: 测试无效的 JSON 字符串
    std::string invalid_json = "{ invalid }";

    EXPECT_THROW({
        loader->LoadFromJson(invalid_json);
    }, std::runtime_error);
}
