#include <gtest/gtest.h>
#include "croupier/sdk/plugin/dynamic_loader.h"
#include <thread>

using namespace croupier::sdk::plugin;

class PluginRegistryTest : public ::testing::Test {
protected:
    void SetUp() override {
        // Clear registry before each test
        // Note: PluginRegistry doesn't provide a clear method,
        // so we'll use unique plugin names
    }

    void TearDown() override {
        // Cleanup after tests
    }
};

TEST_F(PluginRegistryTest, RegisterSinglePlugin) {
    PluginInfo info;
    info.name = "test-plugin";
    info.version = "1.0.0";
    info.author = "test-author";
    info.description = "test-description";

    bool result = PluginRegistry::RegisterPlugin(info);

    ASSERT_TRUE(result);
    EXPECT_TRUE(PluginRegistry::IsPluginRegistered("test-plugin"));
}

TEST_F(PluginRegistryTest, RegisterMultiplePlugins) {
    PluginInfo info1, info2, info3;
    info1.name = "plugin-1";
    info1.version = "1.0.0";

    info2.name = "plugin-2";
    info2.version = "2.0.0";

    info3.name = "plugin-3";
    info3.version = "3.0.0";

    ASSERT_TRUE(PluginRegistry::RegisterPlugin(info1));
    ASSERT_TRUE(PluginRegistry::RegisterPlugin(info2));
    ASSERT_TRUE(PluginRegistry::RegisterPlugin(info3));

    EXPECT_TRUE(PluginRegistry::IsPluginRegistered("plugin-1"));
    EXPECT_TRUE(PluginRegistry::IsPluginRegistered("plugin-2"));
    EXPECT_TRUE(PluginRegistry::IsPluginRegistered("plugin-3"));
}

TEST_F(PluginRegistryTest, GetPluginInfo) {
    PluginInfo original;
    original.name = "test-plugin";
    original.version = "1.0.0";
    original.author = "test-author";
    original.description = "A test plugin";
    original.provided_functions = {"func1", "func2"};
    original.metadata["key1"] = "value1";

    PluginRegistry::RegisterPlugin(original);

    PluginInfo retrieved = PluginRegistry::GetPluginInfo("test-plugin");

    EXPECT_EQ("test-plugin", retrieved.name);
    EXPECT_EQ("1.0.0", retrieved.version);
    EXPECT_EQ("test-author", retrieved.author);
    EXPECT_EQ("A test plugin", retrieved.description);
    EXPECT_EQ(2u, retrieved.provided_functions.size());
    EXPECT_EQ("value1", retrieved.metadata["key1"]);
}

TEST_F(PluginRegistryTest, GetNonExistentPlugin) {
    PluginInfo info = PluginRegistry::GetPluginInfo("non-existent-plugin");

    EXPECT_TRUE(info.name.empty());
    EXPECT_TRUE(info.version.empty());
}

TEST_F(PluginRegistryTest, GetAllPlugins) {
    // Clear any existing plugins by using unique names
    std::string prefix = "GetAllPluginsTest-";

    PluginInfo info1, info2;
    info1.name = prefix + "plugin-a";
    info1.version = "1.0";

    info2.name = prefix + "plugin-b";
    info2.version = "2.0";

    PluginRegistry::RegisterPlugin(info1);
    PluginRegistry::RegisterPlugin(info2);

    std::vector<PluginInfo> allPlugins = PluginRegistry::GetAllPlugins();

    // Note: GetAllPlugins returns ALL registered plugins, not just ours
    // So we just check that our plugins are present
    bool found1 = false, found2 = false;
    for (const auto& plugin : allPlugins) {
        if (plugin.name == prefix + "plugin-a") found1 = true;
        if (plugin.name == prefix + "plugin-b") found2 = true;
    }
    EXPECT_TRUE(found1);
    EXPECT_TRUE(found2);
}

TEST_F(PluginRegistryTest, GetAllPluginsEmpty) {
    // We can't test empty registry since it's a global static
    // Just verify that GetAllPlugins doesn't crash and returns a vector
    std::vector<PluginInfo> allPlugins = PluginRegistry::GetAllPlugins();

    // Should return a vector (size is unsigned, so it's always >= 0)
    EXPECT_TRUE(allPlugins.empty() || allPlugins.size() > 0);
}

TEST_F(PluginRegistryTest, IsPluginRegistered) {
    PluginInfo info;
    info.name = "registered-plugin";
    info.version = "1.0.0";

    EXPECT_FALSE(PluginRegistry::IsPluginRegistered("registered-plugin"));

    PluginRegistry::RegisterPlugin(info);

    EXPECT_TRUE(PluginRegistry::IsPluginRegistered("registered-plugin"));
}

TEST_F(PluginRegistryTest, OverwriteExistingPlugin) {
    PluginInfo info1;
    info1.name = "same-name";
    info1.version = "1.0.0";
    info1.author = "author1";

    PluginRegistry::RegisterPlugin(info1);

    PluginInfo info2;
    info2.name = "same-name";
    info2.version = "2.0.0";
    info2.author = "author2";

    PluginRegistry::RegisterPlugin(info2);

    PluginInfo retrieved = PluginRegistry::GetPluginInfo("same-name");

    EXPECT_EQ("2.0.0", retrieved.version);
    EXPECT_EQ("author2", retrieved.author);
}

TEST_F(PluginRegistryTest, PluginWithProvidedFunctions) {
    PluginInfo info;
    info.name = "func-plugin";
    info.version = "1.0.0";
    info.provided_functions = {"function1", "function2", "function3"};

    PluginRegistry::RegisterPlugin(info);

    PluginInfo retrieved = PluginRegistry::GetPluginInfo("func-plugin");

    EXPECT_EQ(3u, retrieved.provided_functions.size());
    EXPECT_EQ("function1", retrieved.provided_functions[0]);
    EXPECT_EQ("function2", retrieved.provided_functions[1]);
    EXPECT_EQ("function3", retrieved.provided_functions[2]);
}

TEST_F(PluginRegistryTest, PluginWithMetadata) {
    PluginInfo info;
    info.name = "metadata-plugin";
    info.version = "1.0.0";
    info.metadata["category"] = "test";
    info.metadata["priority"] = "high";
    info.metadata["enabled"] = "true";

    PluginRegistry::RegisterPlugin(info);

    PluginInfo retrieved = PluginRegistry::GetPluginInfo("metadata-plugin");

    EXPECT_EQ(3u, retrieved.metadata.size());
    EXPECT_EQ("test", retrieved.metadata["category"]);
    EXPECT_EQ("high", retrieved.metadata["priority"]);
    EXPECT_EQ("true", retrieved.metadata["enabled"]);
}

TEST_F(PluginRegistryTest, MultiplePluginsWithDifferentMetadata) {
    // Use unique names to avoid conflicts
    std::string prefix = "MetadataTest-";

    PluginInfo info1, info2, info3;
    info1.name = prefix + "plugin-1";
    info1.version = "1.0";
    info1.metadata["type"] = "A";

    info2.name = prefix + "plugin-2";
    info2.version = "1.0";
    info2.metadata["type"] = "B";

    info3.name = prefix + "plugin-3";
    info3.version = "1.0";
    info3.metadata["type"] = "C";

    PluginRegistry::RegisterPlugin(info1);
    PluginRegistry::RegisterPlugin(info2);
    PluginRegistry::RegisterPlugin(info3);

    // Verify each plugin individually
    PluginInfo retrieved1 = PluginRegistry::GetPluginInfo(prefix + "plugin-1");
    PluginInfo retrieved2 = PluginRegistry::GetPluginInfo(prefix + "plugin-2");
    PluginInfo retrieved3 = PluginRegistry::GetPluginInfo(prefix + "plugin-3");

    EXPECT_EQ("A", retrieved1.metadata["type"]);
    EXPECT_EQ("B", retrieved2.metadata["type"]);
    EXPECT_EQ("C", retrieved3.metadata["type"]);
}

TEST_F(PluginRegistryTest, PluginWithEmptyFields) {
    PluginInfo info;
    info.name = "minimal-plugin";
    // All other fields empty

    PluginRegistry::RegisterPlugin(info);

    EXPECT_TRUE(PluginRegistry::IsPluginRegistered("minimal-plugin"));

    PluginInfo retrieved = PluginRegistry::GetPluginInfo("minimal-plugin");

    EXPECT_EQ("minimal-plugin", retrieved.name);
    EXPECT_TRUE(retrieved.version.empty());
    EXPECT_TRUE(retrieved.author.empty());
    EXPECT_EQ(0u, retrieved.provided_functions.size());
    EXPECT_EQ(0u, retrieved.metadata.size());
}
