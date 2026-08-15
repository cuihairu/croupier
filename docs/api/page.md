---
title: 页面与控制台 API
icon: object-group
order: 97
category:
  - API 参考
tag:
  - PageProposal
  - PageSpec
  - Page Studio
  - Console
---

# 页面与控制台 API

> **状态**：Current -- 路由注册于 `internal/handler/routes.go`（Proposal / Pages / Versioning / Console 四组）。默认工作流是语义化编辑与发布。

这组 API 管理当前 `game_id + env` scope 的 PageProposal、PageDraft、PublishedPageSpec、三方合并与 Console 运行时。运行控制台只读取 PublishedPageSpec 和由它生成的 ConsoleMenuSpec。

## Scope

所有接口从统一请求上下文读取：

- `X-Game-ID`
- `X-Env`

URL、request body、selector 或业务 payload 均不能覆盖 scope。

## Proposal 与 Inbox

```http
GET  /api/v1/proposals?status={status}&resourceKey={resourceKey}
GET  /api/v1/proposals/inbox
GET  /api/v1/proposals/{proposalKey}
POST /api/v1/proposals/{proposalKey}/accept
POST /api/v1/proposals/{proposalKey}/accept-and-publish
POST /api/v1/proposals/{proposalKey}/reject
```

`proposalKey` 是生成器幂等身份：`resource:<resourceKey>` 或 `<kind>:<functionId>`（如 `operation:mail.send`）；URL 中需编码 `:`（`operation%3Amail.send`）。

- 列表返回 ProposalDTO 数组（含 pageKey/pageType/quality/status/pageSpec/diagnostics）。
- `inbox` 返回 Proposal Inbox 四队列：`publishable`（ready/basic）、`needsReview`、`blockedIssues`、`contractChanges`（stale 的 draft/published 页，含 `bindingFreshness` 诊断），以及 `summary` 计数。
- `accept` 将 Proposal 物化为新的 PageDraft；`accept-and-publish` 一步接受并发布（仅 `ready`/`basic` 可用）。
- `needs_review` 返回结构化 diagnostics，必须处理后才能发布。不可物化的问题进入 BlockedProposalIssue，不能作为 Proposal quality。
- Proposal 是可重新生成的默认建议，不会覆盖 Draft 或 Published 页面。

## Draft API（页面管理）

```http
GET  /api/v1/pages
GET  /api/v1/pages/{pageKey}
PUT  /api/v1/pages/{pageKey}
POST /api/v1/pages/{pageKey}/regenerate
POST /api/v1/pages/{pageKey}/validate
POST /api/v1/pages/{pageKey}/preview
POST /api/v1/pages/{pageKey}/publish
POST /api/v1/pages/{pageKey}/unpublish
GET  /api/v1/pages/{pageKey}/versions
GET  /api/v1/pages/{pageKey}/versions/{versionId}
POST /api/v1/pages/{pageKey}/rollback
```

`PUT` / `publish` / `rollback` 请求体使用 `draftRevision`（或 `expectedDraftRevision`）乐观并发，不匹配返回 409。

PageSpec 使用导航、列表、详情、表单动作、确认动作、任务、报表和 typed selector DTO（见 [PageSpec 协议规范](../architecture/pagespec-protocol.md)）。它不含具体 React 组件 props、无约束 JSON mapping 或浏览器可控 target。

## 变更链与三方合并（Versioning）

```http
GET  /api/v1/versioning/pages/{pageKey}/chain
GET  /api/v1/versioning/pages/{pageKey}/diff
POST /api/v1/versioning/pages/{pageKey}/merge
POST /api/v1/versioning/pages/{pageKey}/rollback-draft
POST /api/v1/versioning/pages/{pageKey}/rollback-publish
POST /api/v1/versioning/pages/{pageKey}/regenerate
POST /api/v1/versioning/pages/{pageKey}/republish
```

`diff` 返回 base Proposal、latest Proposal、Draft、PublishedPageSpec 的差异，以及：

- 可自动合并的展示项（`autoMergeItems`）。
- 需要用户决定的 binding、identity、selector、权限、风险、执行模式冲突（`conflictItems`）。
- stale 原因和受影响页面执行状态。

`merge` 请求必须带 `expectedDraftRevision` 与 `strategy`：

```json
{
  "expectedDraftRevision": 2,
  "strategy": "auto | manual",
  "dryRun": false,
  "reason": "接受新增必填字段",
  "conflicts": [{ "path": "operation.form.jsonSchema", "acceptNew": true }]
}
```

- `strategy: manual` 必须精确覆盖当前冲突集（缺/多/重复均 422）；全量接受（accept-all）被禁止。
- `auto` 只落安全集；冲突保留，`BaseProposalVersion` 不前进。
- `dryRun: true` 只返回预览不落库。

## Console 运行时 API

```http
GET  /api/v1/console/menu
GET  /api/v1/console/pages
GET  /api/v1/console/pages/{pageKey}
POST /api/v1/console/pages/{pageKey}/bindings/{bindingId}/execute
```

- `menu` 返回由 active PublishedPageSpec 生成的 `ConsoleMenuSpec`（`{ items: [...] }`），是动态菜单唯一来源。
- `pages/{pageKey}` 返回发布快照与 `bindingContracts`（函数版本、schema digest、risk、permission、approval、renderer 版本）及只读 `bindingFreshness` 诊断。
- `execute` 请求只提交 selector context：

```json
{ "context": { "form": {}, "row": {}, "selection": [], "pageState": {} } }
```

服务端按发布快照校验 binding、stale、权限、审批后组装 payload 并 dispatch。合同变化的 stale 页返回 `409 { "error": "binding_stale", "details": { "statuses": [...] } }`。浏览器不得提交 functionId、target、gameId、env。

## 发布

发布前校验：

- PageSpec version 和所有业务节点。
- 导航默认语言、binding、typed selector 与类型可赋值性。
- 函数可执行性、权限、风险、审批和 task/report 真实数据语义。
- 当前 scope 与 revision。

发布生成不可变 PublishedPageSpec，冻结 PageSpec、FormPresentationSpec、函数契约/语义摘要、权限、风险、执行模式、generator version 和 renderer version。

## 权限

- Proposal/草稿读取、diff、预览：`pages:read`。
- 接受 Proposal、保存草稿、解决合并：`pages:edit`。
- 发布/取消发布：`pages:publish`。
- 回滚：`pages:rollback`。
- Console 执行：`function:invoke` 及发布快照中的 permission。
- Resource Catalog 语义补充：独立 `resources:semantics:edit`。

## API 边界

- Page API 只接受和返回强类型 PageSpec、Proposal、Draft、PublishedPageSpec 和 diff DTO。
- Page Studio 默认流程是语义化编辑；原始 JSON 仅用于导入、导出和受控诊断。
- Proposal 不能覆盖 Draft 或 Published 页面。
- Page Studio 只使用全局 scope，页面内不选择第二套 game/env。
- 页面 API 不接受 `functionId`、route、target 或 scope 的浏览器覆盖值。
