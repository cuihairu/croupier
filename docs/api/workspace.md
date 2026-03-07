---
title: Workspace API
icon: appstore
order: 99
category:
  - API 参考
tag:
  - workspace
  - 编排
---

# Workspace API

## 接口列表

- `GET /api/v1/workspaces/configs`：工作台配置列表
- `GET /api/v1/workspaces/published`：已发布工作台列表
- `GET /api/v1/workspaces/:objectKey/config`：读取工作台配置
- `PUT /api/v1/workspaces/:objectKey/config`：保存工作台配置
- `DELETE /api/v1/workspaces/:objectKey/config`：删除工作台配置
- `POST /api/v1/workspaces/:objectKey/publish`：发布工作台
- `POST /api/v1/workspaces/:objectKey/unpublish`：取消发布
- `GET /api/v1/workspaces/:objectKey/versions`：版本列表
- `POST /api/v1/workspaces/:objectKey/rollback`：版本回滚

## 状态机约定（V1）

状态枚举：`draft` / `published` / `archived`

- `PUT /:objectKey/config`（保存草稿）
  - `published=false` 时，状态归一为 `draft` 或 `archived`
  - 非法状态值会自动归一为 `draft`
- `POST /:objectKey/publish`
  - 状态强制归一为 `published`
- `POST /:objectKey/unpublish`
  - 状态归一为 `draft`
- `POST /:objectKey/rollback`
  - 按目标版本恢复配置后再次归一状态，随后创建新版本快照
- `DELETE /:objectKey/config`
  - V1 语义为物理删除；删除后不再返回配置记录

草稿与发布版并行策略：

- 同一 `objectKey` 仅维护一份当前配置记录（含 `published` 标记）
- 每次保存/发布/取消发布/回滚都会落版本快照，用于回溯

## 能力边界（V1）

以下能力在后端 `workspace` API 中尚未提供独立接口：

- `clone workspace`
- `import workspace`
- `export workspace`

前端应避免暴露“需要独立后端接口”的入口，避免形成伪能力。

## 统一错误结构

`workspace` 接口统一返回：

```json
{
  "code": "workspace_not_found",
  "error": "workspace_not_found",
  "message": "workspace config not found",
  "request_id": "1741331289799108000"
}
```

字段说明：

- `code`：前后端约定的稳定错误码（前端优先基于此处理）
- `error`：兼容旧错误处理链路，值与 `code` 保持一致
- `message`：可直接展示给用户/运维的错误信息
- `request_id`：请求追踪 ID（用于日志与审计定位）

## 错误码约定（V1）

- `unauthorized`：401，未登录或令牌失效
- `forbidden`：403，无权限执行操作
- `workspace_not_found`：404，配置不存在
- `workspace_invalid_config`：400/422，配置参数或结构非法
- `workspace_publish_failed`：发布/取消发布失败
- `workspace_version_not_found`：回滚版本不存在或版本号无效
- `internal_error`：服务端内部错误

## 审计覆盖

以下动作会写入审计日志：

- `workspace.save`
- `workspace.publish`
- `workspace.unpublish`
- `workspace.rollback`
- `workspace.delete`

审计日志包含：`action/userId/target/result/traceId(created request_id)/metadata/createdAt`。
