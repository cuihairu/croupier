# Croupier JS/TS SDK 约定

## 命名约定

函数 ID 使用 `[namespace.]resource.operation`：

```typescript
"player.get";
"player.ban";
"inventory.item.add";
```

避免 camelCase、下划线或连字符：

```typescript
"PlayerGet";
"player_get";
"player-get";
```

## 注册约定

所有函数必须在 `connect()` 之前注册：

```typescript
client.registerFunction(descriptor, handler);
await client.connect();
```

同一服务实例内函数 ID 必须唯一。

## 描述符建议

可注册的最小字段：

- `id`
- `version`

填写基础描述字段：

- `summary`
- `description`
- `tags`
- `inputSchema`
- `outputSchema`

填写业务与治理字段：

- `resource`
- `operation`
- `capability`
- `risk`
- `enabled`
- `permission`

语义约束：

- `operation` 是业务操作键，如 `ban`、`grant`、`send`。
- SDK 描述符不提供页面 schema、组件树、页面映射、菜单、分类显示名、页面标题、按钮文案或页面摆放。
- 动态分类、页面标题、按钮文案与页面摆放只在 PageSpec / Page Studio 中决定。它们不写入 SDK 描述符，也不写入前端静态 locale 文件。
- `capability` 只允许受控的资源语义：`collection_query/item_query/create/update/delete/action/task/report`。它不是页面类型，也不是按钮摆放。
- 默认 PageProposal 由 Server 基于 FunctionContract、JSON Schema、CapabilitySemantics 与诊断信息生成；SDK 不提供列、映射、菜单或 UI。

## OpenAPI 导入（Descriptor v2）

`registerFromOpenAPI` 在本地解析 OpenAPI 3 JSON（不连接服务端），把每个 operation 转换为 Descriptor v2 的 `FunctionDescriptor` 并注册。handler 通过派生的函数 ID 查找：

```typescript
import { createClient, registerFromOpenAPI } from "@croupier/sdk";

const client = createClient({ serviceId: "my-service" });

const registered = registerFromOpenAPI(
  client,
  specJson, // string or an already-parsed object
  {
    resourcePrefix: "game",
    tagPrefix: "svc-",
    defaultTimeoutMs: 30000,
    continueOnError: true,
  },
  (functionId) => handlers[functionId], // throws on undefined when continueOnError is off
);
```

转换规则：

- `id` 来自 `operationId`；缺失时从 path 生成 `a.b.c` 风格的 ID。
- `name`/`summary` 来自 `summary`；缺失时使用 `operationId` 的 titleCase。
- `inputSchema` 来自 requestBody 的 `application/json` schema；`outputSchema` 来自 200 响应的 schema（简化为 JSON Schema）。
- 扩展字段映射：`x-resource`、`x-operation`、`x-capability`、`x-execution`、`x-permission`、`x-risk`。
- `x-approval: { required, policyKey }` 映射到 `approvalRequired` / `approvalPolicyKey`。
- `risk` 词表为 `safe|warning|high|danger`（`low`/`medium` 等遗留别名会被归一化）；`capability` 与 `execution` 是受控枚举——非法值会抛错，可用 `continueOnError` 跳过。

完整契约见 [OpenAPI / SDK Descriptor v2](../../../architecture/openapi-sdk-descriptor-v2.md)。
