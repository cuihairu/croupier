# API 参考

本文档提供 Croupier Java SDK 的完整 API 参考。

## 包结构

```java
import io.github.cuihairu.croupier.sdk.*;
import io.github.cuihairu.croupier.sdk.config.*;
import io.github.cuihairu.croupier.sdk.model.*;
```

## 核心类型

### FunctionHandler

函数处理器接口。

```java
@FunctionalInterface
public interface FunctionHandler {
    /**
     * 处理函数调用
     * @param context 调用上下文（JSON 字符串）
     * @param payload 函数参数（JSON 字符串）
     * @return 函数执行结果（JSON 字符串）
     * @throws Exception 处理异常
     */
    String handle(String context, String payload) throws Exception;
}
```

**Lambda 使用示例:**

```java
FunctionHandler handler = (context, payload) -> {
    JSONObject req = new JSONObject(payload);
    String playerId = req.getString("player_id");
    // 业务逻辑
    return "{\"status\":\"success\"}";
};
```

---

### ClientConfig

客户端配置类（Builder 模式）。

```java
public class ClientConfig {
    // 连接配置
    private String agentAddr;           // Agent gRPC 地址，默认 "localhost:19090"
    private int timeoutSeconds;         // 连接超时（秒），默认 30
    private boolean insecure;           // 使用不安全的 gRPC 连接

    // 多租户隔离
    private String gameId;              // 游戏标识符（必填）
    private String env;                 // 环境：dev/staging/prod
    private String serviceId;           // 服务标识符
    private String serviceVersion;      // 服务版本

    // TLS 配置
    private String caFile;              // CA 证书文件路径
    private String certFile;            // 客户端证书文件路径
    private String keyFile;             // 私钥文件路径

    // 重连配置
    private boolean autoReconnect;      // 是否自动重连，默认 true
    private int reconnectIntervalSecs;  // 重连间隔（秒），默认 5
    private int reconnectMaxAttempts;   // 最大重连次数，0 表示无限

    // 心跳配置
    private int heartbeatIntervalSecs;  // 心跳间隔（秒），默认 10

    // 重试配置
    private int maxRetries;             // 最大重试次数，默认 3
    private int retryBackoffMs;         // 重试退避时间（毫秒），默认 1000
}
```

**Builder 使用:**

```java
ClientConfig config = ClientConfig.builder()
    .agentAddr("localhost:19090")
    .gameId("my-game")
    .env("development")
    .serviceId("player-service")
    .insecure(true)
    .autoReconnect(true)
    .reconnectIntervalSecs(5)
    .maxRetries(3)
    .build();
```

**环境变量覆盖:**

| 环境变量 | 配置字段 | 说明 |
|----------|----------|------|
| `CROUPIER_AGENT_ADDR` | agentAddr | Agent 地址 |
| `CROUPIER_GAME_ID` | gameId | 游戏 ID |
| `CROUPIER_ENV` | env | 环境 |
| `CROUPIER_SERVICE_ID` | serviceId | 服务 ID |
| `CROUPIER_INSECURE` | insecure | 是否跳过 TLS |

---

### FunctionDescriptor

函数描述符。

```java
public class FunctionDescriptor {
    // 必填字段
    private String id;          // 函数 ID，格式: [namespace.]entity.operation
    private String version;     // 语义化版本号，如 "1.0.0"

    // 推荐字段
    private String category;    // 业务分类
    private String risk;        // 风险等级: "low"|"medium"|"high"
    private String entity;      // 关联实体类型
    private String operation;   // 操作类型: "create"|"read"|"update"|"delete"
    private boolean enabled;    // 是否启用，默认 true

    // 可选字段
    private String displayName; // 显示名称
    private String summary;     // 简短描述
    private String description; // 详细描述
    private List<String> tags;  // 标签列表
    private String inputSchema; // 输入参数 JSON Schema
    private String outputSchema;// 输出结果 JSON Schema
    private int timeoutMs;      // 函数超时（毫秒）
}
```

**Builder 使用:**

```java
FunctionDescriptor desc = FunctionDescriptor.builder()
    .id("player.ban")
    .version("1.0.0")
    .category("player")
    .risk("high")
    .entity("player")
    .operation("update")
    .enabled(true)
    .displayName("封禁玩家")
    .description("封禁指定玩家账号")
    .tags(Arrays.asList("player", "moderation"))
    .inputSchema("{\"type\":\"object\",\"properties\":{\"player_id\":{\"type\":\"string\"}}}")
    .build();
```

---

### VirtualObjectDescriptor

虚拟对象描述符。

```java
public class VirtualObjectDescriptor {
    // 必填字段
    private String id;                      // 对象 ID
    private String version;                 // 版本号

    // 推荐字段
    private String name;                    // 显示名称
    private String description;             // 描述

    // 操作映射
    private Map<String, String> operations; // 操作名 -> 函数 ID

    // 可选字段
    private String schema;                  // 实体 JSON Schema
    private Map<String, Object> metadata;   // 元数据
}
```

**使用示例:**

```java
VirtualObjectDescriptor vo = VirtualObjectDescriptor.builder()
    .id("player")
    .version("1.0.0")
    .name("游戏玩家")
    .description("管理玩家信息")
    .operation("create", "player.create")
    .operation("read", "player.get")
    .operation("update", "player.update")
    .operation("delete", "player.delete")
    .operation("ban", "player.ban")
    .build();
```

---

### ComponentDescriptor

组件描述符。

```java
public class ComponentDescriptor {
    private String id;              // 组件 ID
    private String version;         // 版本号
    private String name;            // 显示名称
    private String description;     // 描述
    private List<String> functions; // 包含的函数 ID 列表
    private boolean enabled;        // 是否启用
}
```

---

## CroupierClient 类

主客户端类，管理与 Croupier Agent 的连接和函数注册。

### 构造函数

```java
public CroupierClient(ClientConfig config);
```

### 公共方法

#### registerFunction

注册单个函数。

```java
public void registerFunction(FunctionDescriptor desc, FunctionHandler handler)
    throws RegistrationException;
```

**参数:**
- `desc`: 函数描述符
- `handler`: 函数处理器

**异常:** `RegistrationException` - 注册失败时抛出

---

#### registerVirtualObject

注册虚拟对象。

```java
public void registerVirtualObject(
    VirtualObjectDescriptor desc,
    Map<String, FunctionHandler> handlers
) throws RegistrationException;
```

**参数:**
- `desc`: 虚拟对象描述符
- `handlers`: 操作处理器映射

---

#### registerComponent

注册组件。

```java
public void registerComponent(ComponentDescriptor comp)
    throws RegistrationException;
```

---

#### connect

连接到 Agent。

```java
public void connect() throws ConnectionException;
```

**异常:** `ConnectionException` - 连接失败时抛出

---

#### serve

开始服务（阻塞调用）。

```java
public void serve() throws CroupierException;
```

---

#### serveAsync

异步启动服务。

```java
public CompletableFuture<Void> serveAsync();
```

---

#### stop

停止服务。

```java
public void stop();
```

---

#### close

关闭连接并释放资源。

```java
public void close();
```

实现 `AutoCloseable` 接口，支持 try-with-resources。

---

#### isConnected

检查连接状态。

```java
public boolean isConnected();
```

---

#### onConnectionStateChange

设置连接状态回调。

```java
public void onConnectionStateChange(Consumer<Boolean> callback);
```

---

#### onError

设置错误回调。

```java
public void onError(Consumer<Throwable> callback);
```

---

## CroupierInvoker 类

调用端类，用于调用已注册的函数。

### 构造函数

```java
public CroupierInvoker(ClientConfig config);
```

### 公共方法

#### invoke

同步调用函数。

```java
public String invoke(String functionId, String payload)
    throws InvokeException, TimeoutException;

public String invoke(String functionId, String payload, InvokeOptions options)
    throws InvokeException, TimeoutException;
```

---

#### invokeAsync

异步调用函数。

```java
public CompletableFuture<String> invokeAsync(String functionId, String payload);

public CompletableFuture<String> invokeAsync(
    String functionId,
    String payload,
    InvokeOptions options
);
```

---

#### startJob

启动异步任务。

```java
public String startJob(String functionId, String payload)
    throws InvokeException;

public String startJob(String functionId, String payload, InvokeOptions options)
    throws InvokeException;
```

**返回值:** 任务 ID

---

#### streamTask

流式获取任务事件。

```java
public void streamTask(String taskId, Consumer<TaskEvent> eventHandler)
    throws InvokeException;
```

---

#### cancelTask

取消任务。

```java
public void cancelTask(String taskId) throws InvokeException;
```

---

#### getTaskResult

获取任务结果。

```java
public TaskResult getTaskResult(String taskId) throws InvokeException;
```

---

### InvokeOptions

调用选项。

```java
public class InvokeOptions {
    private Duration timeout;                   // 超时时间
    private String idempotencyKey;              // 幂等键
    private Map<String, String> metadata;       // 元数据
    private int retryCount;                     // 重试次数
}
```

**Builder 使用:**

```java
InvokeOptions options = InvokeOptions.builder()
    .timeout(Duration.ofSeconds(10))
    .idempotencyKey("ban-12345-20260117")
    .metadata(Map.of("operator", "admin"))
    .retryCount(3)
    .build();

String result = invoker.invoke("player.ban", payload, options);
```

---

### TaskEvent

任务事件。

```java
public class TaskEvent {
    private String type;        // 事件类型: "progress"|"log"|"done"|"error"
    private String message;     // 事件消息
    private int progress;       // 进度 0-100
    private byte[] payload;     // 最终结果
}
```

### TaskResult

任务结果。

```java
public class TaskResult {
    private String taskId;      // 任务 ID
    private TaskStatus status;  // 任务状态
    private byte[] payload;     // 结果数据
    private String error;       // 错误信息
}

public enum TaskStatus {
    PENDING,
    RUNNING,
    COMPLETED,
    FAILED,
    CANCELLED
}
```

---

## 配置加载

### ConfigLoader

配置加载器。

```java
public class ConfigLoader {
    /**
     * 从文件加载配置
     */
    public static ClientConfig loadFromFile(String path) throws ConfigException;

    /**
     * 从文件加载配置并应用环境变量覆盖
     */
    public static ClientConfig loadWithEnvironmentOverrides(
        String path,
        String envPrefix
    ) throws ConfigException;

    /**
     * 验证配置
     */
    public static List<String> validateConfig(ClientConfig config);
}
```

**使用示例:**

```java
// 加载 config.yaml，并用 CROUPIER_ 前缀的环境变量覆盖
ClientConfig config = ConfigLoader.loadWithEnvironmentOverrides(
    "config.yaml",
    "CROUPIER_"
);

// 验证配置
List<String> errors = ConfigLoader.validateConfig(config);
if (!errors.isEmpty()) {
    throw new ConfigException("配置错误: " + String.join(", ", errors));
}
```

---

## 异常类型

```java
public class CroupierException extends Exception {
    private String code;
    private String message;
}

public class ConnectionException extends CroupierException {
    // 连接相关异常
}

public class RegistrationException extends CroupierException {
    // 注册相关异常
}

public class InvokeException extends CroupierException {
    // 调用相关异常
}

public class ConfigException extends CroupierException {
    // 配置相关异常
}

public class TimeoutException extends CroupierException {
    // 超时异常
}
```

---

## 完整示例

### Provider 示例

```java
package com.example.game;

import io.github.cuihairu.croupier.sdk.*;
import io.github.cuihairu.croupier.sdk.config.*;
import io.github.cuihairu.croupier.sdk.model.*;
import org.json.JSONObject;

public class PlayerService {
    public static void main(String[] args) {
        // 配置
        ClientConfig config = ClientConfig.builder()
            .agentAddr("localhost:19090")
            .gameId("my-game")
            .env("production")
            .serviceId("player-service")
            .insecure(false)
            .caFile("/etc/tls/ca.crt")
            .certFile("/etc/tls/client.crt")
            .keyFile("/etc/tls/client.key")
            .autoReconnect(true)
            .build();

        try (CroupierClient client = new CroupierClient(config)) {
            // 设置回调
            client.onConnectionStateChange(connected -> {
                System.out.println("连接状态: " + connected);
            });

            client.onError(error -> {
                System.err.println("发生错误: " + error.getMessage());
            });

            // 注册函数
            FunctionDescriptor desc = FunctionDescriptor.builder()
                .id("player.ban")
                .version("1.0.0")
                .category("player")
                .risk("high")
                .entity("player")
                .operation("update")
                .enabled(true)
                .build();

            FunctionHandler handler = (context, payload) -> {
                JSONObject req = new JSONObject(payload);
                String playerId = req.getString("player_id");
                String reason = req.optString("reason", "未指定");

                // 业务逻辑
                System.out.printf("封禁玩家: %s, 原因: %s%n", playerId, reason);

                return "{\"status\":\"success\"}";
            };

            client.registerFunction(desc, handler);

            // 连接并启动服务
            client.connect();
            System.out.println("服务已启动");
            client.serve();

        } catch (Exception e) {
            e.printStackTrace();
            System.exit(1);
        }
    }
}
```

### Invoker 示例

```java
package com.example.admin;

import io.github.cuihairu.croupier.sdk.*;
import io.github.cuihairu.croupier.sdk.config.*;
import io.github.cuihairu.croupier.sdk.model.*;

import java.time.Duration;
import java.util.Map;

public class AdminTool {
    public static void main(String[] args) {
        ClientConfig config = ClientConfig.builder()
            .agentAddr("localhost:19090")
            .gameId("my-game")
            .env("production")
            .insecure(true)
            .build();

        try (CroupierInvoker invoker = new CroupierInvoker(config)) {
            // 同步调用
            InvokeOptions options = InvokeOptions.builder()
                .timeout(Duration.ofSeconds(10))
                .idempotencyKey("ban-12345-20260117")
                .metadata(Map.of("operator", "admin"))
                .build();

            String result = invoker.invoke(
                "player.ban",
                "{\"player_id\": \"12345\", \"reason\": \"违规\"}",
                options
            );
            System.out.println("调用结果: " + result);

            // 异步任务
            String jobId = invoker.startJob(
                "player.export",
                "{\"format\": \"csv\"}"
            );
            System.out.println("任务已启动: " + jobId);

            // 监听任务事件
            invoker.streamJob(jobId, event -> {
                switch (event.getType()) {
                    case "progress":
                        System.out.printf("进度: %d%%%n", event.getProgress());
                        break;
                    case "log":
                        System.out.printf("日志: %s%n", event.getMessage());
                        break;
                    case "done":
                        System.out.printf("完成: %s%n", new String(event.getPayload()));
                        break;
                    case "error":
                        System.err.printf("错误: %s%n", event.getMessage());
                        break;
                }
            });

        } catch (Exception e) {
            e.printStackTrace();
            System.exit(1);
        }
    }
}
```

### 异步编程示例

```java
import java.util.concurrent.CompletableFuture;

public class AsyncExample {
    public static void main(String[] args) {
        CroupierInvoker invoker = new CroupierInvoker(config);

        // 异步调用
        CompletableFuture<String> future = invoker.invokeAsync(
            "player.get",
            "{\"player_id\": \"12345\"}"
        );

        // 链式处理
        future.thenApply(result -> {
            System.out.println("获取玩家信息: " + result);
            return new JSONObject(result);
        }).thenAccept(json -> {
            String name = json.getString("name");
            System.out.println("玩家名称: " + name);
        }).exceptionally(e -> {
            System.err.println("调用失败: " + e.getMessage());
            return null;
        });

        // 等待完成
        future.join();
    }
}
```

---

## Spring Boot 集成

### 自动配置

```java
@Configuration
@EnableCroupier
public class CroupierConfig {
    @Bean
    public ClientConfig croupierClientConfig() {
        return ClientConfig.builder()
            .agentAddr("${croupier.agent-addr:localhost:19090}")
            .gameId("${croupier.game-id}")
            .env("${croupier.env:development}")
            .insecure("${croupier.insecure:true}")
            .build();
    }
}
```

### 使用 @CroupierFunction 注解

```java
@Service
public class PlayerService {
    @CroupierFunction(
        id = "player.get",
        version = "1.0.0",
        category = "player",
        risk = "low"
    )
    public String getPlayer(String context, String payload) {
        JSONObject req = new JSONObject(payload);
        String playerId = req.getString("player_id");
        // 业务逻辑
        return "{\"status\":\"success\",\"data\":{...}}";
    }

    @CroupierFunction(
        id = "player.ban",
        version = "1.0.0",
        category = "player",
        risk = "high"
    )
    public String banPlayer(String context, String payload) {
        // 封禁逻辑
        return "{\"status\":\"success\"}";
    }
}
```
