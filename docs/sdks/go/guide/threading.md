# 并发与调度

Go SDK 不需要单线程调度器，但需要明确并发边界。

## 建议

- 使用上下文超时和取消
- 对共享状态加锁或使用并发安全结构
- 避免无控制 goroutine 泄漏
- 对外部依赖统一处理超时和重试

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

## 双车道（Go 独有）

Go SDK 的入站派发为双车道（`transport/tcp_client.go`）：

- **系统车道** `ctrlInbox`（容量 64 + 单 worker）：`IsControlRequest()`
  识别的心跳/注册/Provider 会话控制（含 drain）走此车道，**永不 reject**
- **业务车道** `inbox`（`InboundWorkers` 默认 NumCPU、`InboundQLen` 默认
  workers×4）：满则 fail-fast 回 busy 帧
- 注意：`InboundWorkers`/`InboundQLen` 是 transport 层配置
  （`transport.Config`）；高层 `ClientConfig` 暂未透传，需自定义 transport 时使用
- 测试参考：`dual_lane_test.go`（业务队列打满时心跳仍被处理）
