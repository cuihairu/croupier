#include <gtest/gtest.h>

#include "croupier/sdk/config_driven_loader.h"
#include "croupier/sdk/croupier_client.h"

#include <fstream>
#include <numeric>

namespace croupier {
namespace sdk {
namespace test {

class ConfigDrivenLoaderTest : public ::testing::Test {
protected:
    void SetUp() override {
        // Create a temporary config file for testing
        temp_config_file_ = "/tmp/test_component_" + std::to_string(std::rand()) + ".json";
    }

    void TearDown() override {
        // Clean up temp file
        std::remove(temp_config_file_.c_str());
    }

    void WriteValidConfigFile() {
        std::ofstream file(temp_config_file_);
        file << R"({
            "component": {
                "id": "test-component",
                "version": "1.0.0",
                "name": "Test Component",
                "description": "A test component"
            },
            "entities": []
        })";
        file.close();
    }

    void WriteInvalidConfigFile() {
        std::ofstream file(temp_config_file_);
        file << R"({ invalid json })";
        file.close();
    }

    void WriteConfigWithEntities() {
        std::ofstream file(temp_config_file_);
        file << R"({
            "component": {
                "id": "game-component",
                "version": "1.0.0",
                "name": "Game Component"
            },
            "entities": [
                {
                    "id": "player",
                    "version": "1.0.0",
                    "name": "Player",
                    "operations": {
                        "create": "player.create",
                        "get": "player.get"
                    }
                }
            ]
        })";
        file.close();
    }

    std::string temp_config_file_;
};

// Test default constructor
TEST_F(ConfigDrivenLoaderTest, DefaultConstructor) {
    ConfigDrivenLoader loader;
    EXPECT_NO_THROW(loader.GetRegisteredHandlers());
}

// Test RegisterHandlerFactory
TEST_F(ConfigDrivenLoaderTest, RegisterHandlerFactory) {
    ConfigDrivenLoader loader;

    loader.RegisterHandlerFactory("test.", [](const std::string& id, const auto&) {
        return [id](const std::string&, const std::string&) -> std::string {
            return "{\"function_id\":\"" + id + "\"}";
        };
    });

    auto handlers = loader.GetRegisteredHandlers();
    EXPECT_FALSE(handlers.empty());
}

// Test RegisterHandler
TEST_F(ConfigDrivenLoaderTest, RegisterHandler) {
    ConfigDrivenLoader loader;

    FunctionHandler handler = [](const std::string&, const std::string&) -> std::string {
        return "{\"status\":\"ok\"}";
    };

    loader.RegisterHandler("test.function", handler);

    auto handlers = loader.GetRegisteredHandlers();
    EXPECT_FALSE(handlers.empty());
}

// Test GetHandler for directly registered handler
TEST_F(ConfigDrivenLoaderTest, GetHandlerDirect) {
    ConfigDrivenLoader loader;

    FunctionHandler handler = [](const std::string&, const std::string&) -> std::string {
        return "{\"status\":\"ok\"}";
    };

    loader.RegisterHandler("test.function", handler);

    auto retrieved = loader.GetHandler("test.function", {});
    EXPECT_NE(retrieved, nullptr);
}

// Test GetHandler from factory
TEST_F(ConfigDrivenLoaderTest, GetHandlerFromFactory) {
    ConfigDrivenLoader loader;

    loader.RegisterHandlerFactory("factory.", [](const std::string& id, const auto&) {
        return [id](const std::string&, const std::string&) -> std::string {
            return "{\"function_id\":\"" + id + "\"}";
        };
    });

    auto retrieved = loader.GetHandler("factory.test", {});
    EXPECT_NE(retrieved, nullptr);
}

// Test GetHandler not found
TEST_F(ConfigDrivenLoaderTest, GetHandlerNotFound) {
    ConfigDrivenLoader loader;

    auto retrieved = loader.GetHandler("nonexistent.function", {});
    EXPECT_EQ(retrieved, nullptr);
}

// Test HasHandler
TEST_F(ConfigDrivenLoaderTest, HasHandler) {
    ConfigDrivenLoader loader;

    FunctionHandler handler = [](const std::string&, const std::string&) -> std::string {
        return "{}";
    };

    EXPECT_FALSE(loader.HasHandler("test.function"));

    loader.RegisterHandler("test.function", handler);
    EXPECT_TRUE(loader.HasHandler("test.function"));
}

// Test HasHandler with factory prefix
TEST_F(ConfigDrivenLoaderTest, HasHandlerWithFactory) {
    ConfigDrivenLoader loader;

    loader.RegisterHandlerFactory("factory.", [](const std::string&, const auto&) {
        return [](const std::string&, const std::string&) -> std::string {
            return "{}";
        };
    });

    EXPECT_TRUE(loader.HasHandler("factory.test"));
    EXPECT_FALSE(loader.HasHandler("other.test"));
}

// Test SetDynamicLibLoader
TEST_F(ConfigDrivenLoaderTest, SetDynamicLibLoader) {
    ConfigDrivenLoader loader;

    ConfigDrivenLoader::DynamicLibLoader custom_loader =
        [](const std::string&, const std::string&) -> FunctionHandler {
        return [](const std::string&, const std::string&) -> std::string {
            return "{}";
        };
    };

    EXPECT_NO_THROW(loader.SetDynamicLibLoader(custom_loader));
}

// Test LoadComponentFromFile with valid file
TEST_F(ConfigDrivenLoaderTest, LoadComponentFromFileValid) {
    WriteValidConfigFile();

    ConfigDrivenLoader loader;
    ComponentDescriptor comp = loader.LoadComponentFromFile(temp_config_file_);

    EXPECT_EQ(comp.id, "test-component");
    EXPECT_EQ(comp.version, "1.0.0");
    EXPECT_EQ(comp.name, "Test Component");
    EXPECT_EQ(comp.description, "A test component");
}

// Test LoadComponentFromFile with non-existent file
TEST_F(ConfigDrivenLoaderTest, LoadComponentFromFileNonExistent) {
    ConfigDrivenLoader loader;

    EXPECT_THROW(loader.LoadComponentFromFile("/nonexistent/file.json"), std::runtime_error);
}

// Test LoadComponentFromFile with invalid JSON
TEST_F(ConfigDrivenLoaderTest, LoadComponentFromFileInvalidJson) {
    WriteInvalidConfigFile();

    ConfigDrivenLoader loader;
    EXPECT_THROW(loader.LoadComponentFromFile(temp_config_file_), std::runtime_error);
}

// Test LoadComponentFromJson with valid JSON
TEST_F(ConfigDrivenLoaderTest, LoadComponentFromJsonValid) {
    std::string json = R"({
        "component": {
            "id": "json-component",
            "version": "2.0.0"
        }
    })";

    ConfigDrivenLoader loader;
    ComponentDescriptor comp = loader.LoadComponentFromJson(json);

    EXPECT_EQ(comp.id, "json-component");
    EXPECT_EQ(comp.version, "2.0.0");
}

// Test LoadComponentFromJson with invalid JSON
TEST_F(ConfigDrivenLoaderTest, LoadComponentFromJsonInvalid) {
    std::string invalid_json = R"({ not valid json })";

    ConfigDrivenLoader loader;
    EXPECT_THROW(loader.LoadComponentFromJson(invalid_json), std::runtime_error);
}

// Test ValidateConfigFile with valid file
TEST_F(ConfigDrivenLoaderTest, ValidateConfigFileValid) {
    WriteValidConfigFile();

    ConfigDrivenLoader loader;
    EXPECT_TRUE(loader.ValidateConfigFile(temp_config_file_));
}

// Test ValidateConfigFile with invalid file
TEST_F(ConfigDrivenLoaderTest, ValidateConfigFileInvalid) {
    WriteInvalidConfigFile();

    ConfigDrivenLoader loader;
    EXPECT_FALSE(loader.ValidateConfigFile(temp_config_file_));
}

// Test ValidateConfigFile non-existent
TEST_F(ConfigDrivenLoaderTest, ValidateConfigFileNonExistent) {
    ConfigDrivenLoader loader;
    EXPECT_FALSE(loader.ValidateConfigFile("/nonexistent/file.json"));
}

// Test ValidateJsonConfig valid
TEST_F(ConfigDrivenLoaderTest, ValidateJsonConfigValid) {
    std::string json = R"({
        "component": {
            "id": "test",
            "version": "1.0.0"
        }
    })";

    ConfigDrivenLoader loader;
    EXPECT_TRUE(loader.ValidateJsonConfig(json));
}

// Test ValidateJsonConfig missing component
TEST_F(ConfigDrivenLoaderTest, ValidateJsonConfigMissingComponent) {
    std::string json = R"({"id": "test"})";

    ConfigDrivenLoader loader;
    EXPECT_FALSE(loader.ValidateJsonConfig(json));
}

// Test ParseJsonToComponent with entities
TEST_F(ConfigDrivenLoaderTest, ParseJsonToComponentWithEntities) {
    WriteConfigWithEntities();

    ConfigDrivenLoader loader;
    ComponentDescriptor comp = loader.LoadComponentFromFile(temp_config_file_);

    EXPECT_EQ(comp.id, "game-component");
    EXPECT_EQ(comp.entities.size(), 1);
    EXPECT_EQ(comp.entities[0].id, "player");
    EXPECT_EQ(comp.entities[0].operations["create"], "player.create");
    EXPECT_EQ(comp.entities[0].operations["get"], "player.get");
}

// Test ParseJsonToVirtualObject
TEST_F(ConfigDrivenLoaderTest, ParseJsonToVirtualObject) {
    std::string json = R"({
        "id": "item",
        "version": "1.0.0",
        "name": "Item",
        "operations": {
            "create": "item.create",
            "delete": "item.delete"
        },
        "relationships": {
            "owner": {
                "type": "many-to-one",
                "entity": "player",
                "foreign_key": "player_id"
            }
        }
    })";

    ConfigDrivenLoader loader;
    VirtualObjectDescriptor obj = loader.ParseJsonToVirtualObject(json);

    EXPECT_EQ(obj.id, "item");
    EXPECT_EQ(obj.version, "1.0.0");
    EXPECT_EQ(obj.name, "Item");
    EXPECT_EQ(obj.operations["create"], "item.create");
    EXPECT_EQ(obj.operations["delete"], "item.delete");
    EXPECT_EQ(obj.relationships["owner"].type, "many-to-one");
    EXPECT_EQ(obj.relationships["owner"].entity, "player");
    EXPECT_EQ(obj.relationships["owner"].foreign_key, "player_id");
}

// Test CreateHandlerFromConfig echo type
TEST_F(ConfigDrivenLoaderTest, CreateHandlerFromConfigEcho) {
    ConfigDrivenLoader loader;

    std::map<std::string, std::string> config = {
        {"type", "echo"}
    };

    auto handler = loader.CreateHandlerFromConfig("test.echo", config);
    EXPECT_NE(handler, nullptr);

    std::string result = handler("test-context", R"({"data":"test"})");
    EXPECT_NE(result.find("\"type\": \"echo\""), std::string::npos);
    EXPECT_NE(result.find("test-context"), std::string::npos);
}

// Test CreateHandlerFromConfig error type
TEST_F(ConfigDrivenLoaderTest, CreateHandlerFromConfigError) {
    ConfigDrivenLoader loader;

    std::map<std::string, std::string> config = {
        {"type", "error"},
        {"message", "Custom error message"}
    };

    auto handler = loader.CreateHandlerFromConfig("test.error", config);
    EXPECT_NE(handler, nullptr);

    std::string result = handler("context", "{}");
    EXPECT_NE(result.find("Custom error message"), std::string::npos);
}

// Test CreateHandlerFromConfig proxy type
TEST_F(ConfigDrivenLoaderTest, CreateHandlerFromConfigProxy) {
    ConfigDrivenLoader loader;

    std::map<std::string, std::string> config = {
        {"type", "proxy"},
        {"target_url", "http://example.com/api"}
    };

    auto handler = loader.CreateHandlerFromConfig("test.proxy", config);
    EXPECT_NE(handler, nullptr);

    std::string result = handler("context", "{}");
    EXPECT_NE(result.find("http://example.com/api"), std::string::npos);
}

// Test CreateHandlerFromConfig template type
TEST_F(ConfigDrivenLoaderTest, CreateHandlerFromConfigTemplate) {
    ConfigDrivenLoader loader;

    std::map<std::string, std::string> config = {
        {"type", "template"},
        {"template", R"( {"result": "{{context}}:{{payload}"} })"}
    };

    auto handler = loader.CreateHandlerFromConfig("test.template", config);
    EXPECT_NE(handler, nullptr);

    std::string result = handler("mycontext", "mypayload");
    // Debug output to see actual result
    std::cout << "DEBUG: template handler result = [" << result << "]" << std::endl;
    std::cout << "DEBUG: looking for [mycontext:mypayload]" << std::endl;
    EXPECT_NE(result.find("mycontext:mypayload"), std::string::npos);
}

// Test CreateHandlerFromConfig unknown type
TEST_F(ConfigDrivenLoaderTest, CreateHandlerFromConfigUnknownType) {
    ConfigDrivenLoader loader;

    std::map<std::string, std::string> config = {
        {"type", "unknown_type"}
    };

    auto handler = loader.CreateHandlerFromConfig("test.unknown", config);
    EXPECT_EQ(handler, nullptr);
}

// Test CreateHandlerFromConfig without type
TEST_F(ConfigDrivenLoaderTest, CreateHandlerFromConfigNoType) {
    ConfigDrivenLoader loader;

    std::map<std::string, std::string> config = {
        {"other_field", "value"}
    };

    auto handler = loader.CreateHandlerFromConfig("test.notype", config);
    EXPECT_EQ(handler, nullptr);
}

// Test CreateDefaultHandler
TEST_F(ConfigDrivenLoaderTest, CreateDefaultHandler) {
    ConfigDrivenLoader loader;

    auto handler = loader.CreateDefaultHandler("test.function");
    EXPECT_NE(handler, nullptr);

    std::string result = handler("context", "payload");
    EXPECT_NE(result.find("test.function"), std::string::npos);
    EXPECT_NE(result.find("not_implemented"), std::string::npos);
}

// Test BasicHandlerFactory CreateEchoHandler
TEST_F(ConfigDrivenLoaderTest, BasicHandlerFactoryCreateEcho) {
    std::map<std::string, std::string> config;

    auto handler = BasicHandlerFactory::CreateEchoHandler(config);
    EXPECT_NE(handler, nullptr);

    std::string result = handler("ctx", R"({"msg":"hello"})");
    EXPECT_NE(result.find("echo"), std::string::npos);
    EXPECT_NE(result.find("ctx"), std::string::npos);
}

// Test BasicHandlerFactory CreateErrorHandler
TEST_F(ConfigDrivenLoaderTest, BasicHandlerFactoryCreateError) {
    std::string error_msg = "Test error";
    auto handler = BasicHandlerFactory::CreateErrorHandler(error_msg);
    EXPECT_NE(handler, nullptr);

    std::string result = handler("ctx", "payload");
    EXPECT_NE(result.find("Test error"), std::string::npos);
}

// Test BasicHandlerFactory CreateProxyHandler
TEST_F(ConfigDrivenLoaderTest, BasicHandlerFactoryCreateProxy) {
    std::string target_url = "http://example.com";
    std::map<std::string, std::string> config;

    auto handler = BasicHandlerFactory::CreateProxyHandler(target_url, config);
    EXPECT_NE(handler, nullptr);

    std::string result = handler("ctx", "payload");
    EXPECT_NE(result.find(target_url), std::string::npos);
}

// Test BasicHandlerFactory CreateTemplateHandler
TEST_F(ConfigDrivenLoaderTest, BasicHandlerFactoryCreateTemplate) {
    std::string tmpl = R"({"user":"{{context}}","data":"{{payload}}"})";
    std::map<std::string, std::string> config;

    auto handler = BasicHandlerFactory::CreateTemplateHandler(tmpl, config);
    EXPECT_NE(handler, nullptr);

    std::string result = handler("alice", R"({"gold":100})");
    EXPECT_NE(result.find("alice"), std::string::npos);
    EXPECT_NE(result.find("gold"), std::string::npos);
}

// Test LoadAndRegisterComponent with valid config
TEST_F(ConfigDrivenLoaderTest, LoadAndRegisterComponentValid) {
    WriteConfigWithEntities();

    ClientConfig client_config;
    client_config.game_id = "test-game";
    client_config.env = "testing";
    client_config.agent_addr = "";
    client_config.disable_logging = true;

    CroupierClient client(client_config);
    ConfigDrivenLoader loader;

    // Register a handler for one of the functions
    loader.RegisterHandler("player.create", [](const std::string&, const std::string&) -> std::string {
        return R"({"status":"created"})";
    });

    bool success = loader.LoadAndRegisterComponent(client, temp_config_file_);
    EXPECT_TRUE(success);
}

// Test LoadAndRegisterComponent with non-existent file
TEST_F(ConfigDrivenLoaderTest, LoadAndRegisterComponentNonExistent) {
    ClientConfig client_config;
    client_config.game_id = "test-game";
    client_config.env = "testing";
    client_config.agent_addr = "";
    client_config.disable_logging = true;

    CroupierClient client(client_config);
    ConfigDrivenLoader loader;

    bool success = loader.LoadAndRegisterComponent(client, "/nonexistent/file.json");
    EXPECT_FALSE(success);
}

// Test ResolveHandlers with empty component
TEST_F(ConfigDrivenLoaderTest, ResolveHandlersEmptyComponent) {
    ConfigDrivenLoader loader;
    ComponentDescriptor comp;
    comp.id = "empty-comp";
    comp.version = "1.0.0";

    auto handlers = loader.ResolveHandlers(comp);
    EXPECT_TRUE(handlers.empty());
}

// Test GetRegisteredHandlers
TEST_F(ConfigDrivenLoaderTest, GetRegisteredHandlers) {
    ConfigDrivenLoader loader;

    auto handlers = loader.GetRegisteredHandlers();
    EXPECT_TRUE(handlers.empty());

    loader.RegisterHandler("func1", [](const std::string&, const std::string&) -> std::string {
        return "{}";
    });

    loader.RegisterHandlerFactory("factory.", [](const std::string&, const auto&) {
        return [](const std::string&, const std::string&) -> std::string {
            return "{}";
        };
    });

    handlers = loader.GetRegisteredHandlers();
    EXPECT_GE(handlers.size(), 2);
}

}  // namespace test
}  // namespace sdk
}  // namespace croupier
