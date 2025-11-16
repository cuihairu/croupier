# Security

- mTLS for Server/Agent
- OIDC/MFA for users
- RBAC/ABAC, approvals, audit log chain

Approvals (Two-person rule)
- Enable: set `auth.two_person_rule: true` in function descriptor; HTTP 提交将返回 `202` 和 `approval_id`。
- Storage:
  - Memory: 默认（不提供 DATABASE_URL）
  - PostgreSQL: 设置 `DATABASE_URL=postgres://...`，二进制以 `-tags pg` 构建；表结构参见 `database/schema.sql` 中 `approvals`。
  - SQLite (可选): 设置 `DATABASE_URL=sqlite:///path/to/croupier.db` 或 `file:/path/to/croupier.db`，二进制以 `-tags sqlite` 构建；首次启动会自动建表。
- API：
  - 列表：`GET /api/approvals?state=pending&function_id=&game_id=&env=&actor=&mode=&page=1&size=20&sort=created_at_desc`
    - 返回：`{ approvals: [...], total, page, size }`
  - 详情：`GET /api/approvals/get?id=...`（含 `payload_preview` 脱敏快照）
  - 同意：`POST /api/approvals/approve`，body `{ "id": "..." }`（同意后立即执行原调用并返回结果/Job）
  - 拒绝：`POST /api/approvals/reject`，body `{ "id": "...", "reason": "..." }`
- 审计：`approval_approve`/`approval_reject` 事件记录在审计链中；调用审计包含 `trace_id` 与脱敏快照。

Notes
- UI 审批页已提供（/gm/approvals）：待办列表（分页/筛选）→ 详情侧栏 → 同意/拒绝；对高危函数已支持二次确认与 MFA（OTP）。
- 生产建议：优先 PostgreSQL，并为 approvals 表添加备份策略与告警（待办积压/拒绝率异常）。SQLite 适用于单机/PoC/嵌入式部署。

RBAC/ABAC
- RBAC：基于角色/用户的 permission 检查，支持 game 作用域（`game:<game_id>:permission`）。
- ABAC（简易表达式）：在函数描述 `auth.allow_if` 中配置表达式（==、!=、&&、||、has_role('admin')）
  - 可用变量：`user`、`game_id`、`env`、`function_id`
  - 示例：`env == "prod" && has_role('admin')`

Rate limit & Concurrency
- 在函数描述 `semantics.rate_limit`（例如 `10rps`）与 `semantics.concurrency`（整数）启用限流/并发限制。
- 触达限制时返回 HTTP 429。
