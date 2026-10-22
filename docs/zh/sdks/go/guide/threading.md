# 并发与调度

Go SDK 不需要单线程 dispatcher，但并发边界必须显式。

## 建议

- 使用 context 的 timeout 与 cancellation
- 共享状态加锁或使用并发安全结构
- 避免 goroutine 泄漏失控
- 对外部依赖的 timeout 与 retry 做统一处理

## 入站并发与背压

入站请求（Agent→SDK 的 Invoke 等）由**有界 worker pool** 处理：
read loop 只做入队，从不执行业务逻辑（串行处理会让一个慢 handler 拖住整条连接）。

- 默认 worker 数 ≈ CPU 核数；pending 队列容量 ≈ `workers × 4`（约可吸收 4 轮满负荷突发）
- 业务队列打满时，立即返回**忙错误帧**（`inbound queue full, retry on
another instance`），Agent 随即 failover 到其他实例——SDK 内不累积内存
- 调参位于传输层配置（Go：`InboundWorkers`/`InboundQLen`；
  其他语言见各自的配置章节），六语言语义一致

控制消息（heartbeat/registration/drain）的**双车道隔离**目前仅 Go 具备：控制请求走
专用队列、永不拒绝，因此业务流量激增时 session 仍能保活（通过
`protocol.IsControlRequest()` 分类）。其他语言在 heartbeat 与业务流量之间共享单车道，待迁移到 Go 基线。

## 双车道（仅 Go）

Go SDK 的入站分发使用双车道（`transport/tcp_client.go`）：

- **系统车道** `ctrlInbox`（容量 64 + 单 worker）：由 `IsControlRequest()` 识别的 heartbeat/registration/Provider session 控制（包括 drain）走这里，**永不拒绝**
- **业务车道** `inbox`（`InboundWorkers` 默认 NumCPU，`InboundQLen` 默认为
  workers×4）：打满即快速失败返回忙帧
- 注意：`InboundWorkers`/`InboundQLen` 属于传输层配置
  （`transport.Config`）；上层 `ClientConfig` 尚未转发它们——构建自定义 transport 时使用
- 测试参考：`dual_lane_test.go`（业务队列打满时 heartbeat 仍被处理）
