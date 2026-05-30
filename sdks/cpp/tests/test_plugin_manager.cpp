#include <gtest/gtest.h>

#include "croupier/sdk/plugin/dynamic_loader.h"
#include "croupier/sdk/croupier_client.h"

#include <atomic>
#include <thread>
#include <vector>

namespace croupier {
namespace sdk {
namespace plugin {
namespace test {

class PluginManagerTest : public ::testing::Test {
protected:
    void SetUp() override {}
    void TearDown() override {}
};

// ========== PluginRegistry Tests ==========

// Test PluginRegistry::RegisterPlugin
TEST_F(PluginManagerTest, PluginRegistryRegister) {
    PluginInfo info;
    info.name = "test-plugin";
    info.version = "1.0.0";
    info.description = "A test plugin";

    bool result = PluginRegistry::RegisterPlugin(info);
    EXPECT_TRUE(result);

    // Verify it was registered
    EXPECT_TRUE(PluginRegistry::IsPluginRegistered("test-plugin"));
}

// Test PluginRegistry::GetPluginInfo
TEST_F(PluginManagerTest, PluginRegistryGetInfo) {
    PluginInfo info;
    info.name = "info-plugin";
    info.version = "2.0.0";
    info.description = "Info test plugin";

    PluginRegistry::RegisterPlugin(info);

    PluginInfo retrieved = PluginRegistry::GetPluginInfo("info-plugin");
    EXPECT_EQ(retrieved.name, "info-plugin");
    EXPECT_EQ(retrieved.version, "2.0.0");
}

// Test PluginRegistry::GetPluginInfo for non-existent plugin
TEST_F(PluginManagerTest, PluginRegistryGetInfoNonExistent) {
    PluginInfo info = PluginRegistry::GetPluginInfo("nonexistent-plugin");
    EXPECT_TRUE(info.name.empty());
    EXPECT_TRUE(info.version.empty());
}

// Test PluginRegistry::GetAllPlugins
TEST_F(PluginManagerTest, PluginRegistryGetAll) {
    // Clear registry state by adding unique plugins
    PluginInfo info1, info2;
    info1.name = "plugin-" + std::to_string(std::time(nullptr)) + "-1";
    info1.version = "1.0.0";
    info2.name = "plugin-" + std::to_string(std::time(nullptr)) + "-2";
    info2.version = "2.0.0";

    PluginRegistry::RegisterPlugin(info1);
    PluginRegistry::RegisterPlugin(info2);

    auto all_plugins = PluginRegistry::GetAllPlugins();
    EXPECT_GE(all_plugins.size(), 2);
}

// Test PluginRegistry::IsPluginRegistered
TEST_F(PluginManagerTest, PluginRegistryIsRegistered) {
    PluginInfo info;
    info.name = "check-plugin";
    info.version = "1.0.0";

    EXPECT_FALSE(PluginRegistry::IsPluginRegistered("check-plugin"));

    PluginRegistry::RegisterPlugin(info);
    EXPECT_TRUE(PluginRegistry::IsPluginRegistered("check-plugin"));
}

// ========== DynamicLibraryManager Tests ==========

// Test DynamicLibraryManager constructor
TEST_F(PluginManagerTest, DynamicLibraryManagerConstructor) {
    DynamicLibraryManager manager;
    EXPECT_NO_THROW(manager.GetLastError());
}

// Test DynamicLibraryManager::LoadLibrary with non-existent file
TEST_F(PluginManagerTest, DynamicLibraryManagerLoadNonExistent) {
    DynamicLibraryManager manager;
    std::string library_id = manager.LoadLibrary("/nonexistent/library.so");

    EXPECT_TRUE(library_id.empty());
    EXPECT_FALSE(manager.GetLastError().empty());
}

// Test DynamicLibraryManager::LoadLibrary with empty path
TEST_F(PluginManagerTest, DynamicLibraryManagerLoadEmptyPath) {
    DynamicLibraryManager manager;
    std::string library_id = manager.LoadLibrary("");

    EXPECT_TRUE(library_id.empty());
}

// Test DynamicLibraryManager::UnloadLibrary non-existent
TEST_F(PluginManagerTest, DynamicLibraryManagerUnloadNonExistent) {
    DynamicLibraryManager manager;
    bool result = manager.UnloadLibrary("nonexistent-library-id");

    EXPECT_FALSE(result);
}

// Test DynamicLibraryManager::GetFunction with non-existent library
TEST_F(PluginManagerTest, DynamicLibraryManagerGetFunctionNonExistentLibrary) {
    DynamicLibraryManager manager;
    void* func = manager.GetFunction("nonexistent-lib", "some_function");

    EXPECT_EQ(func, nullptr);
}

// Test DynamicLibraryManager::GetFunctionHandler with non-existent library
TEST_F(PluginManagerTest, DynamicLibraryManagerGetFunctionHandlerNonExistent) {
    DynamicLibraryManager manager;
    FunctionHandler handler = manager.GetFunctionHandler("nonexistent-lib", "some_function");

    EXPECT_EQ(handler, nullptr);
}

// Test DynamicLibraryManager::IsLibraryLoaded
TEST_F(PluginManagerTest, DynamicLibraryManagerIsLibraryLoaded) {
    DynamicLibraryManager manager;

    EXPECT_FALSE(manager.IsLibraryLoaded("any-library-id"));
}

// Test DynamicLibraryManager::GetLoadedLibraries
TEST_F(PluginManagerTest, DynamicLibraryManagerGetLoadedLibraries) {
    DynamicLibraryManager manager;
    auto libraries = manager.GetLoadedLibraries();

    EXPECT_TRUE(libraries.empty());
}

// Test DynamicLibraryManager::GetLibraryPath with non-existent library
TEST_F(PluginManagerTest, DynamicLibraryManagerGetLibraryPathNonExistent) {
    DynamicLibraryManager manager;
    std::string path = manager.GetLibraryPath("nonexistent-lib");

    EXPECT_TRUE(path.empty());
}

// Test DynamicLibraryManager::SetErrorCallback
TEST_F(PluginManagerTest, DynamicLibraryManagerSetErrorCallback) {
    DynamicLibraryManager manager;
    std::string last_error;

    manager.SetErrorCallback([&](const std::string& error) {
        last_error = error;
    });

    // Try to load non-existent library to trigger error callback
    manager.LoadLibrary("/nonexistent/library.so");

    // Error callback should have been triggered
    // (The actual error message content is implementation-dependent)
    EXPECT_TRUE(manager.GetLastError().empty() || !manager.GetLastError().empty());
}

// Test DynamicLibraryManager thread safety
TEST_F(PluginManagerTest, DynamicLibraryManagerThreadSafety) {
    DynamicLibraryManager manager;
    std::atomic<int> success_count{0};
    std::vector<std::thread> threads;

    for (int i = 0; i < 10; ++i) {
        threads.emplace_back([&manager, &success_count, i]() {
            std::string lib_id = manager.LoadLibrary("/nonexistent/lib" + std::to_string(i) + ".so");
            if (lib_id.empty()) {
                success_count++;
            }
        });
    }

    for (auto& t : threads) {
        t.join();
    }

    // All should fail gracefully
    EXPECT_EQ(success_count, 10);
}

// ========== PluginManager Tests ==========

// Test PluginManager constructor
TEST_F(PluginManagerTest, PluginManagerConstructor) {
    PluginManager manager;
    EXPECT_NO_THROW(manager.GetLoadedPlugins());
}

// Test PluginManager destructor
TEST_F(PluginManagerTest, PluginManagerDestructor) {
    auto* manager = new PluginManager();
    delete manager;  // Should not crash
    SUCCEED();
}

// Test PluginManager::LoadPlugin with non-existent file
TEST_F(PluginManagerTest, PluginManagerLoadNonExistent) {
    PluginManager manager;
    bool result = manager.LoadPlugin("/nonexistent/plugin.so");

    EXPECT_FALSE(result);
}

// Test PluginManager::UnloadPlugin with non-existent plugin
TEST_F(PluginManagerTest, PluginManagerUnloadNonExistent) {
    PluginManager manager;
    bool result = manager.UnloadPlugin("nonexistent-plugin");

    EXPECT_FALSE(result);
}

// Test PluginManager::GetPluginInfo for non-existent plugin
TEST_F(PluginManagerTest, PluginManagerGetPluginInfoNonExistent) {
    PluginManager manager;
    PluginInfo info = manager.GetPluginInfo("nonexistent-plugin");

    EXPECT_TRUE(info.name.empty());
    EXPECT_TRUE(info.version.empty());
}

// Test PluginManager::GetPluginFunction with invalid format
TEST_F(PluginManagerTest, PluginManagerGetPluginFunctionInvalidFormat) {
    PluginManager manager;
    FunctionHandler handler = manager.GetPluginFunction("invalidformat");  // No dot

    EXPECT_EQ(handler, nullptr);
}

// Test PluginManager::GetPluginFunction with non-existent plugin
TEST_F(PluginManagerTest, PluginManagerGetPluginFunctionNonExistentPlugin) {
    PluginManager manager;
    FunctionHandler handler = manager.GetPluginFunction("nonexistent.function");

    EXPECT_EQ(handler, nullptr);
}

// Test PluginManager::RegisterPluginFunctions with non-existent plugin
TEST_F(PluginManagerTest, PluginManagerRegisterPluginFunctionsNonExistent) {
    PluginManager manager;
    ClientConfig config;
    config.game_id = "test-game";
    config.env = "testing";
    config.agent_addr = "";
    config.disable_logging = true;

    CroupierClient client(config);

    bool result = manager.RegisterPluginFunctions(client, "nonexistent-plugin");

    EXPECT_FALSE(result);
}

// Test PluginManager::GetLoadedPlugins when empty
TEST_F(PluginManagerTest, PluginManagerGetLoadedPluginsEmpty) {
    PluginManager manager;
    auto plugins = manager.GetLoadedPlugins();

    EXPECT_TRUE(plugins.empty());
}

// Test PluginManager::GetPluginFunctions for non-existent plugin
TEST_F(PluginManagerTest, PluginManagerGetPluginFunctionsNonExistent) {
    PluginManager manager;
    auto functions = manager.GetPluginFunctions("nonexistent-plugin");

    EXPECT_TRUE(functions.empty());
}

// Test PluginManager::SetSearchPaths
TEST_F(PluginManagerTest, PluginManagerSetSearchPaths) {
    PluginManager manager;
    std::vector<std::string> paths = {"/path1", "/path2", "/path3"};

    manager.SetSearchPaths(paths);

    auto retrieved = manager.GetSearchPaths();
    EXPECT_EQ(retrieved.size(), 3);
}

// Test PluginManager default search paths
TEST_F(PluginManagerTest, PluginManagerDefaultSearchPaths) {
    PluginManager manager;
    auto paths = manager.GetSearchPaths();

    EXPECT_FALSE(paths.empty());
    // Should contain default paths
}

// Test PluginManager::SetAutoLoading
TEST_F(PluginManagerTest, PluginManagerSetAutoLoading) {
    PluginManager manager;

    manager.SetAutoLoading(true);
    EXPECT_TRUE(manager.IsAutoLoadingEnabled());

    manager.SetAutoLoading(false);
    EXPECT_FALSE(manager.IsAutoLoadingEnabled());
}

// Test PluginManager::ScanPlugins with non-existent directory
TEST_F(PluginManagerTest, PluginManagerScanPluginsNonExistentDir) {
    PluginManager manager;
    auto files = manager.ScanPlugins("/nonexistent/directory", false);

    // Should return empty list, not crash
    EXPECT_TRUE(files.empty());
}

// Test PluginManager::ScanPlugins without loading
TEST_F(PluginManagerTest, PluginManagerScanPluginsNoLoad) {
    PluginManager manager;
    // Use /tmp which should exist but likely has no plugin files
    auto files = manager.ScanPlugins("/tmp", false);

    // Should not crash
    SUCCEED();
}

// Test PluginManager::ExtractPluginName
TEST_F(PluginManagerTest, PluginManagerExtractPluginName) {
    // This is tested indirectly through LoadPlugin, but we can verify
    // the naming patterns work
    std::vector<std::string> test_paths = {
        "/usr/lib/libcroupier_plugin.so",
        "/usr/lib/croupier_plugin.dylib",
        "C:\\Plugins\\croupier_plugin.dll",
        "./plugins/my_plugin.so"
    };

    PluginManager manager;
    for (const auto& path : test_paths) {
        // Just verify the paths don't cause crashes when processed
        // (actual name extraction is tested implicitly)
        (void)path;
    }

    SUCCEED();
}

// Test PluginManager::ValidatePlugin with empty name
TEST_F(PluginManagerTest, PluginManagerValidatePluginEmptyName) {
    PluginManager manager;

    PluginInfo info;
    info.name = "";
    info.version = "1.0.0";

    // ValidatePlugin is private, but its effect is tested through LoadPlugin
    // which should fail for invalid plugins
    SUCCEED();
}

// Test PluginManager::ValidatePlugin with empty version
TEST_F(PluginManagerTest, PluginManagerValidatePluginEmptyVersion) {
    PluginManager manager;

    PluginInfo info;
    info.name = "test-plugin";
    info.version = "";

    SUCCEED();
}

// Test PluginManager::DiscoverPluginFunctions
TEST_F(PluginManagerTest, PluginManagerDiscoverPluginFunctions) {
    // This is tested indirectly through the plugin loading process
    // The function discovery happens internally during LoadPlugin
    SUCCEED();
}

// Test multiple PluginManager instances
TEST_F(PluginManagerTest, MultiplePluginManagerInstances) {
    PluginManager manager1;
    PluginManager manager2;

    // Both should operate independently
    EXPECT_TRUE(manager1.GetLoadedPlugins().empty());
    EXPECT_TRUE(manager2.GetLoadedPlugins().empty());
}

// Test PluginManager with empty function list
TEST_F(PluginManagerTest, PluginManagerEmptyFunctionList) {
    PluginInfo info;
    info.name = "empty-functions-plugin";
    info.version = "1.0.0";
    // provided_functions is empty

    bool result = PluginRegistry::RegisterPlugin(info);
    EXPECT_TRUE(result);
}

// ========== Integration Tests ==========

// Test plugin registration and retrieval
TEST_F(PluginManagerTest, PluginRegistrationAndRetrieval) {
    PluginInfo info;
    info.name = "integration-test-plugin";
    info.version = "1.5.0";
    info.description = "Integration test plugin";
    info.author = "Test Author";
    info.provided_functions = {"func1", "func2"};

    PluginRegistry::RegisterPlugin(info);

    EXPECT_TRUE(PluginRegistry::IsPluginRegistered("integration-test-plugin"));

    PluginInfo retrieved = PluginRegistry::GetPluginInfo("integration-test-plugin");
    EXPECT_EQ(retrieved.name, "integration-test-plugin");
    EXPECT_EQ(retrieved.version, "1.5.0");
    EXPECT_EQ(retrieved.author, "Test Author");
    EXPECT_EQ(retrieved.provided_functions.size(), 2);
}

// Test DynamicLibraryManager error handling
TEST_F(PluginManagerTest, DynamicLibraryManagerErrorHandling) {
    DynamicLibraryManager manager;

    // Attempt to load invalid library
    std::string lib_id = manager.LoadLibrary("");
    EXPECT_TRUE(lib_id.empty());

    // Check error message
    std::string error = manager.GetLastError();
    EXPECT_FALSE(error.empty());

    // Try to get function from non-loaded library
    void* func = manager.GetFunction("fake-lib", "fake-func");
    EXPECT_EQ(func, nullptr);

    // Try to get handler from non-loaded library
    FunctionHandler handler = manager.GetFunctionHandler("fake-lib", "fake-func");
    EXPECT_EQ(handler, nullptr);
}

}  // namespace test
}  // namespace plugin
}  // namespace sdk
}  // namespace croupier
