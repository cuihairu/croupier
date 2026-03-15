# official.notification 迁移草案

更新时间：2026-03-15  
状态：Draft（Phase 7 预备）

## 1. 目标

将当前通知配置与分发能力从核心 `ops` 兼容接口迁移为 `official.notification` 官方扩展，核心只保留权限、审计、扩展运行时与事件底座。

## 2. 当前范围

- 现有入口：`internal/api/ops` 中的 `notifications` 读写接口
- 当前状态：通知模型尚未完整实现，接口主要返回占位结构

## 3. 迁移边界

核心保留：

- 权限、审计与扩展安装生命周期
- extension installation/runtime binding/event 基础设施
- 兼容路由与过渡期回包语义

扩展迁移：

- 通知渠道管理（webhook、email、im 等）
- 通知规则管理（事件到渠道映射）
- 通知页面与配置渲染

## 4. 目标 binding 草案

- page:
  - `notifications.overview`（`/ops/notifications`）
- capability:
  - `notifications.management`（`get/update`）
- function:
  - `notifications.get`
  - `notifications.update`

## 5. 分步建议

1. 先落 runtime bindings 骨架和 extension 事件桥接（本轮完成）。
2. 将通知配置读写切到 installation config（保留兼容字段）。
3. 在 Dashboard 通过 `/extensions/:id/pages` 渲染通知页面入口。
4. 收敛旧 `ops notifications` 直接业务实现为兼容代理层。

## 6. 风险

- 旧前端可能依赖固定 `ops notifications` 数据结构，需要保持兼容字段。
- 通知渠道密钥需要统一迁移到 `secret_binding`，避免明文配置扩散。
