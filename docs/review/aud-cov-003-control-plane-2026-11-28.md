# AUD-COV-003 收尾审计：控制面真实装配、重连与背压组合（2026-11-28）

范围承接 todo.md AUD-COV-003 的进度记录：此前轮次已覆盖 Provider 与互联关闭边界
（AUD-070～075、AUD-081）、注册就绪/停止后重连/配置版本回退（AUD-092～094）、协议协商
缺口（AUD-101）。本报告补齐最后一项遗留——「控制面真实装配、重连与背压组合」——并收口。

## 1. 控制面真实装配（real assembly）

**结论：装配链每一跳都有生产入口；超时/队列参数全部落在可解释的具体值上。无新缺陷。**

- Server 侧监听（`cmd/server/root.go:631-642`）：`TCPListenerConfig{RecvTimeout: 1s,
SendTimeout: 10s}` 接 `sessionStore`/`RegistryStore` 装配为控制 TCP；`RecvTimeout: 1s`
  并非激进断连——`core/transport/tcp/mux_conn.go` 读循环把读超时当轮询粒度
  （`isTimeout(err)` → `continue`，mux_conn.go:207-216），1s 即优雅停机的检查周期。
- Server 每连接 mux（`internal/server/tcp_listener.go:246-279`）：TLS 握手门禁先于注册
  读取（AUD-040，10s deadline，卡握手对端按时断开），随后 `NewMuxConn` 挂
  `agentSessionHandler`。
- Agent 上行（`internal/app/agent/control_client.go:64-74`）：上行 mux
  `RecvTimeout: 30s / SendTimeout: 10s`；`DispatchWorkers`/`BusinessQLen`/`ControlQLen`
  未显式配置 → 走 `NewMuxConn` 默认（workers=max(NumCPU,2)、bizQ=workers×4、
  ctrlQ=64，mux_conn.go:84-95）——默认值即真实生产值，与本审计按默认值推演的结论一致。
- 注册/心跳/重连上收 `core/register`（K2 上收，`internal/app/agent/upstream.go:145`）：
  `Dial`/`Payload`/`OnConnected` 闭包晚绑定，`SetTLSConfig`/`SetLocalHandler`/
  `WithMetadata` 在 Start 后仍生效；`dialUpstream`（upstream.go:254-262）按是否设置
  `localHandler` 选择双向 mux 或单向上行。
- Agent 本地 provider 监听（`internal/app/agent/app.go:248-260`）：`RecvTimeout: 2min`
  带有书面依据（Go SDK 心跳默认 60s，监听必须留出心跳+调度抖动的余量），外加 5s
  provider keepalive 探针（CROUPIER_AGENTLOCAL_PROBE_INTERVAL）。

## 2. 重连（reconnection）

**结论：重连自愈链完整且停机闸/快照发布语义闭环；注册载荷晚组装使重连自动携带最新
函数快照。无新缺陷。**

- `core/register/loops.go`：`RunReconnectLoop`（loops.go:127-148）指数退避
  5s→60s×1.5（`newDialBackoff`，手动驱动 `NextBackOff`，不因累计时长放弃，退出仅由
  ctx 取消或成功触发）；`RunHeartbeatLoop`（loops.go:155-205）3s 心跳、连续 2 次失败
  主动重连、恢复后立即重注册——断连走「主动重连」而非跳过心跳（否则无恢复路径）。
- `DialAndRegister`（loops.go:58-107）：`dialInflight` CAS 防并发拨号；旧连接关闭即
  `clearPublishedIf` 撤销业务侧可见快照（AUD-092：置换窗口业务侧看到 not-ready，
  不是指向已关闭连接的过期 ready）；落位前在 mu 临界区内复核停机闸（AUD-093：拨号
  可能晚于 Stop 返回，禁止复活已停止客户端）；注册调用应用 `requestTimeout` 边界
  （ARCH-022：无注册响应时 Register 不永久阻塞重连/心跳路径）。
- 重连即最新快照：`buildRegisterPayload` 由 `registerOnce` 在注册时刻闭包调用
  （upstream.go:393 注释明确「注册前重新取连接快照，覆盖组装期间连接被置空的窗口」）
  ——重连注册天然重报当前函数/进程清单，无需额外 resync；本地变更另经 500ms 去抖
  `updateLoop` + `SyncAttempts`（3 次尝试、200ms→2s 退避，registration.go:79-108）。
- 发布链停机安全：`publishRegistered` 在 mu 临界区内复核停机闸后才发布+触发
  `OnConnected`（registration.go:44-64），ack 不复活已停止客户端。
- Agent 侧 ops 进程监视的重启退避/熔断（`ops_server_monitor.go:116`）是另一条独立
  自愈链，前轮已审，不在此重复。

## 3. 背压组合（backpressure combinations）

**结论：双车道语义、busy 终态、写路径串行化、pending 泄漏防护均有实现与回归测试双
落地；组合推演未发现死锁或无界路径。一条 P3 死代码发现（见第 4 节）。**

- 双车道入站分发（`core/transport/tcp/mux_conn.go:135-160`）：控制请求走 `ctrlInbox`
  （cap 64）**阻塞入队**（select closed/ctx 逃逸——控制帧绝不静默丢弃）；业务请求走
  `bizInbox`（cap workers×4）**非阻塞入队**，队满即以统一编码（AUD-002）回 busy 失败
  帧 `"inbound queue full, retry on another instance"`。回归测试：
  `mux_conn_dual_lane_test.go` 的 `BusinessSaturation`（业务饱和 busy）、
  `ControlNeverRejected`（业务饱和时控制仍可达）、`ConcurrentWritesSafe`（并发写安全）。
- busy 是终态响应、不触发换实例（CR-FIX-10 回归契约，
  `internal/platform/dispatch/dispatcher_delivery_semantics_test.go`）：busy 帧是成功
  完成的协议往返，对非幂等 Ops 命令换实例重试 = 隐式重复执行入口；Invoke 与
  StartTask（空 body 形态）两条用例钉死「busy 绝不 failover」。本审计确认这是有意
  设计而非遗漏。
- failover 的真实边界（`internal/platform/dispatch/dispatcher_invoke.go:34-107`）：
  仅 `errAgentUnreachable`（本地无连接且转发失败/无路由，callAgent/dispatcher_invoke.go
  一带显式包装）触发换候选，lb 路径上限 3 次，targeted/hash 粘性路由豁免；**调用中途
  连接断开（`Call` 返回 connection closed）不归类为 unreachable、不换候选**——结果
  未知 ≠ 不可达，与 busy 不 failover 同一诚实投递原则，结构上由错误分类保证。
- 写路径（mux_conn.go:396-420）：`writeMu` 串行化帧写 + `sendTimeout` 写 deadline
  （stalled 对端拖死 worker 的时间有上界）；读路径读 deadline 即轮询粒度（见第 1 节），
  超时 continue 不断连。
- pending 表（mux_conn.go:304-348、422-445）：每 Call 独立 cap-1 响应通道；
  `fulfillPending`/`failPending` 非阻塞投递——调用方超时迟到响应被静默丢弃，无
  goroutine/通道泄漏；`Close` 一次性失败全部 pending。关闭竞态回归：
  `mux_conn_close_race_test.go`（`Call_PrefersDeliveredResponseOverClosed`、
  `Call_ClosedBeforeMountEmptyQueue`）。
- 组合推演（读循环 × 控制车道）：控制车道单 worker + 阻塞入队意味着控制 handler 长期
  阻塞且 ctrl 队列满时读循环会停在 `ctrlInbox <- task`。该路径有界且自愈——控制入站
  集合仅为会话控制（`pkg/protocol/message.go:315-331`：register/heartbeat/capabilities/
  provider-client-cluster hello/drain，业务与 Ops 不在列）；单连接心跳速率 1/3s，填满
  64 深队列需约 3 分钟的持续 handler 阻塞；期间 agent 心跳 Call 3s 超时 ×2 次失败即
  主动重连，旧连接 `Close` → `closed` 通道使卡住的入队立即逃逸。无死锁路径。
- 版本协商与背压的交叉（AUD-101）：首条受支持帧粘滞锁定连接版本，偏离帧按不支持
  处理并**以对端版本回写拒绝帧、会话保持**（mux_conn.go:223-254）——背压 busy 帧
  与版本拒绝帧同走 `writeFrameVersion`，互不破坏 reqID 配对。回归：
  `UnsupportedVersionRejectsRequestAndKeepsSession`。

## 4. 发现：ReconnectionPolicy 是未接线组件（P3）

`internal/platform/dispatch/reconnection_policy.go` 的 `ReconnectionPolicy`/
`ReconnectionState`/`ReconnectionManager` 在全仓**零生产消费方**（仅自身单测
`dispatcher_ha_test.go:410` 引用）。真实重连在 `core/register` 循环、真实换候选在
dispatcher 的 `errAgentUnreachable` failover——该组件以其命名误导审计者以为它就是
agent 重连机制（本次审计即险些按它下结论）。属「组件存在不等于已接线」同族问题
（CLAUDE.md 对 SQLStore 的告诫）。建议后续删除或显式接线；不构成立即风险，不立新
编号项，记入本报告留档。

## 收口判定

AUD-COV-003 声明的子范围（鉴权、TLS、帧解析、并发/背压、重连、取消、错误传播与关闭，
覆盖 core、agents、internal/agent、internal/transport、core/transport）经多轮累计至此
全部复核：本轮三项（控制面真实装配、重连链、背压组合）无新增缺陷；一条 P3 死代码
发现（ReconnectionPolicy 未接线）留档待后续清理。既有边界均有编号条目跟踪
（AUD-070～075、081、092～094、101、CR-FIX-10），不以「扫描器无告警」替代人工复核。
