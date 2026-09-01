# 组件模板 API

> 状态：Current（V4 三层组合：函数 → 组件模板 → 页面）
> 模型：`internal/model/component_template.go`；handler：`internal/api/component/handler.go`

组件模板是「多函数封装为可复用组件」的载体（组合页编辑器 V4）。
所有端点在登录态 + scope 头（`X-Game-ID` / `X-Env`）下访问，
路径前缀 `/api/v1/component-templates`。

## 端点清单

| 方法   | 路径                                     | 说明                                          |
| ------ | ---------------------------------------- | --------------------------------------------- |
| GET    | `/api/v1/component-templates`            | 分页列表                                      |
| GET    | `/api/v1/component-templates/:key`       | 详情                                          |
| POST   | `/api/v1/component-templates`            | 创建（保存为组件）                            |
| PUT    | `/api/v1/component-templates/:key`       | 更新                                          |
| DELETE | `/api/v1/component-templates/:key`       | 删除（builtin 模板拒删）                      |
| POST   | `/api/v1/component-templates/regenerate` | 从当前 scope 契约重新生成内置模板（手动触发） |

## 响应契约

TemplateDTO（lowerCamelCase；`name`/`description` 为 `LocalizedText` JSON，
`tree`/`requiredFunctions` 为 JSON 值）：

```json
{
  "key": "player.crud",
  "name": { "zh-CN": "player 管理", "en-US": "Player CRUD" },
  "description": {},
  "category": "资源管理",
  "icon": "appstore",
  "requiredFunctions": [
    "player.list",
    "player.create",
    "player.update",
    "player.delete"
  ],
  "tree": [{ "type": "fnTable", "props": {} }],
  "builtin": false,
  "createdBy": "admin"
}
```

### 列表

`GET ?page=1&pageSize=50&category=资源管理&builtinOnly=false`

响应（裸 payload，无 envelope）：

```json
{ "items": [/* TemplateDTO */], "total": 24 }
```

### 创建

`POST` body（`key`/`name`/`tree` 必填）：

```json
{
  "key": "player.crud",
  "name": { "zh-CN": "player 管理", "en-US": "Player CRUD" },
  "category": "资源管理",
  "requiredFunctions": ["player.list"],
  "tree": [{ "type": "fnTable", "props": { "functionId": "player.list" } }]
}
```

错误：`400` `{ "error": "...", "message": "key 不能为空" }`；
key 重复 `409`。

### regenerate

`POST /regenerate` body `{ "keys": ["player.crud"] }`（空 = 全部 builtin）。
按当前 scope 的 FunctionContract 重新生成内置模板（scaffold 逻辑见
`internal/api/component/generator.go`）。**仅手动触发**——agent 注册
不会自动 regenerate。

## 前端本地可用性检查

无服务端 `check` 端点：组件库面板以当前 scope 函数集比对
`requiredFunctions`，缺失时组件卡置灰并提示缺少的函数
（`web/src/pages/PageStudio/CompositeEditor/ComponentLibrary.tsx`）。

## 已知边界

- 更新/删除对 `builtin=true` 模板受限（builtin 模板由 regenerate 维护）
- 模板 `tree` 的 PageNode 结构由前端组合编辑器定义，
  语义见 [组合页编辑器](../dashboard/composite-editor-v3.md)
