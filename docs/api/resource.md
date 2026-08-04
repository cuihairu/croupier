---
title: Resource Catalog API
icon: cubes
order: 98
category:
  - API 参考
tag:
  - ResourceCapability
  - CapabilitySemantics
  - PageProposal
---

# Resource Catalog API

> **状态**：Target API -- Resource 是 CRUD 与非 CRUD 页面生成的领域资源，事实来源是持久化 FunctionContract、ResourceCapability 与 CapabilitySemantics。

Resource Catalog 读取 scope 化、持久化的 FunctionContract、ResourceCapability 与 CapabilitySemantics，帮助用户理解平台为什么能够或不能生成默认页面。

## 资源列表

```http
GET /api/v1/resources?q={query}&state={state}
```

```ts
interface ResourceSummary {
  key: string;
  labels: LocalizedText;
  state: 'ready' | 'needs_review' | 'conflict' | 'unavailable';
  capabilities: CapabilitySummary[];
  proposalCounts: Record<'ready' | 'basic' | 'needs_review', number>;
  blockedIssueCount: number;
  diagnostics: Diagnostic[];
}
```

## 资源详情与能力语义

```http
GET /api/v1/resources/{resourceKey}
GET /api/v1/resources/{resourceKey}/semantics
GET /api/v1/resources/{resourceKey}/history
PUT /api/v1/resources/{resourceKey}/semantics
```

详情必须显示：

- collection、item、create、update、delete、action、task、report capability 的函数绑定。
- identity、collection response、识别来源、置信度、source digest 和 diagnostics。
- 哪些能力来自 OpenAPI REST、SDK 显式 capability 或管理员补充。
- 当前 Proposal 与 Published 页面受影响情况。

`PUT /semantics` 只编辑能力语义，不编辑 PageSpec。任何语义补充均需版本、权限、审计和 OTel。

## Proposal

```http
GET /api/v1/resources/{resourceKey}/page-proposals
POST /api/v1/resources/{resourceKey}/page-proposals/rebuild
```

重建 Proposal 只依据持久化 FunctionContract 与 CapabilitySemantics。它不从函数名、浏览器请求、静态 locale 或任意历史页面结果猜测 UI。

## 资源边界

- Resource CRUD 页面是默认高质量路径，但 Resource 不是数据库表强绑定，也不开放通用数据库 CRUD API。
- 业务数据只能通过已注册函数和 PublishedPageSpec binding 执行。
- Operation、Task、Report 可关联 Resource，也可独立存在。
- Resource Catalog 不生成 Console 菜单；菜单仅由 PublishedPageSpec 生成。

## API 边界

- Resource API 读取持久化语义事实，不把内存 Registry 聚合作为唯一事实。
- Resource API 只编辑 CapabilitySemantics，不接受页面列、动作位置、导航、表单展示或 mapping。
- Resource API 不直接修改业务对象；业务数据只能通过 PublishedPageSpec binding 执行。
