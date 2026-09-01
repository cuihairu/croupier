# 线程与并发

C++ SDK 集成时需要显式控制线程模型和资源生命周期。

## 建议

- 明确连接线程、业务线程和后台任务边界
- 共享状态使用锁或消息传递保护
- 关闭流程要显式 drain 和 cleanup

## 入站并发与背压

入站请求（Agent→SDK 的 Invoke 等）由**有界 worker 池**处理：
读循环只投递不执行业务（串行处理会让一个慢 handler 卡住整条连接）。

- 默认 worker 数 ≈ CPU 核数；待处理队列容量 ≈ `workers × 4`（突发吸收 4 轮满载）
- 业务队列打满时**立即回 busy 错误帧**（`inbound queue full, retry on
another instance`），Agent 侧 failover 换实例重试——SDK 内存不积累
- 调优入口在 transport 层配置（Go：`InboundWorkers`/`InboundQLen`；
  其他语言见各自配置章节），语义六语言一致

控制消息（心跳/注册/drain）的**双车道隔离**目前仅 Go 落地：控制请求走
专用队列永不 reject，业务洪峰时会话仍存活（`protocol.IsControlRequest()`
分类）。其余语言心跳与业务共用车道，待按 Go 基准迁移。
