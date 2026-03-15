# official.approval 迁移草案

更新时间：2026-03-15  
状态：Draft（Phase 7 预备）

## 1. 目标

将当前审批业务入口从核心 API 逐步迁移为 `official.approval` 官方扩展，核心仅保留审批底座依赖、权限与审计。

## 2. 当前范围

- 现有入口：`internal/api/approval/*`
- 现有能力：
  - 审批列表与详情
  - 审批通过
  - 审批拒绝

## 3. 迁移边界

核心保留：

- 权限、审计、扩展 installation/runtime/event 基础设施
- 审批调用链基础约束（与函数执行链路耦合部分）

扩展迁移：

- 审批页面、审批规则配置、审批策略细节
- 审批业务操作入口（list/get/approve/reject）

## 4. 目标 binding 草案

- page:
  - `approvals.overview`（`/approvals`）
- capability:
  - `approvals.management`（`list/get/approve/reject`）
- function:
  - `approvals.approve`
  - `approvals.reject`

## 5. 分步建议

1. 先完成 runtime bindings 骨架与事件桥接（本轮完成）。
2. 审批配置迁到 installation config，核心 API 改为 extension-first。
3. Dashboard 审批入口改为扩展页面渲染。
4. 在稳定后缩减核心审批业务实现，仅保留兼容代理层。

## 6. 风险

- 审批与函数调用链路耦合较深，迁移过程中需要严格保持状态语义不变。
- 若旧前端直接依赖审批字段结构，需要保留兼容期 DTO。
