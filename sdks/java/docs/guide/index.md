# 入门指南

欢迎使用 Croupier Java SDK！

## 系统要求

- Java 17+
- Maven 或 Gradle

## 安装

### 方式一：Spring Boot Starter（推荐）

如果你使用 Spring Boot，推荐使用 Spring Boot Starter，它提供自动配置和简化的集成方式。

#### Maven

```xml
<dependency>
    <groupId>io.github.cuihairu.croupier</groupId>
    <artifactId>croupier-spring-boot-starter</artifactId>
    <version>0.1.1</version>
</dependency>
```

#### Gradle

```groovy
implementation 'io.github.cuihairu.croupier:croupier-spring-boot-starter:0.1.1'
```

#### 配置

在 `application.yml` 中添加配置：

```yaml
croupier:
  game-id: my-game
  service-id: my-service
  agent-address: localhost:19090
  env: development
  insecure: true
```

#### 使用

```java
import io.github.cuihairu.croupier.sdk.CroupierClient;
import io.github.cuihairu.croupier.sdk.FunctionDescriptor;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Component;

@Component
public class MyGameService {

    @Autowired
    private CroupierClient croupierClient;

    @PostConstruct
    public void init() {
        // 注册函数
        FunctionDescriptor desc = FunctionDescriptor.builder()
            .id("hello.world")
            .version("0.1.1")
            .name("Hello World")
            .description("A simple hello function")
            .build();

        croupierClient.registerFunction(desc, (ctx, payload) -> {
            return "{\"message\":\"Hello from Spring Boot!\"}";
        });
    }
}
```

### 方式二：手动集成 SDK

如果你不使用 Spring Boot，可以手动集成 SDK。

#### Maven

```xml
<dependency>
    <groupId>io.github.cuihairu.croupier</groupId>
    <artifactId>croupier-java-sdk</artifactId>
    <version>0.1.1</version>
</dependency>
```

#### Gradle

```groovy
implementation 'io.github.cuihairu.croupier:croupier-java-sdk:0.1.1'
```

#### 使用

```java
import io.github.cuihairu.croupier.sdk.*;

public class Example {
    public static void main(String[] args) {
        // 创建配置
        ClientConfig config = new ClientConfig("demo-game", "demo-service");
        config.setAgentAddr("localhost:19090");
        config.setEnv("development");
        config.setInsecure(true);

        // 创建客户端
        CroupierClient client = CroupierSDK.createClient(config);

        // 注册函数
        FunctionDescriptor desc = FunctionDescriptor.builder()
            .id("hello.world")
            .version("0.1.1")
            .name("Hello World")
            .description("A simple hello function")
            .build();

        client.registerFunction(desc, (ctx, payload) -> {
            return "{\"message\":\"Hello from Java!\"}";
        });

        // 连接并启动服务
        client.connect().join();
        client.serve();
    }
}
```

## GitHub Packages 仓库配置

由于 SDK 发布在 GitHub Packages，你需要添加仓库配置：

### Maven (settings.xml)

```xml
<settings>
    <servers>
        <server>
            <id>github</id>
            <username>YOUR_GITHUB_USERNAME</username>
            <password>YOUR_GITHUB_TOKEN</password>
        </server>
    </servers>
</settings>
```

### Maven (pom.xml)

```xml
<repositories>
    <repository>
        <id>github</id>
        <url>https://maven.pkg.github.com/cuihairu/croupier-sdk-java</url>
    </repository>
</repositories>
```

### Gradle

```groovy
repositories {
    maven {
        url = uri('https://maven.pkg.github.com/cuihairu/croupier-sdk-java')
        credentials {
            username = System.getenv('GITHUB_USERNAME') ?: project.findProperty('github.username')
            password = System.getenv('GITHUB_TOKEN') ?: project.findProperty('github.token')
        }
    }
}
```

## 下一步

- [主线程调度器](./threading.md) - gRPC 回调线程安全处理
- [API 参考](../api/) - 完整的 API 文档
