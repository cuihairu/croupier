// JSON schema validation branches and JSON utility helpers.
#include <gtest/gtest.h>
#include "croupier/sdk/utils/json_utils.h"
#include "croupier/sdk/croupier_client.h"

namespace croupier::sdk::utils::test {
namespace {

using croupier::sdk::utils::JsonUtils;

TEST(JsonSchemaTest, TypeChecksForAllPrimitiveTypes) {
    EXPECT_TRUE(JsonUtils::ValidateJsonSchema(R"({"a":1})", R"({"type":"object"})"));
    EXPECT_TRUE(JsonUtils::ValidateJsonSchema("[1,2]", R"({"type":"array"})"));
    EXPECT_TRUE(JsonUtils::ValidateJsonSchema(R"("s")", R"({"type":"string"})"));
    EXPECT_TRUE(JsonUtils::ValidateJsonSchema("1.5", R"({"type":"number"})"));
    EXPECT_TRUE(JsonUtils::ValidateJsonSchema("7", R"({"type":"number"})"));
    EXPECT_TRUE(JsonUtils::ValidateJsonSchema("7", R"({"type":"integer"})"));
    EXPECT_TRUE(JsonUtils::ValidateJsonSchema("true", R"({"type":"boolean"})"));
    EXPECT_TRUE(JsonUtils::ValidateJsonSchema("null", R"({"type":"null"})"));

    EXPECT_FALSE(JsonUtils::ValidateJsonSchema("1", R"({"type":"object"})"));
    EXPECT_FALSE(JsonUtils::ValidateJsonSchema(R"("x")", R"({"type":"integer"})"));
    EXPECT_FALSE(JsonUtils::ValidateJsonSchema("1", R"({"type":"boolean"})"));
    EXPECT_FALSE(JsonUtils::ValidateJsonSchema("{}", R"({"type":"null"})"));
}

TEST(JsonSchemaTest, RequiredProperties) {
    const std::string schema = R"({"type":"object","required":["gameId","env"]})";
    EXPECT_TRUE(JsonUtils::ValidateJsonSchema(R"({"gameId":"g","env":"dev"})", schema));
    EXPECT_FALSE(JsonUtils::ValidateJsonSchema(R"({"gameId":"g"})", schema));
    EXPECT_FALSE(JsonUtils::ValidateJsonSchema("{}", schema));
}

TEST(JsonSchemaTest, PropertyTypeValidation) {
    const std::string schema = R"({"properties":{"name":{"type":"string"},"count":{"type":"integer"}}})";
    EXPECT_TRUE(JsonUtils::ValidateJsonSchema(R"({"name":"n","count":2})", schema));
    EXPECT_FALSE(JsonUtils::ValidateJsonSchema(R"({"name":5,"count":2})", schema));
    EXPECT_FALSE(JsonUtils::ValidateJsonSchema(R"({"name":"n","count":"two"})", schema));
    // Unknown properties are ignored.
    EXPECT_TRUE(JsonUtils::ValidateJsonSchema(R"({"other":true})", schema));
}

TEST(JsonSchemaTest, NumericBounds) {
    EXPECT_TRUE(JsonUtils::ValidateJsonSchema("5", R"({"minimum":1,"maximum":10})"));
    EXPECT_FALSE(JsonUtils::ValidateJsonSchema("0", R"({"minimum":1})"));
    EXPECT_FALSE(JsonUtils::ValidateJsonSchema("11", R"({"maximum":10})"));
    EXPECT_TRUE(JsonUtils::ValidateJsonSchema("10", R"({"minimum":1,"maximum":10})"));
}

TEST(JsonSchemaTest, StringLengthBounds) {
    EXPECT_TRUE(JsonUtils::ValidateJsonSchema(R"("abcd")", R"({"minLength":2,"maxLength":6})"));
    EXPECT_FALSE(JsonUtils::ValidateJsonSchema(R"("a")", R"({"minLength":2})"));
    EXPECT_FALSE(JsonUtils::ValidateJsonSchema(R"("abcdefg")", R"({"maxLength":6})"));
}

TEST(JsonSchemaTest, ArraySizeBounds) {
    EXPECT_TRUE(JsonUtils::ValidateJsonSchema("[1,2,3]", R"({"minItems":2,"maxItems":4})"));
    EXPECT_FALSE(JsonUtils::ValidateJsonSchema("[1]", R"({"minItems":2})"));
    EXPECT_FALSE(JsonUtils::ValidateJsonSchema("[1,2,3,4,5]", R"({"maxItems":4})"));
}

TEST(JsonSchemaTest, EnumValidation) {
    const std::string schema = R"({"enum":["development","staging","production"]})";
    EXPECT_TRUE(JsonUtils::ValidateJsonSchema(R"("staging")", schema));
    EXPECT_FALSE(JsonUtils::ValidateJsonSchema(R"("qa")", schema));
}

TEST(JsonSchemaTest, InvalidInputsAreRejected) {
    EXPECT_FALSE(JsonUtils::ValidateJsonSchema("not json", R"({"type":"object"})"));
    EXPECT_FALSE(JsonUtils::ValidateJsonSchema("{}", "not a schema"));
}

TEST(JsonPrettyPrintTest, DumpWithIndent) {
    nlohmann::json obj = {{"b", 1}, {"a", {{"c", true}}}};
    const std::string printed = JsonUtils::PrettyPrint(obj, 4);
    EXPECT_NE(std::string::npos, printed.find("    \"a\""));
    EXPECT_EQ("{}", JsonUtils::PrettyPrint(nlohmann::json::object(), 2));
}

TEST(JsonConversionTest, ToJsonStringSpecializations) {
    EXPECT_EQ(R"({"a":"1","b":"2"})", JsonUtils::ToJsonString(std::map<std::string, std::string>{{"a", "1"}, {"b", "2"}}));
    EXPECT_EQ(R"(["x","y"])", JsonUtils::ToJsonString(std::vector<std::string>{"x", "y"}));
}

TEST(JsonValueExtractionTest, WrongTypesFallBackToDefaults) {
    const auto obj = JsonUtils::ParseJson(R"({"s":"str","i":42,"b":true,"nested":{"v":"deep"}})");
    EXPECT_EQ("str", JsonUtils::GetStringValue(obj, "s"));
    EXPECT_EQ("fallback", JsonUtils::GetStringValue(obj, "missing", "fallback"));
    EXPECT_EQ("fallback", JsonUtils::GetStringValue(obj, "i", "fallback"));  // not a string
    EXPECT_EQ(42, JsonUtils::GetIntValue(obj, "i"));
    EXPECT_EQ(-1, JsonUtils::GetIntValue(obj, "s", -1));  // not an int
    EXPECT_EQ(-1, JsonUtils::GetIntValue(obj, "nested", -1));
    EXPECT_TRUE(JsonUtils::GetBoolValue(obj, "b"));
    EXPECT_TRUE(JsonUtils::GetBoolValue(obj, "missing", true));
    EXPECT_TRUE(JsonUtils::GetBoolValue(obj, "i", true));  // not a bool
    EXPECT_EQ("deep", JsonUtils::GetStringValue(obj, "nested.v"));
    EXPECT_THROW(JsonUtils::ParseJson("{invalid"), std::runtime_error);
}

TEST(JsonMergeTest, NestedMergeAndOverride) {
    const auto base = JsonUtils::ParseJson(R"({"a":{"x":1,"y":2},"b":1})");
    const auto overlay = JsonUtils::ParseJson(R"({"a":{"y":9,"z":3},"c":4})");
    const auto merged = JsonUtils::MergeJson(base, overlay);
    EXPECT_EQ(1, merged["a"]["x"].get<int>());
    EXPECT_EQ(9, merged["a"]["y"].get<int>());
    EXPECT_EQ(3, merged["a"]["z"].get<int>());
    EXPECT_EQ(4, merged["c"].get<int>());

    // Non-object overlay replaces the base entirely.
    const auto replaced = JsonUtils::MergeJson(base, JsonUtils::ParseJson("[1]"));
    EXPECT_TRUE(replaced.is_array());
}

TEST(SimpleJsonUtilsTest, FreeFunctionHelpers) {
    // utils::ParseJSON extracts flat string pairs.
    std::map<std::string, std::string> parsed = ParseJSON(R"({"game":"demo","env":"dev"})");
    EXPECT_EQ("demo", parsed["game"]);
    EXPECT_EQ("dev", parsed["env"]);
    EXPECT_TRUE(ParseJSON("").empty());
    EXPECT_TRUE(ParseJSON("[1,2]").empty());  // no "key":"value" pairs

    // utils::ToJSON serializes string maps.
    EXPECT_EQ(R"({"a":"1"})", ToJSON({{"a", "1"}}));
    EXPECT_EQ("{}", ToJSON({}));

    // utils::ValidateJSON with an empty schema only validates JSON syntax.
    EXPECT_TRUE(ValidateJSON("{}", {}));
    EXPECT_FALSE(ValidateJSON("nope", {}));

    // With a schema it delegates to JSON schema validation.
    const std::map<std::string, std::string> schema = {{"type", "object"}, {"required", R"(["gameId"])"}};
    EXPECT_TRUE(ValidateJSON(R"({"gameId":"g"})", schema));
    EXPECT_FALSE(ValidateJSON(R"({"other":1})", schema));

    // Schema values that are JSON literals are embedded verbatim. NOTE
    // (recorded limitation): plain numeric strings such as "2" are quoted,
    // so numeric constraints cannot be expressed through this map API.
    const std::map<std::string, std::string> limits = {{"type", "integer"}, {"minimum", "2"}};
    EXPECT_FALSE(ValidateJSON("3", limits));  // "minimum":"2" (string) fails to compare
}

TEST(IsValidJsonTest, StructuralChecks) {
    EXPECT_TRUE(JsonUtils::IsValidJson(R"({"a":{"b":[1,{"c":"]"}]}})"));
    EXPECT_TRUE(JsonUtils::IsValidJson("  []  "));
    EXPECT_FALSE(JsonUtils::IsValidJson(""));
    EXPECT_FALSE(JsonUtils::IsValidJson("   "));
    EXPECT_FALSE(JsonUtils::IsValidJson("{"));
    EXPECT_FALSE(JsonUtils::IsValidJson("[1}"));
    EXPECT_FALSE(JsonUtils::IsValidJson("}"));
    EXPECT_FALSE(JsonUtils::IsValidJson(R"({"a":"unterminated})"));
    EXPECT_FALSE(JsonUtils::IsValidJson("plain"));
}

}  // namespace
}  // namespace croupier::sdk::utils::test
