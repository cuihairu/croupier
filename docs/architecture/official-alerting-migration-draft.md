# official.alerting 迁移草案

更新时间：2026-03-15  
状态：Draft（Phase 7 预备）

## 1. 目标

将当前告警能力从核心 API 迁移为 `official.alerting` 官方扩展，核心只保留告警底座与权限审计能力。

## 2. 当前范围

- 现有入口：`internal/api/alert/*`
- 相关能力：
  - 告警列表查询
  - 告警静默与取消静默
  - silences 管理

## 3. 迁移边界

核心保留：

- 权限校验与操作审计
- 扩展 installation/runtime/sync
- 统一事件流与健康检查基础设施

扩展迁移：

- 告警业务 API
- 告警页面与操作入口
- 告警配置（静默策略等）

## 4. 目标 binding 草案

- page:
  - `alerts.overview` (`/alerts`)
- capability:
  - `alerts.management` (`list/silence/unsilence`)
- function:
  - `alerts.list`
  - `alerts.silence`

## 5. 分步建议

1. 先以 runtime bindings 骨架承载页面与能力发现（已开始）。
2. 将 `internal/api/alert` 服务改为 extension-first，保留短期兼容路由。
3. 在 Dashboard 接入 `/extensions/:id/pages` 渲染告警入口。
4. 验证“未安装不影响核心、安装后能力恢复”。

## 6. 风险

- 旧前端可能直接依赖 `/api/v1/alerts/*` 结构，需要兼容期转发。
- 告警静默策略与权限模型耦合较深，需要保留核心权限校验。
