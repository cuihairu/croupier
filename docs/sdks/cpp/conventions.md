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

## 实践建议

- 虚拟对象使用 ID 引用而不是传递笨重对象
- 高风险操作显式设置 `risk`
