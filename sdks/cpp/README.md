<p align="center">
  <h1 align="center">Croupier C++ SDK</h1>
  <p align="center">
    <strong>高性能 C++ SDK，用于 Croupier 游戏函数注册与虚拟对象管理</strong>
  </p>
</p>

<p align="center">
  <a href="https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-cpp.yml">
    <img src="https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-cpp.yml/badge.svg" alt="CI">
  </a>
  <a href="https://codecov.io/gh/cuihairu/croupier">
    <img src="https://codecov.io/gh/cuihairu/croupier/branch/main/graph/badge.svg?flag=cpp-sdk" alt="Coverage">
  </a>
  <a href="https://www.apache.org/licenses/LICENSE-2.0">
    <img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg" alt="License">
  </a>
  <a href="https://en.cppreference.com/w/cpp/17">
    <img src="https://img.shields.io/badge/C%2B%2B-17-blue.svg" alt="C++17">
  </a>
</p>

<p align="center">
  <a href="#支持平台">
    <img src="https://img.shields.io/badge/platform-Windows%20%7C%20Linux%20%7C%20macOS-lightgrey.svg" alt="Platform">
  </a>
  <a href="https://github.com/cuihairu/croupier">
    <img src="https://img.shields.io/badge/Main%20Project-Croupier-green.svg" alt="Main Project">
  </a>
</p>

---

## 📋 目录

- [简介](#简介)
- [主项目](#主项目)
- [其他语言 SDK](#其他语言-sdk)
- [支持平台](#支持平台)
- [核心特性](#核心特性)
- [快速开始](#快速开始)
- [使用示例](#使用示例)
- [完整文档](#完整文档)
- [架构设计](#架构设计)
- [API 参考](#api-参考)
- [部署与分发](#部署与分发)
- [开发指南](#开发指南)
- [贡献指南](#贡献指南)
- [许可证](#许可证)

---

## 简介

Croupier C++ SDK 是 [Croupier](https://github.com/cuihairu/croupier) 游戏后端平台的官方 C++ 客户端实现。SDK 作为 **Provider 端被调用方**，通过 **单条 TCP session**（`sdk-agent subprotocol`）接入 Agent，提供高性能函数注册、心跳、自动重连、TLS；并在此基础上提供 C++ 专属的虚拟对象（VirtualObject）/ 组件（Component）/ Lua 绑定 / 动态插件等语言扩展。

## 主项目

| 项目 | 描述 | 链接 |
|------|------|------|
| **Croupier** | 游戏后端平台主项目 | [cuihairu/croupier](https://github.com/cuihairu/croupier) |
| **Croupier Proto** | Protobuf 协议定义 | [proto/](https://github.com/cuihairu/croupier/tree/main/proto) |

## 其他语言 SDK

所有 SDK 现已整合到主 monorepo 的 `sdks/` 目录下：

| 语言 | 目录 | CI | Docs |
| --- | --- | --- | --- |
| Go | [sdks/go/](https://github.com/cuihairu/croupier/tree/main/sdks/go) | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-go.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-go.yml) | [README](../go/README.md) |
| Java | [sdks/java/](https://github.com/cuihairu/croupier/tree/main/sdks/java) | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-java.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-java.yml) | [README](../java/README.md) |
| JS/TS | [sdks/js/](https://github.com/cuihairu/croupier/tree/main/sdks/js) | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-js.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-js.yml) | [README](../js/README.md) |
| Python | [sdks/python/](https://github.com/cuihairu/croupier/tree/main/sdks/python) | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-python.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-python.yml) | [README](../python/README.md) |
| C# | [sdks/csharp/](https://github.com/cuihairu/croupier/tree/main/sdks/csharp) | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-csharp.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-csharp.yml) | [README](../csharp/README.md) |

## 支持平台

> **⚠️ 重要说明**：本 SDK **仅支持 64 位 (x64/ARM64) 架构**，不支持 32 位 (x86) 架构。

| 平台 | 架构 | 状态 |
|------|------|------|
| **Windows** | x64 | ✅ 支持 |
| **Linux** | x64 | ✅ 支持 |
| **macOS** | x64, ARM64 (Apple Silicon) | ✅ 支持 |

## 核心特性

按 [功能矩阵](../SDK_FEATURE_MATRIX.md) 分层：

**L1 Core Provider（必备）**

- 📡 **TCP session 客户端** - 单条 `sdk-agent subprotocol` 长连接，不监听本地端口
- 🤝 **握手与心跳** - `ProviderConnectRequest` 协商，`ProviderHeartbeatRequest` 保活
- 🔁 **自动重连** - `auto_reconnect` / `reconnect_interval_seconds` / `reconnect_max_attempts`
- 📝 **函数注册** - `FunctionDescriptor` + `FunctionHandler`
- 🛡️ **类型安全** - 编译时类型检查

**L2 Provider 扩展（可选）**

- 🔐 **TLS** - `cert_file` / `key_file` / `ca_file` / `server_name`
- 📋 **JSON Schema 校验** - 描述符 `input_schema` / `output_schema`
- 📦 **文件传输** - `enable_file_transfer=true`，受白名单（`allowed_extensions` / `allowed_mime_types`）与上限（`max_file_size`）约束

**L3 Invoker（独立调用方）**

- 🚀 `CroupierInvoker` 提供同步调用 / 异步任务 / 流式事件 / 取消

**L4 语言/引擎扩展（仅 C++ 提供）**

- 🏗️ **虚拟对象注册** - `RegisterVirtualObject`（业务对象模型 + 操作映射）
- 🧩 **组件系统** - `RegisterComponent` / `LoadComponentFromFile`（Function → Entity → Resource → Component 四层）
- 🔌 **动态插件** - `plugin/dynamic_loader`
- 🐯 **Lua 绑定** - `bindings/lua_binding_sol2`
- 🌌 **Skynet 集成** - `skynet/`

## 快速开始

### 系统要求

- **64位操作系统** (Windows x64 / Linux x64 / macOS x64 or ARM64)
- **C++17** 编译器（GCC 8+, Clang 10+, MSVC 2019+）
- **CMake 3.20+**
- **vcpkg**（推荐，用于依赖管理）
- **Ninja**（推荐，用于更快构建）

### 依赖库（自动安装）

- **Protobuf 4.25.x** (通过 vcpkg) - **版本固定以确保 ABI 兼容性**
- nlohmann/json 3.12.x (通过 vcpkg)

> **⚠️ 重要提示**：Protobuf 版本已固定为 **4.25.x** 系列，确保生成的消息代码与运行时 ABI 一致。请勿擅自升级到 5.x 版本，否则可能导致 ABI 不兼容问题。
>
> 📖 **详细版本管理策略**：查看 [`proto/README.md`](../../proto/README.md) 了解完整的版本固定策略和升级流程。版本在三个层面保持一致：
> - `vcpkg.json` - C++ 编译库版本
> - `proto/buf.yaml` - Buf 依赖管理版本
> - `buf.gen.yaml` - 代码生成插件版本

### 一键构建

**Linux/macOS:**
```bash
# 基础构建
./scripts/build.sh

# 清理重构建
./scripts/build.sh --clean

# 启用测试
./scripts/build.sh --tests ON
```

**Windows:**
```powershell
# 基础构建
.\scripts\build.ps1

# Debug 构建
.\scripts\build.ps1 -BuildType Debug
```

### 手动 CMake 构建

```bash
# 1. 配置构建（默认会自动 clone + bootstrap vcpkg 到 ./vcpkg）
cmake -B build \
  -DCMAKE_TOOLCHAIN_FILE=./cmake/vcpkg-bootstrap.cmake \
  -DVCPKG_OVERLAY_PORTS=./vcpkg-overlays \
  -DVCPKG_OVERLAY_TRIPLETS=./vcpkg-overlays/triplets \
  -DCMAKE_BUILD_TYPE=Release

# 2. 构建
cmake --build build --parallel

# 3. 运行示例（需要本地 Agent）
#
# SDK 示例默认连接 `127.0.0.1:19090`（见 `examples/*.cpp` 里的 `agent_addr`），请先确保 Croupier Agent 已启动。
#
# 如果你在 croupier 主仓库里（包含 `server/`），可以用下面命令启动本地 Server + Agent：
#
#   cd server
#   go run ./services/server -f services/server/etc/server.yaml
#   go run ./services/agent  -f services/agent/etc/agent.yaml
#
# 然后运行 C++ 示例：
./build/bin/croupier-example
./build/bin/croupier-virtual-object-demo
```

### VS Code (CMake Tools) 使用 vcpkg（固定 Protobuf 4.25.x）

`CMake Tools` 本身不会"自动使用 vcpkg"，它只会按你当前的 CMake 配置去 `find_package()`。
如果你本机装过 Homebrew 的 `protobuf`，而 CMake 没用 vcpkg toolchain，就会误用系统 protobuf，进而报：
`Protobuf C++ gencode is built with an incompatible version of Protobuf C++ headers/runtime`。

本仓库提供了 `croupier-sdk-cpp/CMakePresets.json`（默认走 vcpkg）：
- VS Code：`CMake: Select Configure Preset` → 选择 `macos-*-*-vcpkg`（Apple Silicon 选 `macos-arm64-...`）
- 如果之前已经 Configure 过，请先删掉 `croupier-sdk-cpp/build/*` 再重新 Configure（toolchain 必须在第一次 configure 时生效）
- macOS 下我们额外提供了 `vcpkg-overlays/triplets`（preset 已自动配置），避免 AppleClang 优先使用 `/usr/local/include` 导致误用 Homebrew 的 `protobuf/absl` 头文件

跨平台预设（可提交到 Git，推荐）：
- macOS：`macos-arm64-*-vcpkg` / `macos-x64-*-vcpkg`
- Linux：`linux-x64-*-vcpkg`
- Windows：`windows-x64-*-vcpkg`（Visual Studio 17 2022 生成器，不依赖 Ninja）

个人本机差异（例如你想用全局安装的 vcpkg 路径，而不是仓库内 `./vcpkg`），建议放到 `CMakeUserPresets.json`，不要提交到 Git。

### 自动安装 vcpkg

默认 preset 使用 `croupier-sdk-cpp/cmake/vcpkg-bootstrap.cmake` 作为 toolchain：
- 如果仓库根目录下没有 `./vcpkg`，会自动 `git clone` 并执行 `bootstrap-vcpkg`（只做一次）
- 如需禁用自动下载（离线环境），configure 时加 `-DCROUPIER_BOOTSTRAP_VCPKG=OFF`，然后手动准备 `./vcpkg`

## 使用示例

### 基础函数注册

```cpp
#include "croupier/sdk/croupier_client.h"
using namespace croupier::sdk;

std::string TransferHandler(const std::string& context, const std::string& payload) {
    auto data = utils::ParseJSON(payload);
    std::string from_player = data["from_player_id"];
    std::string to_player = data["to_player_id"];
    return ExecuteTransfer(from_player, to_player, data["amount"]);
}

int main() {
    ClientConfig config;
    config.game_id = "mmorpg-demo";
    config.env = "development";
    config.agent_addr = "127.0.0.1:19090";

    CroupierClient client(config);

    FunctionDescriptor desc{"wallet.transfer", "1.0.0"};
    client.RegisterFunction(desc, TransferHandler);

    client.Connect();
    client.Serve();
}
```

### 虚拟对象注册

```cpp
VirtualObjectDescriptor wallet_entity;
wallet_entity.id = "wallet.entity";
wallet_entity.version = "1.0.0";
wallet_entity.operations["read"] = "wallet.get";
wallet_entity.operations["transfer"] = "wallet.transfer";

std::map<std::string, FunctionHandler> handlers;
handlers["wallet.get"] = WalletGetHandler;
handlers["wallet.transfer"] = WalletTransferHandler;

client.RegisterVirtualObject(wallet_entity, handlers);
```

### 示例程序

SDK 包含多个完整的示例程序，展示各种使用场景：

| 示例 | 描述 | 运行命令 |
|------|------|----------|
| `example.cpp` | 基础函数注册示例 | `./build/bin/croupier-example` |
| `virtual_object_demo.cpp` | 虚拟对象注册和管理 | `./build/bin/croupier-virtual-object-demo` |
| `config_example.cpp` | 高级配置管理 | `./build/bin/croupier-config-example` |
| `plugin_demo.cpp` | 动态插件系统 | `./build/bin/croupier-plugin-demo` |
| `comprehensive_demo.cpp` | 完整 API 演示 | `./build/bin/croupier-comprehensive-demo` |
| `production_example.cpp` | 生产环境最佳实践 | `./build/bin/croupier-production-example` |
| `game_demo.cpp` | 完整游戏后台 19 个函数 | `./build/bin/croupier-game-demo` |

## 完整文档

详细的集成指南和 API 文档请参考：

- 📖 [集成指南](../../docs/sdks/cpp/integration.md) - 快速开始、配置说明、生产部署
- 📚 [API 参考](../../docs/api/) - 完整的类和接口文档
- 🚀 [生产部署指南](../../docs/sdks/cpp/integration.md#生产部署) - Docker/Kubernetes 配置
- 🔧 [故障排查](../../docs/sdks/cpp/integration.md#故障排查) - 常见问题解决方案

## 架构设计

### Provider session（跨语言基线）

```
Game Server → C++ SDK (Provider) → Agent → Croupier Server → Web UI
                                       ↑
                          单条 TCP session（sdk-agent subprotocol）
```

SDK 是 `sdk-agent subprotocol` 上的 Provider 端：

1. 拨号到 Agent，发送 `ProviderConnectRequest`，接收 `ProviderConnectResponse(session_id)`
2. 周期性发送 `ProviderHeartbeatRequest`
3. 接收 `InvokeRequest`，调用 handler 后回填 `InvokeResponse`
4. 收到 `ProviderDrainRequest` 时进入 drain 状态，完成在途再关闭

### C++ 四层组件化架构（L4 语言扩展）

C++ 在 Provider 基线之上提供业务建模的语法糖，**不要求其他语言对齐**：

```
Function Level    ← wallet.transfer (具体函数实现)
     ↓
Entity Level      ← wallet.entity (业务对象模型)
     ↓
Resource Level    ← 钱包管理面板 (UI 资源组织)
     ↓
Component Level   ← economy-system (可分发模块)
```

### ID 引用模式优势

- 🚀 **极致性能**：网络传输只有几十字节
- 🛡️ **线程安全**：无状态函数，天然支持并发
- 🔄 **水平扩展**：函数可以部署到任意节点
- 🧩 **松耦合**：对象管理与业务逻辑完全分离

## API 参考

### 核心类

```cpp
class CroupierClient {
public:
    // L1 Core Provider
    bool RegisterFunction(const FunctionDescriptor& desc, FunctionHandler handler);
    bool Connect();
    void Serve();
    void Stop();
    void Close();

    // L4 语言扩展（仅 C++ 提供）
    bool RegisterVirtualObject(const VirtualObjectDescriptor& desc,
                               const std::map<std::string, FunctionHandler>& handlers);
    bool RegisterComponent(const ComponentDescriptor& comp);
    bool LoadComponentFromFile(const std::string& config_file);
};

class CroupierInvoker {
public:
    // L3 独立调用方
    bool Connect();
    std::string Invoke(const std::string& function_id, const std::string& payload,
                       const InvokeOptions& options = {});
    std::string StartTask(const std::string& function_id, const std::string& payload,
                          const InvokeOptions& options = {});
    std::future<std::vector<TaskEvent>> StreamTask(const std::string& task_id);
    bool CancelTask(const std::string& task_id);
    void Close();
};
```

### 配置结构

```cpp
struct ClientConfig {
    std::string agent_addr = "127.0.0.1:19090";
    std::string game_id;
    std::string env = "development";
    std::string service_id = "cpp-service";
    bool insecure = true;
    bool auto_reconnect = true;
    int reconnect_interval_seconds = 5;
    int reconnect_max_attempts = 0; // 0 = unlimited
};
```

### 连接与重连配置说明

- `auto_reconnect`：默认 `true`；当 Agent 重启/断开时，`Serve()` 会自动尝试重连并重新注册函数；设为 `false` 时，连接断开会退出 `Serve()`。
- `reconnect_interval_seconds`：重连间隔（秒），默认 `5`（最小 `1`）。
- `reconnect_max_attempts`：最大重连次数，默认 `0` 表示无限重试；大于 0 时，达到次数后停止重连并退出 `Serve()`。

### 环境变量覆盖（`CROUPIER_` 前缀）

- `CROUPIER_AUTO_RECONNECT=true|false`
- `CROUPIER_RECONNECT_INTERVAL_SECONDS=5`
- `CROUPIER_RECONNECT_MAX_ATTEMPTS=0`

## 部署与分发

### GitHub Actions 自动构建

每日自动构建，支持多平台产物：

- 静态库 (.a/.lib)
- 动态库 (.so/.dylib/.dll)
- 头文件包
- 示例程序

### 下载预构建包

访问 [Releases 页面](https://github.com/cuihairu/croupier-sdk-cpp/releases) 下载：

**静态库包：**
- `croupier-cpp-sdk-static-{version}-windows-x64.zip`
- `croupier-cpp-sdk-static-{version}-linux-x64.tar.gz`
- `croupier-cpp-sdk-static-{version}-macos-x64.tar.gz`
- `croupier-cpp-sdk-static-{version}-macos-arm64.tar.gz`

**动态库包：**
- `croupier-cpp-sdk-dynamic-{version}-windows-x64.zip`
- `croupier-cpp-sdk-dynamic-{version}-linux-x64.tar.gz`
- `croupier-cpp-sdk-dynamic-{version}-macos-x64.tar.gz`
- `croupier-cpp-sdk-dynamic-{version}-macos-arm64.tar.gz`

> 💡 **提示**：推送 `v*` 格式的 tag（如 `v1.0.0`）会自动触发正式 Release 构建

## 开发指南

### 项目结构

```
croupier-sdk-cpp/
├── include/           # 公共头文件
├── src/               # 源代码
├── examples/          # 示例程序
├── scripts/           # 构建脚本
├── cmake/             # CMake 模块
├── configs/           # 配置文件示例
└── vcpkg.json         # vcpkg 依赖清单
```

### 开发规范

- 遵循 **C++17** 标准
- 使用 **clang-format** 格式化代码
- 编写单元测试
- 更新相关文档

## 贡献指南

1. **Fork** 项目
2. 创建特性分支：`git checkout -b feature/amazing-feature`
3. 提交更改：`git commit -m 'Add amazing feature'`
4. 推送分支：`git push origin feature/amazing-feature`
5. 创建 **Pull Request**

## 许可证

本项目采用 [Apache License 2.0](LICENSE) 开源协议。

---

<p align="center">
  <a href="https://github.com/cuihairu/croupier">🏠 主项目</a> •
  <a href="https://github.com/cuihairu/croupier-sdk-cpp/issues">🐛 问题反馈</a> •
  <a href="https://github.com/cuihairu/croupier/discussions">💬 讨论区</a>
</p>
