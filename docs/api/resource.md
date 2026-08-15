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

> **状态**：Current -- 路由注册于 `internal/handler/routes.go`。Resource 是 CRUD 与非 CRUD 页面生成的领域资源，事实来源是持久化 FunctionContract、ResourceCapability 与 CapabilitySemantics。

Resource Catalog 读取 scope 化、持久化的 FunctionContract、ResourceCapability 与 CapabilitySemantics，帮助用户理解平台为什么能够或不能生成默认页面。

注意两个前缀的分工：`/api/v1/resource-catalog` 是语义治理 API；`/api/v1/resources` 是函数目录侧的只读资源聚合（`GET /api/v1/resources`、`GET /api/v1/resources/{resourceKey}`、`GET /api/v1/resources/{resourceKey}/operations`）。

## 资源列表与详情

```http
GET /api/v1/resource-catalog
GET /api/v1/resource-catalog/{resourceKey}
```

```ts
interface ResourceCatalogItem {
  resourceKey: string;
  status: "identified" | "pending";
  functions: Array<{ functionId: string; capability: string; source: string }>;
  semantics?: {
    source: string; // sdk_explicit | openapi_rest | platform_review
    hasIdentity: boolean;
    identityField?: string;
    hasCollection: boolean;
    hasCreate: boolean;
    hasUpdate: boolean;
    hasDelete: boolean;
  };
  diagnostics: Diagnostic[]; // 如 resource_identity_not_verifiable
}
```

详情必须显示：

- collection、item、create、update、delete、action、task、report capability 的函数绑定。
- identity、collection response、识别来源、置信度、source digest 和 diagnostics。
- 哪些能力来自 OpenAPI REST、SDK 显式 capability 或管理员补充。
- 当前 Proposal 与 Published 页面受影响情况。

## 语义编辑与版本

```http
PUT  /api/v1/resource-catalog/{resourceKey}/semantics
GET  /api/v1/resource-catalog/{resourceKey}/semantics/versions
```

`PUT /semantics` 只编辑能力语义（identity、lifecycle binding、action subject、task/report semantic），不编辑 PageSpec。任何语义补充均需版本、权限、审计和 OTel，并触发受影响 Proposal 重建。

## 语义冲突与裁决

```http
GET  /api/v1/resource-catalog/{resourceKey}/conflicts
POST /api/v1/resource-catalog/{resourceKey}/conflicts/{field}/resolve
```

多来源语义（`platform_review > sdk_explicit > openapi_rest`）逐字段冲突时，管理员选择来源后更新 provenance/conflict resolution，并重建受影响 Proposal。

## Proposal 查询与重建

按资源过滤 Proposal 使用 [Proposal API](./page.md)：

```http
GET  /api/v1/proposals?resourceKey={resourceKey}
POST /api/v1/versioning/pages/{pageKey}/regenerate
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
