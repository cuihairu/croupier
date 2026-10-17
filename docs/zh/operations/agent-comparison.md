---
title: 四类 Agent 职能、部署与构建产物
---

# 四类 Agent 职能、部署与构建产物

Croupier 规划四类 Agent，目前有三个可执行入口；synthetics 仍待实现。四类实例均按唯一 `gameId + env` 纳管，网络连接、注册心跳、配置同步与探测内核尽量复用公共能力，独立职责分别装配。

## 1. 选型总表

| 类型       | 核心职责                                             | 部署位置                                      | 当前入口 / 二进制                      | 当前实现边界                                                                                  |
| ---------- | ---------------------------------------------------- | --------------------------------------------- | -------------------------------------- | --------------------------------------------------------------------------------------------- |
| sidecar    | 游戏 SDK 函数注册、调用/任务转发、游戏旁路监管与观测 | 游戏服务器本机或可访问游戏服务的网段          | `cmd/sidecar-agent` / `croupier-agent` | SDK 会话与调用主体已有；进程监管等模块实现与 CLI 配置/启动接线仍有差距                        |
| devops     | 授权人员从后台进入内网机器操作，白名单与审计前置     | 可访问受控主机的内网运维节点                  | `cmd/devops-agent` / `devops-agent`    | 注册、心跳、Broker、白名单、审计与会话元数据已有；SSH/RDP/VNC、下行会话与内容回放未接通       |
| capture    | CDC 捕获业务库变更、过滤及对账/CEP/血缘检测、告警    | 可访问源库复制日志的旁路节点                  | `cmd/capture-agent` / `capture-agent`  | MySQL/MariaDB、PostgreSQL、MongoDB 与规则链已有；位点确认、规则窗口恢复和告警可靠投递仍需收口 |
| synthetics | 游戏外部及第三方服务主动探测，拉起开发者多语言插件   | 代表玩家或服务消费方的网络位置，可多地域/网段 | 待实现，无入口/二进制/包               | 独立载体、go-plugin 运行时和对应页面待实现，内置 Checker 不等于该 Agent 已完成                |

`connector-agent` 只是外部系统事件接入路线图占位，不算第五个已实现 Agent。完整职责及页面规划见 [Agent 清单](../design/agents-inventory.md)、[四类后台页面](../design/agent-dashboard-design.md)。

## 2. sidecar：游戏函数与运行旁挂

当游戏需要接入 Croupier 函数注册、GM 调用及任务执行时使用。SDK 主动连接其本地网关（常用端口 19091），sidecar 主动连接 Server 控制面（常用端口 19090），Server 调用复用已有双向连接；不要求 Server 反向进入游戏内网。

函数来源可以是游戏进程内的六语言 SDK，或已有 Provider/OpenAPI 接入；可选 go-plugin 外部函数适配器是后续设计，负责把函数映射到游戏已有 HTTP/TCP 管理接口，不意味着任意游戏无需接口就能被操作。

监管模块包含启停/重启、退避熔断、期望状态、定时任务、快照和受管程序更新，但当前 CLI 未完整透传 OpsConfig，也未接通 OpsServer.Start；配置示例中的监管项不能视为已通过生产入口验收。远程控制需显式权限/本地开关，sidecar 不提供堡垒机式 SSH/RDP/VNC 接入。

- 输入：SDK 注册、Server 调用/任务/明确授权动作、配置策略。
- 输出：函数与 Provider 清单、调用结果/任务事件、系统/监管/探测信息。
- 配置：`configs/agent.yaml`；字段现状见 [配置全解](agent-config.md)。

## 3. devops：内网远程会话

用于研发/运维访问白名单机器，例如排查内网构建机；它不承担游戏函数注册或数据库变更采集。目标必须按 host+port+protocol 明确列入白名单，会话建立需先持久记录审计，缺审计时拒绝放行。

当前 `devops.enabled=false` 时只注册上线；即使设 true，生产装配仍为 noop Target，不能建立真实 SSH/RDP/VNC 会话。现有会话开始/结束元数据不是终端录像或可回放内容，后台入口随真实通道交付。

- 输入：白名单与会话配置；后续接入受授权的会话请求。
- 输出：在线身份、会话审计与元数据；真实终端/桌面流待实现。
- 配置示例：`configs/devops-agent.example.yaml`；安装后配置名为 `devops-agent.yaml`。

## 4. capture：CDC 与业务变更检查

持续读取数据库变更日志，把不同来源转换为共同 ChangeEvent，再做事件过滤与三道闸：业务流水对账、时序模式 CEP、写入血缘检测。支持 MySQL/MariaDB binlog、PostgreSQL WAL 逻辑复制及 MongoDB change streams；MongoDB 需要 replica set/mongos 等支持环境。

规则从平台按 scope 拉取，保留版本及 appliedRevision；采集源只读，检测命中只上报告警，不直接修改游戏资产。凭据通过 DSN 环境变量引用传递，表/集合使用显式白名单。

当前告警队列满或发送失败可能丢弃，连接器 checkpoint 与已处理事件的确认关系也有待故障验证与收口，不能承诺断网/崩溃场景下可靠投递或恰好执行一次。源端快照、前镜像、事务元数据等能力按连接器分别声明，不能把一种源能力推广到全部源。

- 输入：源配置、续读位点、过滤和三闸策略。
- 输出：采集统计、位点、规则版本与命中告警。
- 配置示例：`configs/capture-agent.example.yaml`；安装后配置名为 `capture-agent.yaml`。
- ARMv7 不支持该依赖组合，选择 amd64/arm64；详见下方构建平台表。

## 5. synthetics：外部游戏与第三方服务探测（待实现）

从游戏进程外检查握手、登录及按游戏定义的业务路径，同时检查第三方支付、账号、消息等依赖；目标必填 `targetCategory=game/third_party`，两类均携带 gameId/env，类别与协议/插件能力分别建模。

宿主计划使用 HashiCorp go-plugin 拉起常驻、可复用的开发者程序，通过本地 gRPC IPC 调用；游戏协议由开发者以适合的语言实现，结果回到公共 healthprobe 内核形成阈值、故障窗口与统计。插件故障、端口可达和目标业务可用分别呈现。

本地插件的 gRPC 依赖留在独立探测宿主/插件，游戏 SDK 保持轻量 TCP 接入，Agent→Server 共用 TCP/TLS。插件与 OTel 默认集成、运行时可关及编译裁剪是后续交付要求，当前没有 synthetics 安装命令、可运行配置或 nightly 占位包。见 [go-plugin 调研](../research/synthetics-agent-go-plugin.md)。

## 6. 每个实例明确游戏环境

```yaml
agent:
  id: demo-prod-capture-01
  gameId: demo
  env: prod
```

这只是三类共有身份字段示意，各二进制上游地址和业务配置按各自示例设置；ID 命名不替代显式字段。跨游戏或环境运行多个独立实例，数据目录、凭证、位点与插件进程分别隔离。

当前 sidecar 已强制检查两个字段；devops/capture 配置中 env 仍可空，属于待修复缺口，部署时仍须显式填写。Server 归属、页面标注及跨 scope 验收以 [scope 规范](../architecture/game-environment-scope.md) 为准。

## 7. Nightly 与稳定版下载

发布链调整后，保留总包 `croupier-bin-<os>-<arch>.tar.gz`（Windows 为 zip），同时提供：

| 独立包                                | 内容                                                         |
| ------------------------------------- | ------------------------------------------------------------ |
| `croupier-agent-<os>-<arch>.tar.gz`   | sidecar 二进制、`configs/agent.yaml`、AGENTS.md、INSTALL.md  |
| `devops-agent-<os>-<arch>.tar.gz`     | devops 二进制、`configs/devops-agent.yaml`、职责和安装文档   |
| `capture-agent-<os>-<arch>.tar.gz`    | capture 二进制、`configs/capture-agent.yaml`、职责和安装文档 |
| `agents-<os>-<arch>.json` / `.sha256` | 实际包含角色、归档/二进制 SHA256 与校验清单                  |

Windows 使用 `.zip` 和 `.exe`，同样附配置与说明。支持目标：

| 平台                      | sidecar | devops         | capture                     | Server 总包    |
| ------------------------- | ------- | -------------- | --------------------------- | -------------- |
| Linux amd64/arm64         | 有      | 有（会话骨架） | 有                          | 有             |
| Linux ARMv7（包名 armv7） | 有      | 有（会话骨架） | 无，32 位 CDC parser 不兼容 | 无，同依赖限制 |
| macOS amd64/arm64         | 有      | 有（会话骨架） | 有                          | 有             |
| Windows amd64             | 有      | 有（会话骨架） | 有                          | 有             |

`nightly.yml` 是唯一每日发布入口，`release.yml` 仅发布 v* tag；工作流修改需合并并成功运行后才会出现在 GitHub Release，本文不宣称已发布新资产。跨平台构建成功不等于完成真实环境功能验收。

Linux/macOS 安装器继续支持从总包按角色选择安装：

```bash
# 同样可以将 sidecar 换为 devops 或 capture（capture 需 64 位）
curl -fsSL https://raw.githubusercontent.com/cuihairu/croupier/main/scripts/install.sh | bash -s -- --agent sidecar --version nightly --game-id demo --env prod
```

手动安装时先配置真实 server 地址、gameId/env 和凭证引用，不能直接把示例当生产配置；校验归档 SHA256 后解压。Windows 当前安装脚本仅覆盖 sidecar，其他角色手动部署。详细流程见 [一键安装](agent-install.md)。

## 8. 源码、公共能力与测试

二进制入口统一在根 `cmd/`，独立业务放 `agents/<role>`，公共件放 `core/`；sidecar 的历史业务仍位于 internal，完整目录及待抽取 MuxConn/监管边界见 [公共运行时与目录](../development/agent-runtime-layout.md)。

```bash
make agents              # 三个已有 Agent；单独构建仍可用 agent/agent-devops/agent-capture
# 公共件和角色实现均纳入 CI，具体环境依赖另见集成测试说明
go test ./core/... ./agents/...
```
