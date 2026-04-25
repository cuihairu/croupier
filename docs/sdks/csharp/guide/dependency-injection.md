# 依赖注入

C# SDK 适合在 ASP.NET Core 或后台服务中通过依赖注入集成。

## 建议

- `CroupierClient` 使用单例或受控生命周期
- 处理器和业务服务解耦
- 配置通过 `IOptions<T>` 注入
