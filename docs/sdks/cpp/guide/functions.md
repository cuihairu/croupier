# 函数注册

## 基本流程

1. 定义 `FunctionDescriptor`
2. 绑定处理器
3. 在 `Connect()` 之前完成注册

## 建议

- 函数 ID 采用 `[namespace.]resource.operation`
- 高风险操作设置 `risk`
- 处理器保持无状态或显式管理状态
