# Croupier C++ SDK - Dynamic Plugin System

The Croupier C++ SDK includes a comprehensive dynamic plugin system that allows you to extend functionality at runtime through dynamically loaded libraries (.dll/.so/.dylib).

## 🚀 Quick Start

### 1. Build the SDK with Plugin Support

```bash
cmake -B build -DBUILD_EXAMPLES=ON \
    -DCMAKE_TOOLCHAIN_FILE=[vcpkg-root]/scripts/buildsystems/vcpkg.cmake
cmake --build build --config Release
```

### 2. Run the Plugin Demo

```bash
# This will demonstrate plugin loading and function calling
./build/bin/croupier-plugin-demo
```

### 3. Test with the Example Plugin

```bash
# The example plugin will be built automatically to:
# - Windows: build/plugins/example_plugin.dll
# - macOS:   build/plugins/libexample_plugin.dylib
# - Linux:   build/plugins/libexample_plugin.so
```

## 📚 Plugin Development Guide

### Plugin Interface Requirements

Every plugin must export these standard C functions:

```cpp
extern "C" {
    // Initialize plugin (return 0 for success)
    int croupier_plugin_init();

    // Return plugin metadata
    PluginInfo* croupier_plugin_info();

    // Cleanup plugin resources
    void croupier_plugin_cleanup();
}
```

### Plugin Function Signature

All plugin functions must use this signature:

```cpp
extern "C" {
    const char* your_function_name(const char* context, const char* payload);
}
```

- **context**: Execution context (usually JSON string)
- **payload**: Input data (JSON string)
- **return**: Result data (JSON string, must be persistent)

### Example Plugin Implementation

```cpp
#include "croupier/sdk/plugin/dynamic_loader.h"

// Plugin metadata
static PluginInfo plugin_info = {
    "my_plugin",                     // name
    "1.0.0",                        // version
    "Your Name",                    // author
    "Plugin description",           // description
    {"function1", "function2"},     // provided_functions
    {{"license", "MIT"}}           // metadata
};

extern "C" {
    int croupier_plugin_init() {
        // Initialize resources
        return 0; // Success
    }

    PluginInfo* croupier_plugin_info() {
        return &plugin_info;
    }

    void croupier_plugin_cleanup() {
        // Cleanup resources
    }

    // Your custom functions
    const char* function1(const char* context, const char* payload) {
        static std::string result = R"({"result": "Hello from plugin!"})";
        return result.c_str();
    }
}
```

### CMakeLists.txt for Plugin

```cmake
# Create plugin shared library
add_library(my_plugin SHARED my_plugin.cpp)

# Include SDK headers
target_include_directories(my_plugin PRIVATE
    path/to/croupier-sdk/include
)

# Platform-specific settings
if(WIN32)
    set_target_properties(my_plugin PROPERTIES SUFFIX ".dll")
elseif(APPLE)
    set_target_properties(my_plugin PROPERTIES SUFFIX ".dylib")
else()
    set_target_properties(my_plugin PROPERTIES SUFFIX ".so")
endif()
```

## 🔧 Using the Plugin System

### Loading Plugins

```cpp
#include "croupier/sdk/plugin/dynamic_loader.h"

PluginManager plugin_manager;

// Load single plugin
bool success = plugin_manager.LoadPlugin("./plugins/my_plugin.so");

// Scan directory for plugins
auto found_plugins = plugin_manager.ScanPlugins("./plugins", true); // auto-load

// Enable auto-loading
plugin_manager.SetAutoLoading(true);
```

### Calling Plugin Functions

```cpp
// Get function handler
auto handler = plugin_manager.GetPluginFunction("my_plugin.function1");
if (handler) {
    std::string result = handler("context", R"({"input": "data"})");
    std::cout << "Result: " << result << std::endl;
}
```

### Integration with CroupierClient

```cpp
// Register all plugin functions with client
plugin_manager.RegisterPluginFunctions(client, "my_plugin");

// Now functions are available through the normal client interface
```

### Plugin Information

```cpp
// Get plugin metadata
PluginInfo info = plugin_manager.GetPluginInfo("my_plugin");
std::cout << "Plugin: " << info.name << " v" << info.version << std::endl;

// List plugin functions
auto functions = plugin_manager.GetPluginFunctions("my_plugin");
for (const auto& func : functions) {
    std::cout << "Function: " << func << std::endl;
}
```

## 🛡️ Security Considerations

### Safe Loading Practices

1. **Validate Plugin Sources**: Only load plugins from trusted sources
2. **Sandbox Plugins**: Consider running plugins in isolated processes
3. **Version Checking**: Verify plugin compatibility before loading
4. **Error Isolation**: Handle plugin errors gracefully without crashing

### Example Security Wrapper

```cpp
class SecurePluginManager {
public:
    bool LoadPluginSafely(const std::string& plugin_path) {
        // 1. Verify file signature/checksum
        if (!VerifyPluginIntegrity(plugin_path)) {
            return false;
        }

        // 2. Check plugin metadata
        PluginManager temp_manager;
        if (!temp_manager.LoadPlugin(plugin_path)) {
            return false;
        }

        auto info = temp_manager.GetPluginInfo(ExtractPluginName(plugin_path));
        if (!ValidatePluginSafety(info)) {
            temp_manager.UnloadPlugin(info.name);
            return false;
        }

        // 3. Load into main manager
        return main_manager_.LoadPlugin(plugin_path);
    }

private:
    PluginManager main_manager_;
};
```

## 🎯 Example Functions

The included example plugin provides these functions:

### hello
```bash
Input:  {"name": "World"}
Output: {"message": "Hello, World! Greetings from the example plugin."}
```

### calculate
```bash
Input:  {"operation": "add", "a": 15, "b": 25}
Output: {"result": 40.0, "operation": "add", "operands": {"a": 15, "b": 25}}
```

### time
```bash
Input:  {}
Output: {"timestamp": 1637123456, "formatted_time": "Wed Nov 17 10:30:56 2021"}
```

### echo
```bash
Input:  {"test": "data"}
Output: {"echo": {"context": "...", "payload": {"test": "data"}}}
```

## 🔍 Debugging Plugins

### Common Issues

**Plugin fails to load**
```cpp
// Check error message
std::string error = library_manager.GetLastError();
std::cout << "Load error: " << error << std::endl;
```

**Function not found**
```cpp
// Verify exported functions
auto functions = plugin_manager.GetPluginFunctions("my_plugin");
for (const auto& func : functions) {
    std::cout << "Available: " << func << std::endl;
}
```

**Runtime errors**
```cpp
// Set error callback
plugin_manager.SetErrorCallback([](const std::string& error) {
    std::cerr << "Plugin error: " << error << std::endl;
});
```

### Plugin Development Tips

1. **Use Static Storage**: Return values must persist after function returns
2. **Handle Exceptions**: Wrap plugin code in try-catch blocks
3. **Memory Management**: Be careful with dynamic allocations
4. **JSON Validation**: Validate input payload format
5. **Thread Safety**: Ensure functions are thread-safe if needed

## 📦 Distribution

### Plugin Packaging

```bash
# Create plugin package
mkdir my_plugin_v1.0.0/
cp my_plugin.so my_plugin_v1.0.0/
cp plugin_info.json my_plugin_v1.0.0/
cp README.md my_plugin_v1.0.0/
tar -czf my_plugin_v1.0.0.tar.gz my_plugin_v1.0.0/
```

### Installation

```bash
# Install plugin
tar -xzf my_plugin_v1.0.0.tar.gz
cp my_plugin_v1.0.0/my_plugin.so /usr/local/lib/croupier/plugins/
```

---

🎮 **Build powerful, extensible game backends with the Croupier Plugin System!**