# Croupier C++ SDK：虚拟对象注册系统

[![Build Status](https://github.com/cuihairu/croupier-sdk-cpp/actions/workflows/cpp-sdk-build.yml/badge.svg)](https://github.com/cuihairu/croupier-sdk-cpp/actions/workflows/cpp-sdk-build.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![C++17](https://img.shields.io/badge/C%2B%2B-17-blue.svg)](https://en.cppreference.com/w/cpp/17)

高性能的C++ SDK，支持虚拟对象注册、gRPC通信、多平台构建。采用**ID引用模式**解决对象参数传递性能问题，通过**四层组件化架构**实现优雅的函数和对象管理。

## 🎯 核心特性

### ✅ **已实现功能**
- 🏗️ **虚拟对象注册系统** - 完整的对象和函数组注册机制
- 📡 **gRPC集成** - 真实的protobuf+gRPC通信（通过vcpkg）
- 🔧 **多平台支持** - Windows、Linux、macOS（x64/ARM64）
- 📦 **vcpkg包管理** - 自动依赖管理和跨平台构建
- 🚀 **高性能设计** - ID引用模式，无状态函数，轻量参数
- 🛡️ **类型安全** - JSON Schema验证，编译时类型检查
- 🔄 **自动构建** - GitHub Actions每日构建发布

### 🔧 **架构亮点**
- **四层抽象模型**：Function → Entity → Resource → Component
- **ID引用模式**：只传递ID字符串，避免对象实例序列化
- **声明式配置**：JSON驱动，配置与代码分离
- **多环境支持**：game_id + env 后台环境隔离

## 📋 快速开始

### 系统要求

**基础要求：**
- **C++17** 编译器（GCC 8+, Clang 10+, MSVC 2019+）
- **CMake 3.20+**
- **vcpkg**（推荐，用于依赖管理）

**依赖库（自动安装）：**
- gRPC (通过vcpkg)
- Protobuf (通过vcpkg)
- nlohmann/json (通过vcpkg)

### 🚀 一键构建

#### **方式1：使用构建脚本（推荐）**

**Linux/macOS:**
```bash
# 基础构建
./scripts/build.sh

# 清理重构建
./scripts/build.sh --clean

# 启用测试
./scripts/build.sh --tests ON

# 交叉编译 ARM64（Linux）
./scripts/build.sh --arch arm64

# 自定义 vcpkg 路径
./scripts/build.sh --vcpkg-root /path/to/vcpkg
```

**Windows:**
```powershell
# 基础构建
.\scripts\build.ps1

# x86 构建
.\scripts\build.ps1 -Platform x86

# Debug 构建
.\scripts\build.ps1 -BuildType Debug

# 启用测试
.\scripts\build.ps1 -Tests ON
```

#### **方式2：手动CMake构建**

```bash
# 1. 设置 vcpkg（如果未安装）
git clone https://github.com/Microsoft/vcpkg.git
cd vcpkg
./bootstrap-vcpkg.sh  # Windows: .\bootstrap-vcpkg.bat
export VCPKG_ROOT=$(pwd)

# 2. 配置构建
cmake -B build \
  -DCMAKE_TOOLCHAIN_FILE=$VCPKG_ROOT/scripts/buildsystems/vcpkg.cmake \
  -DCMAKE_BUILD_TYPE=Release \
  -DENABLE_GRPC=ON \
  -DBUILD_EXAMPLES=ON

# 3. 构建
cmake --build build --parallel

# 4. 运行示例
./build/bin/virtual-object-demo
```

## 💡 使用示例

### 示例1：基础函数注册

```cpp
#include "croupier/sdk/croupier_client.h"
using namespace croupier::sdk;

// 简单函数处理器
std::string TransferHandler(const std::string& context, const std::string& payload) {
    auto data = utils::ParseJSON(payload);

    // 通过ID获取对象（避免传递重对象）
    std::string from_player = data["from_player_id"];
    std::string to_player = data["to_player_id"];
    std::string amount = data["amount"];

    // 执行业务逻辑
    return ExecuteTransfer(from_player, to_player, amount);
}

int main() {
    ClientConfig config;
    config.game_id = "mmorpg-demo";      // 🎮 游戏标识
    config.env = "development";          // 🔧 环境隔离
    config.agent_addr = "127.0.0.1:19090";

    CroupierClient client(config);

    // 注册函数
    FunctionDescriptor desc{"wallet.transfer", "1.0.0"};
    client.RegisterFunction(desc, TransferHandler);

    client.Connect();
    client.Serve();  // 阻塞服务
}
```

### 示例2：虚拟对象注册（推荐）

```cpp
#include "croupier/sdk/croupier_client.h"
using namespace croupier::sdk;

int main() {
    ClientConfig config;
    config.game_id = "economy-system";
    config.env = "production";

    CroupierClient client(config);

    // ========== 定义钱包实体 ==========
    VirtualObjectDescriptor wallet_entity;
    wallet_entity.id = "wallet.entity";
    wallet_entity.version = "1.0.0";
    wallet_entity.name = "钱包实体";
    wallet_entity.description = "玩家钱包管理虚拟对象";

    // 定义 Schema
    wallet_entity.schema["type"] = "object";
    wallet_entity.schema["properties"] = R"({
        "wallet_id": {"type": "string"},
        "player_id": {"type": "string"},
        "balance": {"type": "string", "pattern": "^[0-9]+\\.?[0-9]*$"}
    })";

    // 定义操作映射（核心设计！）
    wallet_entity.operations["read"] = "wallet.get";
    wallet_entity.operations["transfer"] = "wallet.transfer";
    wallet_entity.operations["deposit"] = "wallet.deposit";

    // 定义关系
    RelationshipDef currency_rel;
    currency_rel.type = "many-to-one";
    currency_rel.entity = "currency";
    currency_rel.foreign_key = "currency_id";
    wallet_entity.relationships["currency"] = currency_rel;

    // ========== 准备函数处理器 ==========
    std::map<std::string, FunctionHandler> handlers;
    handlers["wallet.get"] = WalletGetHandler;
    handlers["wallet.transfer"] = WalletTransferHandler;
    handlers["wallet.deposit"] = WalletDepositHandler;

    // ========== 一键注册整个对象 ==========
    if (client.RegisterVirtualObject(wallet_entity, handlers)) {
        std::cout << "✅ 成功注册虚拟对象: " << wallet_entity.id << std::endl;
    }

    client.Connect();
    client.Serve();
}
```

### 示例3：组件级注册（最优雅）

```cpp
// ========== 经济系统组件注册 ==========
ComponentDescriptor economy_system;
economy_system.id = "economy-system";
economy_system.version = "1.0.0";
economy_system.name = "完整经济系统";

// 添加多个实体
economy_system.entities.push_back(wallet_entity);
economy_system.entities.push_back(currency_entity);
economy_system.entities.push_back(market_entity);

// 添加跨实体函数
FunctionDescriptor market_trade{"market.trade", "1.0.0"};
economy_system.functions.push_back(market_trade);

// ========== 一键注册整个组件 ==========
client.RegisterComponent(economy_system);
```

### 示例4：配置文件驱动（生产推荐）

```cpp
int main() {
    CroupierClient client(config);

    // 🎯 一行代码注册整个系统！
    client.LoadComponentFromFile("economy-system.json");

    client.Connect();
    client.Serve();
}
```

**economy-system.json:**
```json
{
  "id": "economy-system",
  "version": "1.0.0",
  "name": "经济系统组件",
  "entities": [
    {
      "id": "wallet.entity",
      "operations": {
        "read": "wallet.get",
        "transfer": "wallet.transfer"
      },
      "relationships": {
        "currency": {
          "type": "many-to-one",
          "entity": "currency",
          "foreign_key": "currency_id"
        }
      }
    }
  ]
}
```

## 🏗️ 架构设计

### 四层组件化架构

```
Function Level    ← wallet.transfer (具体函数实现)
     ↓
Entity Level      ← wallet.entity (业务对象模型)
     ↓
Resource Level    ← 钱包管理面板 (UI资源组织)
     ↓
Component Level   ← economy-system (可分发模块)
```

### ID引用模式的优势

**❌ 避免笨重的对象参数：**
```cpp
// 传统方式：传递整个对象实例
invoke("wallet.transfer", {object: wallet_instance, params: {...}})
```

**✅ 优雅的ID引用设计：**
```cpp
// Croupier方式：只传递ID，后端通过Repository获取对象
invoke("wallet.transfer", {
  from_player_id: "player123",  // 轻量级ID引用
  to_player_id: "player456",
  currency_code: "gold",
  amount: "100.0"
})
```

**核心优势：**
- 🚀 **极致性能**：网络传输只有几十字节
- 🛡️ **线程安全**：无状态函数，天然支持并发
- 🔄 **水平扩展**：函数可以部署到任意节点
- 🧩 **松耦合**：对象管理与业务逻辑完全分离

## 🔧 API参考

### 核心数据结构

```cpp
// 虚拟对象描述符
struct VirtualObjectDescriptor {
    std::string id;                              // 对象标识
    std::string version;                         // 版本号
    std::string name;                            // 显示名称
    std::string description;                     // 描述
    std::map<std::string, std::string> schema;   // JSON Schema
    std::map<std::string, std::string> operations; // 操作映射
    std::map<std::string, RelationshipDef> relationships; // 关系定义
};

// 客户端配置
struct ClientConfig {
    std::string agent_addr = "127.0.0.1:19090";
    std::string game_id;           // 🎮 必需：游戏标识符
    std::string env = "development"; // 🔧 环境：development/staging/production
    std::string service_id = "cpp-service";

    // TLS配置
    bool insecure = true;          // 开发环境：true，生产环境：false
    std::string cert_file;         // 客户端证书
    std::string key_file;          // 私钥
    std::string ca_file;           // CA证书
};
```

### 主要API

```cpp
class CroupierClient {
public:
    // ========== 基础函数注册 ==========
    bool RegisterFunction(const FunctionDescriptor& desc, FunctionHandler handler);

    // ========== 虚拟对象注册（推荐） ==========
    bool RegisterVirtualObject(
        const VirtualObjectDescriptor& desc,
        const std::map<std::string, FunctionHandler>& handlers
    );

    // ========== 组件级注册（最优雅） ==========
    bool RegisterComponent(const ComponentDescriptor& comp);
    bool LoadComponentFromFile(const std::string& config_file);

    // ========== 管理接口 ==========
    std::vector<VirtualObjectDescriptor> GetRegisteredObjects() const;
    std::vector<ComponentDescriptor> GetRegisteredComponents() const;
    bool UnregisterVirtualObject(const std::string& object_id);
    bool UnregisterComponent(const std::string& component_id);

    // ========== 核心操作 ==========
    bool Connect();
    void Serve();    // 阻塞服务
    void Stop();
    void Close();
};
```

### 工具函数

```cpp
namespace croupier::sdk::utils {
    // 验证
    bool ValidateObjectDescriptor(const VirtualObjectDescriptor& desc);
    bool ValidateComponentDescriptor(const ComponentDescriptor& comp);

    // 配置文件
    VirtualObjectDescriptor LoadObjectDescriptor(const std::string& file_path);
    ComponentDescriptor LoadComponentDescriptor(const std::string& file_path);

    // 模板生成
    std::string GenerateObjectTemplate(const std::string& object_id);
    std::string GenerateComponentTemplate(const std::string& component_id);

    // 序列化
    std::string ObjectDescriptorToJSON(const VirtualObjectDescriptor& desc);
    std::string ComponentDescriptorToJSON(const ComponentDescriptor& comp);
}
```

## 🚀 部署和分发

### GitHub Actions自动构建

每日自动构建，支持多平台：

- ✅ **Windows** (x64, x86)
- ✅ **Linux** (x64, ARM64)
- ✅ **macOS** (x64, ARM64 Apple Silicon)

**构建产物：**
- 静态库 (.a/.lib)
- 动态库 (.so/.dylib/.dll)
- 头文件包
- 示例程序
- CMake配置文件

### 手动触发构建

```bash
# 在GitHub仓库页面
Actions → C++ SDK Build & Release → Run workflow

# 选择构建类型
- nightly: 每日构建
- release: 正式发布
- patch: 补丁版本
```

### 下载预构建包

访问 [Releases页面](https://github.com/cuihairu/croupier/releases) 下载最新构建：

- `croupier-cpp-sdk-{version}-windows-x64.zip`
- `croupier-cpp-sdk-{version}-linux-x64.tar.gz`
- `croupier-cpp-sdk-{version}-macos-arm64.tar.gz`

## 📦 在项目中使用

### 方式1：CMake find_package

```cmake
find_package(croupier-sdk REQUIRED)

add_executable(my_game src/main.cpp)
target_link_libraries(my_game croupier::sdk)
```

### 方式2：vcpkg集成

```bash
# 安装 SDK（未来支持）
vcpkg install croupier-cpp-sdk
```

```cmake
find_package(croupier-cpp-sdk CONFIG REQUIRED)
target_link_libraries(my_game croupier::sdk)
```

### 方式3：Git子模块

```bash
git submodule add https://github.com/cuihairu/croupier.git
```

```cmake
add_subdirectory(croupier/sdks/cpp)
target_link_libraries(my_game croupier::sdk)
```

## 🔧 开发环境搭建

### 本地开发

```bash
# 1. 克隆仓库
git clone --recursive https://github.com/cuihairu/croupier.git
cd croupier/sdks/cpp

# 2. 安装 vcpkg（如果没有）
git clone https://github.com/Microsoft/vcpkg.git
cd vcpkg && ./bootstrap-vcpkg.sh
export VCPKG_ROOT=$(pwd)
cd ..

# 3. 构建
./scripts/build.sh --examples ON --tests ON

# 4. 运行示例
./build/bin/virtual-object-demo
```

### IDE集成

**Visual Studio Code:**
```json
{
  "cmake.configureArgs": [
    "-DCMAKE_TOOLCHAIN_FILE=${env:VCPKG_ROOT}/scripts/buildsystems/vcpkg.cmake"
  ],
  "cmake.buildArgs": ["--parallel"]
}
```

**CLion:**
- Settings → Build → CMake
- CMake options: `-DCMAKE_TOOLCHAIN_FILE=$VCPKG_ROOT/scripts/buildsystems/vcpkg.cmake`

## 📚 进阶主题

### 生产环境配置

```cpp
ClientConfig production_config;
production_config.game_id = "my-production-game";
production_config.env = "production";
production_config.agent_addr = "croupier-agent.internal:19090";

// 🛡️ 启用安全配置
production_config.insecure = false;
production_config.cert_file = "/etc/croupier/client.crt";
production_config.key_file = "/etc/croupier/client.key";
production_config.ca_file = "/etc/croupier/ca.crt";
production_config.auth_token = "your-jwt-token";

CroupierClient client(production_config);
```

### 性能优化

```cpp
// 1. 连接池配置
config.timeout_seconds = 30;
config.heartbeat_interval = 60;

// 2. 并发处理
// SDK内部线程安全，支持多线程并发注册

// 3. 内存优化
// 使用move语义避免拷贝
auto handlers = std::move(handler_map);
client.RegisterVirtualObject(std::move(descriptor), handlers);
```

### 错误处理

```cpp
try {
    client.RegisterVirtualObject(desc, handlers);
    client.Connect();
    client.Serve();
} catch (const std::exception& e) {
    spdlog::error("SDK error: {}", e.what());

    // 实现重试逻辑
    std::this_thread::sleep_for(std::chrono::seconds(5));
    // 重新连接...
}
```

## 🤝 贡献指南

1. **Fork** 项目
2. 创建特性分支：`git checkout -b feature/amazing-feature`
3. 提交更改：`git commit -m 'Add amazing feature'`
4. 推送分支：`git push origin feature/amazing-feature`
5. 创建 **Pull Request**

### 开发规范

- 遵循 **C++17** 标准
- 使用 **clang-format** 格式化代码
- 编写单元测试
- 更新相关文档

## 📄 License

本项目采用 [MIT License](LICENSE) 开源协议。

## 🔗 相关链接

- 📖 **主项目**: [cuihairu/croupier](https://github.com/cuihairu/croupier)
- 🏗️ **架构文档**: [虚拟对象设计](VIRTUAL_OBJECT_REGISTRATION.md)
- 🐛 **问题报告**: [GitHub Issues](https://github.com/cuihairu/croupier/issues)
- 💬 **讨论区**: [GitHub Discussions](https://github.com/cuihairu/croupier/discussions)

---

**通过这套架构，您可以优雅地管理复杂的游戏业务对象，同时保持高性能和良好的开发体验！** 🎮✨
