# 快速开始

本教程将带你创建第一个使用 Croupier C++ SDK 的应用程序。

## 前置准备

确保你已经：
1. 安装了 [Croupier Agent](https://github.com/cuihairu/croupier)
2. 完成了 SDK 的 [安装](./installation.md)

## 第一个程序

### 1. 创建项目文件

`main.cpp`:

```cpp
#include "croupier/sdk/croupier_client.h"
#include <iostream>

using namespace croupier::sdk;

// 定义函数处理器
std::string HelloHandler(const std::string& context, const std::string& payload) {
    std::cout << "Context: " << context << std::endl;
    std::cout << "Payload: " << payload << std::endl;
    return R"({"message": "Hello from C++ SDK!"})";
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
    FunctionDescriptor desc;
    desc.id = "hello.world";
    desc.version = "0.1.0";
    desc.name = "Hello World";
    desc.description = "A simple hello world function";

    client.RegisterFunction(desc, HelloHandler);

    // 连接并服务
    if (client.Connect()) {
        std::cout << "Connected to agent!" << std::endl;
        client.Serve(); // 阻塞运行
    }

    return 0;
}
```

### 2. 创建 CMakeLists.txt

```cmake
cmake_minimum_required(VERSION 3.20)
project(HelloCroupier)

set(CMAKE_CXX_STANDARD 17)

# 查找 Croupier SDK
find_package(croupier-sdk REQUIRED)

add_executable(hello-croupier main.cpp)
target_link_libraries(hello-croupier PRIVATE croupier-sdk::croupier-sdk)
```

### 3. 构建和运行

```bash
# 配置
cmake -B build -DCMAKE_TOOLCHAIN_FILE=/path/to/vcpkg/scripts/buildsystems/vcpkg.cmake

# 构建
cmake --build build

# 运行（确保 Agent 已启动）
./build/hello-croupier
```

## 虚拟对象示例

注册一个虚拟对象（如钱包）：

```cpp
#include "croupier/sdk/croupier_client.h"

// 钱包获取处理器
std::string WalletGetHandler(const std::string& context, const std::string& payload) {
    // 解析 payload 获取玩家 ID
    auto data = utils::ParseJSON(payload);
    std::string player_id = data["player_id"];

    // 返回钱包信息
    return utils::ToJSON({
        {"player_id", player_id},
        {"balance", 1000},
        {"currency", "gold"}
    });
}

// 转账处理器
std::string WalletTransferHandler(const std::string& context, const std::string& payload) {
    auto data = utils::ParseJSON(payload);
    // 执行转账逻辑...
    return R"({"success": true})";
}

int main() {
    ClientConfig config;
    config.game_id = "mmorpg-demo";
    config.env = "development";
    config.agent_addr = "127.0.0.1:19090";

    CroupierClient client(config);

    // 注册虚拟对象
    VirtualObjectDescriptor wallet_entity;
    wallet_entity.id = "wallet.entity";
    wallet_entity.version = "0.1.0";
    wallet_entity.name = "Wallet";
    wallet_entity.description = "Player wallet entity";

    // 定义操作
    wallet_entity.operations["get"] = "wallet.get";
    wallet_entity.operations["transfer"] = "wallet.transfer";

    // 注册操作处理器
    std::map<std::string, FunctionHandler> handlers;
    handlers["wallet.get"] = WalletGetHandler;
    handlers["wallet.transfer"] = WalletTransferHandler;

    client.RegisterVirtualObject(wallet_entity, handlers);

    client.Connect();
    client.Serve();

    return 0;
}
```

## 使用配置文件

### 创建配置文件 `config.json`:

```json
{
  "game_id": "demo-game",
  "env": "development",
  "service_id": "backend-service",
  "agent_addr": "127.0.0.1:19090",
  "insecure": true,
  "timeout_seconds": 30
}
```

### 使用配置加载器：

```cpp
#include "croupier/sdk/config/client_config_loader.h"

int main() {
    // 从文件加载配置
    ClientConfigLoader loader;
    ClientConfig config = loader.LoadFromFile("./config.json");

    // 验证配置
    auto errors = loader.ValidateConfig(config);
    if (!errors.empty()) {
        for (const auto& error : errors) {
            std::cerr << "Config error: " << error << std::endl;
        }
        return 1;
    }

    CroupierClient client(config);
    // ... 继续使用客户端
}
```

## 下一步

- [构建指南](./building.md) - 了解更多构建选项
- [API 参考](../api/) - 详细 API 文档
- [示例](../examples/) - 更多使用示例
