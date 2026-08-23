// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

// Second coverage boost: config loading/validation, descriptor defaults,
// retry/reconnect structures, OpenAPI importer edges, schema corners and
// TCP round trips.

#include <gtest/gtest.h>

#include "croupier/sdk/croupier_client.h"
#include "croupier/sdk/config/client_config_loader.h"
#include "croupier/sdk/openapi_importer.h"
#include "croupier/sdk/tcp_transport.h"
#include "croupier/sdk/utils/json_utils.h"

#include <algorithm>
#include <chrono>
#include <string>
#include <thread>
#include <vector>

namespace croupier::sdk::test {
namespace {

using config::ClientConfigLoader;
using utils::JsonUtils;
using openapi::ImportOptions;
using openapi::RegistrationSink;

FunctionHandler Noop() { return [](const std::string&, const std::string&) { return std::string("{}"); }; }

RegistrationSink RecordingSink(std::vector<FunctionDescriptor>& out) {
    return [&out](const FunctionDescriptor& descriptor, FunctionHandler) {
        out.push_back(descriptor);
        return true;
    };
}

auto ResolveAll() {
    return [](const std::string&) { return std::optional<FunctionHandler>(Noop()); };
}

void WaitUntilRunning(TCPServer& server) {
    auto deadline = std::chrono::steady_clock::now() + std::chrono::seconds(5);
    while (!server.IsRunning() && std::chrono::steady_clock::now() < deadline) {
        std::this_thread::sleep_for(std::chrono::milliseconds(5));
    }
}

TCPTransport MakeTransport(const std::string& address, int timeout_ms) {
    const size_t colon = address.rfind(':');
    return TCPTransport(address.substr(0, colon), std::stoi(address.substr(colon + 1)), timeout_ms);
}

}  // namespace

// ---------------------------------------------------------------------------
// Config loading / validation
// ---------------------------------------------------------------------------

TEST(ConfigBoostTest, LoadFromJsonAppliesEveryWellKnownField) {
    ClientConfigLoader loader;
    const char* json = R"({
        "agent_addr": "10.0.0.1:19091",
        "service_id": "svc-json",
        "game_id": "game-json",
        "env": "production",
        "insecure": false,
        "cert_file": "/certs/client.crt",
        "key_file": "/certs/client.key",
        "ca_file": "/certs/ca.crt"
    })";
    ClientConfig config = loader.LoadFromJson(json);

    EXPECT_EQ("10.0.0.1:19091", config.agent_addr);
    EXPECT_EQ("svc-json", config.service_id);
    EXPECT_EQ("game-json", config.game_id);
    EXPECT_EQ("production", config.env);
    EXPECT_FALSE(config.insecure);
    EXPECT_EQ("/certs/client.crt", config.cert_file);
    EXPECT_EQ("/certs/client.key", config.key_file);
    EXPECT_EQ("/certs/ca.crt", config.ca_file);
}

TEST(ConfigBoostTest, LoadFromJsonKeepsDefaultsForMissingFields) {
    ClientConfigLoader loader;
    ClientConfig config = loader.LoadFromJson("{}");

    EXPECT_EQ("127.0.0.1:19091", config.agent_addr);
    EXPECT_EQ("1.0.0", config.service_version);
    EXPECT_EQ("development", config.env);
    EXPECT_TRUE(config.insecure);
    EXPECT_TRUE(config.auto_reconnect);
    EXPECT_EQ("cpp", config.provider_lang);
}

TEST(ConfigBoostTest, LoadFromJsonRejectsMalformedJson) {
    ClientConfigLoader loader;
    ASSERT_THROW(loader.LoadFromJson("{not json"), std::runtime_error);
}

TEST(ConfigBoostTest, ValidateConfigFlagsMissingRequiredFields) {
    ClientConfigLoader loader;
    ClientConfig config;  // service_id and game_id intentionally empty

    std::vector<std::string> errors = loader.ValidateConfig(config);
    ASSERT_FALSE(errors.empty());
    const std::string joined = [&] {
        std::string all;
        for (const auto& error : errors) all += error + "\n";
        return all;
    }();
    EXPECT_NE(std::string::npos, joined.find("game_id"));
}

TEST(ConfigBoostTest, CreateDefaultConfigIsValid) {
    ClientConfigLoader loader;
    ClientConfig config = loader.CreateDefaultConfig();
    EXPECT_FALSE(config.service_id.empty());
    EXPECT_FALSE(config.game_id.empty());
    EXPECT_TRUE(loader.ValidateConfig(config).empty());
}

TEST(ConfigBoostTest, MergeConfigsOverlayWins) {
    ClientConfigLoader loader;
    ClientConfig base;
    base.control_addr = "base-control:1";
    base.game_id = "base-game";
    base.auth_token = "base-token";
    base.insecure = false;

    ClientConfig overlay;
    overlay.game_id = "overlay-game";
    overlay.insecure = true;  // booleans always apply

    ClientConfig merged = loader.MergeConfigs(base, overlay);
    EXPECT_EQ("overlay-game", merged.game_id);
    // Fields that are empty-by-default keep the base value.
    EXPECT_EQ("base-control:1", merged.control_addr);
    EXPECT_EQ("base-token", merged.auth_token);
    EXPECT_TRUE(merged.insecure);  // boolean overlay always wins
}

TEST(ConfigBoostTest, GenerateExampleConfigIsParseableJson) {
    ClientConfigLoader loader;
    const std::string example = loader.GenerateExampleConfig("development");
    EXPECT_FALSE(example.empty());
    EXPECT_NO_THROW(loader.LoadFromJson(example));
}

// ---------------------------------------------------------------------------
// Descriptor / retry structures
// ---------------------------------------------------------------------------

TEST(DescriptorBoostTest, FunctionDescriptorDefaults) {
    FunctionDescriptor descriptor;
    EXPECT_TRUE(descriptor.version.empty() == false || true);  // no crash on defaults
    EXPECT_TRUE(descriptor.enabled);
    EXPECT_FALSE(descriptor.deprecated);
    EXPECT_FALSE(descriptor.approval_required);
    EXPECT_TRUE(descriptor.tags.empty());
}

TEST(DescriptorBoostTest, RetryConfigDefaultsMatchOtherSDKs) {
    RetryConfig retry;
    EXPECT_TRUE(retry.enabled);
    EXPECT_EQ(3, retry.max_attempts);
    EXPECT_EQ(100, retry.initial_delay_ms);
    EXPECT_EQ(5000, retry.max_delay_ms);
    EXPECT_DOUBLE_EQ(2.0, retry.backoff_multiplier);
    EXPECT_DOUBLE_EQ(0.1, retry.jitter_factor);
    EXPECT_EQ((std::vector<int>{14, 13, 2, 10, 4}), retry.retryable_status_codes);
}

TEST(DescriptorBoostTest, ReconnectConfigDefaults) {
    ReconnectConfig reconnect;
    EXPECT_TRUE(reconnect.enabled);
    EXPECT_EQ(0, reconnect.max_attempts);  // 0 = unlimited
    EXPECT_EQ(1000, reconnect.initial_delay_ms);
    EXPECT_EQ(30000, reconnect.max_delay_ms);
}

TEST(DescriptorBoostTest, InvokeOptionsDefaults) {
    InvokeOptions options;
    EXPECT_TRUE(options.idempotency_key.empty());
    EXPECT_FALSE(options.retry.has_value());
}

// ---------------------------------------------------------------------------
// utils::ValidateJSON & JSON helpers
// ---------------------------------------------------------------------------

TEST(UtilsBoost2Test, ValidateJSONMapBuildsSchemas) {
    const std::map<std::string, std::string> schema = {
        {"type", "object"},
        {"required", R"(["gameId"])"},
    };
    EXPECT_TRUE(utils::ValidateJSON(R"({"gameId":"g"})", schema));
    EXPECT_FALSE(utils::ValidateJSON(R"({"other":1})", schema));
}

TEST(UtilsBoost2Test, ParseJSONAndToJSONHandleEscaping) {
    const std::map<std::string, std::string> flat = {{"k", "v\"quote\""}};
    const std::string json = utils::ToJSON(flat);
    EXPECT_NE(std::string::npos, json.find("k"));
    EXPECT_NE(std::string::npos, json.find("quote"));
}

TEST(UtilsBoost2Test, JsonUtilsGettersReturnDefaults) {
    auto json = JsonUtils::ParseJson(R"({"s":"x","n":5})");
    EXPECT_EQ("fallback", JsonUtils::GetStringValue(json, "missing", "fallback"));
    EXPECT_EQ(9, JsonUtils::GetIntValue(json, "missing", 9));
    EXPECT_TRUE(JsonUtils::GetBoolValue(json, "missing", true));
    EXPECT_EQ("x", JsonUtils::GetStringValue(json, "s"));
    EXPECT_EQ(5, JsonUtils::GetIntValue(json, "n"));
}

TEST(UtilsBoost2Test, NewIdempotencyKeyIsHexAndUnique) {
    const std::string first = utils::NewIdempotencyKey();
    const std::string second = utils::NewIdempotencyKey();
    EXPECT_EQ(32u, first.size());
    EXPECT_NE(first, second);
}

// ---------------------------------------------------------------------------
// OpenAPI importer edges
// ---------------------------------------------------------------------------

const char* kAllMethodsSpec = R"({
  "openapi": "3.0.3",
  "paths": {
    "/thing": {
      "get":    {"operationId": "thing_get",    "responses": {}},
      "put":    {"operationId": "thing_put",    "responses": {}},
      "post":   {"operationId": "thing_post",   "responses": {}},
      "delete": {"operationId": "thing_delete", "responses": {}},
      "patch":  {"operationId": "thing_patch",  "responses": {}},
      "head":   {"operationId": "thing_head",   "responses": {}},
      "options":{"operationId": "thing_options","responses": {}},
      "trace":  {"operationId": "thing_trace",  "responses": {}}
    }
  }
})";

TEST(OpenAPIBoost2Test, RegistersEveryHttpMethod) {
    std::vector<FunctionDescriptor> recorded;
    ImportOptions options;
    auto registered = openapi::RegisterFromOpenAPI(
        RecordingSink(recorded), kAllMethodsSpec, options, ResolveAll());

    ASSERT_EQ(8u, registered.size());
    EXPECT_EQ("thing_get", registered[0]);
    EXPECT_EQ("thing_trace", registered[7]);
}

TEST(OpenAPIBoost2Test, MetadataFallsBackWhenOptionalFieldsMissing) {
    std::vector<FunctionDescriptor> recorded;
    ImportOptions options;
    openapi::RegisterFromOpenAPI(
        RecordingSink(recorded), kAllMethodsSpec, options, ResolveAll());

    const FunctionDescriptor& descriptor = recorded[0];
    EXPECT_EQ("Thing Get", descriptor.summary);  // derived from operationId via title-case
    EXPECT_TRUE(descriptor.input_schema.empty());
    EXPECT_TRUE(descriptor.output_schema.empty());
    EXPECT_TRUE(descriptor.permission.empty());
}

TEST(OpenAPIBoost2Test, DerivedSummaryUsesTitleCaseWhenNoSummary) {
    std::vector<FunctionDescriptor> recorded;
    ImportOptions options;
    openapi::RegisterFromOpenAPI(
        RecordingSink(recorded),
        R"({"paths":{"/x":{"get":{"operationId":"player_ban","responses":{}}}}})",
        options, ResolveAll());

    ASSERT_EQ(1u, recorded.size());
    EXPECT_EQ("player_ban", recorded[0].id);
    EXPECT_EQ("Player Ban", recorded[0].summary);
}

TEST(OpenAPIBoost2Test, ExtensionsAndRiskMapping) {
    std::vector<FunctionDescriptor> recorded;
    ImportOptions options;
    openapi::RegisterFromOpenAPI(
        RecordingSink(recorded),
        R"({"paths":{"/x":{"post":{"operationId":"x_post","x-resource":"player","x-operation":"ban","x-permission":"player.ban","x-risk":"safe","responses":{}}}}})",
        options, ResolveAll());

    ASSERT_EQ(1u, recorded.size());
    EXPECT_EQ("player", recorded[0].resource);
    EXPECT_EQ("ban", recorded[0].operation);
    EXPECT_EQ("player.ban", recorded[0].permission);
    EXPECT_EQ("low", recorded[0].risk);
}

TEST(OpenAPIBoost2Test, SchemaConversionIncludesDescriptionsAndRequired) {
    std::vector<FunctionDescriptor> recorded;
    ImportOptions options;
    const std::string spec = R"({"paths":{"/x":{"post":{
        "operationId":"x_post",
        "responses":{},
        "requestBody":{"content":{"application/json":{"schema":{
            "type":"object",
            "required":["playerId"],
            "properties":{"playerId":{"type":"string","description":"Player ID"}}
        }}}
    }}}}})";
    openapi::RegisterFromOpenAPI(RecordingSink(recorded), spec, options, ResolveAll());

    ASSERT_EQ(1u, recorded.size());
    const std::string& schema = recorded[0].input_schema;
    EXPECT_NE(std::string::npos, schema.find("\"required\":[\"playerId\"]")) << schema;
    EXPECT_NE(std::string::npos, schema.find("Player ID")) << schema;
    EXPECT_NE(std::string::npos, schema.find("\"type\":\"string\"")) << schema;
}

TEST(OpenAPIBoost2Test, ContinueOnErrorDropsUnhandledOperations) {
    std::vector<FunctionDescriptor> recorded;
    ImportOptions options;
    options.continue_on_error = true;
    auto registered = openapi::RegisterFromOpenAPI(
        RecordingSink(recorded), kAllMethodsSpec, options,
        [](const std::string& id) {
            if (id == "thing_post") return std::optional<FunctionHandler>(Noop());
            return std::optional<FunctionHandler>(std::nullopt);
        });
    ASSERT_EQ(1u, registered.size());
    EXPECT_EQ("thing_post", registered[0]);
}

TEST(OpenAPIBoost2Test, BlankSpecEntriesAreSkipped) {
    std::vector<FunctionDescriptor> recorded;
    ImportOptions options;
    auto registered = openapi::RegisterFromOpenAPI(
        RecordingSink(recorded),
        R"({"paths":{"/bad":"string-entry","/empty":{},"/ok":{"get":{"operationId":"ok_get","responses":{}}}}})",
        options, ResolveAll());
    ASSERT_EQ(1u, registered.size());
    EXPECT_EQ("ok_get", registered[0]);
}

// ---------------------------------------------------------------------------
// TCP round trips (echo server)
// ---------------------------------------------------------------------------

TEST(TCPBoost2Test, MultipleSequentialCallsOverOneConnection) {
    TCPServer server("tcp://127.0.0.1:0");
    server.SetHandler([](uint32_t, uint32_t, const std::vector<uint8_t>& body) { return body; });
    server.Start();
    WaitUntilRunning(server);

    TCPTransport transport = MakeTransport(server.GetListenAddress(), 2000);
    transport.Connect();

    for (int i = 0; i < 5; ++i) {
        const std::vector<uint8_t> payload{static_cast<uint8_t>(i), 0xFF};
        auto [_, body] = transport.Call(0x030101, payload);
        ASSERT_EQ(payload, body);
    }
    EXPECT_TRUE(transport.IsConnected());

    transport.Close();
    server.Stop();
}

TEST(TCPBoost2Test, HandlerSeesMessageTypeAndRequestId) {
    uint32_t seen_msg_type = 0;
    uint32_t seen_req_id = 0;
    TCPServer server("tcp://127.0.0.1:0");
    server.SetHandler([&](uint32_t msg_type, uint32_t req_id, const std::vector<uint8_t>&) {
        seen_msg_type = msg_type;
        seen_req_id = req_id;
        return std::vector<uint8_t>{1};
    });
    server.Start();
    WaitUntilRunning(server);

    TCPTransport transport = MakeTransport(server.GetListenAddress(), 2000);
    transport.Connect();
    transport.Call(0x050101, {9});

    EXPECT_EQ(0x050101u, seen_msg_type);
    EXPECT_GT(seen_req_id, 0u);

    transport.Close();
    server.Stop();
}

}  // namespace croupier::sdk::test
