# Agent 业务实现

四类角色及当前状态：sidecar 游戏函数网关（入口 `cmd/sidecar-agent`，SDK Provider 会话与函数插件已在 `agents/sidecar`，其余装配暂在 `internal/app/agent` 与 `internal/agent`）；devops 内网会话代理（骨架，真实协议通道未接）；capture 数据库 CDC 与规则告警（已实现）；synthetics 外部 HTTP 探测、持久化结果查询与 go-plugin 探测插件已实现。

- `controlconn/`：devops/capture/synthetics 共用上行消息适配，不是另一套完整公共 session。
- `sidecar/`：SDK Provider 会话与索引、`funcplugin/` 函数插件协议适配，复用 core transport/pluginhost。
- `devops/`：配置、运行装配、白名单与会话审计。
- `capture/`：配置、运行装配、连接器、位点、过滤和规则。
- `synthetics/`：配置、HTTP 探测、运行装配与探测插件协议适配，复用 core register/pluginhost；原生 HTTP 探测尚未接入 core healthprobe。
- 新二进制入口统一放根 `cmd/`；通用能力进入 `core/`，角色业务留此目录。

所有实例显式声明唯一 `gameId/env`，多环境同机运行多个独立实例。公共网络、目录目标与迁移余量见 [运行时目录边界](../docs/zh/development/agent-runtime-layout.md)，实际职责、配置和安装见 [选型指南](../docs/zh/operations/agent-comparison.md)。
