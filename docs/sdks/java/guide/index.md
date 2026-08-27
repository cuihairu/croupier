---
title: Java SDK 指南
---

# Java SDK 指南

## 集成建议

- 统一通过 Gradle 或 Maven 管理 SDK 版本
- 把业务处理器与连接配置分层，避免耦合在启动类中
- 协议升级时同步检查生成代码和测试

## 并发模型

Java SDK 更适合作为长生命周期服务组件运行。线程池、超时和重试策略应由业务方显式控制。

## OpenAPI 导入（Descriptor v2）

`OpenAPIImporter.registerFromOpenAPI(client, spec, options, resolver)` 在本地解析 OpenAPI 3 JSON，把每个 operation 转换为 `FunctionDescriptor` 并注册，handler 按 function ID（即 operationId，缺失时由 path 回退为 `a.b.c`）查找：

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

转换规则与 Go SDK 一致：`inputSchema` 取 requestBody 的 `application/json` schema，`outputSchema` 取 200 响应 schema；`x-resource`/`x-operation`/`x-permission`/`x-capability`/`x-execution`/`x-risk` 直接映射到同名字段；`x-approval: {required, policyKey}` 映射到 `approvalRequired`/`approvalPolicyKey`。

- `capability` 枚举：`collection_query|item_query|create|update|delete|action|task|report`
- `execution` 枚举：`sync|task`
- `risk` 词表：`safe|warning|high|danger`
- `defaultTimeoutMs` 为 Go 契约对齐项，当前 Java descriptor 尚无超时字段，仅记录不生效
- `continueOnError` 开启后单个 operation 缺 handler 或注册失败会跳过并继续

## 继续阅读

- [线程与并发](./threading)
- [API 参考](../api/)
