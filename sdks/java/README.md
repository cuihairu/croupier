<p align="center">
  <h1 align="center">Croupier Java SDK</h1>
  <p align="center">
    <strong>生产级 Java SDK，用于 Croupier 游戏函数注册与执行系统</strong>
  </p>
</p>

<p align="center">
  <a href="https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-java.yml">
    <img src="https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-java.yml/badge.svg" alt="CI">
  </a>
  <a href="https://codecov.io/gh/cuihairu/croupier">
    <img src="https://codecov.io/gh/cuihairu/croupier/branch/main/graph/badge.svg?flag=java-sdk" alt="Coverage">
  </a>
  <a href="https://www.apache.org/licenses/LICENSE-2.0">
    <img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg" alt="License">
  </a>
  <a href="https://www.oracle.com/java/">
    <img src="https://img.shields.io/badge/Java-17+-orange.svg" alt="Java Version">
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
- [架构设计](#架构设计)
- [API 参考](#api-参考)
- [开发指南](#开发指南)
- [贡献指南](#贡献指南)
- [许可证](#许可证)

---

## 简介

Croupier Java SDK 是 [Croupier](https://github.com/cuihairu/croupier) 游戏后端平台的官方 Java 客户端实现。SDK 作为 **Provider 端被调用方**，通过 **单条 TCP session**（`sdk-agent subprotocol`）接入 Agent，提供函数注册、心跳、自动重连、TLS、控制面 manifest 上传以及独立的 Invoker 能力。

## 正式文档

- 功能矩阵（跨语言一致性的单一事实来源）：[`sdks/SDK_FEATURE_MATRIX.md`](../SDK_FEATURE_MATRIX.md)
- 线协议约定：[`docs/architecture/sdk-wire-protocol.md`](../../docs/architecture/sdk-wire-protocol.md)
- 统一文档站入口：`/docs/sdks/java/`
- 仓库内路径：`docs/sdks/java`

## 主项目

| 项目         | 描述               | 链接                                                      |
| ------------ | ------------------ | --------------------------------------------------------- |
| **Croupier** | 游戏后端平台主项目 | [cuihairu/croupier](https://github.com/cuihairu/croupier) |

## 其他语言 SDK

所有 SDK 现已整合到主 monorepo 的 `sdks/` 目录下：

| 语言   | 目录                                                                       | CI                                                                                                                                                                    | Docs                          |
| ------ | -------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------- |
| Go     | [sdks/go/](https://github.com/cuihairu/croupier/tree/main/sdks/go)         | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-go.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-go.yml)         | [README](../go/README.md)     |
| C++    | [sdks/cpp/](https://github.com/cuihairu/croupier/tree/main/sdks/cpp)       | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-cpp.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-cpp.yml)       | [README](../cpp/README.md)    |
| JS/TS  | [sdks/js/](https://github.com/cuihairu/croupier/tree/main/sdks/js)         | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-js.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-js.yml)         | [README](../js/README.md)     |
| Python | [sdks/python/](https://github.com/cuihairu/croupier/tree/main/sdks/python) | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-python.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-python.yml) | [README](../python/README.md) |
| C#     | [sdks/csharp/](https://github.com/cuihairu/croupier/tree/main/sdks/csharp) | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-csharp.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-csharp.yml) | [README](../csharp/README.md) |

## 支持平台

| 平台        | 架构                       | 状态    |
| ----------- | -------------------------- | ------- |
| **Windows** | x64                        | ✅ 支持 |
| **Linux**   | x64, ARM64                 | ✅ 支持 |
| **macOS**   | x64, ARM64 (Apple Silicon) | ✅ 支持 |

## 核心特性

按 [功能矩阵](../SDK_FEATURE_MATRIX.md) 分层：

**L1 Core Provider（必备）**

- 📡 **TCP session 客户端** - 单条 `sdk-agent subprotocol` 长连接，不监听本地端口
- 🤝 **握手与心跳** - `ProviderConnectRequest` 协商，`ProviderHeartbeatRequest` 保活
- 🔁 **自动重连** - 指数退避 + jitter
- 📝 **函数注册** - `FunctionDescriptor` + `FunctionHandler`，handler 签名 `(context, payload: byte[]) -> String`
- ⚡ **异步 API** - `connect()` / `serveAsync()` 基于 `CompletableFuture`
- 🏢 **多游戏多环境作用域** - 内置 `gameId` / `env` / `serviceId` 维度

**L2 Provider 扩展（可选）**

- 🔐 **TLS** - `caFile` / `certFile` / `keyFile` / `serverName`
- 📤 **Provider Manifest** - 配置 `controlAddr` 后自动通过 `RegisterCapabilitiesRequest` 推送
- 📦 **文件传输** - `enableFileTransfer=true`

**L3 Invoker（独立调用方）**

- 🚀 `Invoker` 提供同步调用 / 异步作业 / 流式事件，独立配置入口

## 快速开始

### 系统要求

- **JDK 17+**（Gradle Wrapper 已锁定 8.x，默认编译到 `options.release = 17`）
- **Gradle Wrapper** 已随仓库提供（无需全局安装）

### 安装

Maven:

```xml
<dependency>
    <groupId>croupier.cuihairu.github.io</groupId>
    <artifactId>croupier-sdk-java</artifactId>
    <version>0.1.1</version>
</dependency>
```

Gradle:

```groovy
implementation 'croupier.cuihairu.github.io:croupier-sdk-java:0.1.1'
```

### 基础使用

```java
import com.croupier.sdk.*;

public class GameServer {
    public static void main(String[] args) throws Exception {
        // 创建客户端配置
        ClientConfig config = new ClientConfig("my-game", "my-service");
        config.setAgentAddr("localhost:19091");
        config.setControlAddr("localhost:18080"); // 控制面用于 manifest 上传
        config.setEnv("development");
        config.setInsecure(true); // 开发环境
        config.setProviderLang("java");
        config.setProviderSdk("croupier-java-sdk");

        // 创建客户端
        CroupierClient client = CroupierSDK.createClient(config);

        // 注册函数
        FunctionDescriptor desc = CroupierSDK.functionDescriptor("player.ban", "1.0.0")
                .resource("player")
                .operation("ban")
                .risk("danger")
                .summary("Ban player")
                .description("Ban a player account by player ID.")
                .build();

        FunctionHandler handler = (context, payload) -> {
            // 处理函数调用
            return "{\"status\":\"success\"}";
        };

        client.registerFunction(desc, handler);

        // 启动服务
        client.serve(); // 阻塞直到停止
    }
}
```

## 使用示例

### 异步使用

```java
// 异步连接和服务
client.connect()
    .thenCompose(v -> client.serveAsync())
    .thenRun(() -> System.out.println("服务已启动"))
    .exceptionally(throwable -> {
        System.err.println("失败: " + throwable.getMessage());
        return null;
    });
```

### 函数描述符

跨语言统一的 `ProviderFunctionDescriptor` 字段（对应 `proto/croupier/sdk/v1/provider.proto`）：

```java
FunctionDescriptor descriptor = CroupierSDK.functionDescriptor("player.ban", "1.0.0")
        .resource("player")        // 业务资源或能力域
        .operation("ban")          // 业务动作 key
        .risk("danger")            // "safe"|"warning"|"high"|"danger"
        .enabled(true)             // 是否启用
        .build();
```

### 本地函数描述符

`sdk-agent subprotocol` 上承载的函数描述符（对应 `proto/croupier/sdk/v1/provider.proto` 的 `ProviderFunctionDescriptor`）：

```java
ProviderFunctionDescriptor localDesc = new ProviderFunctionDescriptor("player.ban", "1.0.0");
```

### 函数处理器

实现游戏函数的函数式接口：

```java
FunctionHandler handler = (context, payload) -> {
    // context: 执行上下文（JSON 字符串）
    // payload: 函数载荷（JSON 字符串）
    // return: 结果（JSON 字符串）
    return "{\"status\":\"success\"}";
};
```

## 架构设计

### 数据流

```
Game Server → Java SDK (Provider) → Agent → Croupier Server → Web UI
                                       ↑
                          单条 TCP session（sdk-agent subprotocol）
```

SDK 是 `sdk-agent subprotocol` 上的 Provider 端：

1. 拨号到 Agent，发送 `ProviderConnectRequest`，接收 `ProviderConnectResponse(sessionId)`
2. 周期性发送 `ProviderHeartbeatRequest`
3. 接收 `InvokeRequest`，调用 handler 后回填 `InvokeResponse`
4. 可选：通过 `controlAddr` 向控制面推送 `RegisterCapabilitiesRequest`（manifest）
5. 收到 `ProviderDrainRequest` 时进入 drain 状态，完成在途再关闭

### Proto 与构建流水线

- `proto/`：Protobuf 协议定义文件（主仓库 `proto/croupier/sdk/v1`）
- `generated/`：已提交的 stub，方便依赖方直接使用
- `./gradlew`：内置 Gradle Wrapper + `com.google.protobuf` 插件
- CI 会在 JDK 17/21 上运行 `./gradlew --no-daemon clean build`

## API 参考

### ClientConfig

```java
ClientConfig config = new ClientConfig();
config.setAgentAddr("localhost:19091");     // Agent 本地 SDK gateway 地址
config.setGameId("my-game");                // 游戏标识符
config.setEnv("development");               // 环境
config.setServiceId("my-service");          // 服务标识符
config.setServiceVersion("1.0.0");          // 服务版本
config.setLocalListen(":0");                // 兼容字段，新版本不再监听本地端口
config.setControlAddr("localhost:18080");   // 可选控制面端点（manifest 上传）
config.setTimeoutSeconds(30);               // 连接超时
config.setInsecure(true);                   // 是否跳过 TLS
config.setProviderLang("java");             // Provider 元数据
config.setProviderSdk("croupier-java-sdk");

// TLS 设置（非 insecure 模式）
config.setCaFile("/path/to/ca.pem");
config.setCertFile("/path/to/cert.pem");
config.setKeyFile("/path/to/key.pem");
```

### CroupierClient 接口

```java
public interface CroupierClient {
    // 函数注册
    void registerFunction(FunctionDescriptor descriptor, FunctionHandler handler);

    // 连接管理
    CompletableFuture<Void> connect();

    // 服务操作
    void serve();                           // 阻塞直到停止
    CompletableFuture<Void> serveAsync();   // 非阻塞

    // 生命周期
    void stop();
    void close();

    // 状态
    boolean isConnected();
    boolean isServing();
}
```

### 工厂方法

```java
// 简单创建
CroupierClient client = CroupierSDK.createClient("game-id", "service-id");

// 带 Agent 地址
CroupierClient client = CroupierSDK.createClient("game-id", "service-id", "localhost:19091");

// 完整配置
CroupierClient client = CroupierSDK.createClient(config);
```

### 错误处理

```java
try {
    client.registerFunction(descriptor, handler);
    client.serve();
} catch (CroupierException e) {
    System.err.println("Croupier 错误: " + e.getMessage());
    e.printStackTrace();
} catch (Exception e) {
    System.err.println("意外错误: " + e.getMessage());
}
```

异步错误处理：

```java
client.connect()
    .exceptionally(throwable -> {
        if (throwable.getCause() instanceof CroupierException) {
            System.err.println("Croupier 错误: " + throwable.getCause().getMessage());
        }
        return null;
    });
```

## 开发指南

### 项目结构

```
croupier-sdk-java/
├── proto/                    # 子模块：官方 API/SDK proto
├── generated/                # 已提交的 Protobuf stubs
├── src/
│   ├── main/java/com/croupier/sdk/
│   │   ├── CroupierSDK.java
│   │   ├── CroupierClient*.java
│   │   ├── descriptors / handlers / config
│   │   └── scripts/ProtoGenerator.java
│   ├── main/resources/
│   └── test/java/
└── examples/
    ├── basic/
    └── comprehensive/
```

### 构建命令

```bash
# 全量构建（编译 + 测试 + jar）
./gradlew --no-daemon clean build

# 仅运行单元测试
./gradlew --no-daemon test

# 查看生成的 Protobuf 代码
ls build/generated/source/proto/main/java
```

### 运行示例

```bash
cd examples/comprehensive
../gradlew --no-daemon execute
```

### 完整游戏后台 Demo

`examples/game-demo/` 包含19个业务动作函数（player/order/leaderboard/inventory/mail），与 Go SDK demo 功能对齐。

## 贡献指南

1. 确保所有类型与 proto 定义对齐
2. 为新功能添加测试
3. 更新 API 变更的文档
4. 测试本地和 CI 两种构建模式
5. 遵循 Java 编码规范

## 许可证

本项目采用 [Apache License 2.0](LICENSE) 开源协议。

---

<p align="center">
  <a href="https://github.com/cuihairu/croupier">🏠 主项目</a> •
  <a href="https://github.com/cuihairu/croupier-sdk-java/issues">🐛 问题反馈</a> •
  <a href="https://github.com/cuihairu/croupier/discussions">💬 讨论区</a>
</p>
