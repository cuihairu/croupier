# Security

- mTLS for Server/Agent
- OIDC/MFA for users
- RBAC/ABAC, approvals, audit log chain

Approvals (Two-person rule)
- Enable: set `auth.two_person_rule: true` in function descriptor; HTTP 提交将返回 `202` 和 `approval_id`。
- Storage: 默认内存；如提供 `DATABASE_URL=postgres://...` 且二进制以 `-tags pg` 构建，则使用 Postgres 持久化（表 `approvals`）。
- API：
  - 列表：`GET /api/approvals?state=pending&function_id=&game_id=&env=&actor=&mode=&page=1&size=20&sort=created_at_desc`
    - 返回：`{ approvals: [...], total, page, size }`
  - 详情：`GET /api/approvals/get?id=...`（含 `payload_preview` 脱敏快照）
  - 同意：`POST /api/approvals/approve`，body `{ "id": "..." }`（同意后立即执行原调用并返回结果/Job）
  - 拒绝：`POST /api/approvals/reject`，body `{ "id": "...", "reason": "..." }`
- 审计：`approval_approve`/`approval_reject` 事件记录在审计链中；调用审计包含 `trace_id` 与脱敏快照。

Notes
- UI 审批页可基于上述 API 实现：待办列表（分页/筛选）→ 详情侧栏 → 同意/拒绝；对高危函数可追加二次确认和 MFA。
- 生产建议：开启 Postgres 持久化，并为 approvals 表添加备份策略与告警（待办积压/拒绝率异常）。
