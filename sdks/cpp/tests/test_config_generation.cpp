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
    // A profile only overrides fields it actually contains (AUD-103).
    EXPECT_EQ("svc-base", config.service_id);
    EXPECT_EQ("demo", config.game_id);
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

TEST(ConfigProfileGenerationTest, LoadProfileEnvOnlyKeepsBaseSettings) {
    // AUD-103: a profile that only changes env must not rewrite the base
    // file's security, reconnect or timeout settings.
    const std::string dir = MakeTempDir("envonly");
    utils::FileSystemUtils::WriteFileContent(dir + "/base.json",
                                             R"({
                                                 "game_id":"demo",
                                                 "env":"development",
                                                 "service_id":"svc-base",
                                                 "insecure":false,
                                                 "auto_reconnect":false,
                                                 "timeout_seconds":17,
                                                 "reconnect_interval_seconds":11,
                                                 "reconnect_max_attempts":9,
                                                 "cert_file":"/etc/tls/client.crt",
                                                 "key_file":"/etc/tls/client.key",
                                                 "ca_file":"/etc/tls/ca.crt",
                                                 "server_name":"prod.example.com"
                                             })");
    utils::FileSystemUtils::WriteFileContent(dir + "/staging.json", R"({"env":"staging"})");

    ClientConfigLoader loader;
    const ClientConfig config = loader.LoadProfile(dir, "staging");

    EXPECT_EQ("staging", config.env);
    EXPECT_EQ("demo", config.game_id);
    EXPECT_EQ("svc-base", config.service_id);
    EXPECT_FALSE(config.insecure);
    EXPECT_FALSE(config.auto_reconnect);
    EXPECT_EQ(17, config.timeout_seconds);
    EXPECT_EQ(11, config.reconnect_interval_seconds);
    EXPECT_EQ(9, config.reconnect_max_attempts);
    EXPECT_EQ("/etc/tls/client.crt", config.cert_file);
    EXPECT_EQ("/etc/tls/client.key", config.key_file);
    EXPECT_EQ("/etc/tls/ca.crt", config.ca_file);
    EXPECT_EQ("prod.example.com", config.server_name);
}

TEST(ConfigProfileGenerationTest, LoadProfileExplicitFalseAndZeroOverrideBase) {
    // AUD-103: explicit false booleans and an explicit zero retry count are
    // real values, not "unset"; presence decides, not the value.
    const std::string dir = MakeTempDir("explicit");
    utils::FileSystemUtils::WriteFileContent(dir + "/base.json",
                                             R"({
                                                 "game_id":"demo",
                                                 "env":"development",
                                                 "service_id":"svc-base",
                                                 "insecure":true,
                                                 "auto_reconnect":true,
                                                 "timeout_seconds":30,
                                                 "reconnect_max_attempts":5
                                             })");
    utils::FileSystemUtils::WriteFileContent(dir + "/prod.json",
                                             R"({
                                                 "env":"production",
                                                 "insecure":false,
                                                 "auto_reconnect":false,
                                                 "reconnect_max_attempts":0
                                             })");

    ClientConfigLoader loader;
    const ClientConfig config = loader.LoadProfile(dir, "prod");

    EXPECT_EQ("production", config.env);
    EXPECT_FALSE(config.insecure);
    EXPECT_FALSE(config.auto_reconnect);
    EXPECT_EQ(0, config.reconnect_max_attempts);
    EXPECT_EQ("demo", config.game_id);
    EXPECT_EQ("svc-base", config.service_id);
    EXPECT_EQ(30, config.timeout_seconds);
}

TEST(ConfigProfileGenerationTest, LoadProfileSecurityHeadersAndServiceId) {
    // AUD-103: nested security/auth sections and explicit header entries in a
    // profile overlay the base; fields the profile omits survive.
    const std::string dir = MakeTempDir("security");
    utils::FileSystemUtils::WriteFileContent(dir + "/base.json",
                                             R"({
                                                 "game_id":"demo",
                                                 "env":"development",
                                                 "service_id":"svc-base",
                                                 "insecure":true,
                                                 "agent_addr":"127.0.0.1:19090",
                                                 "headers":{"X-Base":"base","X-Shared":"old"}
                                             })");
    utils::FileSystemUtils::WriteFileContent(
        dir + "/tls.json",
        R"({
            "env":"production",
            "service_id":"cpp-service",
            "insecure":false,
            "security":{
                "cert_file":"/etc/tls/client.crt",
                "key_file":"/etc/tls/client.key",
                "ca_file":"/etc/tls/ca.crt",
                "server_name":"prod.example.com"
            },
            "auth":{"token":"Bearer prod-token","headers":{"X-Profile":"profile","X-Shared":"new"}}
        })");

    ClientConfigLoader loader;
    const ClientConfig config = loader.LoadProfile(dir, "tls");

    EXPECT_EQ("production", config.env);
    EXPECT_EQ("cpp-service", config.service_id);
    EXPECT_FALSE(config.insecure);
    EXPECT_EQ("127.0.0.1:19090", config.agent_addr);
    EXPECT_EQ("/etc/tls/client.crt", config.cert_file);
    EXPECT_EQ("/etc/tls/client.key", config.key_file);
    EXPECT_EQ("/etc/tls/ca.crt", config.ca_file);
    EXPECT_EQ("prod.example.com", config.server_name);
    EXPECT_EQ("Bearer prod-token", config.auth_token);
    EXPECT_EQ("base", config.headers.at("X-Base"));
    EXPECT_EQ("profile", config.headers.at("X-Profile"));
    EXPECT_EQ("new", config.headers.at("X-Shared"));
}

TEST(ConfigProfileGenerationTest, LoadProfileInvalidJsonThrows) {
    const std::string dir = MakeTempDir("bad");
    utils::FileSystemUtils::WriteFileContent(dir + "/base.json", R"({"game_id":"demo"})");
    utils::FileSystemUtils::WriteFileContent(dir + "/broken.json", R"({"env":})");

    ClientConfigLoader loader;
    EXPECT_THROW(loader.LoadProfile(dir, "broken"), std::runtime_error);
}

TEST(ConfigProfileGenerationTest, LoadProfileExplicitStringServiceIdIsApplied) {
    // AUD-103: an explicitly configured default-looking service_id must win
    // over the base value; presence, not the "cpp-service" sentinel.
    const std::string dir = MakeTempDir("svc");
    utils::FileSystemUtils::WriteFileContent(dir + "/base.json",
                                             R"({"game_id":"demo","service_id":"svc-base"})");
    utils::FileSystemUtils::WriteFileContent(dir + "/dev.json", R"({"service_id":"cpp-service"})");

    ClientConfigLoader loader;
    const ClientConfig config = loader.LoadProfile(dir, "dev");
    EXPECT_EQ("cpp-service", config.service_id);
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
