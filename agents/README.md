# Agent 业务实现

四类角色及当前状态：sidecar 游戏函数网关（入口 `cmd/sidecar-agent`，业务暂在 `internal/app/agent` 与 `internal/agent`）；devops 内网会话代理（骨架，真实协议通道未接）；capture 数据库 CDC 与规则告警（已实现）；synthetics 游戏/第三方外部探测与多语言插件宿主（待实现，无源码入口或发布包）。

- `controlconn/`：devops/capture 共用上行消息适配，不是另一套完整公共 session。
- `devops/`：配置、运行装配、白名单与会话审计。
- `capture/`：配置、运行装配、连接器、位点、过滤和规则。
- 新二进制入口统一放根 `cmd/`；通用能力进入 `core/`，角色业务留此目录。

所有实例显式声明唯一 `gameId/env`，多环境同机运行多个独立实例。公共网络、目录目标与迁移余量见 [运行时目录边界](../docs/zh/development/agent-runtime-layout.md)，实际职责、配置和安装见 [选型指南](../docs/zh/operations/agent-comparison.md)。
