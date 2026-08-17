# Croupier C++ SDK

Croupier C++ SDK 是 Croupier 的 C++ Provider/Invoker 客户端实现。SDK 只负责函数能力注册、长连接、调用和语言扩展，不承载控制台页面、菜单、分类或对象管理 UI 语义。

## 边界

- 函数注册只描述能力契约：`id`、`version`、`resource`、`operation`、`risk`、`enabled`、`permission`、`input_schema`、`output_schema`、`summary`、`description`、`tags`。
- SDK 不注册页面、菜单、分类展示名、Formily Schema 或对象管理配置。
- 控制台 UI 由后端 PageSpec / ConsoleMenuSpec / Page Studio 生成和编辑，不由 SDK 决定。
- C++ SDK 公开 API 只保留函数 Provider 和 Invoker。

## 能力

- Provider：`CroupierClient` 通过单条 TCP session 接入 Agent，注册本进程提供的函数并处理调用。
- Invoker：`CroupierInvoker` 仅调用 Server HTTP API，支持同步调用、异步任务、任务状态、流式事件和取消；不会复用 Provider TCP session。
- 连接韧性：心跳、自动重连、重注册。
- 安全配置：TLS、认证头、文件传输白名单。
- 扩展：动态插件、Lua/Skynet 绑定。

## 快速开始

```bash
cmake -B build \
  -DCMAKE_TOOLCHAIN_FILE=./cmake/vcpkg-bootstrap.cmake \
  -DCMAKE_BUILD_TYPE=Release
cmake --build build --parallel
```

运行示例前请先启动 Croupier Agent，本地 SDK gateway 默认地址为 `127.0.0.1:19091`。

```bash
./build/bin/croupier-example
./build/bin/croupier-game-demo
./build/bin/croupier-config-example
./build/bin/croupier-plugin-demo
```

## Provider 示例

```cpp
#include "croupier/sdk/croupier_client.h"

using namespace croupier::sdk;

std::string GetProfile(const std::string&, const std::string&) {
    return R"({"player_id":"p1","level":10})";
}

int main() {
    ClientConfig config;
    config.game_id = "demo-game";
    config.env = "development";
    config.service_id = "cpp-provider";
    config.agent_addr = "127.0.0.1:19091";

    CroupierClient client(config);

    FunctionDescriptor desc;
    desc.id = "player.profile.get";
    desc.version = "1.0.0";
    desc.resource = "player";
    desc.operation = "profile.get";
    desc.risk = "safe";
    desc.summary = "Get player profile";

    client.RegisterFunction(desc, GetProfile);
    client.Connect();
    client.Serve();
}
```

## Invoker 示例

```cpp
#include "croupier/sdk/croupier_client.h"

using namespace croupier::sdk;

int main() {
    InvokerConfig config;
    config.address = "http://127.0.0.1:18780/api/v1";
    config.game_id = "demo-game";
    config.env = "development";

    CroupierInvoker invoker(config);
    invoker.Connect();

    std::string result = invoker.Invoke("player.profile.get", R"({"player_id":"p1"})");
    TaskStatus status = invoker.GetTaskStatus("server-task-id");
    invoker.Close();
}
```

## 示例程序

| 示例                 | 说明                   | 命令                                  |
| -------------------- | ---------------------- | ------------------------------------- |
| `example.cpp`        | 最小函数 Provider 示例 | `./build/bin/croupier-example`        |
| `game_demo.cpp`      | 游戏后台函数注册示例   | `./build/bin/croupier-game-demo`      |
| `config_example.cpp` | 客户端配置示例         | `./build/bin/croupier-config-example` |
| `plugin_demo.cpp`    | 动态插件注册函数示例   | `./build/bin/croupier-plugin-demo`    |

## 主要 API

```cpp
class CroupierClient {
public:
    bool RegisterFunction(const FunctionDescriptor& desc, FunctionHandler handler);
    bool Connect();
    bool IsConnected() const;
    void Serve();
    void Stop();
    void Close();
};

class CroupierInvoker {
public:
    bool Connect();
    std::string Invoke(const std::string& function_id,
                       const std::string& payload,
                       const InvokeOptions& options = {});
    std::string StartTask(const std::string& function_id,
                          const std::string& payload,
                          const InvokeOptions& options = {});
    TaskStatus GetTaskStatus(const std::string& task_id);
    std::future<std::vector<TaskEvent>> StreamTask(const std::string& task_id);
    bool CancelTask(const std::string& task_id);
    void Close();
};
```

## 构建选项

| 选项                 | 默认值 | 说明                 |
| -------------------- | ------ | -------------------- |
| `BUILD_SHARED_LIBS`  | `OFF`  | 构建动态库           |
| `BUILD_STATIC_LIBS`  | `ON`   | 构建静态库           |
| `BUILD_EXAMPLES`     | `ON`   | 构建示例程序         |
| `BUILD_TESTS`        | `OFF`  | 构建测试             |
| `ENABLE_LUA_BINDING` | `OFF`  | 构建 Lua/Skynet 绑定 |

## 依赖

- C++17
- CMake 3.20+
- Protobuf 4.25.x
- nlohmann/json 3.12.x
- libcurl（Server HTTP transport）

依赖通过 vcpkg 管理，默认 preset 使用 `cmake/vcpkg-bootstrap.cmake` 自动准备仓库内 vcpkg。

## 目录

```text
sdks/cpp/
├── include/     # 公共头文件
├── src/         # SDK 实现
├── examples/    # 示例程序
├── tests/       # 单元测试
├── skynet/      # Skynet 集成
├── cmake/       # CMake 模块
└── vcpkg.json   # vcpkg 依赖清单
```
