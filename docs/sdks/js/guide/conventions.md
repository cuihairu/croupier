# Croupier JS/TS SDK 约定规范

## 命名约定

函数 ID 使用 `[namespace.]resource.operation`：

```typescript
"player.get";
"player.ban";
"inventory.item.add";
```

避免使用驼峰、下划线或连字符：

```typescript
"PlayerGet";
"player_get";
"player-get";
```

## 注册约定

所有函数必须在 `connect()` 前注册完成：

```typescript
client.registerFunction(descriptor, handler);
await client.connect();
```

同一服务实例内，函数 ID 必须唯一。

## 描述符建议

最小可注册字段：

- `id`
- `version`

建议补齐基础说明：

- `summary`
- `description`
- `tags`
- `inputSchema`
- `outputSchema`

建议补齐业务与治理信息：

- `resource`
- `operation`
- `capability`
- `risk`
- `enabled`
- `permission`

语义约束：

- `operation` 是业务操作 key，例如 `ban`、`grant`、`send`。
- SDK descriptor 不提供页面 schema、组件树、页面 mapping、菜单、分类显示名、页面标题、按钮文案或页面位置。
- 动态分类、页面标题、按钮文案和页面位置只在 PageSpec / Page Studio 中确定，不写入 SDK descriptor，也不写入前端静态 locale 文件。
- `capability` 只允许受控资源语义：`collection_query/item_query/create/update/delete/action/task/report`；它不是页面类型或按钮位置。
- 默认 PageProposal 由 Server 根据 FunctionContract、JSON Schema、CapabilitySemantics 和 diagnostics 生成；SDK 不提供列、mapping、菜单或 UI。

## OpenAPI 导入（Descriptor v2）

`registerFromOpenAPI` 在本地解析 OpenAPI 3 JSON（不连接 server），把每个 operation 转成 Descriptor v2 的 `FunctionDescriptor` 并注册，handler 按派生的函数 ID 查找：

```typescript
import { createClient, registerFromOpenAPI } from "@croupier/sdk";

const client = createClient({ serviceId: "my-service" });

const registered = registerFromOpenAPI(
  client,
  specJson, // string 或已解析的对象
  {
    resourcePrefix: "game",
    tagPrefix: "svc-",
    defaultTimeoutMs: 30000,
    continueOnError: true,
  },
  (functionId) => handlers[functionId], // 返回 undefined 且未开 continueOnError 时报错
);
```

转换规则：

- `id` 取 `operationId`；缺失时由 path 生成 `a.b.c` 风格 ID。
- `name`/`summary` 取 `summary`；缺失时用 `operationId` 的 titleCase。
- `inputSchema` 取 requestBody 的 `application/json` schema；`outputSchema` 取 200 响应 schema（简化为 JSON Schema）。
- 扩展字段映射：`x-resource`、`x-operation`、`x-capability`、`x-execution`、`x-permission`、`x-risk`。
- `x-approval: { required, policyKey }` 映射 `approvalRequired` / `approvalPolicyKey`。
- `risk` 词表为 `safe|warning|high|danger`（`low`/`medium` 等旧别名会被归一）；`capability`、`execution` 为受控枚举，非法值抛错，可用 `continueOnError` 跳过。

完整契约见 [OpenAPI / SDK Descriptor v2](../../../architecture/openapi-sdk-descriptor-v2.md)。
