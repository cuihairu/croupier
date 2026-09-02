// F：SetFieldHint/SetFieldWidget（x-ui 呈现契约便捷层）单测。

#include "croupier/sdk/field_hints.h"

#include <gtest/gtest.h>

namespace croupier::sdk::test {

TEST(FieldHintsTest, EmptySchemaCreatesObjectSkeleton) {
    FunctionDescriptor desc;
    ASSERT_TRUE(SetFieldWidget(desc, "id", "Select"));
    auto schema = nlohmann::json::parse(desc.input_schema);
    EXPECT_EQ(schema["type"], "object");
    EXPECT_EQ(schema["properties"]["id"]["x-widget"], "Select");
}

TEST(FieldHintsTest, PreservesExistingAttributesAndOverrides) {
    FunctionDescriptor desc;
    desc.input_schema = R"({"type":"object","properties":{"id":{"type":"string","title":"玩家 ID","x-widget":"Input"}}})";
    ASSERT_TRUE(SetFieldWidget(desc, "id", "TreeSelect"));
    auto prop = nlohmann::json::parse(desc.input_schema)["properties"]["id"];
    EXPECT_EQ(prop["title"], "玩家 ID");
    EXPECT_EQ(prop["x-widget"], "TreeSelect");
}

TEST(FieldHintsTest, OptionsSourceObject) {
    FunctionDescriptor desc;
    ASSERT_TRUE(SetFieldHint(desc, "id", "x-options-source",
                             nlohmann::json{{"functionId", "player.list"},
                                            {"labelPath", "/items/*/name"},
                                            {"valuePath", "/items/*/id"}}));
    auto prop = nlohmann::json::parse(desc.input_schema)["properties"]["id"];
    EXPECT_EQ(prop["x-options-source"]["functionId"], "player.list");
    EXPECT_EQ(prop["x-options-source"]["labelPath"], "/items/*/name");
}

TEST(FieldHintsTest, XUnderscoreNormalizedToXDash) {
    FunctionDescriptor desc;
    ASSERT_TRUE(SetFieldHint(desc, "a", "x_widget", "Input"));
    auto prop = nlohmann::json::parse(desc.input_schema)["properties"]["a"];
    EXPECT_TRUE(prop.contains("x-widget"));
    EXPECT_FALSE(prop.contains("x_widget"));
}

TEST(FieldHintsTest, InvalidHintRejected) {
    FunctionDescriptor desc;
    EXPECT_FALSE(SetFieldHint(desc, "a", "widget", "Input"));
    EXPECT_TRUE(desc.input_schema.empty());
}

TEST(FieldHintsTest, EmptyFieldRejected) {
    FunctionDescriptor desc;
    EXPECT_FALSE(SetFieldHint(desc, "", "x-widget", "Input"));
}

TEST(FieldHintsTest, EmptyWidgetRejected) {
    FunctionDescriptor desc;
    EXPECT_FALSE(SetFieldWidget(desc, "a", ""));
}

TEST(FieldHintsTest, InvalidExistingSchemaRejected) {
    FunctionDescriptor desc;
    desc.input_schema = "not-json";
    EXPECT_FALSE(SetFieldHint(desc, "a", "x-widget", "Input"));
}

TEST(FieldHintsTest, ShortHintRejected) {
    FunctionDescriptor desc;
    EXPECT_FALSE(SetFieldHint(desc, "a", "x-", "Input"));
}

}  // namespace croupier::sdk::test
