// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

// Tests for OpenAPI import (Go RegisterFromOpenAPI parity) and the enhanced
// JSON Schema subset validator.

#include <gtest/gtest.h>

#include "croupier/sdk/openapi_importer.h"
#include "croupier/sdk/utils/json_utils.h"

#include <map>
#include <optional>
#include <string>
#include <vector>

namespace croupier::sdk::openapi::test {
namespace {

using utils::JsonUtils;

class RecordingSink {
public:
    std::vector<std::pair<FunctionDescriptor, FunctionHandler>> registered;
    std::string fail_id;

    RegistrationSink Sink() {
        return [this](const FunctionDescriptor& descriptor, FunctionHandler handler) {
            if (descriptor.id == fail_id) return false;
            registered.emplace_back(descriptor, std::move(handler));
            return true;
        };
    }
};

FunctionHandler Noop() { return [](const std::string&, const std::string&) { return std::string("{}"); }; }

const char* kSpec = R"({
  "openapi": "3.0.3",
  "info": {"title": "GM API", "version": "1.0.0"},
  "paths": {
    "/players/{id}/ban": {
      "put": {
        "operationId": "player_ban",
        "summary": "Ban player",
        "description": "Bans a player account",
        "tags": ["gm", "risk"],
        "x-resource": "player",
        "x-operation": "ban",
        "x-permission": "player.ban",
        "x-risk": "high",
        "x-capability": "action",
        "x-execution": "sync",
        "x-approval": {"required": true, "policyKey": "player.ban.double_check"},
        "requestBody": {"content": {"application/json": {"schema": {
          "type": "object",
          "required": ["playerId", "reason"],
          "properties": {
            "playerId": {"type": "string", "description": "Player ID"},
            "reason": {"type": "string"}
          }
        }}}},
        "responses": {"200": {"content": {"application/json": {"schema": {
          "type": "object",
          "properties": {"ok": {"type": "boolean"}}
        }}}}}
      }
    },
    "/players/search": {
      "get": {
        "tags": ["query"],
        "responses": {"200": {"content": {"application/json": {"schema": {"type": "array"}}}}}
      }
    }
  }
})";

// ---------------------------------------------------------------------------
// OpenAPI import
// ---------------------------------------------------------------------------

TEST(OpenAPIImporterTest, RegistersAllOperations) {
    RecordingSink sink;
    ImportOptions options;
    std::vector<std::string> registered = RegisterFromOpenAPI(sink.Sink(), kSpec, options,
        [](const std::string&) { return std::optional<FunctionHandler>(Noop()); });
    ASSERT_EQ(2u, registered.size());
    EXPECT_EQ("player_ban", registered[0]);
    EXPECT_EQ("players.search", registered[1]);
}

TEST(OpenAPIImporterTest, MapsOperationMetadata) {
    RecordingSink sink;
    ImportOptions options;
    RegisterFromOpenAPI(sink.Sink(), kSpec, options,
        [](const std::string&) { return std::optional<FunctionHandler>(Noop()); });

    ASSERT_EQ(2u, sink.registered.size());
    const FunctionDescriptor& descriptor = sink.registered[0].first;
    EXPECT_EQ("player_ban", descriptor.id);
    EXPECT_EQ("Ban player", descriptor.summary);
    EXPECT_EQ("Bans a player account", descriptor.description);
    ASSERT_EQ(2u, descriptor.tags.size());
    EXPECT_EQ("gm", descriptor.tags[0]);
    EXPECT_EQ("risk", descriptor.tags[1]);
    EXPECT_EQ("player", descriptor.resource);
    EXPECT_EQ("ban", descriptor.operation);
    EXPECT_EQ("player.ban", descriptor.permission);
    EXPECT_EQ("high", descriptor.risk);
}

TEST(OpenAPIImporterTest, ConvertsSchemas) {
    RecordingSink sink;
    ImportOptions options;
    RegisterFromOpenAPI(sink.Sink(), kSpec, options,
        [](const std::string&) { return std::optional<FunctionHandler>(Noop()); });

    const FunctionDescriptor& descriptor = sink.registered[0].first;
    EXPECT_NE(std::string::npos, descriptor.input_schema.find("\"required\":[\"playerId\",\"reason\"]"));
    EXPECT_NE(std::string::npos, descriptor.input_schema.find("Player ID"));
    EXPECT_NE(std::string::npos, descriptor.output_schema.find("\"type\":\"boolean\""));
}

TEST(OpenAPIImporterTest, DerivesIdAndDefaultsWhenOperationIdMissing) {
    RecordingSink sink;
    ImportOptions options;
    RegisterFromOpenAPI(sink.Sink(), kSpec, options,
        [](const std::string&) { return std::optional<FunctionHandler>(Noop()); });

    const FunctionDescriptor& descriptor = sink.registered[1].first;
    EXPECT_EQ("players.search", descriptor.id);
    EXPECT_EQ("Players.search", descriptor.summary);
    EXPECT_EQ("medium", descriptor.risk);
}

TEST(OpenAPIImporterTest, MapsV2ExecutionContract) {
    RecordingSink sink;
    ImportOptions options;
    RegisterFromOpenAPI(sink.Sink(), kSpec, options,
        [](const std::string&) { return std::optional<FunctionHandler>(Noop()); });

    const FunctionDescriptor& with_ext = sink.registered[0].first;
    EXPECT_EQ("action", with_ext.capability);
    EXPECT_EQ("sync", with_ext.execution);

    // Operations without x-capability/x-execution stay empty.
    const FunctionDescriptor& without_ext = sink.registered[1].first;
    EXPECT_TRUE(without_ext.capability.empty());
    EXPECT_TRUE(without_ext.execution.empty());
}

TEST(OpenAPIImporterTest, MapsV2ApprovalExtension) {
    RecordingSink sink;
    ImportOptions options;
    RegisterFromOpenAPI(sink.Sink(), kSpec, options,
        [](const std::string&) { return std::optional<FunctionHandler>(Noop()); });

    const FunctionDescriptor& descriptor = sink.registered[0].first;
    EXPECT_TRUE(descriptor.approval_required);
    EXPECT_EQ("player.ban.double_check", descriptor.approval_policy_key);

    const FunctionDescriptor& plain = sink.registered[1].first;
    EXPECT_FALSE(plain.approval_required);
    EXPECT_TRUE(plain.approval_policy_key.empty());
}

TEST(OpenAPIImporterTest, ApprovalNotRequiredKeptExplicitlyFalse) {
    const char* spec = R"({
      "paths": {
        "/players/{id}/kick": {
          "post": {
            "operationId": "player_kick",
            "x-approval": {"required": false, "policyKey": "player.kick.optional"}
          }
        }
      }
    })";
    RecordingSink sink;
    ImportOptions options;
    RegisterFromOpenAPI(sink.Sink(), spec, options,
        [](const std::string&) { return std::optional<FunctionHandler>(Noop()); });

    ASSERT_EQ(1u, sink.registered.size());
    EXPECT_FALSE(sink.registered[0].first.approval_required);
    EXPECT_EQ("player.kick.optional", sink.registered[0].first.approval_policy_key);
}

TEST(OpenAPIImporterTest, DefaultTimeoutMsOptionAccepted) {
    RecordingSink sink;
    ImportOptions options;
    options.default_timeout_ms = 60000;
    std::vector<std::string> registered = RegisterFromOpenAPI(sink.Sink(), kSpec, options,
        [](const std::string&) { return std::optional<FunctionHandler>(Noop()); });
    EXPECT_EQ(2u, registered.size());
}

TEST(OpenAPIImporterTest, AppliesPrefixOptions) {
    RecordingSink sink;
    ImportOptions options;
    options.resource_prefix = "game";
    options.tag_prefix = "svc-";
    RegisterFromOpenAPI(sink.Sink(), kSpec, options,
        [](const std::string&) { return std::optional<FunctionHandler>(Noop()); });

    const FunctionDescriptor& descriptor = sink.registered[0].first;
    EXPECT_EQ("game.player", descriptor.resource);
    ASSERT_EQ(2u, descriptor.tags.size());
    EXPECT_EQ("svc-gm", descriptor.tags[0]);
    EXPECT_EQ("svc-risk", descriptor.tags[1]);
}

TEST(OpenAPIImporterTest, MissingHandlerThrows) {
    RecordingSink sink;
    ImportOptions options;
    HandlerResolver none = [](const std::string&) { return std::optional<FunctionHandler>(std::nullopt); };
    ASSERT_THROW(RegisterFromOpenAPI(sink.Sink(), kSpec, options, none), std::runtime_error);
}

TEST(OpenAPIImporterTest, MissingHandlerContinueOnError) {
    RecordingSink sink;
    ImportOptions options;
    options.continue_on_error = true;
    std::vector<std::string> registered = RegisterFromOpenAPI(
        sink.Sink(), kSpec, options,
        [](const std::string& id) {
            if (id == "players.search") return std::optional<FunctionHandler>(Noop());
            return std::optional<FunctionHandler>(std::nullopt);
        });
    ASSERT_EQ(1u, registered.size());
    EXPECT_EQ("players.search", registered[0]);
}

TEST(OpenAPIImporterTest, RegistrationFailureContinueOnError) {
    RecordingSink lenient;
    lenient.fail_id = "player_ban";
    ImportOptions options;
    options.continue_on_error = true;
    std::vector<std::string> registered = RegisterFromOpenAPI(
        lenient.Sink(), kSpec, options,
        [](const std::string&) { return std::optional<FunctionHandler>(Noop()); });
    EXPECT_EQ(1u, registered.size());

    RecordingSink strict;
    strict.fail_id = "player_ban";
    ImportOptions strict_options;
    ASSERT_THROW(RegisterFromOpenAPI(strict.Sink(), kSpec, strict_options,
                                     [](const std::string&) { return std::optional<FunctionHandler>(Noop()); }),
                 std::runtime_error);
}

TEST(OpenAPIImporterTest, ResolverReceivesDerivedIds) {
    RecordingSink sink;
    ImportOptions options;
    std::vector<std::string> seen;
    auto registered = RegisterFromOpenAPI(
        sink.Sink(), kSpec, options,
        [&seen](const std::string& function_id) {
            seen.push_back(function_id);
            return std::optional<FunctionHandler>(Noop());
        });
    EXPECT_EQ(2u, seen.size());
    EXPECT_EQ(2u, registered.size());
}

TEST(OpenAPIImporterTest, InvalidSpecThrows) {
    RecordingSink sink;
    ImportOptions options;
    auto resolver = [](const std::string&) { return std::optional<FunctionHandler>(Noop()); };
    ASSERT_THROW(RegisterFromOpenAPI(sink.Sink(), "{not json", options, resolver),
                 std::runtime_error);
    ASSERT_THROW(RegisterFromOpenAPI(sink.Sink(), R"({"openapi":"3.0.3"})", options, resolver),
                 std::runtime_error);
}

TEST(OpenAPIImporterTest, EmptyPathsYieldsNothing) {
    RecordingSink sink;
    ImportOptions options;
    std::vector<std::string> registered = RegisterFromOpenAPI(
        sink.Sink(), R"({"paths":{}})", options,
        [](const std::string&) { return std::optional<FunctionHandler>(Noop()); });
    EXPECT_TRUE(registered.empty());
}

// ---------------------------------------------------------------------------
// Enhanced JSON Schema validation
// ---------------------------------------------------------------------------

TEST(SchemaBoostTest, ConstConstraint) {
    EXPECT_TRUE(JsonUtils::ValidateJsonSchema("7", R"({"const":7})"));
    EXPECT_FALSE(JsonUtils::ValidateJsonSchema("8", R"({"const":7})"));
}

TEST(SchemaBoostTest, PatternConstraint) {
    EXPECT_TRUE(JsonUtils::ValidateJsonSchema(R"("abc")", R"({"type":"string","pattern":"^[a-z]+$"})"));
    EXPECT_FALSE(JsonUtils::ValidateJsonSchema(R"("AB")", R"({"type":"string","pattern":"^[a-z]+$"})"));
}

TEST(SchemaBoostTest, ExclusiveBoundsAndMultipleOf) {
    EXPECT_FALSE(JsonUtils::ValidateJsonSchema("1", R"({"type":"number","exclusiveMinimum":1})"));
    EXPECT_TRUE(JsonUtils::ValidateJsonSchema("1.5", R"({"type":"number","exclusiveMinimum":1})"));
    EXPECT_FALSE(JsonUtils::ValidateJsonSchema("10", R"({"type":"number","exclusiveMaximum":10})"));
    EXPECT_TRUE(JsonUtils::ValidateJsonSchema("9", R"({"type":"number","exclusiveMaximum":10})"));
    EXPECT_TRUE(JsonUtils::ValidateJsonSchema("9", R"({"type":"number","multipleOf":3})"));
    EXPECT_FALSE(JsonUtils::ValidateJsonSchema("10", R"({"type":"number","multipleOf":3})"));
}

TEST(SchemaBoostTest, UniqueItems) {
    EXPECT_TRUE(JsonUtils::ValidateJsonSchema("[1,2,3]", R"({"type":"array","uniqueItems":true})"));
    EXPECT_FALSE(JsonUtils::ValidateJsonSchema("[1,2,2]", R"({"type":"array","uniqueItems":true})"));
}

TEST(SchemaBoostTest, ItemsRecursion) {
    EXPECT_TRUE(JsonUtils::ValidateJsonSchema("[1,2,3]", R"({"type":"array","items":{"type":"integer"}})"));
    EXPECT_FALSE(JsonUtils::ValidateJsonSchema("[1,\"x\"]", R"({"type":"array","items":{"type":"integer"}})"));
    EXPECT_FALSE(JsonUtils::ValidateJsonSchema("[5]", R"({"type":"array","items":{"type":"integer","minimum":10}})"));
}

TEST(SchemaBoostTest, PropertyRecursionKeepsNestedConstraints) {
    const char* schema = R"({
        "type":"object",
        "properties":{"name":{"type":"string","minLength":3}}
    })";
    EXPECT_TRUE(JsonUtils::ValidateJsonSchema(R"({"name":"abc"})", schema));
    EXPECT_FALSE(JsonUtils::ValidateJsonSchema(R"({"name":"ab"})", schema));
}

TEST(SchemaBoostTest, AdditionalPropertiesBooleanAndSchema) {
    EXPECT_TRUE(JsonUtils::ValidateJsonSchema(
        R"({"a":1})", R"({"type":"object","properties":{"a":{}},"additionalProperties":false})"));
    EXPECT_FALSE(JsonUtils::ValidateJsonSchema(
        R"({"a":1,"b":2})", R"({"type":"object","properties":{"a":{}},"additionalProperties":false})"));
    EXPECT_FALSE(JsonUtils::ValidateJsonSchema(
        R"({"b":"x"})", R"({"type":"object","properties":{},"additionalProperties":{"type":"integer"}})"));
    EXPECT_TRUE(JsonUtils::ValidateJsonSchema(
        R"({"b":5})", R"({"type":"object","properties":{},"additionalProperties":{"type":"integer"}})"));
}

TEST(SchemaBoostTest, ExistingKeywordsStillEnforced) {
    EXPECT_TRUE(JsonUtils::ValidateJsonSchema(R"({"a":1})", R"({"type":"object","required":["a"]})"));
    EXPECT_FALSE(JsonUtils::ValidateJsonSchema(R"({})", R"({"type":"object","required":["a"]})"));
    EXPECT_FALSE(JsonUtils::ValidateJsonSchema("0.5", R"({"type":"number","minimum":1})"));
    EXPECT_FALSE(JsonUtils::ValidateJsonSchema(R"("c")", R"({"enum":["a","b"]})"));
}

}  // namespace
}  // namespace croupier::sdk::openapi::test
