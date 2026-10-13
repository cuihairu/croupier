# 告警 API

### 1. "获取告警列表"

1. route definition

- Url: /api/v1/alerts
- Method: GET
- Request: `AlertsListRequest`
- Response: `AlertsListResponse`

2. request definition

```go
type AlertsListRequest struct {
	Page int `form:"page,optional,default=1"`
	PageSize int `form:"pageSize,optional,default=20"`
	Level string `form:"level,optional"`
	Status string `form:"status,optional"`
}
```

3. response definition

```go
type AlertsListResponse struct {
	Items []Alert `json:"items"`
	Total int64 `json:"total"`
	Page int `json:"page"`
	Size int `json:"pageSize"`
}
```

### 2. "静默告警"

1. route definition

- Url: /api/v1/alerts/:id/silence
- Method: POST
- Request: `AlertSilenceRequest`
- Response: `-`

2. request definition

```go
type AlertSilenceRequest struct {
	ID string `path:"id"`
	Duration int `json:"duration"` // 分钟
	Reason string `json:"reason,optional"`
}
```

3. response definition

### 3. "获取静默规则列表"

1. route definition

- Url: /api/v1/alerts/silences
- Method: GET
- Request: `SilencesListRequest`
- Response: `SilencesListResponse`

2. request definition

```go
type SilencesListRequest struct {
}
```

3. response definition

```go
type SilencesListResponse struct {
	Items []Silence `json:"items"`
}
```

### 4. "删除静默规则"

1. route definition

- Url: /api/v1/alerts/silences/:id
- Method: DELETE
- Request: `SilenceDeleteRequest`
- Response: `-`

2. request definition

```go
type SilenceDeleteRequest struct {
	ID string `path:"id"`
}
```

3. response definition

---

### 风险事件工作流流转

- Url: /api/v1/ops/alerts/:id/handle
- Method: POST
- Auth: Bearer Token（ops 域）

```json
{
  "action": "acknowledge",
  "remark": "checked ledger"
}
```

`action`：`acknowledge` 已确认 / `ignore` 已忽略 / `escalate` 已升级；`remark` 随流转落审计（处理人取认证上下文，时间取库钟）。

响应：`{ "alertId", "handleStatus", "handledBy", "handledAt" }`。

## 告警规则（阈值评估）

Agent 指标上报时实时评估。命中（可选连续 N 次）即生成告警并走通知渠道（站内信/钉钉/webhook），冷却期内不重复触发。

- `GET /api/v1/alerts/rules` — 列表（可按 metric/enabled 过滤）
- `POST /api/v1/alerts/rules` — 创建：`{name, metric, operator, threshold, forCount?, cooldownSeconds?, level?, agentFilter?, enabled?}`
- `PUT /api/v1/alerts/rules/:id` — 部分更新
- `DELETE /api/v1/alerts/rules/:id`

指标路径：`cpu.usagePercent`、`memory.usagePercent`、`memory.usedBytes`、`disk.<挂载点>.usedPercent|usedBytes`、`custom.<key>`；operator：`gt/gte/lt/lte`；level：`info/warning/critical`。

> 本域路由受 `featureFlags.ops` 门控：flag 关闭时整组 404（`internal/handler/routes.go` opsSoft 组）。
