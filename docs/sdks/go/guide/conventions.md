# Croupier Go SDK 约定规范

## 命名约定

函数 ID 使用 `[namespace.]entity.operation`：

```go
"player.get"
"player.ban"
"inventory.item.add"
```

## 注册约定

- 在客户端启动前完成函数注册
- 同一服务实例内函数 ID 保持唯一
- 高风险操作明确填写 `Risk`
