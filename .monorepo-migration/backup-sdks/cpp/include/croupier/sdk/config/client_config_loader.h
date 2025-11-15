#pragma once

#include "croupier/sdk/croupier_client.h"
#include <string>
#include <vector>

#ifdef CROUPIER_SDK_ENABLE_JSON
#include <nlohmann/json.hpp>
#else
#include "croupier/sdk/utils/json_utils.h"
#endif

namespace croupier {
namespace sdk {
namespace config {

/**
 * @brief Client configuration loader and validator
 *
 * Handles loading and validation of client connection configurations
 */
class ClientConfigLoader {
public:
    ClientConfigLoader();
    ~ClientConfigLoader();

    /**
     * @brief Load client configuration from file
     *
     * @param config_file Path to JSON configuration file
     * @return ClientConfig loaded configuration
     * @throws std::runtime_error if loading fails
     */
    ClientConfig LoadFromFile(const std::string& config_file);

    /**
     * @brief Load client configuration from JSON string
     *
     * @param json_content JSON configuration content
     * @return ClientConfig loaded configuration
     * @throws std::runtime_error if parsing fails
     */
    ClientConfig LoadFromJson(const std::string& json_content);

    /**
     * @brief Load configuration with environment variable overrides
     *
     * @param config_file Base configuration file
     * @param env_prefix Environment variable prefix (default: "CROUPIER_")
     * @return ClientConfig configuration with environment overrides
     */
    ClientConfig LoadWithEnvironmentOverrides(const std::string& config_file,
                                             const std::string& env_prefix = "CROUPIER_");

    /**
     * @brief Validate client configuration
     *
     * @param config Configuration to validate
     * @return std::vector<std::string> List of validation errors (empty if valid)
     */
    std::vector<std::string> ValidateConfig(const ClientConfig& config);

    /**
     * @brief Create default client configuration
     *
     * @return ClientConfig with sensible defaults
     */
    ClientConfig CreateDefaultConfig();

    /**
     * @brief Generate example configuration JSON
     *
     * @param environment Target environment ("development", "staging", "production")
     * @return std::string JSON configuration example
     */
    std::string GenerateExampleConfig(const std::string& environment = "development");

    /**
     * @brief Merge two configurations (overlay takes precedence)
     *
     * @param base Base configuration
     * @param overlay Configuration to overlay
     * @return ClientConfig merged configuration
     */
    ClientConfig MergeConfigs(const ClientConfig& base, const ClientConfig& overlay);

    /**
     * @brief Load configuration profiles (dev, staging, prod)
     *
     * @param config_dir Directory containing configuration files
     * @param profile Profile name (e.g., "development", "production")
     * @return ClientConfig configuration for the specified profile
     */
    ClientConfig LoadProfile(const std::string& config_dir, const std::string& profile);

private:
    // Helper methods for parsing and validation
    void ParseSecurityConfig(ClientConfig& config, const std::string& json_content);
    void ParseAuthConfig(ClientConfig& config, const std::string& json_content);
    void ApplyEnvironmentOverrides(ClientConfig& config, const std::string& env_prefix);

    std::string GetEnvironmentVariable(const std::string& name, const std::string& default_value = "");
    bool ValidateNetworkAddress(const std::string& address);
    bool ValidateFilePath(const std::string& file_path, bool must_exist = true);
    std::vector<std::string> ValidateSecurityConfig(const ClientConfig& config);
    std::vector<std::string> ValidateAuthConfig(const ClientConfig& config);

#ifdef CROUPIER_SDK_ENABLE_JSON
    ClientConfig ParseJsonToClientConfig(const nlohmann::json& config_json);
#else
    ClientConfig ParseSimpleJsonToClientConfig(const utils::JsonUtils::SimpleJson& config_json);
#endif
};

} // namespace config
} // namespace sdk
} // namespace croupier