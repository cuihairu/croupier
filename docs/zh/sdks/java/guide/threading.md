# 线程与并发

Java SDK 适合作为长驻服务组件使用；线程池与执行模型应由集成方显式控制。

## 建议

- handler 内不要阻塞共享线程池
- I/O 密集与 CPU 密集任务分开
- 显式定义超时、取消与重试策略

## 入站并发与背压

入站请求（Agent→SDK 的 Invoke 等）由**有界 worker 池**处理：
读循环只入队工作，从不执行业务逻辑（串行处理会让一个慢 handler 拖住整条连接）。

- 默认 worker 数 ≈ CPU 核数；待处理队列容量 ≈ `workers × 4`（可吸收约 4 轮满负荷突发）
- 业务队列打满时**立即返回 busy 错误帧**（`inbound queue full, retry on
another instance`），Agent 随即切换到其他实例——SDK 内不堆积内存
- 调参位于传输层配置（Go：`InboundWorkers`/`InboundQLen`；
  其他语言见各自配置节），六种语言语义一致

**控制消息双车道隔离**（心跳/注册/drain）目前仅 Go 支持：控制请求走专用队列、
永不拒绝，业务流量洪峰下 session 仍保活（通过
`protocol.IsControlRequest()` 分类）。其他语言心跳与业务流量共用一条车道，
待迁移到 Go 基线。
