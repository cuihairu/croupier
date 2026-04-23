# 基础函数示例

本示例展示如何注册一个简单的函数。

## 完整代码

```cpp
#include "croupier/sdk/croupier_client.h"
#include "croupier/sdk/utils/json_utils.h"
#include <iostream>

using namespace croupier::sdk;

// 函数处理器
std::string HelloHandler(const std::string& context, const std::string& payload) {
    std::cout << "Context: " << context << std::endl;
    std::cout << "Payload: " << payload << std::endl;

    auto data = utils::ParseJSON(payload);
    std::string name = data.value("name", "World");

    return utils::ToJSON({
        {"message", "Hello, " + name + "!"},
        {"timestamp", std::chrono::system_clock::now()}
    });
}

std::string AddHandler(const std::string& context, const std::string& payload) {
    auto data = utils::ParseJSON(payload);
    int a = data["a"];
    int b = data["b"];

    return utils::ToJSON({
        {"result", a + b}
    });
}

int main() {
    // 配置客户端
    ClientConfig config;
    config.game_id = "demo-game";
    config.env = "development";
    config.agent_addr = "127.0.0.1:19090";

    // 创建客户端
    CroupierClient client(config);

    // 注册函数
    FunctionDescriptor hello_desc;
    hello_desc.id = "hello.world";
    hello_desc.version = "0.1.0";
    hello_desc.name = "Hello World";
    hello_desc.description = "A simple greeting function";

    client.RegisterFunction(hello_desc, HelloHandler);

    // 注册数学函数
    client.RegisterFunction(
        FunctionDescriptor{"math.add", "0.1.0", "Add", "Add two numbers"},
        AddHandler
    );

    // 连接并服务
    std::cout << "Connecting to agent..." << std::endl;
    if (client.Connect()) {
        std::cout << "Connected! Serving..." << std::endl;
        client.Serve();  // 阻塞
    }

    return 0;
}
```

## 编译

### CMakeLists.txt

```cmake
cmake_minimum_required(VERSION 3.20)
project(basic_function)

set(CMAKE_CXX_STANDARD 17)

find_package(croupier-sdk REQUIRED)

add_executable(basic_function main.cpp)
target_link_libraries(basic_function PRIVATE croupier-sdk::croupier-sdk)
```

### 构建命令

```bash
cmake -B build -DCMAKE_TOOLCHAIN_FILE=/path/to/vcpkg/scripts/buildsystems/vcpkg.cmake
cmake --build build
```

## 运行

```bash
./build/basic_function
```

## 测试

### 使用 curl 调用

```bash
# 调用 hello.world
curl -X POST http://localhost:18780/api/v1/functions/hello.world/invoke \
  -H "Content-Type: application/json" \
  -H "X-Game-ID: demo-game" \
  -d '{"name": "Croupier"}'

# 调用 math.add
curl -X POST http://localhost:18780/api/v1/functions/math.add/invoke \
  -H "Content-Type: application/json" \
  -H "X-Game-ID: demo-game" \
  -d '{"a": 5, "b": 3}'
```

### 预期响应

```json
// hello.world
{
  "message": "Hello, Croupier!",
  "timestamp": "2024-01-01T00:00:00Z"
}

// math.add
{
  "result": 8
}
```

## 输出示例

程序运行时输出：

```
Connecting to agent...
Connected! Serving...
Context: {"game_id":"demo-game","env":"development","caller_id":"web-ui"}
Payload: {"name":"Croupier"}
```
