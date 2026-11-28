---
title: 受管程序配置更新方案评审
---

# 受管程序配置更新方案评审

日期：2026-11-28。状态：**提案待 owner 决策**。本页是 OPS-013 方案决策交付物：
把远程更新**受管程序配置**（`OpsConfig.ManagedProcesses` 的
command/args/env/schedule）的五个开放问题收敛成具体提案，复用
`core/configsync` 拉取先例与 §10.4 更新机制。本页不改任何代码——实施以 owner
对下方的批准为门。

**显式非声明（验收口径）**：既有探测面配置拉取（`probe.policy` /
`synthetics.targets` 走 `config_versions`，PROBE-004）只覆盖**探测面配置**。
它**不**分发程序文件或受管程序配置。本提案描述的是*复用同一拉取模式*的新工作
——绝不得记成「既有探针配置拉取已支持」。

## 1. 现状（代码事实）

- 受管程序配置是**静态 yaml**，agent 启动时装入 `OpsConfig.ManagedProcesses`。
  调度表构造期编译一次（`compileProcessSchedules`；「config 构造后不变」——
  快照与调度循环共读编译产物）。在线配置更新因此需要一条**今天不存在的重编译
  路径**——这是真实实施成本，先行声明。
- `ManagedProcessConfig.Validate()` 已存在（`internal/app/agent/ops_config.go`）：
  kind / schedule action / cron（`processcore.ParseCron`，与服务端任务调度共用
  同一实现）/ snapshotProfile 的闭集校验。它是问题 2 的格式校验种子。
- `core/configsync.Puller[T]`（PROBE-004 先例）：版本化拉取循环，`WithOnError`
  （失败保留 last-good）、`WithOnApply`、`PullOnce`、`LastVersion` 跳过未变版本。
  坏版本绝不覆盖运行态。
- 体积先例：`OpsConfig.MaxPushFileBytes`（缺省 **1 MiB**）——push-file 面既有体积闸。
- 原子性+回滚先例：§10.4 `UpdateProcess`（hash pin → 停 → 备份 → 换 → 起 →
  稳定窗 → 回滚；事件闭集；execlog 审计 `source=process_update`）。
- 授权先例：§10.7 三件套（双开关 + allowlist fail-closed + 每次尝试都写 execlog 审计）。

## 2. 五问（提案）

### Q1 — 路径沙箱

配置内容以**版本化配置载荷**传输（服务端 `config_versions`，新键如
`ops.managed_processes`），绝不是任意文件路径——agent 把收到的内容写进
agent 自有的配置目录，不接受 server 指定路径。约束：

- 物化文件只落在 agent 的受管配置目录（与 `SupervisorLog`/`SnapshotDir` 缺省
  同一信任根：相对 agent 工作目录）。进程 working-dir 字段里的绝对路径或 `..`
  逃逸属调用方自有语义，**不**在本面范围内。
- 未来若需引用 staged 二进制，那是 D1 的 staged 中继领地
  （agent-self-upgrade-review §2），不是配置更新。

### Q2 — 格式校验

应用闸 = 动任何东西之前对**候选配置整体**跑 `ManagedProcessConfig.Validate()`，
外加拉取载荷必须满足的结构检查（进程名闭集、未知字段拒绝）。校验失败的候选
丢弃并留错误记录；**last-good 配置继续运行**（configsync 先例）。校验失败
绝不重启进程。

### Q3 — 原子性、重启与回滚

1. 候选 vs 运行配置求 diff → **受影响集** = 有效配置变化的进程
   （新增/移除/command、args、env、schedule、autorestart 变化）。
2. 对每个受影响进程：停（显式动作漏斗，§10.6 语义——desired 状态与 supervisor
   事件照常流动）→ 配置换装（配置目录内原子 rename）→ 起。
3. 稳定性窗口（§10.4 缺省 5s）：窗口内退出 = 受影响进程回滚到 last-good 配置
   并重启。
4. 未受影响进程不受打扰继续运行（无重启风暴）。
5. 事件：supervisor 事件闭集新增 `config_applied` / `config_rolled_back`
   （消息带旧→新配置版本）。execlog 审计 `source=process_config_update`
   是持久查询主入口。
6. desired 语义保留：手动停掉的进程（§10.3）在配置应用后保持停止——新配置
   已物化但不启动（启动需显式运行意图，同 §10.4）。

### Q4 — 体积与带宽

- 拉取式（agent 按既有 configsync 节奏主动拉）：无推送风暴，带宽上界 =
  每次变更每 agent 一个版本化载荷。
- 载荷上限：复用 `MaxPushFileBytes` 闸值（缺省 1 MiB）作为该键
  `config_versions` 载荷天花板——受管程序配置预期远小于此；上限的意义是让滥用
  不可能，而非调优预算。
- `LastVersion` 跳过未变版本（configsync）杜绝无谓 apply 循环。

### Q5 — 审批闸

- agent 侧新增独立 ops 闸（如 `allowProcessConfigUpdate`），缺省 false，
  属 §10.7 双开关族。配置更新 ≠ 任意命令执行许可（§10.7 硬边界继续成立）。
- server 侧：签发端点落入既有 ops REST 族与权限面；**高危双人审批可复用**
  （既有审批 workflow 族）——配置签发是否需要第二审批人属 owner 子决策（D5）。
- 每次尝试（成功/校验失败/agent 不可达）都写 execlog 审计——§10.7 三件套原样适用。

## 3. 验收映射（设计 → 验证场景）

| #   | 场景                                             | 期望                                                           |
| --- | ------------------------------------------------ | -------------------------------------------------------------- |
| A1  | 候选配置未过 `Validate()`                        | last-good 继续运行；留错误记录；任何进程都不重启               |
| A2  | N-of-M 变化进程应用成功                          | 只重启受影响集；未受影响进程 uptime 不中断                     |
| A3  | 新配置进程在稳定窗内退出                         | 回滚 last-good 配置并重启；`config_rolled_back` 事件 + execlog |
| A4  | 手动停止的进程收到配置变更                       | 配置已物化、进程保持停止（desired 状态保留）                   |
| A5  | 载荷超体积上限                                   | 应用前拒绝；保留 last-good；留审计记录                         |
| A6  | agent 闸关闭（`allowProcessConfigUpdate=false`） | server 签发失败；execlog 记录拒绝                              |
| A7  | 仅 schedule 变化                                 | 重编译调度表生效且不重启进程；`next_schedule_at` 刷新          |

## 4. 待 owner 决策

- [ ] **D1** 载荷通道——批准 `config_versions` 拉取（键 `ops.managed_processes`）为分发通道
- [ ] **D2** 校验严格度——批准 apply 前整体 `Validate()` + last-good 保留（含未知字段拒绝）
- [ ] **D3** 原子性形态——批准受影响集重启 + 稳定窗回滚（不整批重启）
- [ ] **D4** 体积上限——批准与 `MaxPushFileBytes` 对齐的天花板（缺省 1 MiB）
- [ ] **D5** 审批强度——配置签发是单操作员（闸+审计即可）还是双人（复用审批 workflow）？
- [ ] 验收映射 §3（A1–A7）——确认其为实施的验证基准
