# Croupier C++ SDK - 完整功能实现

[![CI Build](https://github.com/cuihairu/croupier-sdk-cpp/actions/workflows/cpp-sdk-build.yml/badge.svg)](https://github.com/cuihairu/croupier-sdk-cpp/actions)

## 🎯 概述

Croupier C++ SDK 是一个企业级的游戏后端虚拟对象注册系统，提供完整的 gRPC 通信框架和生产级功能。

### ✨ 主要特性

- **🎮 虚拟对象管理** - 四层架构设计 (Function → Entity → Resource → Component)
- **🌐 gRPC 通信** - 完整的 Agent 注册、心跳和错误恢复机制
- **🔒 安全认证** - 支持 TLS/mTLS、认证令牌和权限控制
- **⚡ 高性能** - ID 引用模式，避免重复序列化
- **🔄 自动重连** - 智能错误处理和连接恢复
- **🧪 完整测试** - GoogleTest 框架，单元测试 + 集成测试

---

## 🚀 快速开始

### 1. 依赖要求

- **C++17** 或更高版本
- **CMake 3.20+**
- **vcpkg** (包管理器)
- **gRPC** (通过 vcpkg 安装)

### 2. 构建项目

```bash
# 克隆项目
git clone https://github.com/cuihairu/croupier-sdk-cpp.git
cd croupier-sdk-cpp

# vcpkg 安装依赖
vcpkg install

# CMake 构建
cmake -B build -DCMAKE_TOOLCHAIN_FILE=[vcpkg-root]/scripts/buildsystems/vcpkg.cmake
cmake --build build --config Release
```

### 3. 运行示例

```bash
# 完整示例 (推荐)
./build/bin/croupier-complete-example

# 虚拟对象演示
./build/bin/croupier-virtual-object-demo
```

### 4. 运行测试

```bash
# 启用测试构建
cmake -B build -DBUILD_TESTS=ON [其他选项...]
cmake --build build

# 运行所有测试
cd build && ctest --output-on-failure
```

---

## 📚 核心概念

### 🎯 虚拟对象 (Virtual Objects)

虚拟对象是业务逻辑的抽象表示，包含操作和关系：

```cpp
// 定义钱包虚拟对象
VirtualObjectDescriptor wallet;
wallet.id = "economy.wallet";
wallet.version = "1.0.0";
wallet.name = "玩家钱包";

// 添加操作
wallet.operations["get"] = "wallet.get";
wallet.operations["transfer"] = "wallet.transfer";

// 定义关系
RelationshipDef player_rel;
player_rel.type = "many-to-one";
player_rel.entity = "player";
player_rel.foreign_key = "player_id";
wallet.relationships["owner"] = player_rel;
```

### 🔧 函数处理器 (Function Handlers)

处理器是实际的业务逻辑实现：

```cpp
// 钱包查询处理器
auto walletGetHandler = [](const std::string& context, const std::string& payload) -> std::string {
    // 解析请求
    auto data = utils::ParseJSON(payload);
    std::string wallet_id = data["wallet_id"];

    // 执行业务逻辑
    WalletInfo info = WalletService::GetWallet(wallet_id);

    // 返回 JSON 结果
    return utils::ToJSON({
        {"wallet_id", info.id},
        {"balance", std::to_string(info.balance)},
        {"currency", info.currency}
    });
};
```

### 🌐 组件系统 (Components)

组件将相关的虚拟对象组织在一起：

```cpp
ComponentDescriptor economy;
economy.id = "economy-system";
economy.version = "1.0.0";
economy.name = "完整经济系统";

// 添加虚拟对象
economy.virtual_objects["wallet"] = wallet;
economy.virtual_objects["shop"] = shop;
economy.virtual_objects["auction"] = auction;

// 添加处理器
economy.function_handlers["wallet.get"] = walletGetHandler;
economy.function_handlers["shop.list"] = shopListHandler;
```

---

## 🔌 使用指南

### 基础用法

```cpp
#include "croupier/sdk/croupier_client.h"

int main() {
    // 1. 配置客户端
    croupier::sdk::ClientConfig config;
    config.game_id = "my-awesome-game";
    config.env = "production";
    config.agent_addr = "croupier-agent:19090";
    config.insecure = false;  // 生产环境使用 TLS

    // 2. 创建客户端
    croupier::sdk::CroupierClient client(config);

    // 3. 注册虚拟对象
    // ... (参见上面的示例)

    // 4. 连接并启动服务
    if (client.Connect()) {
        client.Serve();  // 阻塞，处理来自 Agent 的调用
    }

    return 0;
}
```

### 高级配置

```cpp
// 生产环境完整配置
croupier::sdk::ClientConfig config;
config.game_id = "production-game";
config.env = "production";
config.service_id = "game-backend-01";

// 网络配置
config.agent_addr = "croupier-agent.internal:19090";
config.local_listen = "0.0.0.0:0";  // 自动分配端口
config.timeout_seconds = 60;

// 安全配置
config.insecure = false;
config.cert_file = "/etc/tls/client.crt";
config.key_file = "/etc/tls/client.key";
config.ca_file = "/etc/tls/ca.crt";
config.server_name = "croupier.internal";

// 认证配置
config.auth_token = "Bearer eyJhbGciOiJIUzI1NiIs...";
config.headers["X-Game-Version"] = "2.1.0";
config.headers["X-Client-ID"] = "backend-server-01";
```

---

## 🧪 测试框架

### 单元测试

```bash
# 运行特定测试
./build/croupier-sdk-tests --gtest_filter="VirtualObjectTest.*"

# 运行工具测试
./build/croupier-sdk-tests --gtest_filter="UtilsTest.*"
```

### 集成测试

```bash
# 运行集成测试 (需要 Croupier Agent)
./build/croupier-sdk-tests --gtest_filter="IntegrationTest.*"
```

### 测试覆盖范围

- ✅ **虚拟对象注册/注销**
- ✅ **组件管理**
- ✅ **配置验证**
- ✅ **错误处理**
- ✅ **gRPC 连接管理**
- ✅ **关系定义**
- ✅ **复杂业务场景**

---

## 📊 架构设计

### 🏗️ 四层架构

```
┌─────────────────────────────────────┐
│  Component Layer (组件层)            │  ← 完整子系统 (经济系统、战斗系统)
├─────────────────────────────────────┤
│  Virtual Object Layer (虚拟对象层)   │  ← 业务实体 (玩家、钱包、道具)
├─────────────────────────────────────┤
│  Function Layer (函数层)             │  ← 具体操作 (创建、查询、更新)
├─────────────────────────────────────┤
│  Handler Layer (处理器层)            │  ← 实际业务逻辑实现
└─────────────────────────────────────┘
```

### 🔄 通信流程

```
Game Server (C++ SDK)    ←→    Croupier Agent    ←→    Croupier Server
       ↓                              ↓                        ↓
   本地 gRPC 服务              负载均衡/路由              权限验证/审计
   函数处理器执行              会话管理                  多游戏隔离
   ID引用模式                 心跳监控                  RBAC/ABAC
```

---

## 🔧 配置选项

### 客户端配置 (ClientConfig)

| 选项 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `game_id` | string | *(必需)* | 游戏标识符，用于租户隔离 |
| `env` | string | `development` | 环境：development/staging/production |
| `service_id` | string | `cpp-service` | 服务标识符 |
| `agent_addr` | string | `127.0.0.1:19090` | Croupier Agent 地址 |
| `local_listen` | string | `127.0.0.1:0` | 本地服务监听地址 |
| `insecure` | bool | `true` | 是否使用非安全连接 |
| `timeout_seconds` | int | `30` | 请求超时时间 |

### 安全配置

| 选项 | 类型 | 说明 |
|------|------|------|
| `cert_file` | string | 客户端证书文件路径 |
| `key_file` | string | 私钥文件路径 |
| `ca_file` | string | CA 证书文件路径 |
| `server_name` | string | TLS SNI 服务器名称 |
| `auth_token` | string | 认证令牌 (如 JWT) |
| `headers` | map | 额外的 HTTP 头 |

---

## 🚨 错误处理

### 连接错误

```cpp
client.SetErrorCallback([](const std::string& error) {
    std::cerr << "连接错误: " << error << std::endl;
    // 实现自定义错误处理逻辑
});

client.SetReconnectCallback([]() {
    std::cout << "连接已恢复！" << std::endl;
    // 实现重连后的处理逻辑
});
```

### 常见错误码

| 错误 | 原因 | 解决方案 |
|------|------|----------|
| `Connection failed` | Agent 不可达 | 检查网络和 Agent 状态 |
| `Registration failed` | 认证或权限问题 | 验证配置和令牌 |
| `Function not found` | 函数未注册 | 确保处理器正确注册 |
| `Invalid game_id` | 配置错误 | 检查 game_id 配置 |

---

## 📦 部署指南

### Docker 部署

```dockerfile
FROM ubuntu:22.04

# 安装运行时依赖
RUN apt-get update && apt-get install -y \
    libgrpc++1.54 \
    libprotobuf32 \
    && rm -rf /var/lib/apt/lists/*

# 复制构建产物
COPY build/bin/your-game-server /usr/local/bin/
COPY configs/ /etc/croupier/

# 运行游戏服务器
CMD ["/usr/local/bin/your-game-server"]
```

### Kubernetes 部署

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: game-backend
spec:
  replicas: 3
  selector:
    matchLabels:
      app: game-backend
  template:
    metadata:
      labels:
        app: game-backend
    spec:
      containers:
      - name: game-backend
        image: your-registry/game-backend:latest
        env:
        - name: CROUPIER_GAME_ID
          value: "production-game"
        - name: CROUPIER_ENV
          value: "production"
        - name: CROUPIER_AGENT_ADDR
          value: "croupier-agent.croupier:19090"
        ports:
        - containerPort: 8080
        volumeMounts:
        - name: tls-certs
          mountPath: /etc/tls
          readOnly: true
      volumes:
      - name: tls-certs
        secret:
          secretName: croupier-tls
```

---

## 📋 最佳实践

### 🎯 性能优化

1. **使用 ID 引用模式** - 避免重复序列化大对象
2. **批量注册** - 使用组件系统一次注册多个对象
3. **连接池** - 在生产环境中配置适当的连接池大小
4. **异步处理** - 在处理器中避免阻塞操作

### 🔒 安全考虑

1. **强制 TLS** - 生产环境必须使用 TLS/mTLS
2. **令牌轮换** - 定期更新认证令牌
3. **最小权限** - 仅授予必需的权限
4. **审计日志** - 启用完整的操作审计

### 🐛 调试技巧

1. **详细日志** - 启用详细的 gRPC 日志
2. **健康检查** - 实现服务健康检查端点
3. **指标监控** - 集成 Prometheus/Grafana 监控
4. **分布式追踪** - 使用 OpenTelemetry 追踪请求

---

## 🤝 贡献指南

### 开发环境

```bash
# 安装开发依赖
vcpkg install gtest

# 启用所有构建选项
cmake -B build \
  -DBUILD_TESTS=ON \
  -DBUILD_EXAMPLES=ON \
  -DBUILD_SHARED_LIBS=ON \
  -DBUILD_STATIC_LIBS=ON

# 运行代码格式化
clang-format -i src/*.cpp include/**/*.h

# 运行静态分析
cppcheck --enable=all src/
```

### 提交规范

- `feat:` 新功能
- `fix:` 问题修复
- `docs:` 文档更新
- `test:` 测试相关
- `refactor:` 代码重构

---

## 📄 许可证

本项目采用 [MIT License](LICENSE) 开源许可证。

---

## 🆘 支持与帮助

- **文档**: [完整 API 文档](docs/)
- **示例**: [examples/](examples/) 目录
- **问题**: [GitHub Issues](https://github.com/cuihairu/croupier-sdk-cpp/issues)
- **讨论**: [GitHub Discussions](https://github.com/cuihairu/croupier-sdk-cpp/discussions)

---

**🎮 构建下一代游戏后端，从 Croupier C++ SDK 开始！**