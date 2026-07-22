# Croupier C++ SDK 约定规范

## 命名约定

函数 ID 使用 `[namespace.]entity.operation`：

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
- `input_schema`
- `output_schema`

需要参与默认页面生成时必须明确：

- `category`
- `category_display`
- `entity`
- `entity_display`
- `operation`
- `operation_display`
- `operation_kind`
- `placement`
- `risk`

语义约束：

- `operation` 是业务操作 key，例如 `ban`、`grant`、`send`。
- `operation_kind` 是页面生成语义，例如 `list`、`get`、`action`、`task`、`report`。
- `placement` 是页面放置位置，例如 `tableData`、`rowAction`、`standalone`。
- 动态分类、资源、操作标题必须随 descriptor 或 PageSpec 提供，不写入前端静态 locale 文件。

完整契约见 [OpenAPI / SDK Descriptor v2](../../architecture/openapi-sdk-descriptor-v2.md)。

## 实践建议

- 虚拟对象使用 ID 引用而不是传递笨重对象
- 高风险操作显式设置 `risk`
