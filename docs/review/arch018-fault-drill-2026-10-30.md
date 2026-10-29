# ARCH-018 · 执行中断与停机演练（故障验证记录）

日期：2026-10-30。源码基线：本分支 HEAD。演练对象：本地计数执行器/完整栈（真实 TCP 帧 + sqlite 任务库），非真实游戏。
真实游戏按 GAME-010 验收，本记录不把 fixture 的游戏副作用结果当作真实游戏恢复凭据。

## 背景与前置

ARCH-018 依赖 ARCH-015（事件可靠性/重复 Start 幂等）、ARCH-016（Provider drain 摘流）、
ARCH-017（Server 停机收尾顺序）、ARCH-026（任务主记录持久化失败禁止派发）。四项均已收口。

本次演练补上 D3 的**触发条件**：设计文档 §5 指出 `running → unknown` 的迁移触发需要
「重启身份」（restart identity），否则在每次重注册都对账会误把重连 agent 的活任务标未知。
本演练把重启身份落在既有扩展点 `RegisterRequest.labels` 的 `bootId`（`internal/common/labels`）：

- agent 进程启动时随机生成一次 `bootId`（进程生命周期内不变）：重连沿用、重启换新。
- server 在注册时比较上次会话与本次注册的 `bootId`：同值 = 断线重连（不误标）；
  不同值且两侧非空 = 进程重启（把在途 `dispatching`/`running` 任务收敛为 `unknown` 并追加 `recovery` 事件）；
  任一侧为空 = 未声明重启身份（不判别，保持既有行为）。
- `bootId` 经 `sanitizeAgentLabels` 的「平台键豁免通道」持久化，与 `agentBinarySha256` 同通道，
  不可被配置 labels 覆盖或伪造。

## 故障注入与观察

| #   | 注入                       | 注入点                                             | 观察项                                                          | 结果                                                                                                                                                                                                       | 证据（测试）                                                                                                                                  |
| --- | -------------------------- | -------------------------------------------------- | --------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | 断链（执行中）             | agent 与 server 的 TCP 连接在任务 `running` 时断开 | 新 bootId 重注册后任务终态、recovery 事件、误差信息含 boot 迁移 | 任务 `running → unknown`；`finished_at` 置位；`recovery` 事件追加，消息含 `boot BOOT-A -> BOOT-B`；对账 `inflight=1 reconciled=1`                                                                          | `TestArch018Drill_LinkBreakRestartReconcilesToUnknown`                                                                                        |
| 1b  | 断链 + 重连（同 bootId）   | 同上，但重注册沿用 `BOOT-X`                        | 在途任务不得被误标                                              | 任务保持 `running`；无 `recovery` 事件                                                                                                                                                                     | `TestArch018Drill_ReconnectSameBootKeepsRunning`                                                                                              |
| 2   | Provider drain（调用在途） | sidecar 会话 `StartDraining` 后在途调用未返回      | 新请求拒绝、在途等待截止、超时行为                              | 新调用被拒（`provider session is draining`）；`WaitDrained` 有界等待在途（默认 10s），超时照常 ACK（provider 强制断连）                                                                                    | `TestCallProviderRejectsDrainingSessionV9`、`TestHandleDrainWaitsForInflightCallV9`、`TestHandleDrainTimesOutOnStuckCallV9`                   |
| 3   | SIGTERM（server）          | 活跃 agent 会话在 `mux.Run(ctx)` 阻塞读            | 新请求拒绝、停机截止、资源释放                                  | `rootCancel` 前置 → 会话排空 → `listener.Close` 有界返回（实测 < 5s，非 30s 超时）；`srv.Shutdown` 走预算 ctx；`svcCtx.Close` 冲刷执行日志留痕（`FlushInterval` 10s 仍落库，冲刷由 Stop 完成）；会话数归零 | `TestRunGracefulShutdownDrainsLiveSessionBoundedV9`、`TestRunGracefulShutdownFlushesExecutionLogsV9`、`TestRunGracefulShutdownNilResourcesV9` |

## 单点断言（关键路径）

- **断链 + 重启 → unknown**：`status=unknown`、`finished_at != nil`、`recovery` 事件存在且消息含
  `BOOT-A -> BOOT-B`（真实 TCP 帧注册，非直接调用 reconcile）。
- **重连同 bootId → 不误标**：`status=running`、无 `recovery` 事件。
- **终态单调**：`UpdateRunIfStatusNotIn` 以 `TerminalStatuses()` 作护栏——并发终态先到时
  `updated=false`，不追加 `recovery`（`TestHandleRegisterRequest_RestartTerminalGuardSkipsAppend`）。
- **taskStore 未装配**：对账跳过、注册不失败、不 panic（`TestHandleRegisterRequest_RestartNilTaskStoreNoPanic`）。

## 截止时间与资源释放

- 断链 + 重启对账：`require.Eventually(3s)` 内完成（实际毫秒级）。
- drain 在途等待：`WaitDrained` 有界，超时（50ms 测试窗）照常 ACK，不无限阻塞。
- SIGTERM 停机：`runGracefulShutdown` 在 5s 预算内返回，不耗尽外层 30s；会话排空、日志冲刷。

## 诚实边界（不夸口）

- **fixture 非真实游戏**：本地计数执行器/完整栈验证的是平台故障语义；游戏副作用恢复凭据
  （幂等键去重、凭据查询、逐对象结果）属 GAME-010 与 D5，本记录不宣称已覆盖。
- **重启身份缺位**：旧 agent 不报 `bootId` 时无法判别重连/重启，不误标（保持既有行为）；
  这是「未声明重启身份」的诚实边界，不是静默丢任务。
- **跨 agent 对账**：`taskId` 归属单 agent，无跨 agent 交接的 at-least-once/exactly-once 承诺（D5 边界）。
- **真实 DB 故障注入**：断链/drain/SIGTERM 均在本地 sqlite 上验证；生产 postgres 的故障注入
  属运维演练，不在本记录范围。
