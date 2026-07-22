# Croupier Python SDK 约定规范

## 命名约定

函数 ID 使用 `entity.operation` 或 `[namespace.]entity.operation`：

```python
"player.get"
"player.ban"
"game.player.ban"
```

## 注册约定

所有函数必须在 `connect()` 前完成注册：

```python
client.register_function(descriptor, handler)
client.connect()
```

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
- `enabled`

语义约束：

- `operation` 是业务操作 key，例如 `ban`、`grant`、`send`。
- `operation_kind` 是页面生成语义，例如 `list`、`get`、`action`、`task`、`report`。
- `placement` 是页面放置位置，例如 `tableData`、`rowAction`、`standalone`。
- 动态分类、资源、操作标题必须随 descriptor 或 PageSpec 提供，不写入前端静态 locale 文件。

完整契约见 [OpenAPI / SDK Descriptor v2](../../../architecture/openapi-sdk-descriptor-v2.md)。
