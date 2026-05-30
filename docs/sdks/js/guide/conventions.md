# Croupier JS/TS SDK 约定规范

## 命名约定

函数 ID 使用 `[namespace.]entity.operation`：

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

建议明确：

- `id`
- `version`
- `category`
- `risk`
- `entity`
- `operation`
- `enabled`
