---
title: C# SDK 指南
---

# C# SDK 指南

## 集成建议

- 在 ASP.NET Core 或后台服务中以单例方式管理客户端生命周期
- 将函数处理器与 DI 注册分开，避免启动逻辑过于复杂
- 对取消令牌、超时和日志字段做统一约束

## 能力方向

- 配置管理
- 依赖注入
- 错误处理
- 异步处理器

## 详细页面

- [安装](./installation)
- [快速开始](./quick-start)
- [配置](./configuration)
- [依赖注入](./dependency-injection)
- [异步处理器](./async-handlers)
- [错误处理](./error-handling)
- [主线程调度器](./threading)
