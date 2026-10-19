---
title: Agent 公共运行时与目录边界
---

# Agent 公共运行时与目录边界

2026-10-17 核查。本文区分当前源码位置、已抽取能力和待迁移能力；配套用户入口为 [Agent 职能与选型](../operations/agent-comparison.md)，分类与后台设计见 [Agent 清单](../design/agents-inventory.md)、[四类后台页面](../design/agent-dashboard-design.md)。

## 当前真实目录

```text
cmd/
  server/                     Server CLI
  sidecar-agent/              sidecar CLI，产物仍名 croupier-agent
  devops-agent/               devops CLI
  capture-agent/              capture CLI
core/                         可组合的 Agent 公共运行时
  transport/tcp/              TCP/TLS 客户端与 framing
  register/                   注册、心跳、重连、owner 身份
  backoff/ logx/              通用退避与日志装配
  configsync/                 版本化拉取与 last-good 应用
  supervisable/               进程信号、退出码、本地心跳契约
  healthprobe/                探针、故障窗口与协议步骤
  report/ metrics/            上报适配与系统指标采样
  execlog/ crash/             执行审计与崩溃辅助
  process/                    五字段 cron 与托管进程调度模型
agents/
  controlconn/                devops/capture 共用控制消息适配
  devops/{config,runner,session}/
  capture/{config,runner,connector,source,filter,gate}/
internal/
  app/agent/                 当前 sidecar 装配、监管、Provider、探针
  agent/                     SDK 本地网关、Provider session、任务转发
  transport/tcp/             当前双向 MuxConn / server；基础 client/framing 是 core 转发
pkg/{pb,protocol}/           共享 wire message 与消息编号
```

尚无 `cmd/synthetics-agent`、`agents/synthetics` 或独立插件宿主实现；不能按规划目录生成占位发布包。根目录 `cmd/` 是唯一二进制入口，不能再在 `agents/<role>/cmd` 建第二套入口。

## 公共能力与业务边界

| 能力                           | 当前公共实现                                 | 当前消费/边界                                                                                                   | 后续收敛                                                    |
| ------------------------------ | -------------------------------------------- | --------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------- |
| framing、TCP/TLS               | `core/transport/tcp`                         | internal 通过别名转发；standalone 通过 controlconn 消费                                                         | 同一实现，不复制 TLS/帧格式                                 |
| 双向 session/多路复用/派发背压 | `core/transport/tcp.MuxConn`                 | sidecar 装配直接消费 core；Server 与 SDK Provider 仍可经 internal 兼容 shim 消费，devops/capture 仍串行请求响应 | 实现已上收；逐个迁移真实消费者并收敛兼容 shim，不做双份源码 |
| 注册、心跳与重连               | `core/register`                              | 三类已消费；注册 payload 由各角色组装                                                                           | 生命周期 race 已入 CI；继续收敛真实断网恢复验收             |
| 上行消息适配                   | `agents/controlconn`、`core/report`          | controlconn 共用 Register/Heartbeat/capture 消息，非全部角色统一 session 客户端                                 | session 稳定后抽通用收发，业务消息 handler 留角色侧         |
| 信号与被监管契约               | `core/supervisable`                          | devops/capture 使用；sidecar CLI 有自己的退出装配                                                               | 统一错误传播与清理，不把拥有 main 等同可被正确监管          |
| 探测内核                       | `core/healthprobe`                           | sidecar/Server 装配较完整，capture 有 readiness；devops 探测消费未齐                                            | synthetics 复用内核，仅增加目标与外部插件装配               |
| configsync                     | `core/configsync`                            | 拉取/版本/last-good 为公共件，具体配置解析留业务侧                                                              | 不同 Agent 不各写一份拉取状态机                             |
| 指标/上报/审计/快照            | `core/metrics`、`report`、`execlog`、`crash` | 按需组合，公共包存在不等于所有角色都已接线                                                                      | 以真实消费与端到端证据标注状态                              |
| 受管程序 supervisor            | `core/process`、`internal/app/agent/ops_*`   | cron/调度模型已公共化，sidecar 与 Server scheduler 复用同一实现；supervisor 装配仍留在 sidecar                  | 插件需要复用时继续抽生命周期管理；每进程只有一个管理者      |
| go-plugin/OTel                 | 已有 OTel 内部实现；插件宿主待实现           | 默认集成与 no_go_plugin/no_otel 裁剪是已确定目标                                                                | 依赖隔离在宿主/遥测边界，避免灌入公共 Checker 或游戏 SDK    |

SDK 函数注册与路由属于 sidecar；SSH/RDP/VNC 与白名单属于 devops；CDC 源、位点和三闸属于 capture；外部目标分类与探测任务属于 synthetics，不能为统一目录而把业务搬进 core。

## 目标目录与迁移顺序

1. 先上收双向 session，保留 `internal/transport` 兼容别名，统一测试；新 Agent 直接依赖公共 session。
2. 将通用进程监管/插件宿主能力抽为独立公共包，使用依赖注入接审计/配置；包名和接口在对应实现批次确定，不创建空包。
3. sidecar 的专有业务逐步归位到 `agents/sidecar`，`internal/agent` 与 `internal/app/agent` 在迁移窗口保留薄装配；每批只迁一个职责并验证调用链。
4. synthetics 在契约、公共宿主与真实目标验证后创建 `agents/synthetics/{config,runner,...}` 与根 `cmd/synthetics-agent`，复用前三步公共件。

依赖方向：`cmd → agents/<role> → core → pkg/pb/protocol`；Server 的内部包可消费 core，core/agents 的生产代码不得 import internal。`.github/scripts/architecture_guard.sh` 检查该边界，CI 单测与覆盖范围已纳入 core/agents；sidecar 现存 internal 路径是明确迁移余量，不以文档改名宣称迁移完成。

## 构建与发布职责

- Makefile：`make agents` 集中构建三个已有 Agent，也可分别执行 `make agent`、`make agent-devops`、`make agent-capture`；`make build` 含 Server 与三个 Agent。
- `nightly.yml`：唯一每日构建与 nightly tag 更新入口，保留总包并发布每类独立包、配置、职责文档和 checksum 清单。
- `release.yml`：仅 v* tag 稳定发布，手动运行选择 v* tag；不再作为第二个 nightly 上传器。
- 原生 runner 运行 Agent `--version` 检查，交叉编译产物不冒充本机执行；ARMv7 仅 sidecar/devops，server/capture 的旧 CDC parser 存在 32 位整数溢出，待依赖兼容方案解决后再扩大平台支持。
- 包装输入缺失即失败，manifest 按真实二进制记录 SHA256；synthetics 未实现时无包，未实现编译裁剪开关不列成已发布变体。
