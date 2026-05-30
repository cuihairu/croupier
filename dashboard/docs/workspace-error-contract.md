# Workspace 错误码契约（前后端）

适用范围：`croupier` 的 `/api/v1/workspaces/**` 与 `croupier-dashboard` 的 Workspace 页面族。

## 1. 统一错误结构

后端返回：

```json
{
  "code": "workspace_not_found",
  "error": "workspace_not_found",
  "message": "workspace config not found",
  "request_id": "1741331289799108000"
}
```

字段语义：

- `code`：稳定错误码，前端以此决定提示和分支。
- `error`：兼容字段，保持与 `code` 一致。
- `message`：可读错误描述，允许直接展示。
- `request_id`：请求追踪 ID，用于日志与审计关联。

## 2. 错误码映射

- `unauthorized`：未认证（401）
- `forbidden`：无权限（403）
- `workspace_not_found`：对象配置不存在（404）
- `workspace_invalid_config`：配置非法（400/422）
- `workspace_publish_failed`：发布/取消发布失败（5xx）
- `workspace_version_not_found`：回滚版本不存在/无效
- `internal_error`：未分类服务端错误

## 3. 前端处理约定

前端统一通过 `src/services/workspace/errors.ts` 解析：

- 先按 `code` 映射用户可读文案
- 再按 HTTP 状态兜底
- 最后回退到默认文案

已接入页面：

- `Workspaces/index`
- `WorkspaceEditor/index`
- `Console/index`
- `WorkspaceRenderer/useWorkspaceConfig`

## 4. 审计关联约定

`workspace` 关键动作会写入后端审计并携带 `request_id`：

- `workspace.save`
- `workspace.publish`
- `workspace.unpublish`
- `workspace.rollback`
- `workspace.delete`

排障建议：优先用 `request_id` 在 API 错误响应、服务端日志、审计日志三处串联定位。
