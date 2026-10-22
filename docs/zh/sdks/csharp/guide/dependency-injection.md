# 依赖注入

C# SDK 在 ASP.NET Core 或后台服务中通过依赖注入集成良好。

## 建议

- `CroupierClient` 注册为单例或受控生命周期
- handler 与业务服务解耦
- 配置通过 `IOptions<T>` 注入
