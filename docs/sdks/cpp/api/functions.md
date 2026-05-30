# 函数注册 API

函数注册是 C++ SDK 最基础的能力。

## 关键概念

- `FunctionDescriptor`: 函数元数据
- `FunctionHandler`: 业务处理器
- `RegisterFunction(...)`: 注册函数

## 建议

- 保持函数 ID 稳定
- 描述符至少包含 `id` 和 `version`
- 对输入输出使用 JSON 结构并按需附带 schema
