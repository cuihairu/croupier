#include <gtest/gtest.h>
#include "croupier/sdk/config/client_config_loader.h"
#include "croupier/sdk/croupier_client.h"

using namespace croupier::sdk;
using namespace croupier::sdk::config;

// 测试夹具：网络地址验证测试
class ConfigNetworkTest : public ::testing::Test {
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

TEST_F(ConfigNetworkTest, ValidAgentAddress) {
    // RED: 测试有效的 Agent 地址 - IPv4
    std::string valid_config = R"({
        "game_id": "test-game",
        "agent_addr": "127.0.0.1:19090"
    })";

    EXPECT_NO_THROW({
        ClientConfig config = loader->LoadFromJson(valid_config);
        EXPECT_EQ(config.agent_addr, "127.0.0.1:19090");
    });
}

TEST_F(ConfigNetworkTest, ValidAgentAddressWithHostname) {
    // RED: 测试有效的 Agent 地址 - 主机名
    std::string valid_config = R"({
        "game_id": "test-game",
        "agent_addr": "localhost:19090"
    })";

    EXPECT_NO_THROW({
        ClientConfig config = loader->LoadFromJson(valid_config);
        EXPECT_EQ(config.agent_addr, "localhost:19090");
    });
}

TEST_F(ConfigNetworkTest, ValidAgentAddressWithDomain) {
    // RED: 测试有效的 Agent 地址 - 域名
    std::string valid_config = R"({
        "game_id": "test-game",
        "agent_addr": "croupier.example.com:8080"
    })";

    EXPECT_NO_THROW({
        ClientConfig config = loader->LoadFromJson(valid_config);
        EXPECT_EQ(config.agent_addr, "croupier.example.com:8080");
    });
}

TEST_F(ConfigNetworkTest, InvalidAgentAddressEmpty) {
    // RED: 测试空的 agent_addr
    std::string invalid_config = R"({
        "game_id": "test-game",
        "agent_addr": ""
    })";

    ClientConfig config = loader->LoadFromJson(invalid_config);
    std::vector<std::string> errors = loader->ValidateConfig(config);

    // 应该有错误
    EXPECT_FALSE(errors.empty());
    EXPECT_TRUE(std::any_of(errors.begin(), errors.end(),
        [](const std::string& err) { return err.find("agent_addr") != std::string::npos; }));
}

TEST_F(ConfigNetworkTest, InvalidAgentAddressNoPort) {
    // RED: 测试缺少端口的地址
    std::string invalid_config = R"({
        "game_id": "test-game",
        "agent_addr": "127.0.0.1"
    })";

    ClientConfig config = loader->LoadFromJson(invalid_config);
    std::vector<std::string> errors = loader->ValidateConfig(config);

    // 应该有错误
    EXPECT_FALSE(errors.empty());
    EXPECT_TRUE(std::any_of(errors.begin(), errors.end(),
        [](const std::string& err) { return err.find("agent_addr") != std::string::npos
                                      || err.find("host:port") != std::string::npos; }));
}

TEST_F(ConfigNetworkTest, InvalidAgentAddressInvalidPort) {
    // RED: 测试无效的端口（非数字）
    std::string invalid_config = R"({
        "game_id": "test-game",
        "agent_addr": "127.0.0.1:abc"
    })";

    ClientConfig config = loader->LoadFromJson(invalid_config);
    std::vector<std::string> errors = loader->ValidateConfig(config);

    // 应该有错误（端口不是数字）
    EXPECT_FALSE(errors.empty());
}

TEST_F(ConfigNetworkTest, InvalidAgentAddressInvalidHost) {
    // RED: 测试无效的主机名格式
    std::string invalid_config = R"({
        "game_id": "test-game",
        "agent_addr": ":19090"
    })";

    ClientConfig config = loader->LoadFromJson(invalid_config);
    std::vector<std::string> errors = loader->ValidateConfig(config);

    // 应该有错误（主机名为空）
    EXPECT_FALSE(errors.empty());
}

TEST_F(ConfigNetworkTest, ValidLocalListenAddress) {
    // RED: 测试有效的本地监听地址
    std::string valid_config = R"({
        "game_id": "test-game"
    })";

    EXPECT_NO_THROW({
        ClientConfig config = loader->LoadFromJson(valid_config);
    });
}

TEST_F(ConfigNetworkTest, ValidLocalListenAddressAnyPort) {
    // RED: 测试监听任意端口
    std::string valid_config = R"({
        "game_id": "test-game"
    })";

    EXPECT_NO_THROW({
        ClientConfig config = loader->LoadFromJson(valid_config);
    });
}

TEST_F(ConfigNetworkTest, InvalidLocalListenAddress) {
    // local_listen 已从配置模型中移除，不应再产生校验错误
    std::string invalid_config = R"({
        "game_id": "test-game"
    })";

    ClientConfig config = loader->LoadFromJson(invalid_config);
    std::vector<std::string> errors = loader->ValidateConfig(config);

    EXPECT_TRUE(errors.empty());
}

TEST_F(ConfigNetworkTest, PortRangeValidation) {
    // RED: 测试端口范围边界
    // 有效端口范围：1-65535

    // 最小有效端口
    std::string min_port_config = R"({
        "game_id": "test-game",
        "agent_addr": "127.0.0.1:1"
    })";

    EXPECT_NO_THROW({
        ClientConfig config = loader->LoadFromJson(min_port_config);
        EXPECT_EQ(config.agent_addr, "127.0.0.1:1");
    });

    // 最大有效端口
    std::string max_port_config = R"({
        "game_id": "test-game",
        "agent_addr": "127.0.0.1:65535"
    })";

    EXPECT_NO_THROW({
        ClientConfig config = loader->LoadFromJson(max_port_config);
        EXPECT_EQ(config.agent_addr, "127.0.0.1:65535");
    });
}

TEST_F(ConfigNetworkTest, LocalhostAddressValidation) {
    // RED: 测试 localhost 地址变体
    std::string localhost_config = R"({
        "game_id": "test-game",
        "agent_addr": "localhost:18780"
    })";

    EXPECT_NO_THROW({
        ClientConfig config = loader->LoadFromJson(localhost_config);
        EXPECT_EQ(config.agent_addr, "localhost:18780");
    });
}

TEST_F(ConfigNetworkTest, IPv4AddressValidation) {
    // RED: 测试各种 IPv4 地址格式
    std::vector<std::string> valid_ipv4_addresses = {
        "192.168.1.1:8080",
        "10.0.0.1:19090",
        "172.16.0.1:3000",
        "0.0.0.0:8080",
        "255.255.255.255:8080"
    };

    for (const auto& addr : valid_ipv4_addresses) {
        std::string config_json = R"({"game_id": "test", "agent_addr": ")" + addr + R"("})";

        EXPECT_NO_THROW({
            ClientConfig config = loader->LoadFromJson(config_json);
            EXPECT_EQ(config.agent_addr, addr);
        }) << "Failed for address: " << addr;
    }
}
