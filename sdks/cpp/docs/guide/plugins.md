# 插件系统

Croupier C++ SDK 支持通过插件系统动态扩展功能。

## 插件概述

插件（Plugin）是动态加载的共享库，可以在运行时加载以扩展 SDK 功能。

### 插件类型

1. **函数插件**: 提供额外的函数处理
2. **虚拟对象插件**: 提供虚拟对象和处理器
3. **中间件插件**: 拦截和修改请求/响应

## 创建插件

### 基础插件结构

```cpp
// my_plugin.cpp
#include "croupier/sdk/plugin/plugin.h"

class MyPlugin : public IPlugin {
public:
    std::string GetName() const override {
        return "my-plugin";
    }

    std::string GetVersion() const override {
        return "0.1.0";
    }

    bool Initialize(const nlohmann::json& config) override {
        // 初始化逻辑
        std::cout << "MyPlugin initialized" << std::endl;
        return true;
    }

    void RegisterFunctions(CroupierClient& client) override {
        // 注册函数
        client.RegisterFunction(
            FunctionDescriptor{"plugin.hello", "0.1.0"},
            [](const std::string& ctx, const std::string& payload) {
                return R"({"message": "Hello from plugin!"})";
            }
        );
    }

    void Shutdown() override {
        // 清理资源
        std::cout << "MyPlugin shutdown" << std::endl;
    }
};

// 导出插件
extern "C" {
    IPlugin* CreatePlugin() {
        return new MyPlugin();
    }

    void DestroyPlugin(IPlugin* plugin) {
        delete plugin;
    }
}
```

### 插件 CMakeLists.txt

```cmake
cmake_minimum_required(VERSION 3.20)
project(MyPlugin)

set(CMAKE_CXX_STANDARD 17)

# 查找 Croupier SDK
find_package(croupier-sdk REQUIRED)

# 创建共享库
add_library(my_plugin SHARED my_plugin.cpp)
target_link_libraries(my_plugin PRIVATE croupier-sdk::croupier-sdk)

# 设置输出目录
set_target_properties(my_plugin PROPERTIES
    LIBRARY_OUTPUT_DIRECTORY "${CMAKE_BINARY_DIR}/plugins"
)

# Windows 上导出符号
if(WIN32)
    target_compile_definitions(my_plugin PRIVATE MY_PLUGIN_EXPORTS)
endif()
```

## 加载插件

### 使用插件加载器

```cpp
#include "croupier/sdk/plugin/dynamic_loader.h"

int main() {
    ClientConfig config;
    config.game_id = "demo-game";
    config.env = "development";
    config.agent_addr = "127.0.0.1:19090";

    CroupierClient client(config);

    // 创建插件加载器
    plugin::DynamicLoader loader;

    // 加载插件
    std::string plugin_path = "./plugins/my_plugin.so";  // Linux
    // std::string plugin_path = "./plugins/my_plugin.dll";  // Windows

    if (loader.Load(plugin_path)) {
        auto* plugin = loader.GetPlugin();

        // 初始化插件
        nlohmann::json plugin_config;
        plugin->Initialize(plugin_config);

        // 注册插件函数
        plugin->RegisterFunctions(client);
    }

    client.Connect();
    client.Serve();

    // 清理
    loader.Unload();

    return 0;
}
```

### 批量加载插件

```cpp
void LoadAllPlugins(CroupierClient& client, const std::string& plugin_dir) {
    plugin::DynamicLoader loader;

    for (const auto& entry : std::filesystem::directory_iterator(plugin_dir)) {
        if (entry.path().extension() == ".so" ||
            entry.path().extension() == ".dll" ||
            entry.path().extension() == ".dylib") {

            std::cout << "Loading plugin: " << entry.path() << std::endl;

            if (loader.Load(entry.path().string())) {
                auto* plugin = loader.GetPlugin();
                plugin->Initialize({});
                plugin->RegisterFunctions(client);
            }
        }
    }
}
```

## 插件配置

### 配置文件

```json
{
  "plugins": [
    {
      "name": "my-plugin",
      "path": "./plugins/my_plugin.so",
      "enabled": true,
      "config": {
        "setting1": "value1",
        "setting2": 42
      }
    }
  ]
}
```

### 配置加载器

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

## 中间件插件

### 请求拦截

```cpp
class LoggingMiddleware : public IMiddleware {
public:
    std::string Process(const std::string& context, const std::string& payload,
                       HandlerFunc next) override {
        std::cout << "[Request] " << payload << std::endl;

        // 调用下一个处理器
        std::string result = next(context, payload);

        std::cout << "[Response] " << result << std::endl;

        return result;
    }
};
```

### 认证中间件

```cpp
class AuthMiddleware : public IMiddleware {
public:
    std::string Process(const std::string& context, const std::string& payload,
                       HandlerFunc next) override {
        auto ctx = utils::ParseJSON(context);

        // 检查认证信息
        if (!ctx.contains("auth") || !ctx["auth"]["authenticated"].get<bool>()) {
            return R"({"success": false, "error": {"code": "UNAUTHORIZED"}})";
        }

        // 认证通过，继续处理
        return next(context, payload);
    }
};
```

## 内置插件

### 限流插件

```cpp
class RateLimitPlugin : public IPlugin {
public:
    bool Initialize(const nlohmann::json& config) override {
        max_requests_ = config.value("max_requests", 100);
        window_seconds_ = config.value("window_seconds", 60);
        return true;
    }

    std::string Process(const std::string& ctx, const std::string& payload,
                       HandlerFunc next) override {
        std::string player_id = utils::ParseJSON(ctx)["player_id"];

        if (!CheckRateLimit(player_id)) {
            return R"({"success": false, "error": {"code": "RATE_LIMIT_EXCEEDED"}})";
        }

        return next(ctx, payload);
    }

private:
    bool CheckRateLimit(const std::string& player_id) {
        auto now = std::chrono::system_clock::now();
        // 实现限流逻辑...
        return true;
    }

    int max_requests_;
    int window_seconds_;
};
```

### 缓存插件

```cpp
class CachePlugin : public IPlugin {
public:
    std::string Process(const std::string& ctx, const std::string& payload,
                       HandlerFunc next) override {
        std::string cache_key = GenerateCacheKey(ctx, payload);

        // 检查缓存
        if (auto cached = GetFromCache(cache_key)) {
            return *cached;
        }

        // 执行实际处理
        std::string result = next(ctx, payload);

        // 存入缓存
        SetCache(cache_key, result, std::chrono::minutes(5));

        return result;
    }

private:
    std::string GenerateCacheKey(const std::string& ctx, const std::string& payload) {
        return ctx + ":" + payload;
    }

    std::optional<std::string> GetFromCache(const std::string& key);
    void SetCache(const std::string& key, const std::string& value, std::chrono::seconds ttl);
};
```

## 最佳实践

### 1. 错误处理

```cpp
bool Initialize(const nlohmann::json& config) override {
    try {
        // 初始化逻辑
        return true;
    } catch (const std::exception& e) {
        std::cerr << "Plugin initialization failed: " << e.what() << std::endl;
        return false;
    }
}
```

### 2. 资源清理

```cpp
class DatabasePlugin : public IPlugin {
private:
    DatabaseConnection* connection_;

public:
    bool Initialize(const nlohmann::json& config) override {
        connection_ = new DatabaseConnection(config["connection_string"]);
        return connection_->Connect();
    }

    void Shutdown() override {
        if (connection_) {
            connection_->Close();
            delete connection_;
        }
    }
};
```

### 3. 版本兼容性

```cpp
std::string GetMinSDKVersion() const override {
    return "0.1.0";
}

bool IsCompatible(const std::string& sdk_version) const override {
    // 实现版本兼容性检查
    return true;
}
```
