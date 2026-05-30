#include <gtest/gtest.h>
#include "croupier/sdk/config/client_config_loader.h"
#include "croupier/sdk/croupier_client.h"

using namespace croupier::sdk;
using namespace croupier::sdk::config;

// 测试夹具：配置合并测试
class ConfigMergeTest : public ::testing::Test {
protected:
    void SetUp() override {
        loader = std::make_unique<ClientConfigLoader>();
    }

    void TearDown() override {
        // 清理
    }

    // 辅助函数：创建基础配置
    ClientConfig CreateBaseConfig() {
        ClientConfig config;
        config.game_id = "base-game";
        config.env = "development";
        config.service_id = "base-service";
        config.agent_addr = "127.0.0.1:19090";
        config.timeout_seconds = 30;
        config.insecure = true;
        return config;
    }

    // 辅助函数：创建覆盖配置
    ClientConfig CreateOverlayConfig() {
        ClientConfig config;
        config.game_id = "overlay-game";
        config.env = "production";
        config.agent_addr = "192.168.1.1:8080";
        config.timeout_seconds = 60;
        config.insecure = false;
        return config;
    }

    std::unique_ptr<ClientConfigLoader> loader;
};

// ========== 测试用例 ==========

TEST_F(ConfigMergeTest, MergeConfigsBasic) {
    // RED: 测试基本配置合并
    ClientConfig base = CreateBaseConfig();
    ClientConfig overlay = CreateOverlayConfig();

    ClientConfig result = loader->MergeConfigs(base, overlay);

    // overlay 的值应该覆盖 base
    EXPECT_EQ(result.game_id, "overlay-game");
    EXPECT_EQ(result.env, "production");
    EXPECT_EQ(result.agent_addr, "192.168.1.1:8080");
    EXPECT_EQ(result.timeout_seconds, 60);
    EXPECT_FALSE(result.insecure);  // 布尔值总是应用

    // base 中独有的值应该保留
    EXPECT_EQ(result.service_id, "base-service");
}

TEST_F(ConfigMergeTest, MergeConfigsOverlayPriority) {
    // RED: 测试 overlay 优先级
    ClientConfig base;
    base.game_id = "base-game";
    base.env = "development";
    base.service_id = "base-service";
    base.agent_addr = "127.0.0.1:19090";

    ClientConfig overlay;
    overlay.game_id = "overlay-game";  // 覆盖
    overlay.env = "";  // 空，不覆盖
    overlay.service_id = "";  // 空，不覆盖
    overlay.agent_addr = "10.0.0.1:9999";  // 覆盖

    ClientConfig result = loader->MergeConfigs(base, overlay);

    EXPECT_EQ(result.game_id, "overlay-game");  // 被覆盖
    EXPECT_EQ(result.env, "development");  // 保留 base 值
    EXPECT_EQ(result.service_id, "base-service");  // 保留 base 值
    EXPECT_EQ(result.agent_addr, "10.0.0.1:9999");  // 被覆盖
}

TEST_F(ConfigMergeTest, MergeConfigsPartialOverride) {
    // RED: 测试部分覆盖
    ClientConfig base;
    base.game_id = "base-game";
    base.env = "development";
    base.service_id = "base-service";
    base.agent_addr = "127.0.0.1:19090";
    base.timeout_seconds = 30;

    ClientConfig overlay;
    overlay.game_id = "overlay-game";  // 只覆盖部分字段
    overlay.timeout_seconds = 60;

    ClientConfig result = loader->MergeConfigs(base, overlay);

    EXPECT_EQ(result.game_id, "overlay-game");  // 被覆盖
    EXPECT_EQ(result.env, "development");  // 保留 base 值
    EXPECT_EQ(result.service_id, "base-service");  // 保留 base 值
    EXPECT_EQ(result.agent_addr, "127.0.0.1:19090");  // 保留 base 值
    EXPECT_EQ(result.timeout_seconds, 60);  // 被覆盖
}

TEST_F(ConfigMergeTest, MergeConfigsEmptyBase) {
    // RED: 测试空 base 配置
    ClientConfig base;  // 默认构造，所有字段为空或默认值
    base.service_id = "base-service";  // 设置一个字段

    ClientConfig overlay;
    overlay.game_id = "overlay-game";
    overlay.env = "production";
    overlay.agent_addr = "192.168.1.1:8080";

    ClientConfig result = loader->MergeConfigs(base, overlay);

    // overlay 的值应该应用
    EXPECT_EQ(result.game_id, "overlay-game");
    EXPECT_EQ(result.env, "production");
    EXPECT_EQ(result.agent_addr, "192.168.1.1:8080");
    EXPECT_EQ(result.service_id, "base-service");  // base 的值保留
}

TEST_F(ConfigMergeTest, MergeConfigsEmptyOverlay) {
    // RED: 测试空 overlay 配置
    ClientConfig base = CreateBaseConfig();

    ClientConfig overlay;  // 空 overlay

    ClientConfig result = loader->MergeConfigs(base, overlay);

    // 应该完全使用 base 的值
    EXPECT_EQ(result.game_id, "base-game");
    EXPECT_EQ(result.env, "development");
    EXPECT_EQ(result.service_id, "base-service");
    EXPECT_EQ(result.agent_addr, "127.0.0.1:19090");
    EXPECT_EQ(result.timeout_seconds, 30);
}

TEST_F(ConfigMergeTest, MergeConfigsHeaders) {
    // RED: 测试 headers 合并
    ClientConfig base;
    base.game_id = "base-game";
    base.headers["X-Base-Header"] = "base-value";
    base.headers["X-Common-Header"] = "common-base-value";

    ClientConfig overlay;
    overlay.game_id = "overlay-game";
    overlay.headers["X-Overlay-Header"] = "overlay-value";
    overlay.headers["X-Common-Header"] = "common-overlay-value";  // 覆盖 base 的值

    ClientConfig result = loader->MergeConfigs(base, overlay);

    // 检查 game_id
    EXPECT_EQ(result.game_id, "overlay-game");

    // 检查 headers
    EXPECT_EQ(result.headers.size(), 3U);
    EXPECT_EQ(result.headers["X-Base-Header"], "base-value");  // base 的保留
    EXPECT_EQ(result.headers["X-Overlay-Header"], "overlay-value");  // overlay 的添加
    EXPECT_EQ(result.headers["X-Common-Header"], "common-overlay-value");  // overlay 覆盖 base
}

TEST_F(ConfigMergeTest, MergeConfigsSecurityConfig) {
    // RED: 测试安全配置合并
    ClientConfig base;
    base.game_id = "base-game";
    base.cert_file = "/base/cert.pem";
    base.key_file = "/base/key.pem";
    base.ca_file = "/base/ca.pem";
    base.server_name = "base.example.com";
    base.insecure = true;

    ClientConfig overlay;
    overlay.game_id = "overlay-game";
    overlay.cert_file = "/overlay/cert.pem";  // 覆盖
    overlay.key_file = "";  // 空，不覆盖
    overlay.ca_file = "/overlay/ca.pem";  // 覆盖
    overlay.server_name = "";  // 空，不覆盖
    overlay.insecure = false;  // 布尔值总是应用

    ClientConfig result = loader->MergeConfigs(base, overlay);

    EXPECT_EQ(result.game_id, "overlay-game");
    EXPECT_EQ(result.cert_file, "/overlay/cert.pem");  // 被覆盖
    EXPECT_EQ(result.key_file, "/base/key.pem");  // 保留 base 值
    EXPECT_EQ(result.ca_file, "/overlay/ca.pem");  // 被覆盖
    EXPECT_EQ(result.server_name, "base.example.com");  // 保留 base 值
    EXPECT_FALSE(result.insecure);  // 布尔值总是被应用
}

TEST_F(ConfigMergeTest, MergeConfigsAuthConfig) {
    // RED: 测试认证配置合并
    ClientConfig base;
    base.game_id = "base-game";
    base.auth_token = "Bearer base-token";
    base.provider_lang = "cpp";
    base.provider_sdk = "croupier-cpp-sdk";

    ClientConfig overlay;
    overlay.game_id = "overlay-game";
    overlay.auth_token = "Bearer overlay-token";  // 覆盖
    overlay.provider_lang = "";  // 空，不覆盖
    overlay.provider_sdk = "";  // 空，不覆盖

    ClientConfig result = loader->MergeConfigs(base, overlay);

    EXPECT_EQ(result.game_id, "overlay-game");
    EXPECT_EQ(result.auth_token, "Bearer overlay-token");  // 被覆盖
    EXPECT_EQ(result.provider_lang, "cpp");  // 保留 base 值
    EXPECT_EQ(result.provider_sdk, "croupier-cpp-sdk");  // 保留 base 值
}

TEST_F(ConfigMergeTest, MergeConfigsComplexTypes) {
    // RED: 测试复杂类型合并
    ClientConfig base;
    base.game_id = "base-game";
    base.timeout_seconds = 30;
    base.auto_reconnect = true;
    base.reconnect_interval_seconds = 5;
    base.reconnect_max_attempts = 0;
    base.headers["X-Base"] = "base-value";

    ClientConfig overlay;
    overlay.game_id = "overlay-game";
    overlay.timeout_seconds = 60;  // 覆盖（> 0）
    overlay.timeout_seconds = 0;  // 不覆盖（= 0）
    overlay.auto_reconnect = false;  // 布尔值总是应用
    overlay.headers["X-Overlay"] = "overlay-value";

    ClientConfig result = loader->MergeConfigs(base, overlay);

    EXPECT_EQ(result.game_id, "overlay-game");
    EXPECT_EQ(result.timeout_seconds, 30);  // 0 不覆盖，保留 base 的 30
    EXPECT_FALSE(result.auto_reconnect);  // 布尔值被应用
    EXPECT_EQ(result.reconnect_interval_seconds, 5);  // 保留 base 值
    EXPECT_EQ(result.reconnect_max_attempts, 0);  // 保留 base 值

    // 检查 headers
    EXPECT_EQ(result.headers.size(), 2U);
    EXPECT_EQ(result.headers["X-Base"], "base-value");
    EXPECT_EQ(result.headers["X-Overlay"], "overlay-value");
}

TEST_F(ConfigMergeTest, MergeConfigsAllEmptyFields) {
    // RED: 测试所有字段都为空的情况
    ClientConfig base;  // 所有字段都是默认值/空
    ClientConfig overlay;  // 所有字段都是默认值/空

    ClientConfig result = loader->MergeConfigs(base, overlay);

    // 结果应该也是默认值
    EXPECT_TRUE(result.game_id.empty());
    EXPECT_TRUE(result.env.empty() || result.env == "development");  // 可能有默认值
    EXPECT_TRUE(result.agent_addr.empty() || result.agent_addr == "127.0.0.1:19090");  // 可能有默认值
}

TEST_F(ConfigMergeTest, MergeConfigsNumericValues) {
    // RED: 测试数值型配置合并
    ClientConfig base;
    base.game_id = "base-game";
    base.timeout_seconds = 30;
    base.heartbeat_interval = 60;
    base.reconnect_interval_seconds = 5;
    base.reconnect_max_attempts = 10;

    ClientConfig overlay;
    overlay.game_id = "overlay-game";
    overlay.timeout_seconds = 90;  // > 0，应该覆盖
    overlay.timeout_seconds = 0;  // = 0，不覆盖
    overlay.heartbeat_interval = 120;  // 覆盖
    overlay.reconnect_interval_seconds = 0;  // = 0，不覆盖
    overlay.reconnect_max_attempts = 5;  // 覆盖

    ClientConfig result = loader->MergeConfigs(base, overlay);

    EXPECT_EQ(result.game_id, "overlay-game");
    EXPECT_EQ(result.timeout_seconds, 30);  // 0 不覆盖，保留 30
    EXPECT_EQ(result.heartbeat_interval, 120);  // 被覆盖
    EXPECT_EQ(result.reconnect_interval_seconds, 5);  // 0 不覆盖，保留 5
    EXPECT_EQ(result.reconnect_max_attempts, 5);  // 被覆盖
}

TEST_F(ConfigMergeTest, MergeConfigsMultipleOverlays) {
    // RED: 测试多次合并（链式合并）
    ClientConfig base;
    base.game_id = "base-game";
    base.env = "development";
    base.timeout_seconds = 30;

    // 第一次合并
    ClientConfig overlay1;
    overlay1.game_id = "overlay1-game";
    overlay1.agent_addr = "192.168.1.1:8080";
    overlay1.timeout_seconds = 60;

    ClientConfig result1 = loader->MergeConfigs(base, overlay1);
    EXPECT_EQ(result1.game_id, "overlay1-game");
    EXPECT_EQ(result1.env, "development");  // 保留 base
    EXPECT_EQ(result1.agent_addr, "192.168.1.1:8080");  // 来自 overlay1
    EXPECT_EQ(result1.timeout_seconds, 60);  // 来自 overlay1

    // 第二次合并（基于第一次的结果）
    ClientConfig overlay2;
    overlay2.agent_addr = result1.agent_addr;  // 显式保留第一次的结果
    overlay2.env = "production";  // 覆盖
    overlay2.timeout_seconds = 90;  // 覆盖上次的 60

    ClientConfig result2 = loader->MergeConfigs(result1, overlay2);
    EXPECT_EQ(result2.game_id, "overlay1-game");  // 保留第一次的结果
    EXPECT_EQ(result2.env, "production");  // 被覆盖
    EXPECT_EQ(result2.agent_addr, "192.168.1.1:8080");  // 保留第一次的结果
    EXPECT_EQ(result2.timeout_seconds, 90);  // 被第二次覆盖
}

TEST_F(ConfigMergeTest, MergeConfigsPreservesBaseDefaults) {
    // RED: 测试合并保留 base 的默认值
    ClientConfig base = CreateBaseConfig();  // 有完整的配置
    base.headers["X-Default"] = "default-value";

    ClientConfig overlay;
    overlay.game_id = "overlay-game";  // 只覆盖一个字段

    ClientConfig result = loader->MergeConfigs(base, overlay);

    // 被覆盖的字段
    EXPECT_EQ(result.game_id, "overlay-game");

    // 未被覆盖的字段应该保留
    EXPECT_EQ(result.env, "development");
    EXPECT_EQ(result.service_id, "base-service");
    EXPECT_EQ(result.agent_addr, "127.0.0.1:19090");
    EXPECT_EQ(result.timeout_seconds, 30);
    EXPECT_TRUE(result.insecure);

    // headers 应该完整保留
    EXPECT_EQ(result.headers.size(), 1U);
    EXPECT_EQ(result.headers["X-Default"], "default-value");
}
