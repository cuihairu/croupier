#include "croupier/sdk/plugin/dynamic_loader.h"
#include "croupier/sdk/croupier_client.h"
#include "croupier/sdk/utils/file_utils.h"
#include <iostream>
#include <exception>

using namespace croupier::sdk;
using namespace croupier::sdk::plugin;
using namespace croupier::sdk::utils;

void DisplayPluginInfo(const PluginInfo& info) {
    std::cout << "📋 Plugin Information:" << std::endl;
    std::cout << "  Name: " << info.name << std::endl;
    std::cout << "  Version: " << info.version << std::endl;
    std::cout << "  Author: " << info.author << std::endl;
    std::cout << "  Description: " << info.description << std::endl;

    std::cout << "  Functions:" << std::endl;
    for (const auto& func : info.provided_functions) {
        std::cout << "    - " << func << std::endl;
    }

    if (!info.metadata.empty()) {
        std::cout << "  Metadata:" << std::endl;
        for (const auto& [key, value] : info.metadata) {
            std::cout << "    " << key << ": " << value << std::endl;
        }
    }
}

void TestPluginFunction(PluginManager& plugin_manager, const std::string& function_id, const std::string& test_payload) {
    std::cout << "\n🎯 Testing function: " << function_id << std::endl;
    std::cout << "  Input: " << test_payload << std::endl;

    auto handler = plugin_manager.GetPluginFunction(function_id);
    if (handler) {
        try {
            std::string result = handler("test-context", test_payload);
            std::cout << "  ✅ Output: " << result << std::endl;
        } catch (const std::exception& e) {
            std::cout << "  ❌ Error: " << e.what() << std::endl;
        }
    } else {
        std::cout << "  ❌ Function not found!" << std::endl;
    }
}

int main(int /* argc */, char* /* argv */[]) {
    std::cout << "🚀 Croupier C++ SDK - Dynamic Plugin System Demo" << std::endl;
    std::cout << "================================================" << std::endl;

    try {
        // Initialize plugin manager
        PluginManager plugin_manager;

        // Build the example plugin first (if not already built)
        std::string plugin_file;

#ifdef _WIN32
        plugin_file = "./examples/plugins/example_plugin.dll";
#elif defined(__APPLE__)
        plugin_file = "./examples/plugins/libexample_plugin.dylib";
#else
        plugin_file = "./examples/plugins/libexample_plugin.so";
#endif

        // Check if plugin file exists
        if (!FileSystemUtils::FileExists(plugin_file)) {
            std::cout << "⚠️ Plugin file not found: " << plugin_file << std::endl;
            std::cout << "💡 You need to build the example plugin first:" << std::endl;
            std::cout << "   cmake --build build --target example_plugin" << std::endl;
            std::cout << "\n🔍 Searching for plugins in current directory..." << std::endl;

            // Try to find plugins in various locations
            std::vector<std::string> search_paths = {"./", "./plugins", "./examples/plugins", "./build/plugins"};
            bool found = false;

            for (const std::string& path : search_paths) {
                if (FileSystemUtils::DirectoryExists(path)) {
                    auto plugins = plugin_manager.ScanPlugins(path, false);
                    if (!plugins.empty()) {
                        std::cout << "  Found plugins in " << path << ":" << std::endl;
                        for (const auto& plugin : plugins) {
                            std::cout << "    - " << plugin << std::endl;
                        }
                        plugin_file = plugins[0]; // Use the first found plugin
                        found = true;
                        break;
                    }
                }
            }

            if (!found) {
                std::cout << "❌ No plugins found. Please build the example plugin first." << std::endl;
                return 1;
            }
        }

        std::cout << "📂 Loading plugin: " << plugin_file << std::endl;

        // Load the plugin
        bool loaded = plugin_manager.LoadPlugin(plugin_file);
        if (!loaded) {
            std::cout << "❌ Failed to load plugin!" << std::endl;
            return 1;
        }

        // Get list of loaded plugins
        auto loaded_plugins = plugin_manager.GetLoadedPlugins();
        std::cout << "✅ Loaded plugins (" << loaded_plugins.size() << "):" << std::endl;
        for (const std::string& name : loaded_plugins) {
            std::cout << "  - " << name << std::endl;
        }

        // Get plugin information
        if (!loaded_plugins.empty()) {
            std::string plugin_name = loaded_plugins[0];
            PluginInfo info = plugin_manager.GetPluginInfo(plugin_name);
            std::cout << std::endl;
            DisplayPluginInfo(info);

            // Test plugin functions
            std::cout << "\n🧪 Testing Plugin Functions" << std::endl;
            std::cout << "===========================" << std::endl;

            // Test hello function
            TestPluginFunction(plugin_manager, plugin_name + ".hello", R"({"name": "Croupier User"})");

            // Test calculate function
            TestPluginFunction(plugin_manager, plugin_name + ".calculate", R"({"operation": "add", "a": 15, "b": 25})");
            TestPluginFunction(plugin_manager, plugin_name + ".calculate", R"({"operation": "multiply", "a": 7, "b": 6})");

            // Test time function
            TestPluginFunction(plugin_manager, plugin_name + ".time", "{}");

            // Test echo function
            TestPluginFunction(plugin_manager, plugin_name + ".echo", R"({"test": "data", "number": 42})");

            // Integration with CroupierClient
            std::cout << "\n🔗 Integration with CroupierClient" << std::endl;
            std::cout << "==================================" << std::endl;

            // Create client configuration
            ClientConfig config;
            config.game_id = "plugin-demo-game";
            config.env = "development";
            config.service_id = "plugin-demo-service";
            config.agent_addr = "127.0.0.1:19090";
            config.local_listen = "0.0.0.0:0";
            config.insecure = true;

            CroupierClient client(config);

            // Register plugin functions with the client
            std::cout << "📝 Registering plugin functions with client..." << std::endl;
            bool registered = plugin_manager.RegisterPluginFunctions(client, plugin_name);

            if (registered) {
                std::cout << "✅ All plugin functions registered successfully!" << std::endl;

                // Get list of registered objects from client
                auto client_objects = client.GetRegisteredObjects();
                std::cout << "📋 Client now has " << client_objects.size() << " registered objects:" << std::endl;
                for (const auto& obj : client_objects) {
                    std::cout << "  - " << obj.id << " (v" << obj.version << ")" << std::endl;
                }

                // Try to connect to agent (this will fail if no agent is running, which is normal)
                std::cout << "\n🔌 Testing client connection..." << std::endl;
                bool connected = client.Connect();
                if (connected) {
                    std::cout << "✅ Successfully connected to Croupier Agent!" << std::endl;
                    std::cout << "🎮 Client is ready to handle function calls from external sources" << std::endl;
                } else {
                    std::cout << "⚠️ Could not connect to Croupier Agent (this is normal if no Agent is running)" << std::endl;
                    std::cout << "💡 Plugin functions are registered and ready to use" << std::endl;
                }

            } else {
                std::cout << "❌ Failed to register some plugin functions" << std::endl;
            }

            // Demonstrate direct function calling
            std::cout << "\n🎯 Direct Function Call Demo" << std::endl;
            std::cout << "============================" << std::endl;

            auto hello_handler = plugin_manager.GetPluginFunction(plugin_name + ".hello");
            if (hello_handler) {
                std::string result = hello_handler("demo-context", R"({"name": "Plugin System"})");
                std::cout << "📞 Direct call result: " << result << std::endl;
            }
        }

        // Plugin management demo
        std::cout << "\n🛠️ Plugin Management Demo" << std::endl;
        std::cout << "=========================" << std::endl;

        // Show plugin functions
        for (const std::string& plugin_name : loaded_plugins) {
            auto functions = plugin_manager.GetPluginFunctions(plugin_name);
            std::cout << "Functions in " << plugin_name << ":" << std::endl;
            for (const std::string& func : functions) {
                std::cout << "  - " << func << std::endl;
            }
        }

        // Auto-loading demo
        std::cout << "\n🔄 Auto-loading Demo" << std::endl;
        std::cout << "====================" << std::endl;

        plugin_manager.SetAutoLoading(true);
        std::cout << "Auto-loading enabled. The system will automatically scan and load plugins from search paths." << std::endl;

        auto search_paths = plugin_manager.GetSearchPaths();
        std::cout << "Search paths:" << std::endl;
        for (const std::string& path : search_paths) {
            std::cout << "  - " << path << std::endl;
        }

        std::cout << "\n🎉 Plugin system demonstration completed successfully!" << std::endl;
        std::cout << "💡 Key features demonstrated:" << std::endl;
        std::cout << "   ✅ Dynamic library loading" << std::endl;
        std::cout << "   ✅ Plugin information discovery" << std::endl;
        std::cout << "   ✅ Function resolution and calling" << std::endl;
        std::cout << "   ✅ CroupierClient integration" << std::endl;
        std::cout << "   ✅ Plugin lifecycle management" << std::endl;

    } catch (const std::exception& e) {
        std::cout << "💥 Error: " << e.what() << std::endl;
        return 1;
    }

    return 0;
}