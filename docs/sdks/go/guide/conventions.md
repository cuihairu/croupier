# Croupier Go SDK 约定规范

## 命名约定

函数 ID 使用 `[namespace.]resource.operation`：

```go
"player.get"
"player.ban"
"inventory.item.add"
```

## 注册约定

- 在客户端启动前完成函数注册
- 同一服务实例内函数 ID 保持唯一
- 高风险操作明确填写 `Risk`

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
