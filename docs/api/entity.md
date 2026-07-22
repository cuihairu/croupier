---
title: Resource API
icon: cubes
order: 98
category:
  - API 参考
tag:
  - ResourceSpec
  - OperationSpec
  - PageSpec
---

# Resource API

本文记录 Dashboard 页面生成使用的 Resource / Operation 查询接口。

`Resource` 是页面组织资源或能力域，不是数据库实体 CRUD API。业务对象的创建、更新、删除应通过函数注册和函数调用完成；Dashboard 页面由 `ResourceSpec + OperationSpec + PageSpec` 编排。

## 边界

| 概念 | 职责 |
| --- | --- |
| `FunctionSpec` | 单个函数能力、输入输出、风险和治理字段 |
| `ResourceSpec` | 页面组织用的资源或能力域 |
| `OperationSpec` | 某个函数在资源或页面中的业务动作和放置位置 |
| `PageSpec` | 完整业务页面编排 |

Resource API 只提供归一化查询和诊断，不提供通用实体 CRUD。

## 获取资源列表

```http
GET /api/v1/resources?category={category}
```

查询参数：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `category` | string | 可选，分类 key |
| `q` | string | 可选，资源 key 或显示名搜索 |

响应：

```go
type ResourceListResponse struct {
	Items []ResourceSpec `json:"items"`
}

type ResourceSpec struct {
	Key         string            `json:"key"`
	Labels      map[string]string `json:"labels"`
	Description map[string]string `json:"description,omitempty"`
	Category    ResourceCategory `json:"category"`
	Order       int               `json:"order,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Operations  []OperationSpec   `json:"operations,omitempty"`
	Diagnostics []ResourceDiagnostic `json:"diagnostics,omitempty"`
}

type ResourceCategory struct {
	Key    string            `json:"key"`
	Labels map[string]string `json:"labels"`
	Order  int               `json:"order,omitempty"`
}
```

## 获取资源详情

```http
GET /api/v1/resources/{resourceKey}
```

响应：

```go
type ResourceDetailResponse struct {
	Resource ResourceSpec `json:"resource"`
}
```

## 获取资源操作

```http
GET /api/v1/resources/{resourceKey}/operations
```

响应：

```go
type OperationListResponse struct {
	Items []OperationSpec `json:"items"`
}

type OperationSpec struct {
	FunctionID string            `json:"functionId"`
	ResourceKey string           `json:"resourceKey"`
	Operation  string            `json:"operation"`
	Kind       string            `json:"kind"` // list / get / create / update / delete / action / task / report
	Placement  string            `json:"placement"` // query / tableData / detailData / rowAction / toolbarAction / batchAction / standalone
	Labels     map[string]string `json:"labels"`
	Risk       string            `json:"risk,omitempty"`
	Enabled    bool              `json:"enabled"`
	Diagnostics []ResourceDiagnostic `json:"diagnostics,omitempty"`
}
```

## 获取资源默认页面建议

```http
GET /api/v1/resources/{resourceKey}/pages/generated
```

该接口返回基于当前 ResourceSpec / OperationSpec 生成的 PageSpec 建议。建议不是发布产物，必须进入 Page 工作台确认、编辑和发布。

```go
type ResourceGeneratedPagesResponse struct {
	Items []GeneratedPageSpec `json:"items"`
}
```

## 诊断

```go
type ResourceDiagnostic struct {
	Code       string `json:"code"`
	Severity   string `json:"severity"` // error / warning / info
	Message    string `json:"message"`
	FunctionID string `json:"functionId,omitempty"`
	Field      string `json:"field,omitempty"`
}
```

常见诊断：

- `resource_label_missing`：缺少资源多语言标题。
- `operation_kind_missing`：缺少页面生成语义。
- `placement_missing`：缺少页面放置位置。
- `output_schema_missing`：缺少输出结构，无法生成表格、详情或报表。
- `function_unavailable`：绑定函数没有可调用实例。

## 禁止项

- 禁止把 Resource API 当成通用实体 CRUD API。
- 禁止用 Resource API 直接修改业务对象数据。
- 禁止 Resource API 生成运行控制台菜单。
- 禁止缺少 `operationKind` 或 `placement` 时自动发布页面。

## 相关文档

- [Dashboard Resource/Page 模型](../architecture/dashboard-page-model.md)
- [Page 工作台 API](./workspace.md)
- [函数 API](./function.md)
