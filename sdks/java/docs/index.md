---
home: true
title: Croupier Java SDK
titleTemplate: false
heroImage: /logo.png
heroText: Croupier Java SDK
tagline: Java SDK for Croupier Game Backend Platform
actions:
  - text: 快速开始
    link: /guide/quick-start.html
    type: primary
features:
  - title: gRPC 集成
    details: 完整的 gRPC 通信支持
  - title: 函数注册
    details: 简单的函数注册和处理
  - title: 多平台支持
    details: 支持 JVM 生态系统

footer: Apache License 2.0 | Copyright © 2024 Croupier
---

## 简介

Croupier Java SDK 是 [Croupier](https://github.com/cuihairu/croupier) 的 Java 客户端实现。

## 快速开始

### 安装

```xml
<dependency>
    <groupId>com.github.cuihairu</groupId>
    <artifactId>croupier-sdk-java</artifactId>
    <version>0.1.1</version>
</dependency>
```

### 使用

```java
import com.github.cuihairu.croupier.sdk.*;

public class Main {
    public static void main(String[] args) {
        ClientConfig config = new ClientConfig();
        config.setGameId("my-game");
        config.setEnv("development");
        config.setAgentAddr("localhost:19090");

        CroupierClient client = new CroupierClient(config);
        client.connect();
        client.serve();
    }
}
```
