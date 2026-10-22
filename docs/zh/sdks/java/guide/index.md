---
title: Java SDK 指南
---

# Java SDK 指南

## 集成建议

- SDK 版本通过 Gradle 或 Maven 统一管理
- 业务 handler 与连接配置分层，避免耦合进启动类
- 协议升级时同步检查生成代码与测试

## 并发模型

Java SDK 适合作为长驻服务组件使用；线程池、超时与重试策略应由业务方显式控制。

## OpenAPI 导入（Descriptor v2）

`OpenAPIImporter.registerFromOpenAPI(client, spec, options, resolver)` 在本地解析 OpenAPI 3 JSON，把每个 operation 转成 `FunctionDescriptor` 并注册。handler 按函数 ID（即 operationId，缺失时回退到由 path 推导的 `a.b.c`）查找：

```java
OpenAPIImporter.ImportOptions options = new OpenAPIImporter.ImportOptions()
    .resourcePrefix("game")
    .tagPrefix("svc-")
    .defaultTimeoutMs(30000)
    .continueOnError(true);

OpenAPIImporter.registerFromOpenAPIWithHandlers(
    client, specJson, options,
    Map.of("player_ban", (ctx, payload) -> "{\"ok\":true}"));
```

转换规则与 Go SDK 一致：`inputSchema` 来自 requestBody 的 `application/json` schema，`outputSchema` 来自 200 响应 schema；`x-resource`/`x-operation`/`x-permission`/`x-capability`/`x-execution`/`x-risk` 直接映射到同名字段；`x-approval: {required, policyKey}` 映射到 `approvalRequired`/`approvalPolicyKey`。

- `capability` 枚举：`collection_query|item_query|create|update|delete|action|task|report`
- `execution` 枚举：`sync|task`
- `risk` 词表：`safe|warning|high|danger`
- `defaultTimeoutMs` 为对齐 Go 契约而存在；Java 描述符暂无 timeout 字段，只记录不生效
- 开启 `continueOnError` 后，缺 handler 或注册失败的 operation 会被跳过，导入继续

## Agent 入站请求与探活

Agent 会周期性向 provider 发送 `ProviderHeartbeatRequest` 探测（provider keepalive）。SDK 开箱处理：直接回复空 `ProviderHeartbeatResponse`（pong），无需业务代码。探测不应答会让 agent 判定 session 死亡并移除注册。

收到无法识别的入站消息类型时，SDK 抛出的 `CroupierException` 会带上消息类型名（如 `Unsupported local request type: RegisterRequest (0x...)`)，便于日志排查。

## 继续阅读

- [API 参考](../api/)
