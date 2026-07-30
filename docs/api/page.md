---
title: Page Studio API
icon: object-group
order: 97
category:
  - API 参考
tag:
  - PageProposal
  - PageSpec
  - Page Studio
---

# Page Studio API

> **状态**：Target API -- Page Studio 管理 PageProposal、PageDraft 和 PublishedPageSpec，默认工作流是语义化编辑与发布。

Page Studio 管理当前 `game_id + env` scope 的 PageProposal、PageDraft、PublishedPageSpec 和三方合并。运行控制台只读取 PublishedPageSpec 和由它生成的 ConsoleMenuSpec。

## Scope

所有接口从统一请求上下文读取：

- `X-Game-ID`
- `X-Env`

URL、request body、selector 或业务 payload 均不能覆盖 scope。

## Proposal Inbox

```http
GET /api/v1/page-proposals?resourceKey={resourceKey}&quality={quality}&state={state}
```

响应：

```ts
interface PageProposalSummary {
  id: string;
  pageKey: string;
  kind: 'resource' | 'operation' | 'task' | 'report';
  resourceKey?: string;
  quality: 'ready' | 'basic' | 'needs_review' | 'blocked';
  sourceDigests: SourceDigest[];
  diagnostics: Diagnostic[];
  state: 'new' | 'superseded' | 'accepted' | 'stale';
}
```

Proposal 是可重新生成的默认建议，不会覆盖 Draft 或 Published 页面。

## 查看与接受 Proposal

```http
GET  /api/v1/page-proposals/{proposalId}
POST /api/v1/page-proposals/{proposalId}/accept
```

`accept` 将 Proposal 物化为新的 PageDraft。`ready/basic` 可在接受后立即进入发布校验；`needs_review/blocked` 返回结构化 diagnostics，不能发布。

## Draft API

```http
GET /api/v1/pages
GET /api/v1/pages/{pageKey}
PUT /api/v1/pages/{pageKey}
POST /api/v1/pages/{pageKey}/validate
POST /api/v1/pages/{pageKey}/publish
POST /api/v1/pages/{pageKey}/unpublish
GET /api/v1/pages/{pageKey}/versions
POST /api/v1/pages/{pageKey}/rollback
```

`PUT` 请求体是强类型 PageSpec，使用 `draftRevision` 乐观并发：

```ts
interface PageSaveRequest {
  draftRevision: number;
  spec: ResourcePageSpec | OperationPageSpec | TaskPageSpec | ReportPageSpec;
}
```

PageSpec 使用导航、列表、详情、表单动作、确认动作、任务、报表和 typed selector DTO。它不含具体 React 组件 props、无约束 JSON mapping 或浏览器可控 target。

## 变更与合并

```http
GET  /api/v1/pages/{pageKey}/changes
POST /api/v1/pages/{pageKey}/merge-proposal
```

Changes API 返回 base Proposal、latest Proposal、Draft、PublishedPageSpec 的差异，以及：

- 可自动合并的展示项。
- 需要用户决定的 binding、identity、selector、权限、风险、执行模式冲突。
- stale 原因和受影响页面执行状态。

`merge-proposal` 必须带 `draftRevision` 与每个冲突的显式决策；不得直接用最新 Proposal 覆盖 Draft。

## 发布

发布前校验：

- PageSpec version 和所有业务节点。
- Navigation 默认语言、binding、typed selector 与类型可赋值性。
- 函数可执行性、权限、风险、审批和 task/report 真实数据语义。
- 当前 scope 与 revision。

发布生成不可变 PublishedPageSpec，冻结 PageSpec、FormPresentationSpec、函数契约/语义摘要、权限、风险、执行模式、generator version 和 renderer version。

## 权限

- Proposal/草稿读取、diff、预览：`pages:read`。
- 接受 Proposal、保存草稿、解决合并：`pages:edit`。
- 发布/取消发布：`pages:publish`。
- 回滚：`pages:rollback`。
- Resource Catalog 语义补充：独立 `resources:semantics:edit`。

## API 边界

- Page API 只接受和返回强类型 PageSpec、Proposal、Draft、PublishedPageSpec 和 diff DTO。
- Page Studio 默认流程是语义化编辑；原始 JSON 仅用于导入、导出和受控诊断。
- Proposal 不能覆盖 Draft 或 Published 页面。
- Page Studio 只使用全局 scope，页面内不选择第二套 game/env。
- 页面 API 不接受 `functionId`、route、target 或 scope 的浏览器覆盖值。
