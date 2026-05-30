# Client API 参考

`CroupierClient` 是 C# SDK 的核心入口，负责连接 Agent、注册函数和维持服务生命周期。

## 常见职责

- 建立与 Agent 的连接
- 注册函数描述符和处理器
- 启动服务循环
- 管理重连、心跳和关闭流程
