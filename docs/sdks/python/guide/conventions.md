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

建议描述符包含：

- `id`
- `version`
- `category`
- `risk`
- `entity`
- `operation`
- `enabled`
