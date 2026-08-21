// DynamicLibraryManager and PluginManager exercised against real shared
// objects built by the test suite.
#include <gtest/gtest.h>
#include "croupier/sdk/plugin/dynamic_loader.h"
#include "croupier/sdk/croupier_client.h"
#include "croupier/sdk/utils/file_utils.h"

#include <algorithm>
#include <mutex>

#ifndef CROUPIER_TEST_PLUGIN_DIR
#define CROUPIER_TEST_PLUGIN_DIR "."
#endif

namespace croupier::sdk::plugin::test {
namespace {

std::string PluginPath(const std::string& name) {
    return std::string(CROUPIER_TEST_PLUGIN_DIR) + "/" + name;
}

class DynamicLoaderLifecycleTest : public ::testing::Test {
protected:
    DynamicLibraryManager manager_;
    std::vector<std::string> reported_errors_;
    std::mutex errors_mutex_;

    void SetUp() override {
        reported_errors_.clear();
        manager_.SetErrorCallback([this](const std::string& error) {
            std::lock_guard<std::mutex> lock(errors_mutex_);
            reported_errors_.push_back(error);
        });
    }
};

TEST_F(DynamicLoaderLifecycleTest, LoadNonExistentLibraryReportsError) {
    const std::string id = manager_.LoadLibrary("/definitely/not/a/library.so");
    EXPECT_TRUE(id.empty());
    EXPECT_FALSE(manager_.GetLastError().empty());
    ASSERT_FALSE(reported_errors_.empty());
    EXPECT_NE(manager_.GetLastError().find("/definitely/not/a/library.so"), std::string::npos);
}

TEST_F(DynamicLoaderLifecycleTest, LoadResolveAndUnloadSamplePlugin) {
    const std::string id = manager_.LoadLibrary(PluginPath("libcroupier-sample-plugin.so"));
    ASSERT_FALSE(id.empty());
    EXPECT_TRUE(manager_.IsLibraryLoaded(id));
    ASSERT_EQ(1U, manager_.GetLoadedLibraries().size());
    EXPECT_EQ(PluginPath("libcroupier-sample-plugin.so"), manager_.GetLibraryPath(id));
    EXPECT_TRUE(manager_.GetLastError().empty());

    void* echo = manager_.GetFunction(id, "sample_echo");
    ASSERT_NE(nullptr, echo);
    EXPECT_EQ(nullptr, manager_.GetFunction(id, "no_such_symbol"));
    EXPECT_NE(manager_.GetLastError().find("no_such_symbol"), std::string::npos);

    FunctionHandler handler = manager_.GetFunctionHandler(id, "sample_echo");
    ASSERT_TRUE(handler);
    const std::string result = handler("ctx", R"({"v":3})");
    EXPECT_NE(result.find(R"("echo":true)"), std::string::npos);
    EXPECT_NE(result.find(R"("payload":{"v":3})"), std::string::npos);

    // A missing function yields a null handler.
    EXPECT_FALSE(manager_.GetFunctionHandler(id, "sample_missing"));

    EXPECT_TRUE(manager_.UnloadLibrary(id));
    EXPECT_FALSE(manager_.IsLibraryLoaded(id));
    EXPECT_TRUE(manager_.GetLoadedLibraries().empty());
}

TEST_F(DynamicLoaderLifecycleTest, LoadingSameLibraryTwiceIsIdempotent) {
    const std::string first = manager_.LoadLibrary(PluginPath("libcroupier-sample-plugin.so"));
    ASSERT_FALSE(first.empty());
    const std::string second = manager_.LoadLibrary(PluginPath("libcroupier-sample-plugin.so"));
    EXPECT_EQ(first, second);
    EXPECT_TRUE(manager_.UnloadLibrary(first));
}

TEST_F(DynamicLoaderLifecycleTest, GetFunctionOnUnknownLibrary) {
    EXPECT_EQ(nullptr, manager_.GetFunction("no-such-id", "sample_echo"));
    EXPECT_NE(manager_.GetLastError().find("no-such-id"), std::string::npos);
}

TEST_F(DynamicLoaderLifecycleTest, UnloadUnknownLibraryFails) {
    EXPECT_FALSE(manager_.UnloadLibrary("no-such-id"));
    EXPECT_NE(manager_.GetLastError().find("no-such-id"), std::string::npos);
}

TEST(PluginManagerLifecycleTest, LoadPluginAndResolveFunctions) {
    PluginManager manager;
    ASSERT_TRUE(manager.LoadPlugin(PluginPath("libcroupier-sample-plugin.so")));

    const std::vector<std::string> plugins = manager.GetLoadedPlugins();
    ASSERT_EQ(1U, plugins.size());
    EXPECT_EQ("sample_plugin", plugins[0]);

    const PluginInfo info = manager.GetPluginInfo("sample_plugin");
    EXPECT_EQ("2.1.0", info.version);
    EXPECT_EQ(1U, info.metadata.size());

    const std::vector<std::string> functions = manager.GetPluginFunctions("sample_plugin");
    ASSERT_EQ(1U, functions.size());  // "sample_missing" has no symbol
    EXPECT_EQ("sample_echo", functions[0]);
    EXPECT_TRUE(manager.GetPluginFunctions("unknown_plugin").empty());

    FunctionHandler handler = manager.GetPluginFunction("sample_plugin.sample_echo");
    ASSERT_TRUE(handler);
    EXPECT_NE(handler("c", "{}").find(R"("echo":true)"), std::string::npos);
    EXPECT_FALSE(manager.GetPluginFunction("malformed_function_id"));
    EXPECT_FALSE(manager.GetPluginFunction("unknown_plugin.anything"));
    EXPECT_FALSE(manager.GetPluginFunction("sample_plugin.sample_missing"));

    EXPECT_TRUE(manager.UnloadPlugin("sample_plugin"));
    EXPECT_TRUE(manager.GetLoadedPlugins().empty());
    EXPECT_FALSE(manager.UnloadPlugin("sample_plugin"));  // already gone
    EXPECT_TRUE(manager.GetPluginInfo("sample_plugin").name.empty());  // unloaded -> empty info
}

TEST(PluginManagerLifecycleTest, RegisterPluginFunctionsRegistersWithClient) {
    PluginManager manager;
    ASSERT_TRUE(manager.LoadPlugin(PluginPath("libcroupier-sample-plugin.so")));

    ClientConfig config;
    config.game_id = "game-test";
    config.disable_logging = true;
    CroupierClient client(config);
    EXPECT_TRUE(manager.RegisterPluginFunctions(client, "sample_plugin"));
    EXPECT_FALSE(manager.RegisterPluginFunctions(client, "unknown_plugin"));
}

TEST(PluginManagerLifecycleTest, LoadPluginWithFailingInitFails) {
    PluginManager manager;
    EXPECT_FALSE(manager.LoadPlugin(PluginPath("libcroupier-failing-init-plugin.so")));
    EXPECT_TRUE(manager.GetLoadedPlugins().empty());
}

TEST(PluginManagerLifecycleTest, LoadPluginThatIsNotACroupierPlugin) {
    PluginManager manager;
    // The test binary itself exports no croupier_plugin_info.
    EXPECT_FALSE(manager.LoadPlugin("/proc/self/exe"));
    EXPECT_TRUE(manager.GetLoadedPlugins().empty());
}

TEST(PluginManagerLifecycleTest, ScanPluginsFindsAndOptionallyLoadsPlugins) {
    PluginManager manager;
    const std::vector<std::string> found = manager.ScanPlugins(CROUPIER_TEST_PLUGIN_DIR, false);
    EXPECT_FALSE(found.empty());
    EXPECT_EQ(0U, manager.GetLoadedPlugins().size());

    const std::vector<std::string> loaded = manager.ScanPlugins(CROUPIER_TEST_PLUGIN_DIR, true);
    EXPECT_EQ(found.size(), loaded.size());
    EXPECT_FALSE(manager.GetLoadedPlugins().empty());
}

TEST(PluginManagerLifecycleTest, SearchPathsAndAutoLoading) {
    PluginManager manager;
    EXPECT_FALSE(manager.IsAutoLoadingEnabled());

    manager.SetSearchPaths({"/tmp"});
    const std::vector<std::string> paths = manager.GetSearchPaths();
    ASSERT_EQ(1U, paths.size());
    EXPECT_EQ("/tmp", paths[0]);

    // Enabling auto-loading scans only existing directories.
    manager.SetAutoLoading(true);
    EXPECT_TRUE(manager.IsAutoLoadingEnabled());
    manager.SetAutoLoading(false);
    EXPECT_FALSE(manager.IsAutoLoadingEnabled());
}

TEST(PluginRegistryLifecycleTest, RegisterAndQueryMetadata) {
    PluginInfo info;
    info.name = "registry-test-plugin";
    info.version = "3.0.0";
    info.provided_functions = {"f1"};

    EXPECT_TRUE(PluginRegistry::RegisterPlugin(info));
    EXPECT_TRUE(PluginRegistry::IsPluginRegistered("registry-test-plugin"));
    EXPECT_EQ("3.0.0", PluginRegistry::GetPluginInfo("registry-test-plugin").version);
    EXPECT_FALSE(PluginRegistry::IsPluginRegistered("never-registered"));
    EXPECT_TRUE(PluginRegistry::GetPluginInfo("never-registered").name.empty());

    const std::vector<PluginInfo> all = PluginRegistry::GetAllPlugins();
    EXPECT_NE(std::find_if(all.begin(), all.end(),
                           [](const PluginInfo& item) { return item.name == "registry-test-plugin"; }),
              all.end());
}

}  // namespace
}  // namespace croupier::sdk::plugin::test
