// Profile loading, example-config generation, and file-path validation.
#include <gtest/gtest.h>
#include "croupier/sdk/config/client_config_loader.h"
#include "croupier/sdk/utils/file_utils.h"
#include "croupier/sdk/utils/json_utils.h"

#include <cstdlib>
#include <unistd.h>

namespace croupier::sdk::config::test {
namespace {

std::string MakeTempDir(const std::string& tag) {
    const char* tmpdir = std::getenv("TMPDIR");
    std::string base = (tmpdir && *tmpdir) ? tmpdir : "/tmp";
    std::string dir = base + "/croupier-cfg-" + tag + "-" + std::to_string(::getpid());
    if (!utils::FileSystemUtils::DirectoryExists(dir)) {
        utils::FileSystemUtils::CreateDirectory(dir);
    }
    return dir;
}

std::string JoinErrors(const std::vector<std::string>& errors) {
    std::string joined;
    for (const auto& error : errors) {
        joined += error + "; ";
    }
    return joined;
}

TEST(ConfigProfileGenerationTest, LoadProfileMergesBaseAndProfile) {
    const std::string dir = MakeTempDir("profiles");
    utils::FileSystemUtils::WriteFileContent(dir + "/base.json",
                                             R"({"game_id":"demo","env":"development","service_id":"svc-base"})");
    utils::FileSystemUtils::WriteFileContent(dir + "/production.json",
                                             R"({"env":"production","timeout_seconds":77})");

    ClientConfigLoader loader;
    const ClientConfig config = loader.LoadProfile(dir, "production");

    // Overlay values win where set explicitly.
    EXPECT_EQ("production", config.env);
    EXPECT_EQ(77, config.timeout_seconds);
    // Base service_id survives because the overlay's default service_id does
    // not override (MergeConfigs special case).
    EXPECT_EQ("svc-base", config.service_id);
    // NOTE (recorded behavior, candidate bug): a profile file that omits
    // game_id still overrides the base value because LoadFromJson back-fills
    // the default "default-game" before merging.
    EXPECT_EQ("default-game", config.game_id);
}

TEST(ConfigProfileGenerationTest, LoadProfileWithoutBaseUsesDefaults) {
    const std::string dir = MakeTempDir("nobase");
    utils::FileSystemUtils::WriteFileContent(dir + "/staging.json", R"({"env":"staging"})");

    ClientConfigLoader loader;
    const ClientConfig config = loader.LoadProfile(dir, "staging");
    EXPECT_EQ("staging", config.env);
    EXPECT_EQ("cpp-service", config.service_id);  // default config applied
}

TEST(ConfigProfileGenerationTest, LoadProfileWithoutProfileFileWarnsButReturns) {
    const std::string dir = MakeTempDir("empty");
    utils::FileSystemUtils::WriteFileContent(dir + "/base.json", R"({"game_id":"only-base"})");

    ClientConfigLoader loader;
    const ClientConfig config = loader.LoadProfile(dir, "does-not-exist");
    EXPECT_EQ("only-base", config.game_id);
}

TEST(ConfigGenerationTest, DevelopmentExampleConfigParsesBack) {
    ClientConfigLoader loader;
    const std::string generated = loader.GenerateExampleConfig("development");
    EXPECT_TRUE(utils::JsonUtils::IsValidJson(generated));

    const ClientConfig config = loader.LoadFromJson(generated);
    EXPECT_EQ("development", config.env);
    EXPECT_EQ("your-game-id", config.game_id);
    EXPECT_EQ("backend-service-01", config.service_id);
    EXPECT_TRUE(config.insecure);
}

TEST(ConfigGenerationTest, ProductionExampleConfigIncludesSecurityAndAuth) {
    ClientConfigLoader loader;
    const std::string generated = loader.GenerateExampleConfig("production");
    const ClientConfig config = loader.LoadFromJson(generated);

    EXPECT_EQ("production", config.env);
    EXPECT_FALSE(config.insecure);
    EXPECT_EQ("/etc/tls/client.crt", config.cert_file);
    EXPECT_EQ("/etc/tls/ca.crt", config.ca_file);
    EXPECT_EQ("croupier.internal", config.server_name);
    EXPECT_EQ("Bearer your-jwt-token-here", config.auth_token);
    EXPECT_EQ(2U, config.headers.size());
}

TEST(ConfigFilePathTest, ValidateFilePathViaSecurityConfig) {
    ClientConfigLoader loader;
    const std::string dir = MakeTempDir("paths");
    const std::string existing = dir + "/exists.txt";
    utils::FileSystemUtils::WriteFileContent(existing, "x");

    // Existing TLS material passes.
    ClientConfig ok;
    ok.game_id = "g";
    ok.agent_addr = "127.0.0.1:19091";
    ok.insecure = false;
    ok.cert_file = existing;
    ok.key_file = existing;
    ok.ca_file = existing;
    ok.server_name = "agent";
    EXPECT_TRUE(loader.ValidateConfig(ok).empty());

    // Missing TLS files are reported.
    ClientConfig missing = ok;
    missing.cert_file = dir + "/missing.crt";
    const std::vector<std::string> errors = loader.ValidateConfig(missing);
    ASSERT_EQ(1U, errors.size());
    EXPECT_NE(errors[0].find("does not exist"), std::string::npos);
}

TEST(ConfigNetworkAddressTest, HostPortValidationViaValidateConfig) {
    ClientConfigLoader loader;
    ClientConfig config;
    config.game_id = "g";
    config.insecure = true;

    config.agent_addr = "127.0.0.1:19091";
    EXPECT_TRUE(loader.ValidateConfig(config).empty());

    config.agent_addr = "agent.local:80";
    EXPECT_TRUE(loader.ValidateConfig(config).empty());

    config.agent_addr = "127.0.0.1";  // missing port
    EXPECT_NE(std::string::npos, JoinErrors(loader.ValidateConfig(config)).find("format is invalid"));

    config.agent_addr = "";  // empty address
    EXPECT_NE(std::string::npos, JoinErrors(loader.ValidateConfig(config)).find("agent_addr cannot be empty"));
}

TEST(ConfigAuthValidationTest, TokenAndHeaderRules) {
    ClientConfigLoader loader;
    ClientConfig config;
    config.game_id = "g";
    config.agent_addr = "127.0.0.1:19091";

    // Missing Bearer prefix and empty header values are reported.
    config.auth_token = "raw-token";
    config.headers[""] = "value";
    config.headers["X-Ok"] = "";
    const std::vector<std::string> errors = loader.ValidateConfig(config);
    EXPECT_FALSE(errors.empty());
    bool has_bearer_error = false;
    bool has_header_errors = false;
    for (const auto& error : errors) {
        if (error.find("Bearer") != std::string::npos) has_bearer_error = true;
        if (error.find("Header") != std::string::npos) has_header_errors = true;
    }
    EXPECT_TRUE(has_bearer_error);
    EXPECT_TRUE(has_header_errors);

    // A fully valid production config passes.
    ClientConfig valid;
    valid.game_id = "g";
    valid.env = "production";
    valid.agent_addr = "10.0.0.1:19090";
    valid.insecure = false;
    valid.cert_file = __FILE__;
    valid.key_file = __FILE__;
    valid.ca_file = __FILE__;
    valid.server_name = "agent";
    valid.auth_token = "Bearer abc";
    EXPECT_TRUE(loader.ValidateConfig(valid).empty());
}

}  // namespace
}  // namespace croupier::sdk::config::test
