---
title: agent-core 通用能力层设计简档——能力清单/目录树/抽取批次（已实施）
---

# agent-core 通用能力层设计简档

## 状态

- 状态: **已实施（K1–K4 抽取完成，2026-10-09）**。core 十包建成并自带测试，sidecar-agent 六包已接线、四包建成待消费方接线，接线状态与剩余项见 §5.1/§9。
- 立项口径（用户令，2026-10-09）：agent 越来越多（capture/未来更多），通用能力上收为 `agent-core` Go 公共库；**各 agent = agent-core + 业务插件**；与 [Agent Supervisor 简档](agent-supervisor-design.md) 合并定稿——**core 即 supervisor 的被监管对象，协议一份**；目录 `core/` 归 core、`agents/capture/` 归业务；极简：core 每文件守百行内、能力按需可裁（不用的不编译进来）。
- 补充令（2026-10-09，内网 agent 拆分）：泛监控类能力从主 agent 拆成独立小 agent，**「一个能力一个小 agent」**；首个样例 = **CI/CD 状态监控 agent**（`agents/devops/`，设计见 §7）——与 capture-agent 同构，用第二个业务 agent 验证 core 可裁剪抽象是否成立。
- 定稿补充（2026-10-09，用户令④）：①服务探活升为 core 独立公共模块 **`healthprobe`**——内分 Liveness/Readiness/Heartbeat 三语义（对标 k8s probe 术语），游戏进程探活/内网探 CI 端点/capture 探库连通三处**共用同一份实现**；②路线图登记两个未来独立 agent，**命名用行业词**：`connector-agent`（第三方服务接入，对标 Datadog Integrations/n8n）、`synthetics-agent`（外部合成探测，对标 Datadog Synthetics/Pingdom）——**只登记不实现**（§8）；③三 agent 定稿不变：sidecar-agent=游戏服务器 GM 通道、devops-agent=内网 CI/CD/打包/部署、capture-agent=防私改库。
- 审计令（2026-10-09，用户令）：每个 agent 的每次执行都留审计记录——core 新增统一 **`execlog`** 公共模块（与 healthprobe 同级），含 audit-first 放行闸、面板执行记录页（§6.2）。
- 建模令与报警出口（2026-10-09，用户令）：①healthprobe **按 system 角色建模**——不只看当下活死，记录**故障窗口**（开始/恢复/持续时长）历史可查，探活数据带 timeline（§6.1）；②告警出口对接 **herald**（用户自研通知编排与投递平台）——capture 告警/healthprobe 故障窗口/devops 构建失败经 herald 投递，croupier 当接入方（courier 式接法，渠道/收件人由 herald 侧配置），接法见 [herald 对接简档](agent-herald-integration.md)。
- 定名令（2026-10-09，用户拍板）：三 agent 定名 **sidecar-agent / devops-agent / capture-agent**，旧候选（gameserver-agent/cicd-agent/cdc-agent/ci-agent）弃用；权威口径见 [Agent 清单](agents-inventory.md)（目录：全景 → 三 agent 作用 → 公共模块 → 路线图占位 → scope 归属 → 面板分页）。
- 底稿: [Agent 能力成熟库盘点矩阵](../research/agent-capability-library-matrix.md)（16 项换/留/补已拍板，A–F 批已全部落地——core 的库选型全部沿用该拍板，不重新选型）
- 关联: [Agent 清单](agents-inventory.md)（定名/职责边界/scope 归属权威页）· [Agent 面板设计](agent-dashboard-design.md)（每类 agent 一页）· [herald 对接简档](agent-herald-integration.md)（告警投递出口）· [capture-agent 简档](capture-agent-design.md)（业务 agent 之一）· [Provider 插件设计](provider-plugin-design.md)（可插拔原则同源）· [崩溃快照调研](../research/crash-capture-survey-2026-10.md)（core/crash 的分级方案依据）

## 1. 定位

**core/ 不是框架，是零件箱。** 不提供 god-object（无 `core.App` 大装配），每个能力是独立小包，main 按需 import——没 import 的能力不参与编译，天然满足「按需可裁」。主 croupier agent（internal/agent + internal/app/agent）即 **sidecar-agent**（定名见 [Agent 清单](agents-inventory.md) §2，作用：游戏服务器旁的 GM 基础设施代理——进程监管 + 函数注册调用 + 自探活），是 core 的**第一个消费方**（抽取=原地切换 import，行为零变化），capture-agent 是第二个，devops-agent（§7）是第三个——数据面样例与泛监控样例各验证一次裁剪面。

**「一个能力一个小 agent」原则（内网拆分令）**：泛监控/旁路观测类能力不再长进主 agent——sidecar-agent 收敛为「函数注册调用 + 进程监管」的 GM 基础设施代理；每个监控域（数据变更合规、CI/CD 健康、未来更多）独立小 agent，崩溃域/攻击面/配置面三者天然隔离，sidecar-agent 挂了不连坐监控，监控挂了不拖累 GM 通道。

三条不变量：

1. **协议一份**：所有 wire 类型（注册/心跳/指标/监管快照与事件/告警）唯一来源仍是 `proto/`（ops.proto / sdk proto），core 只封装编解码与通道，不私定协议。
2. **监管契约一份**：supervisor 管「被监管对象」的契约（信号/退出码/心跳上下文）在 core 定义，supervisor（sidecar-agent 的 ops 模块）与一切 core 系 agent 共用；`SupervisorEvent`/`ManagedProcess` 等 wire 结构不复制第二份。
3. **库选型不翻案**：backoff=cenkalti、限流=x/time/rate、日志=slog+lumberjack、传输=自研 TCP（产品契约）——矩阵拍板为准，core 只收拢用法。

## 2. 能力清单表（每项：已有/新建/来源）

| #   | 能力                           | 状态                         | 来源与现状                                                                                                                                                                                                       | core 包（拟）       |
| --- | ------------------------------ | ---------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------- |
| 1   | 重试退避                       | **已有**                     | cenkalti/backoff/v5（矩阵 A 批 a9d250e 已落地三处）；core 只做统一旋钮薄封装，杜绝再出现「三套写法」                                                                                                             | `core/backoff`      |
| 2   | 结构化日志                     | **已有**                     | slog + lumberjack（`internal/cli/common/logging.go`，矩阵 #11「留」）；原样上收                                                                                                                                  | `core/logx`         |
| 3   | 上报协议（统一上行）           | **部分**                     | MetricsReport/指标捎带已有（ops_metrics.go）；**告警通道新建**（capture 简档 §7，wire 加告警上报消息）                                                                                                           | `core/report`       |
| 4   | 注册上线                       | **已有**                     | `internal/agent` upstream 注册链 + game_env 校验；抽取时 local_handler 留 sidecar-agent，注册/重注册上收                                                                                                         | `core/register`     |
| 5   | 心跳与存活                     | **已有**                     | upstream.go 心跳 + 重连退避；与 #1 合并收拢                                                                                                                                                                      | `core/register`     |
| 6   | 进程监管·被监管契约            | **协议已有，契约侧新建**     | S1/S2 的 `SupervisorEvent`/`ManagedProcess`（ops.proto）不动；新建「被监管对象」运行时契约（§3）                                                                                                                 | `core/supervisable` |
| 7   | 指标采集                       | **已有**                     | gopsutil v4 MetricsCollector；抽为可独立上报的采集器（capture 无函数通道也能报指标）                                                                                                                             | `core/metrics`      |
| 8   | 配置拉取与热更                 | **部分**                     | extension sync puller 先例（internal/app/agent/extension_sync_puller.go）；泛化为「版本号轮询→拉取→热生效」通用件                                                                                                | `core/configsync`   |
| 9   | 崩溃采集                       | **新建**                     | crash-capture-survey 分级方案已批（轻档公共件已落 core/crash：snapshot 档位注入 + dump 目录管理；supervisor 接线随 #67 S3；Crashpad 否决不立项）                                                                 | `core/crash`        |
| 10  | 令牌桶限流                     | **已有**                     | x/time/rate（矩阵 B 批）；**不搬**——留 internal/platform/ratelimit 原地，core 系按需直接引包（避免双份）                                                                                                         | （不设包）          |
| 11  | 任务执行（async/幂等/取消）    | **已有，可裁**               | internal/agent jobs；函数通道系能力，capture 无任务面不 import                                                                                                                                                   | （暂不上收）        |
| 12  | 本地网关（provider 注册/调用） | **已有，可裁**               | internal/agent tcp_local_listener + local_handler；sidecar-agent 专属，core 不含                                                                                                                                 | （不上收）          |
| 13  | CI/CD 状态监控（业务能力）     | **新建**                     | 内网拆分令首样例（§7，devops-agent）：GitHub Actions/自建 runner 构建 健康 → 告警页；属业务插件不进 core，列此仅为能力面全景                                                                                     | `agents/devops/`    |
| 14  | 服务探活（healthprobe）        | **新建，公共模块首批交付物** | 三语义对标 k8s probe 术语：Liveness（存活）/ Readiness（依赖就绪）/ Heartbeat（周期心跳打点）；sidecar-agent 游戏进程探活、devops-agent 探 CI 端点、capture-agent 探库连通**共用同一份实现**，不各写各的（§6.1） | `core/healthprobe`  |
| 15  | 执行审计（execlog）            | **新建，公共模块**           | 每个 agent 每次执行留审计记录（operator/agent/入参/结果/耗时/scope/任务 id），命令类执行 audit-first 放行；与 healthprobe 同级三 agent 共用（§6.2）                                                              | `core/execlog`      |

> 上收边界：#11/#12 是「函数注册调用」业务面的骨架，不是通用 agent 的公共件——**留在 sidecar-agent 不进 core**，这正是「可裁」的第一次应用（core 系 agent 天然没有这两块攻击面）。

## 3. 被监管对象契约（与 supervisor 合并定稿的部分）

supervisor（监管方，sidecar-agent 的 ops 模块）已定稿于 #67；本节补齐**被监管方**一侧，协议复用 ops.proto 不新增：

- **信号纪律**：core 系 agent 收 SIGTERM/SIGINT 后 `gracefulTimeout` 内落位点/刷缓冲退出，退出码 0=主动停（supervisor 置 STOPPED 不拉起）、非 0=异常退出（进 S2 退避/熔断状态机）——与 supervisor 的 `exitDetail` 语义对齐。
- **心跳上下文**：core 系 agent 周期写本地心跳（文件 mtime 或本地 ops 端点），供监管方 `last_heartbeat_unix` 事件上下文采样；sidecar-agent 的 supervisor 采样器对 core 系进程直接可用。
- **进程声明**：core 提供 `supervisable.Main(name, run)` 薄入口——统一信号安装/退出码/心跳打点，业务 main 只写 `run(ctx)`。
- 由此，**任何一个 core 系 agent 出厂即是合格的被监管对象**：配置进 sidecar-agent `managedProcesses` 即获得 S1/S2 全部监管能力（快照列/退避拉起/熔断/事件环/面板操作），零额外代码。

## 4. 目录树（拟）

```
core/                        # agent-core（同仓 Go module，包级裁剪）
  backoff/                   #   统一退避旋钮（薄封装，~60 行）
  logx/                      #   slog+lumberjack 装配
  report/                    #   统一上行：指标/事件/告警（通道复用，告警消息新增）
  register/                  #   注册上线 + 心跳存活 + 重连（上收自 internal/agent）
  supervisable/              #   被监管对象契约（信号/退出码/心跳打点）
  metrics/                   #   gopsutil 采集器（可独立运行）
  healthprobe/               #   Liveness/Readiness/Heartbeat 三语义探针（三 agent 共用，首批交付，§6.1）
  execlog/                   #   统一执行审计（audit-first 放行闸，三 agent 共用，§6.2）
  configsync/                #   版本轮询拉取 + 热生效（泛化自 extension sync puller）
  crash/                     #   崩溃采集轻档（snapshot 档位注入 + dump 目录管理；supervisor 接线随 #67 S3；Crashpad 不立项）
agents/
  capture/                   # capture-agent（业务插件，防私改库，见 capture 简档）
    cmd/capture-agent/       #   main：core 零件装配 + source/gate 接线
    source/                  #   mysql.go / postgres.go（ChangeSource Provider）
    gate/                    #   reconcile.go / cep.go / lineage.go
    rule/                    #   规则模板类型 + 求值（纯数据规则）
  devops/                    # devops-agent（业务插件，泛监控首样例，见 §7）
    cmd/devops-agent/        #   main：core 装配（register+logx+report+execlog+configsync，无 metrics/gate）
    source/                  #   github_actions.go（CIBuildSource Provider；jenkins/gitlab-ci 留扩展位）
    rule/                    #   构建健康规则模板（失败/超时/持续不绿/停滞）
internal/                    # 现有主体不动（sidecar-agent = internal 骨架 + core 零件）
```

- 不建独立 Go module（单 module 内目录隔离已够裁剪；独立 module 换来的依赖隔离在单仓单人节奏下是纯摩擦）。
- core 各包互不横向依赖（backoff/logx 等叶包除外），依赖方向：`agents/* → core/* → proto/`，**core 不 import internal/**——上收=搬语义不搬依赖，防止 core 被 sidecar-agent 业务面倒灌。

## 5. 抽取批次（每批独立 PR，CI 绿进下批）

| 批  | 内容                                                                                                                                                                             | 风险 | 测试策略                                                                   | 状态 |
| --- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---- | -------------------------------------------------------------------------- | ---- |
| K1  | core/ 骨架 + `backoff`/`logx` 薄壳上收（sidecar-agent 原地切 import，行为零变化）+ `healthprobe` 三语义探针与故障窗口时间线（公共模块首批交付物，supervisor 心跳采样先切换复用） | 低   | 现有用例随迁 + 行为对拍；探针语义正反用例；窗口开合/恢复时长断言           | ✅   |
| K2  | `register` 上收（注册/心跳/重连）+ `supervisable` 契约 + main 薄入口                                                                                                             | 中   | 注册链既有用例随迁；core 系 dummy agent 集成用例                           | ✅   |
| K3  | `report`（含告警通道 wire 新增）+ `metrics` + `execlog` 上收（执行审计双写与 audit-first 闸）；capture 简档 C1 就绪                                                              | 中   | 上行协议正反用例；捎带增量游标复用 S2 用例形态；审计先落盘后放行的顺序用例 | ✅   |
| K4  | `configsync` 泛化（extension puller 切换到通用件）+ `crash`（#67 S3 同批落地）                                                                                                   | 中   | 拉取热更用例；崩溃采集按调研分级用例                                       | ✅   |

- 与 capture 分期编排：capture C1 依赖 K1+K2（+K3 的告警/审计前置），C2 起 K3/K4 并行推进；devops-agent（§7）同样只依赖 K1–K3，与 capture C2 之后任一时段并行立项均可。
- **回退策略**：每批 sidecar-agent 仍从 internal 路径可编译（上收=内部实现切换 import），任一批叫停不留半成品依赖。

### 5.1 落地进度（2026-10-09，K1–K4 抽取完成）

core 十包全部建成且自带测试（`go test ./core/...` 绿）；sidecar-agent 接线状态分两档：

| 包             | sidecar 接线 | 说明                                                                                                                     |
| -------------- | ------------ | ------------------------------------------------------------------------------------------------------------------------ |
| `backoff`      | ✅ 已切      | 默认档断言迁 sidecar 防漂移（backoff_regression_test）                                                                   |
| `logx`         | ✅ 已切      | `internal/cli/common/logging.go` 收敛为 shim，新代码直用 core/logx                                                       |
| `register`     | ✅ 已切      | upstream.go 注册/心跳/重连全走 core；保语义：payload 先组装后取连接快照、断连 not connected 文案、OnConnected 初始双触发 |
| `report`       | ✅ 已切      | Conn 含 Connected() 短路；**告警通道 wire 未新增**（§9 边界，capture C1 前置）                                           |
| `metrics`      | ✅ 已切      | sidecar 留别名 + GetSystemInfo（OpsConfig 属 sidecar 面）；Sampler 注入受管进程快照                                      |
| `configsync`   | ✅ 已切      | extension sync puller 委托 core 版本轮询；wire 解码留业务侧                                                              |
| `healthprobe`  | 建成未接线   | supervisor 心跳采样切换与探针消费随 capture C1 / 面板批次                                                                |
| `supervisable` | 建成未接线   | sidecar main 继续用现有装配；core 系 agent（capture/devops）落地时即用                                                   |
| `execlog`      | 建成未接线   | 本地真值闭环（双写/闸/导出）；server 摄取端点与 sidecar 执行面接线属后续批次                                             |
| `crash`        | 建成未接线   | 轻档注入映射 + dump 目录管理就绪；ManagedProcesses `snapshotProfile` 接线随 #67 S3                                       |

## 6. 公共模块：healthprobe 与 execlog

三 agent 共用的两块同级公共件（不各写各的），全景与消费映射见 [Agent 清单](agents-inventory.md) §5。

### 6.1 healthprobe（健康探针，首批交付物）

三语义对标 k8s probe 术语（定稿补充④）：

| 语义      | 判什么                       | 消费映射（三 agent）                                                            |
| --------- | ---------------------------- | ------------------------------------------------------------------------------- |
| Liveness  | 自己活没活                   | sidecar-agent：游戏进程存活（与 supervisor 采样同源）                           |
| Readiness | 依赖就绪没就绪               | devops-agent：CI 端点可达；capture-agent：库连通/复制流健康                     |
| Heartbeat | 周期心跳打点（最近活体时刻） | 三 agent 通用：心跳文件/上报时间戳，supervisor 事件上下文与面板「最后上报」共用 |

- 内置 checker：HTTP / TCP / process 三种 + 失败阈值（连续 N 次失败才算不就绪），core 包零外部依赖。
- 一个能力一份实现：业务 agent 只声明「探什么」，探针语义/阈值/打点一律复用本包；探针结果直接上面板（实例状态灯 + liveness/readiness 结论 + 心跳时长）。
- **按 system 角色建模（故障窗口，用户令 2026-10-09）**：探针对象 = 受监控系统（target），状态不是单点布尔而是带时间线的两态机——连续失败达阈值即**故障窗口开始**（unavailable，记 start），恢复达阈值即**窗口关闭**（recovered，记 end 与持续时长）。窗口记录 `{target, semantics, start_ts, end_ts(未恢复=空), duration, last_reason, game_id, env}`：
  - 双通道留存：agent 本地时间线文件（真值）+ 上报捎带（快照带当前窗口，窗口开/合作为事件上行），server 侧历史可查——排查时直接回答「**什么时间段服务不可用**」；
  - 探活数据带 timeline：面板探针卡片展示历史故障窗口列表（未恢复窗口标「进行中」），见 [Agent 面板设计](agent-dashboard-design.md)；
  - 出口：窗口开始/恢复可经 herald 投递（`probe.unavailable` / `probe.recovered`，dedup 状态折叠与窗口二态天然对齐），见 [herald 对接简档](agent-herald-integration.md)。

### 6.2 execlog（统一执行审计）

**每个 agent 的每次执行都留审计记录**（用户令，2026-10-09）。记录单元 = 一次执行（execution）：

| 字段           | 语义                                                                    |
| -------------- | ----------------------------------------------------------------------- |
| operator       | 执行人：GM 账号 / system（系统触发）/ scheduler（定时任务），含来源标识 |
| agent_type     | sidecar / devops / capture                                              |
| agent_instance | 实例（注册 id）                                                         |
| action         | 执行的动作名（闭集，随 agent 扩展）                                     |
| params         | 入参全文（敏感字段沿用现有脱敏规则）                                    |
| result         | success / failure + 摘要（失败含错误摘要）                              |
| duration_ms    | 耗时                                                                    |
| ts_unix_ms     | 时间戳                                                                  |
| game_id + env  | scope（强制，与注册标签一致）                                           |
| task_id        | 任务/命令 id（关联既有 idempotency-key 体系）                           |

- **双写**：本地轮转文件（真值，离线不丢，lumberjack 同族）+ 上报 server 落审计存储（沿用审计链完整性机制，按 scope 归 game 库）。
- **audit-first 放行闸**：命令类执行（写操作：未来 devops 若开部署动作、capture 白名单/豁免变更）必须**先落审计记录才放行执行**——记录未持久化即拒绝执行；观测类轮询不设闸（性能）。
- **保存期与导出**：配置位（保留天数/导出目录/格式），默认值随实施报批。
- **面板**：各 agent 页「执行记录」列表 + 筛选（执行人/结果/时间段），详情下钻看入参/结果全文，全按 scope 过滤（[Agent 清单](agents-inventory.md) §8）。

## 7. CI/CD 状态监控 agent（内网拆分首样例）

三 agent 定稿口径：**sidecar-agent**=游戏服务器 GM 通道、**devops-agent**=内网 CI/CD/打包/部署、**capture-agent**=防私改库（权威边界见 [Agent 清单](agents-inventory.md) §2–§4）。本节是 devops-agent 的设计。

### 7.1 职责（单一）

盯内网 CI/CD/打包/部署链路状态，命中异常上报 croupier 告警页；v1 落地面=GitHub Actions / 自建 runner 的构建状态（打包产物/部署节点状态属后续扩展位，见 7.3 边界）。只做观测与告警，**不做触发构建、不改 CI 配置、不执行部署**（写操作永远是人进 GitHub/Jenkins 面板）。

### 7.2 部署与上线

- 跑在**内网、靠近 runner** 的主机上（能访问 GitHub API / CI 内网端点即可，不需要公网入站）。
- 经 agent-core 注册链上线（K2 交付后）：注册/心跳/日志/上报全部复用 core；进程由 sidecar-agent 的 supervisor 托管（`managedProcesses` 一条配置，崩溃拉起/熔断/事件环全白得——被监管对象契约 §3 的直接受益）。
- 与 capture-agent 完全同构：`agents/devops/` 只有 source/rule/main，验证第二个业务 agent 落地的边际成本≈纯业务代码。
- 内网 CI 端点/runner 可达性探测复用 `core/healthprobe`（Readiness 语义：HTTP/TCP 探测 + 失败阈值 + 故障窗口时间线），不自建探针——与游戏进程探活（Liveness）、capture 探库连通（Readiness）共用同一份实现（定稿补充④）。

### 7.3 数据源（可插拔，同 capture 的 Provider 形态）

```go
type CIBuildSource interface {
    Watch(ctx, WatchConfig) (buildCh <-chan BuildEvent, err)   // 轮询游标
    Close(ctx)
}
// BuildEvent: repo/workflow/branch/run_id/status(conclusion)/duration/
//             started_at/runner_name/url
```

| 源                            | 机制                                             | 成熟度 | 边界                                                                                            |
| ----------------------------- | ------------------------------------------------ | ------ | ----------------------------------------------------------------------------------------------- |
| GitHub Actions                | REST `/actions/runs` 轮询（gh token 走面板配置） | **高** | v1 只轮询不 webhook（内网无公网入站，诚实边界）；API 限速按 token 配额自查                      |
| 自建 runner（GH self-hosted） | 同上（runner 名字段区分）                        | 高     | 无额外成本                                                                                      |
| Jenkins / GitLab CI           | 留扩展位                                         | —      | 接口已留，按需补 Provider                                                                       |
| 打包产物/部署节点状态         | 留扩展位                                         | —      | 定稿口径「CI/CD/打包/部署」的完整面；v1 只交付构建状态，扩展时同样走 source Provider，不改 core |

### 7.4 规则（参数化模板，与 capture 同一「无 DSL」原则）

| 模板               | 参数                          | 告警     |
| ------------------ | ----------------------------- | -------- |
| build_failed       | 仓库/工作流/分支白名单        | 失败即报 |
| build_timeout      | 时长阈值（按 workflow 配置）  | 超时即报 |
| stale_green        | 主分支「最后绿灯」超过 N 小时 | 停滞告警 |
| consecutive_failed | 连续失败 M 次（抖动噪声抑制） | 达阈值报 |

白名单/阈值全部走面板配置（与 capture 同一条 configsync 通道），不写死。

### 7.5 裁剪面验证点（本样例的存在意义）

devops-agent 不 import `core/metrics`（无资源采集需求时的可裁证明）、不引 capture 的 gate/rule 包、无本地网关——如果这三个「没有」在编译期与上线后都成立，core 的按需裁剪抽象即被第二个样例确认；反之在 K 批内修 core 边界，成本最低的时刻。

## 8. 路线图登记（只登记不实现）

两个未来独立 agent 的**命名已定稿（行业词）**，此处仅登记占位、不立项不写码——出现真实需求时按「一个能力一个小 agent」另开简档报批：

| Agent              | 定位                                                                                             | 对标（命名来源）             | 立项触发条件                                                          |
| ------------------ | ------------------------------------------------------------------------------------------------ | ---------------------------- | --------------------------------------------------------------------- |
| `connector-agent`  | 第三方服务接入——外部系统（云厂商健康、工单/IM、支付通道等）的状态与事件汇入 croupier 告警/观测面 | Datadog Integrations / n8n   | 出现 CI 之外的真实接入源需求                                          |
| `synthetics-agent` | 外部合成探测——从外网/多点位主动拨测线上端点可用性                                                | Datadog Synthetics / Pingdom | 需要公网视角的拨测面（内网 devops-agent 的 Readiness 探不到外网视角） |

两者届时与 devops-agent 完全同构：source Provider + 规则模板 + core 装配，探活面复用 `core/healthprobe`，协议与被监管契约零新增。

## 9. 已知边界（诚实清单）

- core/ 与 internal/ 短期并存双路径（K1-K4 过渡期），以批次消灭，不做一次性大迁移（历史教训：19-sync 类协议面事故都出自「一把梭」迁移）。K1–K4 抽取已完成（§5.1），双路径余量收敛到四个「建成未接线」包。
- **report 告警通道 wire 未新增**：core/report 现只承载指标/任务事件（与 sidecar 现状等价）；告警上报消息（capture 简档 §7）与 herald 出口对接未做，是 capture C1 的前置项。
- **execlog 仅本地真值闭环**：双写中的 server 摄取端点（落审计存储、按 scope 归 game 库）未做，Uploader 为接口位；sidecar 执行面（函数调用/任务执行留审计）接线属后续批次；本地落盘为 lumberjack 无缓冲直写——进程崩溃不丢，断电级 fsync 不保证。
- **healthprobe 上行未接**：故障窗口时间线本地文件闭环；上报捎带（快照带当前窗口、窗口事件上行）与面板消费属后续批次；herald 投递（probe.unavailable/recovered）未接。
- **crash 接线拍板已清、代码随 #67 S3**：ManagedProcesses `snapshotProfile` 字段已拍板随 S3 落地（2026-10-10）；中档 core_pattern 宿主机操作边界已拍板=agent 只读提示（扫描现状+显示建议命令，不直接写宿主配置，调研 §8.3）；保留期/配额/下载权限（调研 §8.4）仍待拍板。
- supervisable 未被 sidecar main 使用（sidecar 有既有装配）；首个消费者是 core 系 agent 的 cmd/main。
- #11/#12 不进 core 意味着 core 系 agent **没有函数注册调用能力**——这是定位而非缺陷；若未来某业务 agent 需要函数面，届时再评估以插件位接入（与 #66 Provider 插件设计对齐），不预先上收。
- CI/CD 监控 v1 只轮询不 webhook（内网无公网入站），告警时延下界=轮询间隔；GH API 限速随 token 配额，watch 清单过大时需分片。
- 「每文件百行内」对 metrics/configsync 这类装配密集包可能破线：破线需在 PR 说明拆分层（装配与逻辑分文件），不当死数字硬拆。
- Windows 支持面随 sidecar-agent 现状（supervisor 已有 _windows 契合点）；core 各包新增平台特定代码时同规则拆构建标签文件。
