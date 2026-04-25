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

Croupier Java SDK 是 [Croupier](https://github.com/cuihairu/croupier) 游戏后端平台的官方 Java 客户端实现。它提供了与官方 Croupier proto 定义 100% 对齐的类型、基于 `CompletableFuture` 的异步能力以及完整的函数注册到执行链路。

## 正式文档

- 统一文档站入口：`/docs/sdks/java/`
- 仓库内路径：`docs/sdks/java`

## 主项目

| 项目 | 描述 | 链接 |
|------|------|------|
| **Croupier** | 游戏后端平台主项目 | [cuihairu/croupier](https://github.com/cuihairu/croupier) |

## 其他语言 SDK

所有 SDK 现已整合到主 monorepo 的 `sdks/` 目录下：

| 语言 | 目录 | CI | Docs |
| --- | --- | --- | --- |
| Go | [sdks/go/](https://github.com/cuihairu/croupier/tree/main/sdks/go) | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-go.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-go.yml) | [README](sdks/go/README.md) |
| C++ | [sdks/cpp/](https://github.com/cuihairu/croupier/tree/main/sdks/cpp) | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-cpp.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-cpp.yml) | [README](sdks/cpp/README.md) |
| JS/TS | [sdks/js/](https://github.com/cuihairu/croupier/tree/main/sdks/js) | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-js.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-js.yml) | [README](sdks/js/README.md) |
| Python | [sdks/python/](https://github.com/cuihairu/croupier/tree/main/sdks/python) | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-python.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-python.yml) | [README](sdks/python/README.md) |
| C# | [sdks/csharp/](https://github.com/cuihairu/croupier/tree/main/sdks/csharp) | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-csharp.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-csharp.yml) | [README](sdks/csharp/README.md) |

## 支持平台

| 平台 | 架构 | 状态 |
|------|------|------|
| **Windows** | x64 | ✅ 支持 |
| **Linux** | x64, ARM64 | ✅ 支持 |
| **macOS** | x64, ARM64 (Apple Silicon) | ✅ 支持 |

## 核心特性

- 📡 **Proto 对齐** - `FunctionDescriptor`、`LocalFunctionDescriptor` 与官方 IDL 100% 对齐
- 🏢 **多租户支持** - 内置 `gameId`、`env`、`serviceId` 维度隔离
- 🔄 **完整链路** - 函数注册、心跳、执行、返回值、错误处理
- ⚡ **异步能力** - 基于 `CompletableFuture`，便于与现有任务系统整合
- 📤 **Provider Manifest** - 控制面地址可用时，自动发布能力声明
- 🛠️ **Gradle + Buf** - CI 自动拉取 proto、生成代码、执行测试并发布产物

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
        config.setAgentAddr("localhost:19090");
        config.setControlAddr("localhost:18080"); // 控制面用于 manifest 上传
        config.setEnv("development");
        config.setInsecure(true); // 开发环境
        config.setProviderLang("java");
        config.setProviderSdk("croupier-java-sdk");

        // 创建客户端
        CroupierClient client = CroupierSDK.createClient(config);

        // 注册函数
        FunctionDescriptor desc = CroupierSDK.functionDescriptor("player.ban", "1.0.0")
                .category("moderation")
                .risk("high")
                .entity("player")
                .operation("update")
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

与 `control.proto` 对齐：

```java
FunctionDescriptor descriptor = CroupierSDK.functionDescriptor("player.ban", "1.0.0")
        .category("moderation")     // 分组类别
        .risk("high")              // "low"|"medium"|"high"
        .entity("player")          // 实体类型
        .operation("update")       // "create"|"read"|"update"|"delete"
        .enabled(true)             // 是否启用
        .build();
```

### 本地函数描述符

与 `agent/local/v1/local.proto` 对齐：

```java
LocalFunctionDescriptor localDesc = new LocalFunctionDescriptor("player.ban", "1.0.0");
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
Game Server → Java SDK → Agent → Croupier Server
```

SDK 实现两层注册系统：
1. **SDK → Agent**: 使用 `LocalControlService`（来自 `local.proto`）
2. **Agent → Server**: 使用 `ControlService`（来自 `control.proto`）

### Proto 与构建流水线

- `proto/`：Protobuf 协议定义文件
- `generated/`：已提交的 `.java` gRPC Stubs，方便依赖方直接使用
- `./gradlew`：内置 Gradle Wrapper + `com.google.protobuf` 插件
- CI 会在 JDK 17/21 上运行 `./gradlew --no-daemon clean build`

## API 参考

### ClientConfig

```java
ClientConfig config = new ClientConfig();
config.setAgentAddr("localhost:19090");     // Agent 地址
config.setGameId("my-game");                // 游戏标识符
config.setEnv("development");               // 环境
config.setServiceId("my-service");          // 服务标识符
config.setServiceVersion("1.0.0");          // 服务版本
config.setLocalListen(":0");                // 本地服务器（自动端口）
config.setControlAddr("localhost:18080");   // 可选控制面端点
config.setTimeoutSeconds(30);               // 连接超时
config.setInsecure(true);                   // 使用不安全的 gRPC
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
CroupierClient client = CroupierSDK.createClient("game-id", "service-id", "localhost:19090");

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
├── generated/                # 已提交的 gRPC stubs
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

# 查看生成的 gRPC 代码
ls build/generated/source/proto/main/java
```

### 运行示例

```bash
cd examples/comprehensive
../gradlew --no-daemon execute
```

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
