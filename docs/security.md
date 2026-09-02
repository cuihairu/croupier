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

认证与会话安全（Auth Hardening）

- 登录失败锁定：本地账号（local provider）连续密码失败达到阈值后临时锁定，成功登录清零。
  配置 `auth.loginLockout.threshold`（默认 5）与 `auth.loginLockout.lockMinutes`（默认 15）。
  仅 local provider 计数——LDAP/OIDC 的失败锁定由身份源自己负责；锁定事件入审计链（`auth.login_locked`）。
- token 撤销：`admins.token_version` 随 JWT claims 下发；登出、改密码（自助/重置）、禁用账号时递增，
  旧 token 即时失效。已知边界：中间件有 30 秒版本缓存，撤销最长延迟 30s 生效；
  存储故障时放行（可用性优先，记 warn 日志）。
- TOTP MFA：本地账号可自助绑定（`POST /api/v1/auth/mfa/setup` → `confirm`，`disable` 需验证码+密码双确认）。
  启用后登录需携带 `totpCode`，缺失返回 `401 + error=mfa_required`。
  仅 local provider 生效——OIDC 登录的 MFA 属 IdP 职责；裸 LDAP 部署需要 MFA 时应改用本地账号承载。
  已知边界：前端二次验证码输入 UI 暂未提供（后端契约已就绪），审计事件 `auth.mfa_enabled/disabled` 已接线。

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
