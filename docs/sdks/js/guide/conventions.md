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
- `input_schema`
- `output_schema`

建议补齐业务与治理信息：

- `resource`
- `operation`
- `risk`
- `enabled`
- `permission`

语义约束：

- `operation` 是业务操作 key，例如 `ban`、`grant`、`send`。
- SDK descriptor 不提供 `category_display`、`entity_display`、`operation_display`、`operation_kind`、`placement` 或 `page_hint`。
- 动态分类、页面标题、按钮文案和页面位置只在 PageSpec / Page Studio 中确定，不写入 SDK descriptor，也不写入前端静态 locale 文件。
- 默认 PageSpec 候选由 Server 根据 FunctionSpec、JSON Schema、PageContract 和 diagnostics 生成；缺少可验证 mapping 时只能进入待编排状态。

完整契约见 [OpenAPI / SDK Descriptor v2](../../../architecture/openapi-sdk-descriptor-v2.md)。
