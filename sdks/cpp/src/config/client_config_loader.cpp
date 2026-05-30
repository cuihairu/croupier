#include "croupier/sdk/config/client_config_loader.h"

#include "croupier/sdk/utils/file_utils.h"
#include "croupier/sdk/utils/json_utils.h"

#include <algorithm>
#include <cstdlib>
#include <iostream>
#include <regex>

#ifdef CROUPIER_SDK_ENABLE_JSON
#include <nlohmann/json.hpp>
using json = nlohmann::json;
#endif

namespace croupier {
namespace sdk {
namespace config {

ClientConfigLoader::ClientConfigLoader() {
    std::cout << "Client configuration loader initialized" << '\n';
}

ClientConfigLoader::~ClientConfigLoader() {
    // Cleanup if needed
}

// ========== Configuration Loading ==========

ClientConfig ClientConfigLoader::LoadFromFile(const std::string& config_file) {
    std::cout << "Loading client configuration from file: " << config_file << '\n';

    if (!utils::FileSystemUtils::FileExists(config_file)) {
        throw std::runtime_error("Configuration file does not exist: " + config_file);
    }

    try {
        std::string content = utils::FileSystemUtils::ReadFileContent(config_file);
        return LoadFromJson(content);
    } catch (const std::exception& e) {
        throw std::runtime_error("Failed to load configuration from file: " + std::string(e.what()));
    }
}

ClientConfig ClientConfigLoader::LoadFromJson(const std::string& json_content) {
    std::cout << "Parsing client configuration JSON..." << '\n';

    if (!utils::JsonUtils::IsValidJson(json_content)) {
        throw std::runtime_error("Invalid JSON format in configuration");
    }

    ClientConfig config;

#ifdef CROUPIER_SDK_ENABLE_JSON
    try {
        json config_json = utils::JsonUtils::ParseJson(json_content);
        config = ParseJsonToClientConfig(config_json);
    } catch (const std::exception& e) {
        throw std::runtime_error("Failed to parse client configuration JSON: " + std::string(e.what()));
    }
#else
    try {
        auto config_json = utils::JsonUtils::ParseJson(json_content);
        config = ParseSimpleJsonToClientConfig(config_json);
    } catch (const std::exception& e) {
        throw std::runtime_error("Failed to parse client configuration: " + std::string(e.what()));
    }
#endif

    // Fill in empty fields with default values for file loading
    // This ensures LoadFromFile and LoadFromJson provide sensible defaults
    // but doesn't affect MergeConfigs where overlay should not override base
    if (config.service_id.empty()) {
        ClientConfig default_config = CreateDefaultConfig();
        config.service_id = default_config.service_id;
    }

    return config;
}

ClientConfig ClientConfigLoader::LoadWithEnvironmentOverrides(const std::string& config_file,
                                                              const std::string& env_prefix) {
    std::cout << "Loading configuration with environment overrides, prefix: " << env_prefix << '\n';

    ClientConfig config = LoadFromFile(config_file);
    ApplyEnvironmentOverrides(config, env_prefix);
    return config;
}

ClientConfig ClientConfigLoader::LoadProfile(const std::string& config_dir, const std::string& profile) {
    std::cout << "Loading configuration profile: " << profile << " from directory: " << config_dir << '\n';

    std::string base_file = utils::FileSystemUtils::JoinPath(config_dir, "base.json");
    std::string profile_file = utils::FileSystemUtils::JoinPath(config_dir, profile + ".json");

    ClientConfig config;

    // Load base configuration if it exists
    if (utils::FileSystemUtils::FileExists(base_file)) {
        config = LoadFromFile(base_file);
        std::cout << "Loaded base configuration" << '\n';
    } else {
        config = CreateDefaultConfig();
        std::cout << "Using default configuration as base" << '\n';
    }

    // Apply profile-specific overrides
    if (utils::FileSystemUtils::FileExists(profile_file)) {
        ClientConfig profile_config = LoadFromFile(profile_file);
        config = MergeConfigs(config, profile_config);
        std::cout << "Applied profile-specific configuration" << '\n';
    } else {
        std::cout << "Warning: Profile configuration file not found: " << profile_file << '\n';
    }

    return config;
}

// ========== Configuration Validation ==========

std::vector<std::string> ClientConfigLoader::ValidateConfig(const ClientConfig& config) {
    std::vector<std::string> errors;

    // Basic required fields validation
    if (config.game_id.empty()) {
        errors.push_back("game_id cannot be empty");
    }

    if (config.agent_addr.empty()) {
        errors.push_back("agent_addr cannot be empty");
    } else if (!ValidateNetworkAddress(config.agent_addr)) {
        errors.push_back("agent_addr format is invalid (should be host:port)");
    }

    if (config.timeout_seconds <= 0) {
        errors.push_back("timeout_seconds must be greater than 0");
    }

    if (config.reconnect_interval_seconds <= 0) {
        errors.push_back("reconnect_interval_seconds must be greater than 0");
    }

    if (config.reconnect_max_attempts < 0) {
        errors.push_back("reconnect_max_attempts must be >= 0 (0 means unlimited)");
    }

    // Environment validation
    std::vector<std::string> valid_envs = {"development", "testing", "staging", "production"};
    if (std::find(valid_envs.begin(), valid_envs.end(), config.env) == valid_envs.end()) {
        errors.push_back("env must be one of: development, testing, staging, production");
    }

    // Security configuration validation
    auto security_errors = ValidateSecurityConfig(config);
    errors.insert(errors.end(), security_errors.begin(), security_errors.end());

    // Authentication configuration validation
    auto auth_errors = ValidateAuthConfig(config);
    errors.insert(errors.end(), auth_errors.begin(), auth_errors.end());

    if (errors.empty()) {
        std::cout << "Configuration validation passed" << '\n';
    } else {
        std::cout << "Configuration validation found " << errors.size() << " errors" << '\n';
    }

    return errors;
}

// ========== Configuration Generation ==========

ClientConfig ClientConfigLoader::CreateDefaultConfig() {
    std::cout << "Creating default client configuration" << '\n';

    ClientConfig config;
    config.game_id = "default-game";
    config.env = "development";
    config.service_id = "cpp-service";
    config.agent_addr = "127.0.0.1:19090";
    config.provider_lang = "cpp";
    config.provider_sdk = "croupier-cpp-sdk";
    config.insecure = true;
    config.timeout_seconds = 30;
    config.auto_reconnect = true;
    config.reconnect_interval_seconds = 5;
    config.reconnect_max_attempts = 0;

    return config;
}

std::string ClientConfigLoader::GenerateExampleConfig(const std::string& environment) {
    std::cout << "Generating example configuration for environment: " << environment << '\n';

#ifdef CROUPIER_SDK_ENABLE_JSON
    json config;

    // Basic configuration
    config["game_id"] = "your-game-id";
    config["env"] = environment;
    config["service_id"] = "backend-service-01";
    config["agent_addr"] = "127.0.0.1:19090";
    config["control_addr"] = "127.0.0.1:18080";
    config["timeout_seconds"] = 30;
    config["auto_reconnect"] = true;
    config["reconnect_interval_seconds"] = 5;
    config["reconnect_max_attempts"] = 0;
    config["provider_lang"] = "cpp";
    config["provider_sdk"] = "croupier-cpp-sdk";

    // Environment-specific settings
    if (environment == "development") {
        config["insecure"] = true;
    } else {
        config["insecure"] = false;

        // Security configuration for non-dev environments
        config["security"] = {{"cert_file", "/etc/tls/client.crt"},
                              {"key_file", "/etc/tls/client.key"},
                              {"ca_file", "/etc/tls/ca.crt"},
                              {"server_name", "croupier.internal"}};

        // Authentication configuration
        config["auth"] = {{"token", "Bearer your-jwt-token-here"},
                          {"headers", {{"X-Game-Version", "1.0.0"}, {"X-Service-ID", "backend-service-01"}}}};
    }

    return config.dump(2);
#else
    return R"({
  "game_id": "your-game-id",
  "env": ")" +
           environment + R"(",
  "service_id": "backend-service-01",
  "agent_addr": "127.0.0.1:19090",
  "control_addr": "127.0.0.1:18080",
  "insecure": true,
  "timeout_seconds": 30,
  "auto_reconnect": true,
  "reconnect_interval_seconds": 5,
  "reconnect_max_attempts": 0,
  "provider_lang": "cpp",
  "provider_sdk": "croupier-cpp-sdk"
})";
#endif
}

// ========== Configuration Merging ==========

ClientConfig ClientConfigLoader::MergeConfigs(const ClientConfig& base, const ClientConfig& overlay) {
    std::cout << "Merging configuration (overlay takes precedence)" << '\n';

    ClientConfig result = base;

    // Apply non-empty overlay values
    if (!overlay.game_id.empty())
        result.game_id = overlay.game_id;
    if (!overlay.env.empty())
        result.env = overlay.env;
    // Special handling for service_id: only overlay if it's not the default value
    // This prevents overlay configs with default service_id from overriding base configs
    const std::string default_service_id = "cpp-service";
    if (!overlay.service_id.empty() && overlay.service_id != default_service_id)
        result.service_id = overlay.service_id;
    // If base is empty but overlay has a non-default value, use it
    if (result.service_id.empty() && !overlay.service_id.empty())
        result.service_id = overlay.service_id;
    if (!overlay.agent_addr.empty())
        result.agent_addr = overlay.agent_addr;
    if (!overlay.control_addr.empty())
        result.control_addr = overlay.control_addr;
    if (overlay.timeout_seconds > 0)
        result.timeout_seconds = overlay.timeout_seconds;
    if (overlay.heartbeat_interval > 0)
        result.heartbeat_interval = overlay.heartbeat_interval;

    // Boolean values
    result.insecure = overlay.insecure;  // Always apply boolean values
    result.auto_reconnect = overlay.auto_reconnect;  // Always apply boolean values

    // Reconnect configuration
    if (overlay.reconnect_interval_seconds > 0)
        result.reconnect_interval_seconds = overlay.reconnect_interval_seconds;
    if (overlay.reconnect_max_attempts != 0)  // 0 means unlimited, use explicit check
        result.reconnect_max_attempts = overlay.reconnect_max_attempts;

    // Security configuration
    if (!overlay.cert_file.empty())
        result.cert_file = overlay.cert_file;
    if (!overlay.key_file.empty())
        result.key_file = overlay.key_file;
    if (!overlay.ca_file.empty())
        result.ca_file = overlay.ca_file;
    if (!overlay.server_name.empty())
        result.server_name = overlay.server_name;

    // Authentication configuration
    if (!overlay.auth_token.empty())
        result.auth_token = overlay.auth_token;
    if (!overlay.provider_lang.empty())
        result.provider_lang = overlay.provider_lang;
    if (!overlay.provider_sdk.empty())
        result.provider_sdk = overlay.provider_sdk;

    // Merge headers
    for (const auto& [key, value] : overlay.headers) {
        result.headers[key] = value;
    }

    return result;
}

// ========== Private Helper Methods ==========

void ClientConfigLoader::ApplyEnvironmentOverrides(ClientConfig& config, const std::string& env_prefix) {
    std::cout << "Applying environment variable overrides with prefix: " << env_prefix << '\n';

    config.game_id = GetEnvironmentVariable(env_prefix + "GAME_ID", config.game_id);
    config.env = GetEnvironmentVariable(env_prefix + "ENV", config.env);
    config.service_id = GetEnvironmentVariable(env_prefix + "SERVICE_ID", config.service_id);
    config.agent_addr = GetEnvironmentVariable(env_prefix + "AGENT_ADDR", config.agent_addr);
    config.control_addr = GetEnvironmentVariable(env_prefix + "CONTROL_ADDR", config.control_addr);

    // Boolean environment variables
    std::string insecure_env = GetEnvironmentVariable(env_prefix + "INSECURE", "");
    if (!insecure_env.empty()) {
        std::transform(insecure_env.begin(), insecure_env.end(), insecure_env.begin(), ::tolower);
        config.insecure = (insecure_env == "true" || insecure_env == "1" || insecure_env == "yes");
    }

    std::string auto_reconnect_env = GetEnvironmentVariable(env_prefix + "AUTO_RECONNECT", "");
    if (!auto_reconnect_env.empty()) {
        std::transform(auto_reconnect_env.begin(), auto_reconnect_env.end(), auto_reconnect_env.begin(), ::tolower);
        config.auto_reconnect =
            (auto_reconnect_env == "true" || auto_reconnect_env == "1" || auto_reconnect_env == "yes");
    }

    // Numeric environment variables
    std::string timeout_env = GetEnvironmentVariable(env_prefix + "TIMEOUT_SECONDS", "");
    if (!timeout_env.empty()) {
        try {
            config.timeout_seconds = std::stoi(timeout_env);
        } catch (const std::exception& /* e */) {
            std::cerr << "Invalid timeout value in environment: " << timeout_env << '\n';
        }
    }

    std::string reconnect_interval_env = GetEnvironmentVariable(env_prefix + "RECONNECT_INTERVAL_SECONDS", "");
    if (!reconnect_interval_env.empty()) {
        try {
            config.reconnect_interval_seconds = std::stoi(reconnect_interval_env);
        } catch (const std::exception& /* e */) {
            std::cerr << "Invalid reconnect_interval_seconds value in environment: " << reconnect_interval_env << '\n';
        }
    }

    std::string reconnect_attempts_env = GetEnvironmentVariable(env_prefix + "RECONNECT_MAX_ATTEMPTS", "");
    if (!reconnect_attempts_env.empty()) {
        try {
            config.reconnect_max_attempts = std::stoi(reconnect_attempts_env);
        } catch (const std::exception& /* e */) {
            std::cerr << "Invalid reconnect_max_attempts value in environment: " << reconnect_attempts_env << '\n';
        }
    }

    // Security configuration from environment
    config.cert_file = GetEnvironmentVariable(env_prefix + "CERT_FILE", config.cert_file);
    config.key_file = GetEnvironmentVariable(env_prefix + "KEY_FILE", config.key_file);
    config.ca_file = GetEnvironmentVariable(env_prefix + "CA_FILE", config.ca_file);
    config.server_name = GetEnvironmentVariable(env_prefix + "SERVER_NAME", config.server_name);
    config.auth_token = GetEnvironmentVariable(env_prefix + "AUTH_TOKEN", config.auth_token);
    config.provider_lang = GetEnvironmentVariable(env_prefix + "PROVIDER_LANG", config.provider_lang);
    config.provider_sdk = GetEnvironmentVariable(env_prefix + "PROVIDER_SDK", config.provider_sdk);
}

std::string ClientConfigLoader::GetEnvironmentVariable(const std::string& name, const std::string& default_value) {
    const char* value = std::getenv(name.c_str());
    return value ? std::string(value) : default_value;
}

bool ClientConfigLoader::ValidateNetworkAddress(const std::string& address) {
    // Basic validation for host:port format
    std::regex addr_pattern(R"(^.+:\d+$)");
    return std::regex_match(address, addr_pattern);
}

bool ClientConfigLoader::ValidateFilePath(const std::string& file_path, bool must_exist) {
    if (file_path.empty())
        return false;

    if (must_exist) {
        return utils::FileSystemUtils::FileExists(file_path);
    }

    // Check if the parent directory exists
    std::string parent_dir = utils::FileSystemUtils::GetDirectoryPath(file_path);
    return parent_dir.empty() || utils::FileSystemUtils::DirectoryExists(parent_dir);
}

std::vector<std::string> ClientConfigLoader::ValidateSecurityConfig(const ClientConfig& config) {
    std::vector<std::string> errors;

    if (!config.insecure) {
        // TLS is required
        if (config.cert_file.empty()) {
            errors.push_back("cert_file is required when insecure=false");
        } else if (!ValidateFilePath(config.cert_file, true)) {
            errors.push_back("cert_file does not exist: " + config.cert_file);
        }

        if (config.key_file.empty()) {
            errors.push_back("key_file is required when insecure=false");
        } else if (!ValidateFilePath(config.key_file, true)) {
            errors.push_back("key_file does not exist: " + config.key_file);
        }

        if (config.ca_file.empty()) {
            errors.push_back("ca_file is required when insecure=false");
        } else if (!ValidateFilePath(config.ca_file, true)) {
            errors.push_back("ca_file does not exist: " + config.ca_file);
        }

        if (config.server_name.empty()) {
            errors.push_back("server_name is required when insecure=false");
        }
    }

    return errors;
}

std::vector<std::string> ClientConfigLoader::ValidateAuthConfig(const ClientConfig& config) {
    std::vector<std::string> errors;

    // Authentication token format validation
    if (!config.auth_token.empty()) {
        if (config.auth_token.find("Bearer ") != 0) {
            errors.push_back("auth_token should start with 'Bearer ' prefix");
        }
    }

    // Header validation
    for (const auto& [key, value] : config.headers) {
        if (key.empty()) {
            errors.push_back("Header key cannot be empty");
        }
        if (value.empty()) {
            errors.push_back("Header value cannot be empty for key: " + key);
        }
    }

    return errors;
}

#ifdef CROUPIER_SDK_ENABLE_JSON
ClientConfig ClientConfigLoader::ParseJsonToClientConfig(const nlohmann::json& config_json) {
    ClientConfig config;

    config.game_id = utils::JsonUtils::GetStringValue(config_json, "game_id", "default-game");
    config.env = utils::JsonUtils::GetStringValue(config_json, "env", "development");
    config.service_id = utils::JsonUtils::GetStringValue(config_json, "service_id", "");
    config.agent_addr = utils::JsonUtils::GetStringValue(config_json, "agent_addr", "127.0.0.1:19090");
    config.control_addr = utils::JsonUtils::GetStringValue(config_json, "control_addr", "");
    config.insecure = utils::JsonUtils::GetBoolValue(config_json, "insecure", true);
    config.timeout_seconds = utils::JsonUtils::GetIntValue(config_json, "timeout_seconds", 30);
    config.auto_reconnect = utils::JsonUtils::GetBoolValue(config_json, "auto_reconnect", true);
    config.reconnect_interval_seconds = utils::JsonUtils::GetIntValue(config_json, "reconnect_interval_seconds", 5);
    config.reconnect_max_attempts = utils::JsonUtils::GetIntValue(config_json, "reconnect_max_attempts", 0);
    config.provider_lang = utils::JsonUtils::GetStringValue(config_json, "provider_lang", "cpp");
    config.provider_sdk = utils::JsonUtils::GetStringValue(config_json, "provider_sdk", "croupier-cpp-sdk");

    // Security configuration - support both flat and nested formats
    config.cert_file = utils::JsonUtils::GetStringValue(config_json, "cert_file", "");
    config.cert_file = utils::JsonUtils::GetStringValue(config_json, "security.cert_file", config.cert_file);
    config.key_file = utils::JsonUtils::GetStringValue(config_json, "key_file", "");
    config.key_file = utils::JsonUtils::GetStringValue(config_json, "security.key_file", config.key_file);
    config.ca_file = utils::JsonUtils::GetStringValue(config_json, "ca_file", "");
    config.ca_file = utils::JsonUtils::GetStringValue(config_json, "security.ca_file", config.ca_file);
    config.server_name = utils::JsonUtils::GetStringValue(config_json, "server_name", "");
    config.server_name = utils::JsonUtils::GetStringValue(config_json, "security.server_name", config.server_name);

    // Authentication configuration
    config.auth_token = utils::JsonUtils::GetStringValue(config_json, "auth_token", "");
    config.auth_token = utils::JsonUtils::GetStringValue(config_json, "auth.token", config.auth_token);  // Also support nested format

    // Headers - support both flat and nested formats
    if (config_json.contains("headers")) {
        // Flat format: "headers": { "X-Header": "value" }
        for (const auto& [key, value] : config_json["headers"].items()) {
            if (value.is_string()) {
                config.headers[key] = value.get<std::string>();
            }
        }
    }
    if (config_json.contains("auth") && config_json["auth"].contains("headers")) {
        // Nested format: "auth": { "headers": { "X-Header": "value" } }
        for (const auto& [key, value] : config_json["auth"]["headers"].items()) {
            if (value.is_string()) {
                config.headers[key] = value.get<std::string>();
            }
        }
    }

    return config;
}
#else
ClientConfig ClientConfigLoader::ParseSimpleJsonToClientConfig(const utils::JsonUtils::SimpleJson& config_json) {
    ClientConfig config;

    config.game_id = utils::JsonUtils::GetStringValue(config_json, "game_id", "default-game");
    config.env = utils::JsonUtils::GetStringValue(config_json, "env", "development");
    config.service_id = utils::JsonUtils::GetStringValue(config_json, "service_id", "");
    config.agent_addr = utils::JsonUtils::GetStringValue(config_json, "agent_addr", "127.0.0.1:19090");
    config.control_addr = utils::JsonUtils::GetStringValue(config_json, "control_addr", "");
    config.insecure = utils::JsonUtils::GetBoolValue(config_json, "insecure", true);
    config.timeout_seconds = utils::JsonUtils::GetIntValue(config_json, "timeout_seconds", 30);
    config.auto_reconnect = utils::JsonUtils::GetBoolValue(config_json, "auto_reconnect", true);
    config.reconnect_interval_seconds = utils::JsonUtils::GetIntValue(config_json, "reconnect_interval_seconds", 5);
    config.reconnect_max_attempts = utils::JsonUtils::GetIntValue(config_json, "reconnect_max_attempts", 0);
    config.provider_lang = utils::JsonUtils::GetStringValue(config_json, "provider_lang", "cpp");
    config.provider_sdk = utils::JsonUtils::GetStringValue(config_json, "provider_sdk", "croupier-cpp-sdk");

    return config;
}
#endif

}  // namespace config
}  // namespace sdk
}  // namespace croupier
