---
title: Page Studio API
icon: object-group
order: 97
category:
  - API 参考
tag:
  - PageSpec
  - Page Studio
  - PublishedPageSpec
---

# Page Studio API

Page Studio API 管理当前 `game_id + env` scope 下的 PageSpec 草稿、校验、预览、发布和版本回滚。

Page Studio 是页面装配层，不是运行控制台。运行控制台只读取已发布的 `PublishedPageSpec` 和由它生成的 `ConsoleMenuSpec`。

## Scope

所有接口都从统一请求上下文读取作用域：

- `X-Game-ID`
- `X-Env`

请求 body、URL 和 binding payload 不允许覆盖 scope。

## PageSpec 草稿列表

```http
GET /api/v1/pages?resourceKey={resourceKey}&status={status}
```

响应：

```ts
interface PageDraftListResponse {
  items: PageSpecDraftSummary[];
}
```

## PageSpec 草稿详情

```http
GET /api/v1/pages/{pageKey}
```

响应返回 `PageSpec` 加草稿状态：

```ts
interface PageDraftResponse extends PageSpec {
  gameId?: string;
  env?: string;
  status: 'draft' | 'published' | 'archived';
  draftRevision: number;
  publishedVersion?: number;
  diagnostics?: Diagnostic[];
  updatedAt: string;
  updatedBy?: string;
}
```

## 保存草稿

```http
PUT /api/v1/pages/{pageKey}
Content-Type: application/json
```

请求体：

```ts
interface PageSaveRequest {
  draftRevision: number;
  type: 'entity' | 'operation' | 'task' | 'report';
  resourceKey?: string;
  title: Record<string, string>;
  description?: Record<string, string>;
  category: {
    key: string;
    labels: Record<string, string>;
    order?: number;
  };
  order?: number;
  icon?: string;
  schema: Record<string, unknown>;
  bindings: PageFunctionBinding[];
  metadata?: Record<string, unknown>;
}
```

约束：

- `draftRevision` 必填，冲突返回 409。
- `title.zh-CN` 和 `category.labels.zh-CN` 必填。
- `category.key` 缺失时，Server 只在保存时按 `resourceKey/pageKey` 的第一个 `.` 前缀确定一次。
- `schema` 必须是平台 Page 组件 ABI 支持的 Formily JSON Schema。
- Schema 组件只能引用 `bindingId`，不能引用裸 `functionId`。

## 校验草稿

```http
POST /api/v1/pages/{pageKey}/validate
```

响应：

```ts
interface PageValidateResponse {
  valid: boolean;
  diagnostics: Diagnostic[];
}
```

校验范围包括 PageSpec 基础字段、binding、Formily Page 组件 ABI、组件 props、发布期 schemaVersion 和函数契约可用性。

## 预览草稿

```http
POST /api/v1/pages/{pageKey}/preview
```

预览只返回当前草稿 PageSpec，不写入运行控制台菜单。

## 发布

```http
POST /api/v1/pages/{pageKey}/publish
Content-Type: application/json
```

请求体：

```ts
interface PagePublishRequest {
  draftRevision: number;
}
```

发布成功后生成当前 scope 下不可变 `PublishedPageSpec` 快照，并冻结 binding 的函数版本、输入/输出 schema digest、风险、执行模式和 renderer schema version。

## 取消发布

```http
POST /api/v1/pages/{pageKey}/unpublish
```

取消发布只让运行控制台菜单不再展示该页面，不删除 PageSpec 草稿和版本历史。

## 版本列表

```http
GET /api/v1/pages/{pageKey}/versions
```

响应：

```ts
interface PageVersionsResponse {
  currentDraftRevision: number;
  currentPublishedVersion?: number;
  items: PageVersionItem[];
}
```

`page_versions` 以 `(game_id, env, page_key, version)` 作为唯一逻辑版本。保存草稿和发布同一 revision 时更新同一版本记录，不生成重复版本。

## 版本详情

```http
GET /api/v1/pages/{pageKey}/versions/{versionId}
```

返回历史版本中的完整 PageSpec，用于查看、diff 和回滚。

## 回滚

```http
POST /api/v1/pages/{pageKey}/rollback
Content-Type: application/json
```

请求体：

```ts
interface PageRollbackRequest {
  versionId: string;
}
```

回滚会基于历史 PageSpec 创建新的草稿 revision，不会直接改变运行控制台发布态。

## 权限

- 读取草稿、校验、预览、版本：`pages:read`，或具备 `pages:edit/pages:publish/pages:rollback`
- 保存草稿：`pages:edit`
- 发布和取消发布：`pages:publish`
- 回滚：`pages:rollback`

操作者字段只从服务端上下文获取，客户端提交的 `updatedBy/publishedBy/createdBy` 不可信。

## 禁止项

- 禁止恢复已移除的页面配置模型或非 Formily 页面协议。
- 禁止在 Page Studio 内再选择一套 `game_id/env`。
- 禁止把函数注册字段当作菜单、页面标题或按钮文案来源。
- 禁止发布缺少默认语言 labels、binding、mapping 或组件 ABI 校验失败的 PageSpec。
