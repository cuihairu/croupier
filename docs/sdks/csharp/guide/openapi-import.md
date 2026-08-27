# OpenAPI 导入

`OpenAPIImporter` 提供与 Go SDK `RegisterFromOpenAPI` 对齐的纯本地 OpenAPI 3 导入：解析 JSON spec，把每个 operation 转换为 `FunctionDescriptor` 并连同 handler 注册到 `CroupierClient`，不发起任何网络请求。

## 基本用法

```csharp
using Croupier.Sdk;

var registered = OpenAPIImporter.RegisterFromOpenAPI(
    client,
    specJson,
    new OpenAPIImportOptions
    {
        ResourcePrefix = "demo",
        TagPrefix = "v2:",
        DefaultTimeoutMs = 30000,
        ContinueOnError = true,
    },
    operationId => handlers.TryGetValue(operationId, out var handler) ? handler : null);
```

也可以传入显式 handler 映射（等价 Go 的 `RegisterFromOpenAPIWithHandlers`）：

```csharp
var registered = OpenAPIImporter.RegisterFromOpenAPI(
    client, specJson, options, handlersDictionary);
```

返回值为已注册的函数 ID 列表（按 spec 中出现顺序）。

## ImportOptions

| 属性               | 说明                                                   |
| ------------------ | ------------------------------------------------------ |
| `ResourcePrefix`   | 为 `x-resource` 加前缀，如 `"demo"` → `demo.player`    |
| `TagPrefix`        | 为每个 tag 加前缀                                      |
| `DefaultTimeoutMs` | 默认超时；C# 描述符契约暂无 timeout 字段，仅保留选项位 |
| `ContinueOnError`  | 单个函数缺 handler 或注册失败时跳过而非抛异常          |

未启用 `ContinueOnError` 时，缺少 handler 抛 `InvalidOperationException`，spec 非法抛 `ArgumentException`。

## 转换规则（Descriptor v2）

| 描述符字段          | 来源                                                                                           |
| ------------------- | ---------------------------------------------------------------------------------------------- |
| `Id`                | `operationId`；缺失时由 path 生成 `a.b.c`（`/api/players/{id}` → `api.players.{id}`）          |
| `Summary`           | `summary`；缺失时由 operationId 转标题大小写                                                   |
| `Description`       | `description`                                                                                  |
| `Tags`              | `tags`                                                                                         |
| `InputSchema`       | `requestBody.content["application/json"].schema`（浅层 JSON Schema）                           |
| `OutputSchema`      | `responses["200"].content["application/json"].schema`                                          |
| `Resource`          | `x-resource`                                                                                   |
| `Operation`         | `x-operation`                                                                                  |
| `Capability`        | `x-capability`（`collection_query\|item_query\|create\|update\|delete\|action\|task\|report`） |
| `Execution`         | `x-execution`（`sync\|task`）                                                                  |
| `Permission`        | `x-permission`                                                                                 |
| `Risk`              | `x-risk`（`safe\|warning\|high\|danger`；`low/medium/critical` 等旧别名会被归一）              |
| `Enabled`           | `x-enabled`（默认 `true`）                                                                     |
| `ApprovalRequired`  | `x-approval.required`                                                                          |
| `ApprovalPolicyKey` | `x-approval.policyKey`                                                                         |

`x-approval` 与 `execution` 正交：`execution: "task"` + `approval.required: true` 表示审批通过后再创建任务；审批不是 `execution` 的第三个枚举值。

完整契约见 [OpenAPI / SDK Descriptor v2](../../../architecture/openapi-sdk-descriptor-v2.md)。
