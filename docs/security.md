---
title: 安全配置
icon: shield
order: 6
category:
  - 入门指南
tag:
  - 安全
  - 权限
---

# Security

- mTLS for Server/Agent
- OIDC/MFA for users
- RBAC/ABAC, approvals, audit log chain

Approvals (Two-person rule)

- 契约：函数描述符的 `Approval` 字段（ApprovalPolicy：`approvalRequired` + `approvalPolicyKey`，
  见 [Descriptor v2](./architecture/openapi-sdk-descriptor-v2.md)）；调用高风险函数时生成审批待办。
- Storage：统一 GORM 实现（`internal/platform/approvals`），随主库迁移自动建表
  （MySQL/Postgres/SQLite/SQL Server 均内置，无需 build tag）。
- API（详见 [审批 API](./api/approval.md)）：
  - 列表：`GET /api/v1/approvals`（查询参数 state/functionId/gameId/env/page/size 等）
  - 详情：`GET /api/v1/approvals/:id`（含脱敏 payload 快照）
  - 同意：`POST /api/v1/approvals/:id/approve`（同意后执行原调用并返回结果/Job）
  - 拒绝：`POST /api/v1/approvals/:id/reject`，body `{ "reason": "..." }`
- 审计：`approval_approve`/`approval_reject` 事件记录在审计链中；调用审计包含 `trace_id` 与脱敏快照。

Notes

- UI 审批页已提供（/approvals 顶级菜单「审批中心」）：待办列表（分页/筛选）→ 详情侧栏 → 同意/拒绝；对高危函数已支持二次确认与 MFA（OTP）。
- 生产建议：优先 PostgreSQL，并为 approvals 表添加备份策略与告警（待办积压/拒绝率异常）。SQLite 适用于单机/PoC/嵌入式部署。

RBAC/ABAC

- RBAC：基于角色/用户的 permission 检查，支持 game 作用域（`game:<game_id>:permission`）。
- ABAC（简易表达式）：在函数描述 `auth.allow_if` 中配置表达式（==、!=、&&、||、has_role('admin')）
  - 可用变量：`user`、`game_id`、`env`、`function_id`
  - 示例：`env == "prod" && has_role('admin')`

Rate limit & Concurrency

- 在函数描述 `semantics.rate_limit`（例如 `10rps`）与 `semantics.concurrency`（整数）启用限流/并发限制。
- 触达限制时返回 HTTP 429。
