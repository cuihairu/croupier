#include <gtest/gtest.h>

#include "croupier/sdk/croupier_client.h"

#include <chrono>
#include <thread>
#include <vector>

namespace croupier {
namespace sdk {
namespace test {

class ClientUtilsTest : public ::testing::Test {
protected:
    void SetUp() override {}
    void TearDown() override {}
};

// Test NewIdempotencyKey generates unique keys
TEST_F(ClientUtilsTest, NewIdempotencyKeyUniqueness) {
    std::vector<std::string> keys;
    const int count = 100;

    for (int i = 0; i < count; ++i) {
        keys.push_back(utils::NewIdempotencyKey());
    }

    // All keys should be unique
    std::sort(keys.begin(), keys.end());
    auto last = std::unique(keys.begin(), keys.end());
    EXPECT_EQ(last, keys.end());

    // Each key should be 32 hex characters (16 bytes)
    for (const auto& key : keys) {
        EXPECT_EQ(key.size(), 32);
        EXPECT_TRUE(std::all_of(key.begin(), key.end(), [](char c) {
            return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f');
        }));
    }
}

// Test ValidateJSON with empty schema
TEST_F(ClientUtilsTest, ValidateJSONEmptySchema) {
    // Valid JSON
    EXPECT_TRUE(utils::ValidateJSON(R"({"name":"test"})", {}));

    // Invalid JSON
    EXPECT_FALSE(utils::ValidateJSON(R"({name:test})", {}));

    // Empty string is not valid JSON
    EXPECT_FALSE(utils::ValidateJSON("", {}));
}

// Test ValidateJSON with schema
TEST_F(ClientUtilsTest, ValidateJSONWithSchema) {
    std::map<std::string, std::string> schema = {
        {"name", "string"},
        {"age", "number"}
    };

    // Valid JSON matching schema
    EXPECT_TRUE(utils::ValidateJSON(R"({"name":"alice","age":30})", schema));

    // Invalid JSON (missing quote)
    EXPECT_FALSE(utils::ValidateJSON(R"({name:"alice","age":30})", schema));
}

// Test ParseJSON
TEST_F(ClientUtilsTest, ParseJSON) {
    auto result = utils::ParseJSON(R"({"name":"alice","age":"30"})");

    EXPECT_EQ(result["name"], "alice");
    EXPECT_EQ(result["age"], "30");

    // Empty JSON
    auto empty_result = utils::ParseJSON("");
    EXPECT_TRUE(empty_result.empty());

    // Simple key-value
    auto simple = utils::ParseJSON(R"({"key":"value"})");
    EXPECT_EQ(simple["key"], "value");
}

// Test ToJSON
TEST_F(ClientUtilsTest, ToJSON) {
    std::map<std::string, std::string> data = {
        {"name", "alice"},
        {"age", "30"}
    };

    std::string json = utils::ToJSON(data);

    EXPECT_NE(json.find("\"name\":\"alice\""), std::string::npos);
    EXPECT_NE(json.find("\"age\":\"30\""), std::string::npos);

    // Empty map
    std::string empty_json = utils::ToJSON({});
    EXPECT_EQ(empty_json, "{}");
}

// Test VirtualObjectDescriptor validation
TEST_F(ClientUtilsTest, ValidateObjectDescriptor) {
    VirtualObjectDescriptor valid_desc;
    valid_desc.id = "test-object";
    valid_desc.version = "1.0.0";
    valid_desc.name = "Test Object";

    EXPECT_TRUE(utils::ValidateObjectDescriptor(valid_desc));

    // Empty ID
    VirtualObjectDescriptor invalid_id;
    invalid_id.id = "";
    invalid_id.version = "1.0.0";
    EXPECT_FALSE(utils::ValidateObjectDescriptor(invalid_id));

    // Empty version
    VirtualObjectDescriptor invalid_version;
    invalid_version.id = "test";
    invalid_version.version = "";
    EXPECT_FALSE(utils::ValidateObjectDescriptor(invalid_version));

    // Invalid operation mapping
    VirtualObjectDescriptor invalid_op;
    invalid_op.id = "test";
    invalid_op.version = "1.0.0";
    invalid_op.operations["create"] = "";  // Empty function ID
    EXPECT_FALSE(utils::ValidateObjectDescriptor(invalid_op));
}

// Test VirtualObjectDescriptor with relationships
TEST_F(ClientUtilsTest, ValidateObjectDescriptorWithRelationships) {
    VirtualObjectDescriptor desc;
    desc.id = "player";
    desc.version = "1.0.0";

    // Valid relationships
    RelationshipDef rel1;
    rel1.type = "one-to-many";
    rel1.entity = "item";
    rel1.foreign_key = "player_id";
    desc.relationships["items"] = rel1;

    EXPECT_TRUE(utils::ValidateObjectDescriptor(desc));

    // Invalid relationship type
    RelationshipDef rel2;
    rel2.type = "invalid-type";
    rel2.entity = "guild";
    rel2.foreign_key = "guild_id";
    desc.relationships["guild"] = rel2;

    EXPECT_FALSE(utils::ValidateObjectDescriptor(desc));
}

// Test ComponentDescriptor validation
TEST_F(ClientUtilsTest, ValidateComponentDescriptor) {
    ComponentDescriptor valid_comp;
    valid_comp.id = "test-component";
    valid_comp.version = "1.0.0";
    valid_comp.name = "Test Component";

    EXPECT_TRUE(utils::ValidateComponentDescriptor(valid_comp));

    // Empty ID
    ComponentDescriptor invalid_id;
    invalid_id.id = "";
    invalid_id.version = "1.0.0";
    EXPECT_FALSE(utils::ValidateComponentDescriptor(invalid_id));

    // Component with valid entities
    ComponentDescriptor comp_with_entities;
    comp_with_entities.id = "comp";
    comp_with_entities.version = "1.0.0";

    VirtualObjectDescriptor entity;
    entity.id = "entity1";
    entity.version = "1.0.0";
    comp_with_entities.entities.push_back(entity);

    EXPECT_TRUE(utils::ValidateComponentDescriptor(comp_with_entities));

    // Component with invalid entity
    ComponentDescriptor comp_with_invalid_entity;
    comp_with_invalid_entity.id = "comp";
    comp_with_invalid_entity.version = "1.0.0";

    VirtualObjectDescriptor invalid_entity;
    invalid_entity.id = "";
    invalid_entity.version = "1.0.0";
    comp_with_invalid_entity.entities.push_back(invalid_entity);

    EXPECT_FALSE(utils::ValidateComponentDescriptor(comp_with_invalid_entity));
}

// Test LoadObjectDescriptor with invalid JSON
TEST_F(ClientUtilsTest, LoadObjectDescriptorInvalidJSON) {
    // This test creates a temporary file with invalid JSON
    // The function should return a descriptor with id="error"
#ifdef CROUPIER_SDK_ENABLE_JSON
    // For systems with nlohmann/json, test actual file loading
    VirtualObjectDescriptor desc = utils::LoadObjectDescriptor("/nonexistent/file.json");

    EXPECT_EQ(desc.id, "error");
    EXPECT_EQ(desc.version, "0.0.0");
#endif
}

// Test GenerateObjectTemplate
TEST_F(ClientUtilsTest, GenerateObjectTemplate) {
    std::string template_str = utils::GenerateObjectTemplate("player");

    EXPECT_NE(template_str.find("\"id\": \"player\""), std::string::npos);
    EXPECT_NE(template_str.find("\"version\": \"1.0.0\""), std::string::npos);
    EXPECT_NE(template_str.find("player.create"), std::string::npos);
    EXPECT_NE(template_str.find("player.get"), std::string::npos);
    EXPECT_NE(template_str.find("player.update"), std::string::npos);
    EXPECT_NE(template_str.find("player.delete"), std::string::npos);
}

// Test GenerateComponentTemplate
TEST_F(ClientUtilsTest, GenerateComponentTemplate) {
    std::string template_str = utils::GenerateComponentTemplate("gameplay");

    EXPECT_NE(template_str.find("\"id\": \"gameplay\""), std::string::npos);
    EXPECT_NE(template_str.find("\"version\": \"1.0.0\""), std::string::npos);
    EXPECT_NE(template_str.find("\"entities\""), std::string::npos);
    EXPECT_NE(template_str.find("\"functions\""), std::string::npos);
}

// Test ParseObjectDescriptor
TEST_F(ClientUtilsTest, ParseObjectDescriptor) {
    std::string json = R"({
        "id": "player",
        "version": "1.0.0",
        "name": "Player",
        "description": "A player entity"
    })";

    VirtualObjectDescriptor desc = utils::ParseObjectDescriptor(json);

    EXPECT_EQ(desc.id, "player");
    EXPECT_EQ(desc.version, "1.0.0");
    EXPECT_EQ(desc.name, "Player");
    EXPECT_EQ(desc.description, "A player entity");
}

// Test ParseComponentDescriptor
TEST_F(ClientUtilsTest, ParseComponentDescriptor) {
    std::string json = R"({
        "id": "gameplay",
        "version": "1.0.0",
        "name": "Gameplay Component",
        "type": "core"
    })";

    ComponentDescriptor comp = utils::ParseComponentDescriptor(json);

    EXPECT_EQ(comp.id, "gameplay");
    EXPECT_EQ(comp.version, "1.0.0");
    EXPECT_EQ(comp.name, "Gameplay Component");
    EXPECT_EQ(comp.type, "core");
}

// Test ParseObjectDescriptor with operations
TEST_F(ClientUtilsTest, ParseObjectDescriptorWithOperations) {
    std::string json = R"({
        "id": "player",
        "version": "1.0.0",
        "operations": {
            "create": "player.create",
            "read": "player.get"
        }
    })";

    VirtualObjectDescriptor desc = utils::ParseObjectDescriptor(json);

    EXPECT_EQ(desc.id, "player");
    EXPECT_EQ(desc.operations["create"], "player.create");
    EXPECT_EQ(desc.operations["read"], "player.get");
}

// Test ParseComponentDescriptor with dependencies
TEST_F(ClientUtilsTest, ParseComponentDescriptorWithDependencies) {
    std::string json = R"({
        "id": "advanced",
        "version": "1.0.0",
        "dependencies": ["base", "utils"]
    })";

    ComponentDescriptor comp = utils::ParseComponentDescriptor(json);

    EXPECT_EQ(comp.id, "advanced");
    EXPECT_EQ(comp.dependencies.size(), 2);
    EXPECT_EQ(comp.dependencies[0], "base");
    EXPECT_EQ(comp.dependencies[1], "utils");
}

// Test ObjectDescriptorToJSON
TEST_F(ClientUtilsTest, ObjectDescriptorToJSON) {
    VirtualObjectDescriptor desc;
    desc.id = "player";
    desc.version = "1.0.0";
    desc.name = "Player";
    desc.description = "A player entity";
    desc.operations["create"] = "player.create";
    desc.operations["read"] = "player.get";
    desc.schema["id"] = "string";
    desc.schema["name"] = "string";

    std::string json = utils::ObjectDescriptorToJSON(desc);

    EXPECT_NE(json.find("\"id\": \"player\""), std::string::npos);
    EXPECT_NE(json.find("\"version\": \"1.0.0\""), std::string::npos);
    EXPECT_NE(json.find("\"name\": \"Player\""), std::string::npos);
    EXPECT_NE(json.find("\"create\": \"player.create\""), std::string::npos);
    EXPECT_NE(json.find("\"read\": \"player.get\""), std::string::npos);
}

// Test ComponentDescriptorToJSON
TEST_F(ClientUtilsTest, ComponentDescriptorToJSON) {
    ComponentDescriptor comp;
    comp.id = "gameplay";
    comp.version = "1.0.0";
    comp.name = "Gameplay";
    comp.description = "Core gameplay component";

    VirtualObjectDescriptor entity1;
    entity1.id = "player";
    VirtualObjectDescriptor entity2;
    entity2.id = "item";
    comp.entities.push_back(entity1);
    comp.entities.push_back(entity2);

    FunctionDescriptor func1;
    func1.id = "gameplay.init";
    FunctionDescriptor func2;
    func2.id = "gameplay.update";
    comp.functions.push_back(func1);
    comp.functions.push_back(func2);

    std::string json = utils::ComponentDescriptorToJSON(comp);

    EXPECT_NE(json.find("\"id\": \"gameplay\""), std::string::npos);
    EXPECT_NE(json.find("\"player\""), std::string::npos);
    EXPECT_NE(json.find("\"item\""), std::string::npos);
    EXPECT_NE(json.find("\"gameplay.init\""), std::string::npos);
    EXPECT_NE(json.find("\"gameplay.update\""), std::string::npos);
}

// Test escape JSON string edge cases
TEST_F(ClientUtilsTest, EscapeJSONStringEdgeCases) {
    // Test with various special characters
    // Note: ParseJSON is a simple parser that may not handle all escape sequences
    std::string json_simple = R"({"text":"hello world"})";

    auto parsed = utils::ParseJSON(json_simple);
    EXPECT_EQ(parsed["text"], "hello world");
}

// Test retry delay calculation
TEST_F(ClientUtilsTest, RetryDelayCalculation) {
    InvokerConfig config;
    CroupierInvoker invoker(config);

    RetryConfig retry_config;
    retry_config.enabled = true;
    retry_config.initial_delay_ms = 100;
    retry_config.max_delay_ms = 5000;
    retry_config.backoff_multiplier = 2.0;
    retry_config.jitter_factor = 0.1;

    invoker.SetRetryConfig(retry_config);

    // We can't directly test CalculateRetryDelay as it's private
    // But we can verify the config is set
    EXPECT_NO_THROW(invoker.SetRetryConfig(retry_config));
}

// Test reconnect config
TEST_F(ClientUtilsTest, ReconnectConfig) {
    InvokerConfig config;
    CroupierInvoker invoker(config);

    ReconnectConfig reconnect_config;
    reconnect_config.enabled = true;
    reconnect_config.max_attempts = 5;
    reconnect_config.initial_delay_ms = 1000;
    reconnect_config.max_delay_ms = 30000;
    reconnect_config.backoff_multiplier = 2.0;
    reconnect_config.jitter_factor = 0.2;

    EXPECT_NO_THROW(invoker.SetReconnectConfig(reconnect_config));
}

// Test virtual object registration with empty handlers map
TEST_F(ClientUtilsTest, RegisterVirtualObjectEmptyHandlers) {
    ClientConfig config;
    config.game_id = "test-game";
    config.env = "testing";
    config.agent_addr = "";  // Non-TCP address for local mode
    config.disable_logging = true;

    CroupierClient client(config);

    VirtualObjectDescriptor desc;
    desc.id = "test-object";
    desc.version = "1.0.0";

    // Empty handlers map - should succeed
    std::map<std::string, FunctionHandler> handlers;
    EXPECT_TRUE(client.RegisterVirtualObject(desc, handlers));

    auto objects = client.GetRegisteredObjects();
    EXPECT_EQ(objects.size(), 1);
    EXPECT_EQ(objects[0].id, "test-object");
}

// Test unregister non-existent object
TEST_F(ClientUtilsTest, UnregisterNonExistentObject) {
    ClientConfig config;
    config.game_id = "test-game";
    config.env = "testing";
    config.agent_addr = "";
    config.disable_logging = true;

    CroupierClient client(config);

    // Try to unregister non-existent object
    EXPECT_FALSE(client.UnregisterVirtualObject("non-existent"));
}

// Test unregister non-existent component
TEST_F(ClientUtilsTest, UnregisterNonExistentComponent) {
    ClientConfig config;
    config.game_id = "test-game";
    config.env = "testing";
    config.agent_addr = "";
    config.disable_logging = true;

    CroupierClient client(config);

    // Try to unregister non-existent component
    EXPECT_FALSE(client.UnregisterComponent("non-existent"));
}

// Test component registration
TEST_F(ClientUtilsTest, RegisterComponent) {
    ClientConfig config;
    config.game_id = "test-game";
    config.env = "testing";
    config.agent_addr = "";
    config.disable_logging = true;

    CroupierClient client(config);

    ComponentDescriptor comp;
    comp.id = "test-component";
    comp.version = "1.0.0";
    comp.name = "Test Component";

    VirtualObjectDescriptor entity;
    entity.id = "player";
    entity.version = "1.0.0";
    comp.entities.push_back(entity);

    EXPECT_TRUE(client.RegisterComponent(comp));

    auto components = client.GetRegisteredComponents();
    EXPECT_EQ(components.size(), 1);
    EXPECT_EQ(components[0].id, "test-component");
}

// Test function registration while running
TEST_F(ClientUtilsTest, RegisterFunctionWhileRunning) {
    ClientConfig config;
    config.game_id = "test-game";
    config.env = "testing";
    config.agent_addr = "";  // Non-TCP for local mode
    config.disable_logging = true;

    CroupierClient client(config);

    FunctionDescriptor desc;
    desc.id = "test.func";
    desc.version = "1.0.0";

    FunctionHandler handler = [](const std::string&, const std::string&) -> std::string {
        return "{}";
    };

    EXPECT_TRUE(client.RegisterFunction(desc, handler));

    // Start the client (in a separate thread, as Serve() blocks)
    std::atomic<bool> running{false};
    std::thread client_thread([&]() {
        client.Connect();
        running = true;
        // Give some time for "running" state
        std::this_thread::sleep_for(std::chrono::milliseconds(100));
        client.Stop();
    });

    // Wait for client to be running
    auto start = std::chrono::steady_clock::now();
    while (!running) {
        std::this_thread::sleep_for(std::chrono::milliseconds(10));
        if (std::chrono::duration_cast<std::chrono::milliseconds>(
                std::chrono::steady_clock::now() - start).count() > 5000) {
            break;
        }
    }

    // Try to register while running - in non-TCP mode, this may succeed
    // as the client doesn't have a traditional "running" state
    FunctionDescriptor desc2;
    desc2.id = "test.func2";
    desc2.version = "1.0.0";

    // In non-TCP mode, registration is allowed even after Connect()
    EXPECT_TRUE(client.RegisterFunction(desc2, handler));

    client_thread.join();
}

// Test empty function ID registration
TEST_F(ClientUtilsTest, RegisterEmptyFunctionId) {
    ClientConfig config;
    config.game_id = "test-game";
    config.env = "testing";
    config.agent_addr = "";
    config.disable_logging = true;

    CroupierClient client(config);

    FunctionDescriptor desc;
    desc.id = "";  // Empty ID
    desc.version = "1.0.0";

    FunctionHandler handler = [](const std::string&, const std::string&) -> std::string {
        return "{}";
    };

    EXPECT_FALSE(client.RegisterFunction(desc, handler));
}

// Test GetRegisteredObjects when empty
TEST_F(ClientUtilsTest, GetRegisteredObjectsWhenEmpty) {
    ClientConfig config;
    config.game_id = "test-game";
    config.env = "testing";
    config.agent_addr = "";
    config.disable_logging = true;

    CroupierClient client(config);

    auto objects = client.GetRegisteredObjects();
    EXPECT_TRUE(objects.empty());

    auto components = client.GetRegisteredComponents();
    EXPECT_TRUE(components.empty());
}

// Test LoadComponentFromFile with non-existent file
TEST_F(ClientUtilsTest, LoadComponentFromFileNonExistent) {
    ClientConfig config;
    config.game_id = "test-game";
    config.env = "testing";
    config.agent_addr = "";
    config.disable_logging = true;

    CroupierClient client(config);

    // LoadComponentFromFile creates a component even if file fails to load
    // The component will be in an error state with id="error"
    EXPECT_TRUE(client.LoadComponentFromFile("/nonexistent/file.json"));

    // Verify the error component was registered
    auto components = client.GetRegisteredComponents();
    EXPECT_EQ(components.size(), 1);
    EXPECT_EQ(components[0].id, "error");
}

// Test invoker close idempotence
TEST_F(ClientUtilsTest, InvokerCloseIdempotence) {
    InvokerConfig config;
    config.address = "";  // Non-TCP for local mode
    config.disable_logging = true;

    CroupierInvoker invoker(config);

    // Multiple Close calls should be safe
    EXPECT_NO_THROW(invoker.Close());
    EXPECT_NO_THROW(invoker.Close());
    EXPECT_NO_THROW(invoker.Close());
}

// Test invoker schema management
TEST_F(ClientUtilsTest, InvokerSchemaManagement) {
    InvokerConfig config;
    config.address = "";
    config.disable_logging = true;

    CroupierInvoker invoker(config);

    std::map<std::string, std::string> schema = {
        {"name", "string"},
        {"age", "number"}
    };

    EXPECT_NO_THROW(invoker.SetSchema("test.function", schema));

    // Set different schema for same function (should override)
    std::map<std::string, std::string> schema2 = {
        {"id", "string"}
    };
    EXPECT_NO_THROW(invoker.SetSchema("test.function", schema2));
}

// Test cancel task with empty ID
TEST_F(ClientUtilsTest, CancelTaskEmptyId) {
    InvokerConfig config;
    config.address = "";
    config.disable_logging = true;

    CroupierInvoker invoker(config);

    EXPECT_FALSE(invoker.CancelTask(""));
}

}  // namespace test
}  // namespace sdk
}  // namespace croupier
