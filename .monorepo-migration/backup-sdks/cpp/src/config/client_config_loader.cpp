#include "croupier/sdk/config/client_config_loader.h"
#include "croupier/sdk/utils/json_utils.h"
#include "croupier/sdk/utils/file_utils.h"
#include <iostream>
#include <regex>
#include <cstdlib>

#ifdef CROUPIER_SDK_ENABLE_JSON
#include <nlohmann/json.hpp>
using json = nlohmann::json;
#endif

namespace croupier {
namespace sdk {
namespace config {

ClientConfigLoader::ClientConfigLoader() {
    std::cout << "Client configuration loader initialized" << std::endl;
}

ClientConfigLoader::~ClientConfigLoader() {
    // Cleanup if needed
}

// ========== Configuration Loading ==========

ClientConfig ClientConfigLoader::LoadFromFile(const std::string& config_file) {
    std::cout << "Loading client configuration from file: " << config_file << std::endl;

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
    std::cout << "Parsing client configuration JSON..." << std::endl;

    if (!utils::JsonUtils::IsValidJson(json_content)) {
        throw std::runtime_error("Invalid JSON format in configuration");
    }

#ifdef CROUPIER_SDK_ENABLE_JSON
    try {
        json config_json = utils::JsonUtils::ParseJson(json_content);
        return ParseJsonToClientConfig(config_json);
    } catch (const std::exception& e) {
        throw std::runtime_error("Failed to parse client configuration JSON: " + std::string(e.what()));
    }
#else
    try {
        auto config_json = utils::JsonUtils::ParseJson(json_content);
        return ParseSimpleJsonToClientConfig(config_json);
    } catch (const std::exception& e) {
        throw std::runtime_error("Failed to parse client configuration: " + std::string(e.what()));
    }
#endif
}

ClientConfig ClientConfigLoader::LoadWithEnvironmentOverrides(const std::string& config_file,
                                                             const std::string& env_prefix) {
    std::cout << "Loading configuration with environment overrides, prefix: " << env_prefix << std::endl;

    ClientConfig config = LoadFromFile(config_file);
    ApplyEnvironmentOverrides(config, env_prefix);
    return config;
}

ClientConfig ClientConfigLoader::LoadProfile(const std::string& config_dir, const std::string& profile) {
    std::cout << "Loading configuration profile: " << profile << " from directory: " << config_dir << std::endl;

    std::string base_file = utils::FileSystemUtils::JoinPath(config_dir, "base.json");
    std::string profile_file = utils::FileSystemUtils::JoinPath(config_dir, profile + ".json");

    ClientConfig config;

    // Load base configuration if it exists
    if (utils::FileSystemUtils::FileExists(base_file)) {
        config = LoadFromFile(base_file);
        std::cout << "Loaded base configuration" << std::endl;
    } else {
        config = CreateDefaultConfig();
        std::cout << "Using default configuration as base" << std::endl;
    }

    // Apply profile-specific overrides
    if (utils::FileSystemUtils::FileExists(profile_file)) {
        ClientConfig profile_config = LoadFromFile(profile_file);
        config = MergeConfigs(config, profile_config);
        std::cout << "Applied profile-specific configuration" << std::endl;
    } else {
        std::cout << "Warning: Profile configuration file not found: " << profile_file << std::endl;
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

    if (!config.local_listen.empty() && !ValidateNetworkAddress(config.local_listen)) {
        errors.push_back("local_listen format is invalid (should be host:port)");
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
        std::cout << "Configuration validation passed" << std::endl;
    } else {
        std::cout << "Configuration validation found " << errors.size() << " errors" << std::endl;
    }

    return errors;
}

// ========== Configuration Generation ==========

ClientConfig ClientConfigLoader::CreateDefaultConfig() {
    std::cout << "Creating default client configuration" << std::endl;

    ClientConfig config;
    config.game_id = "default-game";
    config.env = "development";
    config.service_id = "cpp-sdk-service";
    config.agent_addr = "127.0.0.1:19090";
    config.local_listen = "0.0.0.0:0";
    config.insecure = true;
    config.timeout_seconds = 30;

    return config;
}

std::string ClientConfigLoader::GenerateExampleConfig(const std::string& environment) {
    std::cout << "Generating example configuration for environment: " << environment << std::endl;

#ifdef CROUPIER_SDK_ENABLE_JSON
    json config;

    // Basic configuration
    config["game_id"] = "your-game-id";
    config["env"] = environment;
    config["service_id"] = "backend-service-01";
    config["agent_addr"] = "127.0.0.1:19090";
    config["local_listen"] = "0.0.0.0:0";
    config["timeout_seconds"] = 30;

    // Environment-specific settings
    if (environment == "development") {
        config["insecure"] = true;
    } else {
        config["insecure"] = false;

        // Security configuration for non-dev environments
        config["security"] = {
            {"cert_file", "/etc/tls/client.crt"},
            {"key_file", "/etc/tls/client.key"},
            {"ca_file", "/etc/tls/ca.crt"},
            {"server_name", "croupier.internal"}
        };

        // Authentication configuration
        config["auth"] = {
            {"token", "Bearer your-jwt-token-here"},
            {"headers", {
                {"X-Game-Version", "1.0.0"},
                {"X-Service-ID", "backend-service-01"}
            }}
        };
    }

    return config.dump(2);
#else
    return R"({
  "game_id": "your-game-id",
  "env": ")" + environment + R"(",
  "service_id": "backend-service-01",
  "agent_addr": "127.0.0.1:19090",
  "local_listen": "0.0.0.0:0",
  "insecure": true,
  "timeout_seconds": 30
})";
#endif
}

// ========== Configuration Merging ==========

ClientConfig ClientConfigLoader::MergeConfigs(const ClientConfig& base, const ClientConfig& overlay) {
    std::cout << "Merging configuration (overlay takes precedence)" << std::endl;

    ClientConfig result = base;

    // Apply non-empty overlay values
    if (!overlay.game_id.empty()) result.game_id = overlay.game_id;
    if (!overlay.env.empty()) result.env = overlay.env;
    if (!overlay.service_id.empty()) result.service_id = overlay.service_id;
    if (!overlay.agent_addr.empty()) result.agent_addr = overlay.agent_addr;
    if (!overlay.local_listen.empty()) result.local_listen = overlay.local_listen;
    if (overlay.timeout_seconds > 0) result.timeout_seconds = overlay.timeout_seconds;

    // Boolean values
    result.insecure = overlay.insecure;  // Always apply boolean values

    // Security configuration
    if (!overlay.cert_file.empty()) result.cert_file = overlay.cert_file;
    if (!overlay.key_file.empty()) result.key_file = overlay.key_file;
    if (!overlay.ca_file.empty()) result.ca_file = overlay.ca_file;
    if (!overlay.server_name.empty()) result.server_name = overlay.server_name;

    // Authentication configuration
    if (!overlay.auth_token.empty()) result.auth_token = overlay.auth_token;

    // Merge headers
    for (const auto& [key, value] : overlay.headers) {
        result.headers[key] = value;
    }

    return result;
}

// ========== Private Helper Methods ==========

void ClientConfigLoader::ApplyEnvironmentOverrides(ClientConfig& config, const std::string& env_prefix) {
    std::cout << "Applying environment variable overrides with prefix: " << env_prefix << std::endl;

    config.game_id = GetEnvironmentVariable(env_prefix + "GAME_ID", config.game_id);
    config.env = GetEnvironmentVariable(env_prefix + "ENV", config.env);
    config.service_id = GetEnvironmentVariable(env_prefix + "SERVICE_ID", config.service_id);
    config.agent_addr = GetEnvironmentVariable(env_prefix + "AGENT_ADDR", config.agent_addr);
    config.local_listen = GetEnvironmentVariable(env_prefix + "LOCAL_LISTEN", config.local_listen);

    // Boolean environment variables
    std::string insecure_env = GetEnvironmentVariable(env_prefix + "INSECURE", "");
    if (!insecure_env.empty()) {
        std::transform(insecure_env.begin(), insecure_env.end(), insecure_env.begin(), ::tolower);
        config.insecure = (insecure_env == "true" || insecure_env == "1" || insecure_env == "yes");
    }

    // Numeric environment variables
    std::string timeout_env = GetEnvironmentVariable(env_prefix + "TIMEOUT_SECONDS", "");
    if (!timeout_env.empty()) {
        try {
            config.timeout_seconds = std::stoi(timeout_env);
        } catch (const std::exception& e) {
            std::cerr << "Invalid timeout value in environment: " << timeout_env << std::endl;
        }
    }

    // Security configuration from environment
    config.cert_file = GetEnvironmentVariable(env_prefix + "CERT_FILE", config.cert_file);
    config.key_file = GetEnvironmentVariable(env_prefix + "KEY_FILE", config.key_file);
    config.ca_file = GetEnvironmentVariable(env_prefix + "CA_FILE", config.ca_file);
    config.server_name = GetEnvironmentVariable(env_prefix + "SERVER_NAME", config.server_name);
    config.auth_token = GetEnvironmentVariable(env_prefix + "AUTH_TOKEN", config.auth_token);
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
    if (file_path.empty()) return false;

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

    config.game_id = utils::JsonUtils::GetStringValue(config_json, "game_id", "");
    config.env = utils::JsonUtils::GetStringValue(config_json, "env", "development");
    config.service_id = utils::JsonUtils::GetStringValue(config_json, "service_id", "cpp-sdk-service");
    config.agent_addr = utils::JsonUtils::GetStringValue(config_json, "agent_addr", "127.0.0.1:19090");
    config.local_listen = utils::JsonUtils::GetStringValue(config_json, "local_listen", "0.0.0.0:0");
    config.insecure = utils::JsonUtils::GetBoolValue(config_json, "insecure", true);
    config.timeout_seconds = utils::JsonUtils::GetIntValue(config_json, "timeout_seconds", 30);

    // Security configuration
    config.cert_file = utils::JsonUtils::GetStringValue(config_json, "security.cert_file", "");
    config.key_file = utils::JsonUtils::GetStringValue(config_json, "security.key_file", "");
    config.ca_file = utils::JsonUtils::GetStringValue(config_json, "security.ca_file", "");
    config.server_name = utils::JsonUtils::GetStringValue(config_json, "security.server_name", "");

    // Authentication configuration
    config.auth_token = utils::JsonUtils::GetStringValue(config_json, "auth.token", "");

    // Headers
    if (config_json.contains("auth") && config_json["auth"].contains("headers")) {
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

    config.game_id = utils::JsonUtils::GetStringValue(config_json, "game_id", "");
    config.env = utils::JsonUtils::GetStringValue(config_json, "env", "development");
    config.service_id = utils::JsonUtils::GetStringValue(config_json, "service_id", "cpp-sdk-service");
    config.agent_addr = utils::JsonUtils::GetStringValue(config_json, "agent_addr", "127.0.0.1:19090");
    config.local_listen = utils::JsonUtils::GetStringValue(config_json, "local_listen", "0.0.0.0:0");
    config.insecure = utils::JsonUtils::GetBoolValue(config_json, "insecure", true);
    config.timeout_seconds = utils::JsonUtils::GetIntValue(config_json, "timeout_seconds", 30);

    return config;
}
#endif

} // namespace config
} // namespace sdk
} // namespace croupier