---
title: CI/CD 接入 API
icon: rocket
order: 24
---

# CI/CD 接入 API

可插拔 CI provider 接入（jenkins / gitlab-ci / github-actions / generic）与构建状态
回写。实现以 `internal/api/cicd/` 为准；受 `featureFlags.dev` 门控（部署级物理裁剪，
见 [功能开关](../architecture/feature-flags.md)）。

- **认证**：管理端点需 JWT；`POST /api/v1/cicd/webhooks/:id` 为公开端点（外部 CI
  服务器无法携带 JWT），鉴权 = integration Token 的 `X-CICD-Token` 精确匹配——
  **未配 Token 的接入为开放端点（诚实边界）**。
- **出站**：test/trigger 的外呼经出站守卫（`sec.*`）与调用策略（`net.*`）。
- **scope**：integration/build 数据带 `gameId`/`env`，按 game 隔离。

## 端点

| 方法   | 路径                                    | 说明                              |
| ------ | --------------------------------------- | --------------------------------- |
| GET    | `/api/v1/cicd/integrations`             | 列表（query `gameId`/`env` 过滤） |
| POST   | `/api/v1/cicd/integrations`             | 创建接入                          |
| PUT    | `/api/v1/cicd/integrations/:id`         | 更新接入                          |
| DELETE | `/api/v1/cicd/integrations/:id`         | 删除接入                          |
| POST   | `/api/v1/cicd/integrations/:id/test`    | 连通性测试（外呼 provider）       |
| POST   | `/api/v1/cicd/integrations/:id/trigger` | 触发流水线                        |
| GET    | `/api/v1/cicd/builds`                   | 构建列表（按 scope）              |
| POST   | `/api/v1/cicd/builds/:id/refresh`       | 拉取最新构建状态                  |
| POST   | `/api/v1/cicd/webhooks/:id`             | 公开构建状态回写（webhook）       |

## Integration 结构

```go
type Integration struct {
	ID          uint              `json:"id"`
	GameID      string            `json:"gameId"`
	Env         string            `json:"env"`
	Kind        string            `json:"kind"`     // jenkins | gitlab-ci | github-actions | generic
	Name        string            `json:"name"`
	Endpoint    string            `json:"endpoint"`
	TokenSet    bool              `json:"tokenSet"`     // token 只写不读，回显 set 态 + 掩码
	TokenMasked string            `json:"tokenMasked"`
	Extra       map[string]string `json:"extra"`        // kind 特有参数（如 jenkins job 名）
	Enabled     bool              `json:"enabled"`
	CreatedBy   string            `json:"createdBy"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}
```

创建/更新请求：`{ gameId, env, kind, name, endpoint, token, extra, enabled }`
（`GET /cicd/integrations` 响应另带 `kinds`：注册表当前可用类型，前端动态表单用）。

## Webhook 回写

```bash
curl -X POST https://<server>:18780/api/v1/cicd/webhooks/<integrationId> \
  -H "X-CICD-Token: <integration token>" -H "Content-Type: application/json" \
  -d '{ "externalId": "jenkins-job-42", "status": "success", ... }'
```

payload/response 字段以 `internal/api/cicd/webhook.go`（`WebhookPayload` /
`WebhookResponse`）为准；状态与构建记录落 `cicd_builds`，可在研发域构建列表查看。
