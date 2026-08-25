# Croupier C++ SDK 约定规范

## 命名约定

函数 ID 使用 `[namespace.]resource.operation`：

```cpp
"player.get"
"player.ban"
"inventory.item.add"
```

## 注册约定

- 在 `Connect()` 之前完成注册
- 同一服务实例内函数 ID 唯一
- 描述符至少包含 `id` 和 `version`

## 描述符建议

建议补齐基础说明：

- `summary`
- `description`
- `tags`
- `input_schema`（C++ 成员名；对应契约键 `inputSchema`）
- `output_schema`（C++ 成员名；对应契约键 `outputSchema`）

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

完整契约见 [OpenAPI / SDK Descriptor v2](../../architecture/openapi-sdk-descriptor-v2.md)。

## 实践建议

- 函数参数使用 ID 引用业务对象，不传递笨重对象
- 高风险操作显式设置 `risk`
