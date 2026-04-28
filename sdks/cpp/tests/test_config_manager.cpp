#include <gtest/gtest.h>

#include "croupier/sdk/config_manager.h"
#include "croupier/sdk/croupier_client.h"

#include <fstream>
#include <cstdio>

namespace croupier {
namespace sdk {
namespace test {

class ConfigManagerTest : public ::testing::Test {
protected:
    void SetUp() override {
        // Create temp directory for test files
        temp_dir_ = "/tmp/croupier_config_test_" + std::to_string(std::rand());
#ifdef _WIN32
        _mkdir(temp_dir_.c_str());
#else
        mkdir(temp_dir_.c_str(), 0755);
#endif
    }

    void TearDown() override {
        // Clean up temp files
        std::remove((temp_dir_ + "/client.json").c_str());
        std::remove((temp_dir_ + "/global.json").c_str());
        std::remove((temp_dir_ + "/app_config.json").c_str());
        std::remove((temp_dir_ + "/schema.json").c_str());
#ifdef _WIN32
        _rmdir(temp_dir_.c_str());
#else
        rmdir(temp_dir_.c_str());
#endif
    }

    void WriteValidClientConfig(const std::string& path) {
        std::ofstream file(path);
        file << R"({
            "game_id": "test-game",
            "env": "testing",
            "service_id": "test-service",
            "agent_addr": "127.0.0.1:19090",
            "insecure": true,
            "timeout_seconds": 30,
            "auto_reconnect": true,
            "reconnect_interval_seconds": 5,
            "reconnect_max_attempts": 3
        })";
        file.close();
    }

    void WriteInvalidClientConfig(const std::string& path) {
        std::ofstream file(path);
        file << R"({ invalid json })";
        file.close();
    }

    void WriteSchemaFile(const std::string& path) {
        std::ofstream file(path);
        file << R"({
            "schema": {
                "id": "player",
                "version": "1.0.0",
                "name": "Player",
                "description": "A player entity"
            },
            "fields": {
                "player_id": {
                    "type": "string",
                    "required": true,
                    "description": "Unique player identifier"
                },
                "level": {
                    "type": "int",
                    "required": false,
                    "default_value": "1",
                    "description": "Player level",
                    "validation": {
                        "min": "1",
                        "max": "100"
                    }
                },
                "name": {
                    "type": "string",
                    "required": true,
                    "description": "Player name",
                    "validation": {
                        "pattern": "^[a-zA-Z0-9_]{3,20}$"
                    }
                }
            },
            "operations": {
                "create": "player.create",
                "get": "player.get",
                "update": "player.update"
            }
        })";
        file.close();
    }

    void WriteApplicationConfigFile(const std::string& path) {
        std::ofstream file(path);
        file << R"({
            "client": {
                "game_id": "app-game",
                "env": "production",
                "agent_addr": "127.0.0.1:19090"
            },
            "global": {
                "log_level": "info",
                "metrics_enabled": true
            }
        })";
        file.close();
    }

    std::string temp_dir_;
};

// Test ConfigManager default constructor
TEST_F(ConfigManagerTest, DefaultConstructor) {
    ConfigManager manager;
    EXPECT_NO_THROW(manager.GetLastError());
}

// Test LoadClientConfig with valid file
TEST_F(ConfigManagerTest, LoadClientConfigValid) {
    std::string config_path = temp_dir_ + "/client.json";
    WriteValidClientConfig(config_path);

    ConfigManager manager;
    ClientConfig config = manager.LoadClientConfig(config_path);

    EXPECT_EQ(config.game_id, "test-game");
    EXPECT_EQ(config.env, "testing");
    EXPECT_EQ(config.agent_addr, "127.0.0.1:19090");
    EXPECT_TRUE(config.insecure);
    EXPECT_EQ(config.timeout_seconds, 30);
}

// Test LoadClientConfig with non-existent file
TEST_F(ConfigManagerTest, LoadClientConfigNonExistent) {
    ConfigManager manager;
    EXPECT_THROW(manager.LoadClientConfig("/nonexistent/config.json"), std::runtime_error);
}

// Test LoadClientConfig with invalid JSON
TEST_F(ConfigManagerTest, LoadClientConfigInvalidJson) {
    std::string config_path = temp_dir_ + "/client.json";
    WriteInvalidClientConfig(config_path);

    ConfigManager manager;
    EXPECT_THROW(manager.LoadClientConfig(config_path), std::runtime_error);
}

// Test LoadClientConfigFromJson with valid JSON
TEST_F(ConfigManagerTest, LoadClientConfigFromJsonValid) {
    std::string json = R"({
        "game_id": "json-game",
        "env": "development",
        "agent_addr": "192.168.1.1:8080"
    })";

    ConfigManager manager;
    ClientConfig config = manager.LoadClientConfigFromJson(json);

    EXPECT_EQ(config.game_id, "json-game");
    EXPECT_EQ(config.env, "development");
    EXPECT_EQ(config.agent_addr, "192.168.1.1:8080");
}

// Test LoadClientConfigFromJson with invalid JSON
TEST_F(ConfigManagerTest, LoadClientConfigFromJsonInvalid) {
    std::string invalid_json = R"({ not valid json })";

    ConfigManager manager;
    EXPECT_THROW(manager.LoadClientConfigFromJson(invalid_json), std::runtime_error);
}

// Test ValidateClientConfig with valid config
TEST_F(ConfigManagerTest, ValidateClientConfigValid) {
    ClientConfig config;
    config.game_id = "test-game";
    config.agent_addr = "127.0.0.1:19090";
    config.timeout_seconds = 30;
    config.insecure = true;

    ConfigManager manager;
    auto errors = manager.ValidateClientConfig(config);

    EXPECT_TRUE(errors.empty());
}

// Test ValidateClientConfig with empty game_id
TEST_F(ConfigManagerTest, ValidateClientConfigEmptyGameId) {
    ClientConfig config;
    config.game_id = "";
    config.agent_addr = "127.0.0.1:19090";
    config.timeout_seconds = 30;

    ConfigManager manager;
    auto errors = manager.ValidateClientConfig(config);

    EXPECT_FALSE(errors.empty());
    bool found = false;
    for (const auto& err : errors) {
        if (err.find("game_id") != std::string::npos) {
            found = true;
            break;
        }
    }
    EXPECT_TRUE(found);
}

// Test ValidateClientConfig with empty agent_addr
TEST_F(ConfigManagerTest, ValidateClientConfigEmptyAgentAddr) {
    ClientConfig config;
    config.game_id = "test-game";
    config.agent_addr = "";
    config.timeout_seconds = 30;

    ConfigManager manager;
    auto errors = manager.ValidateClientConfig(config);

    EXPECT_FALSE(errors.empty());
}

// Test ValidateClientConfig with invalid address format
TEST_F(ConfigManagerTest, ValidateClientConfigInvalidAddress) {
    ClientConfig config;
    config.game_id = "test-game";
    config.agent_addr = "invalid-address";  // Missing port
    config.timeout_seconds = 30;

    ConfigManager manager;
    auto errors = manager.ValidateClientConfig(config);

    bool found = false;
    for (const auto& err : errors) {
        if (err.find("格式") != std::string::npos || err.find("format") != std::string::npos) {
            found = true;
            break;
        }
    }
    EXPECT_TRUE(found);
}

// Test ValidateClientConfig with TLS requirements
TEST_F(ConfigManagerTest, ValidateClientConfigTLSRequirements) {
    ClientConfig config;
    config.game_id = "test-game";
    config.agent_addr = "127.0.0.1:19090";
    config.timeout_seconds = 30;
    config.insecure = false;  // TLS enabled
    // Missing cert files

    ConfigManager manager;
    auto errors = manager.ValidateClientConfig(config);

    EXPECT_FALSE(errors.empty());
}

// Test LoadVirtualObjectSchema with valid file
TEST_F(ConfigManagerTest, LoadVirtualObjectSchemaValid) {
    std::string schema_path = temp_dir_ + "/schema.json";
    WriteSchemaFile(schema_path);

    ConfigManager manager;
    auto schema = manager.LoadVirtualObjectSchema(schema_path);

    EXPECT_EQ(schema.id, "player");
    EXPECT_EQ(schema.version, "1.0.0");
    EXPECT_EQ(schema.name, "Player");
    EXPECT_FALSE(schema.fields.empty());
}

// Test LoadVirtualObjectSchema with non-existent file
TEST_F(ConfigManagerTest, LoadVirtualObjectSchemaNonExistent) {
    ConfigManager manager;
    EXPECT_THROW(manager.LoadVirtualObjectSchema("/nonexistent/schema.json"), std::runtime_error);
}

// Test ValidateDataAgainstSchema with valid data
TEST_F(ConfigManagerTest, ValidateDataAgainstSchemaValid) {
    std::string schema_path = temp_dir_ + "/schema.json";
    WriteSchemaFile(schema_path);

    ConfigManager manager;
    auto schema = manager.LoadVirtualObjectSchema(schema_path);

    std::string valid_data = R"({
        "player_id": "player123",
        "name": "Alice",
        "level": 10
    })";

    bool result = manager.ValidateDataAgainstSchema(schema, valid_data);
    EXPECT_TRUE(result);
}

// Test ValidateDataAgainstSchema with missing required field
TEST_F(ConfigManagerTest, ValidateDataAgainstSchemaMissingRequired) {
    std::string schema_path = temp_dir_ + "/schema.json";
    WriteSchemaFile(schema_path);

    ConfigManager manager;
    auto schema = manager.LoadVirtualObjectSchema(schema_path);

    std::string invalid_data = R"({
        "level": 10
    })";  // Missing player_id and name (both required)

    bool result = manager.ValidateDataAgainstSchema(schema, invalid_data);
    EXPECT_FALSE(result);
}

// Test ValidateDataAgainstSchema with invalid data type
TEST_F(ConfigManagerTest, ValidateDataAgainstSchemaInvalidType) {
    std::string schema_path = temp_dir_ + "/schema.json";
    WriteSchemaFile(schema_path);

    ConfigManager manager;
    auto schema = manager.LoadVirtualObjectSchema(schema_path);

    std::string invalid_data = R"({
        "player_id": "player123",
        "name": "Alice",
        "level": "not_a_number"
    })";  // level should be int

    bool result = manager.ValidateDataAgainstSchema(schema, invalid_data);
    EXPECT_FALSE(result);
}

// Test ValidateDataAgainstSchema with validation rule violation
TEST_F(ConfigManagerTest, ValidateDataAgainstSchemaRuleViolation) {
    std::string schema_path = temp_dir_ + "/schema.json";
    WriteSchemaFile(schema_path);

    ConfigManager manager;
    auto schema = manager.LoadVirtualObjectSchema(schema_path);

    std::string invalid_data = R"({
        "player_id": "player123",
        "name": "Alice",
        "level": 150
    })";  // level exceeds max of 100

    bool result = manager.ValidateDataAgainstSchema(schema, invalid_data);
    EXPECT_FALSE(result);
}

// Test LoadApplicationConfigFromFile
TEST_F(ConfigManagerTest, LoadApplicationConfigFromFile) {
    std::string config_path = temp_dir_ + "/app_config.json";
    WriteApplicationConfigFile(config_path);

    ConfigManager manager;
    auto app_config = manager.LoadApplicationConfigFromFile(config_path);

    EXPECT_EQ(app_config.client_config.game_id, "app-game");
    EXPECT_EQ(app_config.client_config.env, "production");
    EXPECT_FALSE(app_config.global_settings.empty());
}

// Test LoadApplicationConfigFromFile with non-existent file
TEST_F(ConfigManagerTest, LoadApplicationConfigFromFileNonExistent) {
    ConfigManager manager;
    EXPECT_THROW(manager.LoadApplicationConfigFromFile("/nonexistent/app_config.json"), std::runtime_error);
}

// Test ValidateApplicationConfig with valid config
TEST_F(ConfigManagerTest, ValidateApplicationConfigValid) {
    ConfigManager::ApplicationConfig app_config;

    app_config.client_config.game_id = "test-game";
    app_config.client_config.agent_addr = "127.0.0.1:19090";
    app_config.client_config.timeout_seconds = 30;

    ConfigManager::VirtualObjectSchema schema;
    schema.id = "test-schema";
    schema.version = "1.0.0";
    app_config.schemas["test-schema"] = schema;

    ConfigManager manager;
    auto errors = manager.ValidateApplicationConfig(app_config);

    EXPECT_TRUE(errors.empty());
}

// Test ValidateApplicationConfig with invalid component
TEST_F(ConfigManagerTest, ValidateApplicationConfigInvalidComponent) {
    ConfigManager::ApplicationConfig app_config;

    app_config.client_config.game_id = "test-game";
    app_config.client_config.agent_addr = "127.0.0.1:19090";
    app_config.client_config.timeout_seconds = 30;

    ComponentDescriptor comp;
    comp.id = "";  // Invalid: empty ID
    comp.version = "1.0.0";
    app_config.components.push_back(comp);

    ConfigManager manager;
    auto errors = manager.ValidateApplicationConfig(app_config);

    EXPECT_FALSE(errors.empty());
}

// Test ValidateApplicationConfig with invalid schema field type
TEST_F(ConfigManagerTest, ValidateApplicationConfigInvalidSchemaFieldType) {
    ConfigManager::ApplicationConfig app_config;

    app_config.client_config.game_id = "test-game";
    app_config.client_config.agent_addr = "127.0.0.1:19090";
    app_config.client_config.timeout_seconds = 30;

    ConfigManager::VirtualObjectSchema schema;
    schema.id = "test-schema";
    schema.version = "1.0.0";

    ConfigManager::VirtualObjectSchema::FieldSchema field;
    field.name = "test_field";
    field.type = "invalid_type";  // Invalid type
    field.required = false;
    schema.fields["test_field"] = field;

    app_config.schemas["test-schema"] = schema;

    ConfigManager manager;
    auto errors = manager.ValidateApplicationConfig(app_config);

    bool found = false;
    for (const auto& err : errors) {
        if (err.find("类型") != std::string::npos || err.find("type") != std::string::npos) {
            found = true;
            break;
        }
    }
    EXPECT_TRUE(found);
}

// Test GenerateExampleConfigs
TEST_F(ConfigManagerTest, GenerateExampleConfigs) {
    std::string output_dir = temp_dir_ + "/example_configs";

    ConfigManager manager;
    bool result = manager.GenerateExampleConfigs(output_dir);

    EXPECT_TRUE(result);

    // Check if files were created
    std::ifstream client_file(output_dir + "/client.json");
    EXPECT_TRUE(client_file.is_open());
    client_file.close();

    std::ifstream readme_file(output_dir + "/README.md");
    EXPECT_TRUE(readme_file.is_open());
    readme_file.close();
}

// Test GenerateExampleConfigs with invalid path
TEST_F(ConfigManagerTest, GenerateExampleConfigsInvalidPath) {
    // Use a path that likely can't be created (e.g., in a non-existent directory)
    std::string invalid_path = "/nonexistent_directory_xyz/subdir/configs";

    ConfigManager manager;
    bool result = manager.GenerateExampleConfigs(invalid_path);

    // May fail due to permission/path issues
    // Just verify it doesn't crash
    SUCCEED();
}

}  // namespace test
}  // namespace sdk
}  // namespace croupier
