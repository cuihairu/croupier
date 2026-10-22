# Client API 参考

`CroupierClient` 是 C++ SDK 的核心入口。

## 典型职责

- 与 Agent 建立会话（session）
- 注册函数（functions）、资源能力（resource capabilities）与组件（components）
- 维持心跳、重连与请求处理循环
- 管理关闭与资源释放

## 接入建议

- 连接配置与业务 handler 分离管理
- 在服务启动阶段完成注册
- 高风险函数显式设置风险级别（risk level）与 schema
