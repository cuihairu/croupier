# 审计 API

### 1. "获取审计日志"

1. route definition

- Url: /api/v1/audit
- Method: GET
- Request: `AuditRequest`
- Response: `AuditResponse`

2. request definition

```go
type AuditRequest struct {
	Page int `form:"page,optional"` // 页码
	PageSize int `form:"pageSize,optional"` // 每页数量
	Action string `form:"action,optional"` // 操作类型过滤
	UserID string `form:"userId,optional"` // 用户ID过滤
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

---

## 审计导出与链校验

### 导出审计（SIEM/归档）

- Url: /api/v1/audit/export
- Method: GET
- Auth: Bearer Token（audit:read；作用域限制与列表接口一致）
- 参数: 与 GET /api/v1/audit 相同（actor/kind/gameId/env/ip/start/end），外加 `format=json|csv`
- 行为: 附件下载；上限 50000 行，截断时响应头 `X-Truncated: true`；CSV 带 UTF-8 BOM

### 校验审计链完整性

- Url: /api/v1/audit/chain/verify
- Method: GET
- Response: `{ "valid": bool, "checked": n, "firstBreakSeq": n, "message": "..." }`

建议接入定时任务（如每日）执行导出归档到对象存储，并在合规检查前执行链校验。
