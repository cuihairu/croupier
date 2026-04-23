# 插件示例

本示例展示如何创建和使用动态插件。

## 插件代码

### my_plugin.cpp

```cpp
#include "croupier/sdk/plugin/plugin.h"
#include "croupier/sdk/utils/json_utils.h"
#include <iostream>

class LoggingPlugin : public IPlugin {
public:
    std::string GetName() const override {
        return "logging-plugin";
    }

    std::string GetVersion() const override {
        return "0.1.0";
    }

    bool Initialize(const nlohmann::json& config) override {
        log_level_ = config.value("log_level", "info");
        std::cout << "[LoggingPlugin] Initialized with level: " << log_level_ << std::endl;
        return true;
    }

    void RegisterFunctions(CroupierClient& client) override {
        // 注册日志函数
        FunctionDescriptor log_desc;
        log_desc.id = "logging.write";
        log_desc.version = "0.1.0";
        log_desc.name = "Write Log";
        log_desc.description = "Write a log entry";

        client.RegisterFunction(log_desc, [this](const std::string& ctx, const std::string& payload) {
            return HandleLog(ctx, payload);
        });

        std::cout << "[LoggingPlugin] Functions registered" << std::endl;
    }

    void Shutdown() override {
        std::cout << "[LoggingPlugin] Shutdown" << std::endl;
    }

private:
    std::string HandleLog(const std::string& ctx, const std::string& payload) {
        auto data = utils::ParseJSON(payload);
        std::string level = data.value("level", "info");
        std::string message = data["message"];

        std::cout << "[" << level << "] " << message << std::endl;

        return utils::ToJSON({
            {"success", true},
            {"level", level},
            {"message", message}
        });
    }

    std::string log_level_;
};

// 导出插件
extern "C" {
    IPlugin* CreatePlugin() {
        return new LoggingPlugin();
    }

    void DestroyPlugin(IPlugin* plugin) {
        delete plugin;
    }

    const char* GetPluginAPIVersion() {
        return "0.1.0";
    }
}
```

### 插件 CMakeLists.txt

```cmake
cmake_minimum_required(VERSION 3.20)
project(logging_plugin)

set(CMAKE_CXX_STANDARD 17)

find_package(croupier-sdk REQUIRED)

add_library(logging_plugin SHARED my_plugin.cpp)
target_link_libraries(logging_plugin PRIVATE croupier-sdk::croupier-sdk)

# 设置输出目录
set_target_properties(logging_plugin PROPERTIES
    LIBRARY_OUTPUT_DIRECTORY "${CMAKE_BINARY_DIR}/plugins"
)
```

## 主程序代码

### main.cpp

```cpp
#include "croupier/sdk/croupier_client.h"
#include "croupier/sdk/plugin/dynamic_loader.h"
#include <iostream>
#include <filesystem>

using namespace croupier::sdk;

int main() {
    ClientConfig config;
    config.game_id = "demo-game";
    config.env = "development";
    config.agent_addr = "127.0.0.1:19090";

    CroupierClient client(config);

    // 加载插件
    plugin::DynamicLoader loader;

    std::string plugin_dir = "./plugins";
    std::cout << "Loading plugins from: " << plugin_dir << std::endl;

    for (const auto& entry : std::filesystem::directory_iterator(plugin_dir)) {
        if (entry.path().extension() == ".so" ||
            entry.path().extension() == ".dll" ||
            entry.path().extension() == ".dylib") {

            std::cout << "Loading: " << entry.path() << std::endl;

            if (loader.Load(entry.path().string())) {
                auto* plugin = loader.GetPlugin();

                // 初始化插件
                nlohmann::json plugin_config = {
                    {"log_level", "debug"}
                };

                if (plugin->Initialize(plugin_config)) {
                    // 注册插件函数
                    plugin->RegisterFunctions(client);
                }
            }
        }
    }

    // 连接并服务
    if (client.Connect()) {
        std::cout << "Connected! Serving..." << std::endl;
        client.Serve();
    }

    return 0;
}
```

## 构建

### 构建插件

```bash
cd plugins/logging_plugin
cmake -B build -DCMAKE_TOOLCHAIN_FILE=/path/to/vcpkg/scripts/buildsystems/vcpkg.cmake
cmake --build build
```

### 构建主程序

```bash
cd ..
cmake -B build -DCMAKE_TOOLCHAIN_FILE=/path/to/vcpkg/scripts/buildsystems/vcpkg.cmake
cmake --build build
```

## 目录结构

```
plugin-demo/
├── CMakeLists.txt
├── main.cpp
└── plugins/
    ├── logging_plugin/
    │   ├── CMakeLists.txt
    │   └── my_plugin.cpp
    └── liblogging_plugin.so  (构建后)
```

## 测试

```bash
# 运行主程序
./build/plugin-demo

# 在另一个终端调用插件函数
curl -X POST http://localhost:18780/api/v1/functions/logging.write/invoke \
  -H "Content-Type: application/json" \
  -H "X-Game-ID: demo-game" \
  -d '{
    "level": "info",
    "message": "This is a test log"
  }'
```

## 插件配置

### plugin_config.json

```json
{
  "plugins": [
    {
      "name": "logging-plugin",
      "path": "./plugins/liblogging_plugin.so",
      "enabled": true,
      "config": {
        "log_level": "debug",
        "output_file": "/var/log/game.log"
      }
    }
  ]
}
```

### 加载配置

```cpp
void LoadPluginsFromConfig(CroupierClient& client, const std::string& config_file) {
    auto config = utils::LoadJSON(config_file);

    for (const auto& plugin_config : config["plugins"]) {
        if (!plugin_config["enabled"].get<bool>()) {
            continue;
        }

        plugin::DynamicLoader loader;
        std::string path = plugin_config["path"];

        if (loader.Load(path)) {
            auto* plugin = loader.GetPlugin();
            plugin->Initialize(plugin_config["config"]);
            plugin->RegisterFunctions(client);
        }
    }
}
```
