---
title: Agent 公共运行时与目录边界
---

# Agent 公共运行时与目录边界

2026-10-20 按 `aa75e64` 及 todo:17 第一片落地核查。本文区分当前源码位置、已抽取能力和待迁移能力；配套用户入口为 [Agent 职能与选型](../operations/agent-comparison.md)，分类与后台设计见 [Agent 清单](../design/agents-inventory.md)、[四类后台页面](../design/agent-dashboard-design.md)。

## 当前真实目录

```text
cmd/
  server/                     Server CLI
  sidecar-agent/              sidecar CLI，产物仍名 croupier-agent
  devops-agent/               devops CLI
  capture-agent/              capture CLI
  synthetics-agent/           synthetics CLI（HTTP 探测与外部探测插件）
core/                         可组合的 Agent 公共运行时
  transport/                  Handler/Client/Server 公共契约
    tcp/                      TCP/TLS 客户端、framing、MuxConn、服务端 TLS 配置
  register/                   注册、心跳、重连、owner 身份
  agentscope/                 注册身份（id/serverAddr/gameId/env）空白拒绝公共规则
  backoff/ logx/              通用退避与日志装配
  configsync/                 版本化拉取与 last-good 应用
  supervisable/               进程信号、退出码、本地心跳契约
  healthprobe/                探针、故障窗口与协议步骤
  report/ metrics/            上报适配与系统指标采样
  execlog/ crash/             执行审计与崩溃辅助
  process/                    五字段 cron、托管进程调度与重启生命周期决策
  pluginhost/                 go-plugin 进程、限流、退避、熔断与退出唯一管理者
agents/
  controlconn/                devops/capture/synthetics 共用控制消息适配
  sidecar/
    provider_session.go       SDK Provider 会话、索引与连接关闭（本片归位）
    provider_keepalive.go     Provider 会话传输层保活探针（本片归位）
    task_runner.go            在途任务生命周期（执行 seam、取消、事件上报；本片归位）
    funcplugin/               函数插件声明、协议与宿主适配
  devops/{config,runner,session}/
  capture/{config,runner,connector,source,filter,gate}/
  synthetics/{config,runner,probe,pluginhost}/
internal/
  app/agent/                  sidecar 装配、监管、Provider、探针（迁移余量）
  agent/                      SDK 本地网关与 handler（迁移余量）；Provider session/保活/任务转发仅别名
  transport/tcp/              请求响应 Server；client/framing/MuxConn/TLS 配置转发 core
pkg/{pb,protocol}/           共享 wire message 与消息编号
```

四类 Agent 均已有根 `cmd/` 入口；synthetics 的 HTTP 探测、持久结果闭环与 go-plugin 适配，以及 sidecar 函数插件适配，已在本片前实现。根目录 `cmd/` 是唯一 Agent 二进制入口，不能再在 `agents/<role>/cmd` 建第二套入口；角色目录内的插件示例是独立插件进程示例。

## 公共能力与业务边界

| 能力                           | 当前公共实现                                                  | 当前消费/边界                                                                                                                                         | 后续收敛                                                                      |
| ------------------------------ | ------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------- |
| framing、TCP/TLS               | `core/transport/tcp`                                          | internal 通过别名转发；Server TLS 监听与 sidecar 本地网关共用 core TLS 构造，standalone 通过 controlconn 消费                                         | 同一实现，不复制 TLS/帧格式                                                   |
| 双向 session/多路复用/派发背压 | `core/transport/tcp.MuxConn`                                  | sidecar、SDK Provider、Server 派发与集群客户端已直接消费 core；devops/capture/synthetics 经 controlconn 使用串行 Client                               | 实现早已上收；本片收敛真实消费者，保留 internal 兼容 shim；不改变角色收发模型 |
| SDK Provider 会话/索引/保活    | `agents/sidecar/provider_session.go`、`provider_keepalive.go` | 本片从 internal/agent 归位；sidecar 装配直接消费，旧路径为类型别名和构造转发                                                                          | 本地网关与注册 handler 仍待后续按职责迁移，不进入 core                        |
| 注册、心跳与重连               | `core/register`                                               | 四类已消费；注册 payload 由各角色组装                                                                                                                 | 生命周期 race 已入 CI；继续收敛真实断网恢复验收                               |
| 显式 game/env 归属             | `core/agentscope`                                             | 四类空白拒绝共用单一规则（去空白即拒、按声明顺序报第一个缺项）；各入口保留自己的报错文案与调用时机；sidecar scope 校验只查 game/env 且保留中文文案    | 已集中抽取；报错文案与调用时机留入口侧，四类一致性测试维持同口径              |
| 上行消息适配                   | `agents/controlconn`、`core/report`                           | controlconn 共用 Register/Heartbeat/capture 消息与 synthetics 探测上报，非全部角色统一 session 客户端                                                 | session 稳定后抽通用收发，业务消息 handler 留角色侧                           |
| 信号与被监管契约               | `core/supervisable`                                           | devops/capture/synthetics 使用；sidecar CLI 有自己的退出装配                                                                                          | 统一错误传播与清理，不把拥有 main 等同可被正确监管                            |
| 探测内核                       | `core/healthprobe`                                            | sidecar/Server 装配较完整，capture 有 readiness；synthetics 当前原生 HTTP probe 仍在 agents/synthetics/probe，未消费 healthprobe；devops 探测消费未齐 | 后续核对探测内核复用；外部目标分类与插件协议适配留 synthetics                 |
| configsync                     | `core/configsync`                                             | 拉取/版本/last-good 为公共件，具体配置解析留业务侧                                                                                                    | 不同 Agent 不各写一份拉取状态机                                               |
| 指标/上报/审计/快照            | `core/metrics`、`report`、`execlog`、`crash`                  | 按需组合，公共包存在不等于所有角色都已接线                                                                                                            | 以真实消费与端到端证据标注状态                                                |
| 受管程序 supervisor            | `core/process`、`internal/app/agent/ops_*`                    | cron/调度、重启退避、熔断、退出分类、OOM 线索决策与失败计数/退避/恢复状态机公共化；Ops 事件日志、进程装配与授权仍留在 sidecar                         | 每进程只有一个管理者；探测窗口、spawn 失败与稳定性窗口口径由 Ops 馈入状态机   |
| go-plugin 进程生命周期         | `core/pluginhost`（本片前已抽取）                             | sidecar/funcplugin 与 synthetics/pluginhost 已共用启动、背压、退避、熔断、Stop；角色分别保留函数/探测协议                                             | 保持一个 Controller 管理一组插件进程，不并入受管程序 supervisor               |
| OTel                           | `internal/telemetry`                                          | 已有默认 SDK/禁用开关与 no_otel 裁剪；插件另有 no_go_plugin 裁剪                                                                                      | 本片不移动遥测，避免依赖灌入公共 Checker 或游戏 SDK                           |

SDK 函数注册与路由属于 sidecar；SSH/RDP/VNC 与白名单属于 devops；CDC 源、位点和三闸属于 capture；外部目标分类与探测任务属于 synthetics，不能为统一目录而把业务搬进 core。

## 目标目录与迁移顺序

1. 双向 session 已上收；本片将 Server/Provider/工具与测试消费者改到 core，并上收 TLS 监听配置。`internal/transport` 保留兼容别名与请求响应 Server，不复制 MuxConn 或 TLS 配置逻辑。
2. 插件宿主已在 `core/pluginhost`，sidecar/synthetics 协议适配已在角色目录；受管程序 supervisor 生命周期仍在 sidecar internal 装配，后续以审计/配置依赖注入抽取，不创建空包，不新增第二个进程管理者。
3. sidecar 的专有业务逐步归位到 `agents/sidecar`；本片迁移 Provider 会话模型、索引与原有测试，保留 internal 别名。`internal/agent` 与 `internal/app/agent` 的其他业务仍是迁移余量，每批只迁一个职责并验证调用链。
4. synthetics 的 `agents/synthetics/{config,runner,probe,pluginhost}` 与根 `cmd/synthetics-agent` 已有真实实现，复用 register/controlconn/pluginhost；原生 HTTP 探测尚未复用 healthprobe，后续按真实消费与验证推进。

依赖方向：`cmd → agents/<role> → core → pkg/pb/protocol`；Server 的内部包可消费 core，core/agents 的生产代码不得 import internal。`.github/scripts/architecture_guard.sh` 检查该边界，CI 单测与覆盖范围已纳入 core/agents；sidecar 现存 internal 路径是明确迁移余量，不以文档改名宣称迁移完成。

## 构建与发布职责

- Makefile：`make agents`/`make build` 已含四类 Agent（synthetics 经 `agent-synthetics` 进聚合）。
- `nightly.yml`：唯一每日构建与 nightly tag 更新入口，保留总包并发布每类独立包、配置、职责文档和 checksum 清单。
- `release.yml`：仅 v* tag 稳定发布，手动运行选择 v* tag；不再作为第二个 nightly 上传器。
- 原生 runner 运行 Agent `--version` 检查，交叉编译产物不冒充本机执行；ARMv7 仅 sidecar/devops，server/capture 的旧 CDC parser 存在 32 位整数溢出；synthetics 未在 32 位验证，交付面暂不扩大。
- 包装输入缺失即失败，manifest 按真实二进制记录 SHA256；nightly 与稳定版包装角色（`package-agents.py` AGENTS 单一清单）已含 synthetics，64 位面交付（ARMv7 仍仅 sidecar/devops），`release.yml` 未传 `--roles` 时按同一清单自动对齐。

## 第一片迁移与消费者核对（2026-10-20）

| 能力/归位           | 旧位置 → 当前唯一实现                                                                                                               | 兼容与行为                                                                                                                                                                                    |
| ------------------- | ----------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| TLS 监听配置        | `internal/transport/tcp/server.go` 的 `createServerTLSConfig`/`BuildServerTLSConfig` → `core/transport/tcp/tls_server.go:17`、`:57` | 原函数体仅改 helper 名；`internal/transport/tcp/server.go:219` 与 `cmd/sidecar-agent/root.go:648` 直接消费，旧路径转发；TLS 1.2、CA 启 mTLS、insecure 覆盖、证书必填/可选与错误文本均不变     |
| Provider 会话与索引 | `internal/agent/provider_session.go` → `agents/sidecar/provider_session.go:16`、`:96`                                               | 旧路径类型别名；新增绑定已建立 MuxConn 的构造入口，不拨新连接；索引、TTL、关闭行为不变                                                                                                        |
| Provider 会话测试   | `internal/agent/provider_session_test.go` → `agents/sidecar/provider_session_test.go`                                               | 原测试函数全部随迁，保留断言；补充连接绑定与关闭测试                                                                                                                                          |
| Provider 保活探针   | `internal/agent/provider_keepalive.go` → `agents/sidecar/provider_keepalive.go:16`                                                  | 实现逐行平移（仅包名变化），探测间隔/摘除/超时/Marshal 失败跳过语义不变；旧路径类型别名+构造转发；依赖网关监听器的握手 helper 留 `internal/agent/tcp_local_listener_test.go` 供路由类用例复用 |
| 在途任务生命周期    | `internal/agent/task_runner.go` → `agents/sidecar/task_runner.go:17`                                                                | 实现逐行平移（仅包名变化），Start/Cancel/事件上报/Marshal 语义不变；旧路径类型别名+构造转发；`marshalTaskInvoke` 等私有 helper 用例随迁，仅用导出面的用例经别名留在 internal                  |

| 消费者                  | 引用点（file:line）                                                                                                                                                                                                                                  | 本片状态                                                                                                        |
| ----------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------- |
| sidecar                 | `internal/app/agent/app.go:182`、`:206`、`:213`；`internal/agent/tcp_local_listener.go:330`                                                                                                                                                          | 直接消费 agents/sidecar 会话；网关 `:15`、`:16` 与 `internal/agent/local_handler.go:15` 直接消费 core transport |
| sidecar 上行            | `internal/app/agent/control_client.go:9`、`:10`；`internal/app/agent/upstream.go:122`                                                                                                                                                                | 已直接消费 core，复核保留                                                                                       |
| devops                  | `agents/devops/runner/runner.go:54`、`:57`                                                                                                                                                                                                           | 已用 register/controlconn，无需改 import；经 `agents/controlconn/controlconn.go:12`、`:51` 消费 core Client     |
| capture                 | `agents/capture/runner/runner.go:340`、`:343`                                                                                                                                                                                                        | 已用 register/controlconn，无需改 import；仍为串行请求响应                                                      |
| synthetics              | `agents/synthetics/runner/runner.go:52`、`:53`                                                                                                                                                                                                       | 已用 register/controlconn，无需改 import；结果 wire 与探测行为不变                                              |
| sidecar/synthetics 插件 | `agents/sidecar/funcplugin/host.go:61`；`agents/synthetics/pluginhost/controller.go:71`                                                                                                                                                              | 已消费 core/pluginhost，复核保留，无重复宿主或新增进程管理者                                                    |
| Server/集群/运维派发    | `internal/server/{agent_session.go:10,tcp_listener.go:15,control_handler.go:25}`；`internal/cluster/transport.go:9`；`internal/logic/ops/agent_ops_client.go:9`；`internal/platform/dispatch/dispatcher.go:14`；`internal/svc/service_context.go:17` | session/client 引用改到 core；`cmd/server/cluster.go:215` 的请求响应 NewServer 保留 internal                    |
| CLI/示例/函数适配工具   | `cmd/sidecar-agent/root.go:19`；`examples/cmd/e2e-agent-probe/main.go:32`；`tools/adapters/{http/main.go:15,prom/main.go:10}`                                                                                                                        | Client/Config 直接消费 core；测试中的 NewServer 仍用 internal 构造器                                            |

迁移不修改业务判断、报错、wire、默认超时、队列、重连或退出策略。todo:17 的 MuxConn/session、受管程序 supervisor 生命周期、sidecar 归位、synthetics 入口与发布聚合已交付；控制消息和探测内核的进一步收敛不属于本轮范围，按独立台账跟踪。

2026-10-20 生命周期决策复核：`core/process` 提供重启退避、熔断判定/文案、`cmd.Wait` 退出分类（Unix 信号与 Windows 无信号边界）及 OOM 疑似线索短路逻辑；`internal/app/agent` 保留 supervisor 状态机、事件日志、进程装配与授权。迁移不改默认退避、熔断阈值、事件类型、状态迁移或探测行为。
2026-10-22 supervisor 状态机公共化：`core/process.SupervisorState` 收敛连续失败计数、`allowRestart` 门、熔断/退避决策、backoff 到期消费、重启尝试与稳定性窗口恢复；`internal/app/agent.supervisorState` 为类型别名、`newSupervisorState` 构造转发，Ops monitor 驱动同一状态机并继续拥有 Wait、spawn/stop、授权、事件与稳定性窗口。状态迁移、默认退避、阈值与事件不变；旧 managedProcess 字段仅作展示快照兼容面。

2026-10-20 归属校验集中抽取：`core/agentscope` 提供去空白判定 `NonBlank` 与注册身份四字段 `Identity`（id/serverAddr/gameId/env，按声明顺序报第一个缺项）；devops/capture/synthetics 配置校验与 `internal/app/agent.ValidateAgentScope` 改为消费该规则。各入口报错文案与调用时机不变（sidecar 保留中文文案）；synthetics 首个报错字段由 map 遍历序漂移改为固定声明顺序（有意的行为改进，文案本身不变）。
