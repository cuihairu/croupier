---
title: OpenAPI 函数注册
icon: code
order: 5
category:
  - 集成指南
tag:
  - OpenAPI
  - 函数注册
  - Agent
---

# OpenAPI 函数注册

> **状态**：Current -- OpenAPI 是 FunctionContract 和 CRUD capability 的输入，不是 PageSpec 或 UI schema。

OpenAPI 可以通过 SDK 本地解析或 Dashboard OpenAPI Source 上传接入。两种方式都先得到 FunctionContract，再由 Server 生成 CapabilitySemantics 和 PageProposal。

```text
OpenAPI
  -> FunctionContract
  -> CapabilitySemantics
  -> PageProposal
  -> Page Studio
  -> PublishedPageSpec
```

## 字段映射

| Croupier 字段             | OpenAPI 字段                     | 说明                                                 |
| ------------------------- | -------------------------------- | ---------------------------------------------------- |
| `id`                      | `operationId`                    | 稳定函数 ID                                          |
| `version`                 | `x-version`                      | 契约版本                                             |
| `summary` / `description` | 标准字段                         | 目录和诊断说明                                       |
| `input_schema`            | request body schema              | 输入 JSON Schema                                     |
| `output_schema`           | response schema                  | 输出 JSON Schema                                     |
| `resource`                | REST path 推导或 `x-resource`    | 资源 key                                             |
| `operation`               | method/path 推导或 `x-operation` | 动作 key                                             |
| `capability`              | REST 推导或 `x-capability`       | 受控能力语义                                         |
| `execution`               | `x-execution`                    | `sync` 或 `task`                                     |
| `approval`                | `x-approval`                     | `required` 与可选 `policyKey`；可与同步/异步执行组合 |
| `risk` / `permission`     | `x-risk` / `x-permission`        | 治理字段                                             |

REST 自动识别示例：

```yaml
paths:
  /players:
    get: # collection_query
      operationId: player.list
    post: # create
      operationId: player.create
  /players/{playerId}:
    get: # item_query
      operationId: player.get
    patch: # update
      operationId: player.update
    delete: # delete
      operationId: player.delete
```

Server 必须结合 path parameter、request/response JSON Schema 验证语义。若不确定 collection、identity 或 schema 关系，降级为 Operation Proposal/needs_review，绝不从 operationId 猜测。

## OpenAPI 输入边界

OpenAPI 只描述 API 契约和受控能力语义。以下页面信息必须由 PageProposal/PageSpec 或 Resource Catalog 处理，不允许进入 OpenAPI 导入结果：

- 页面 schema、组件树、组件 props 和布局 DSL。
- 分页绑定、表格列、typed selector、图表配置和任务事件绑定。
- 菜单、路由、分类/页面/按钮多语言显示名、按钮位置和页面类型。
- 任意浏览器运行时 target、scope、secret、connector URL 或 HTTP header。

导入器遇到这些页面字段必须返回 diagnostics，不能由 API 服务开发者把 Dashboard 设计耦合到 OpenAPI 文档。

## OpenAPI Source 与执行

上传 Source 不等于可执行：

```text
OpenAPI Source -> parse/validate -> FunctionContract candidate
Provider binding / controlled HTTP connector -> executable FunctionContract
```

未绑定 Provider 或受控 connector 的 Source 只能展示契约、语义和 diagnostics，不能发布可执行页面。Provider binding 按 `game_id + env` 隔离，浏览器不会看到 URL、Secret 或 target。

### Dashboard OpenAPI Source API

Source 与 Provider binding 由以下端点管理（认证 + `X-Game-ID`/`X-Env` scope）：

```http
GET    /api/v1/openapi/sources                           # Source 列表
POST   /api/v1/openapi/sources                           # 创建 Source（body: { name, spec }，≤2MiB；也支持 multipart 文件上传）
GET    /api/v1/openapi/sources/{sourceId}                # Source 详情（operations 分类结果、bindings、diagnostics）
PUT    /api/v1/openapi/sources/{sourceId}                # 更新 Source 文档
GET    /api/v1/openapi/sources/{sourceId}/diagnostics    # 仅诊断
POST   /api/v1/openapi/sources/{sourceId}/bindings       # 创建 Provider binding
DELETE /api/v1/openapi/sources/{sourceId}/bindings/{bindingId}
```

创建 binding 的请求体：

```json
{
  "operationId": "player.list",
  "kind": "provider",
  "functionId": "players.player.list",
  "bindingId": "player.list"
}
```

要点：

- `functionId` 必须已在当前 game/env 运行时注册（通常来自 Agent 侧 openapi provider 自动注册的 `players.player.list` 形式），否则返回 400。
- 绑定成功后同步物化 FunctionContract（source=`openapi`）、CapabilitySemantics 与受影响 Proposal；响应携带生成的 proposal 摘要。
- 删除最后一个 binding 后，该 Source 物化的合同回收，其余来源（如 SDK）的合同按剩余来源重建；已发布页面进入 stale。
- 不支持 URL 导入：文档必须内联提交（`spec` 字段或文件上传）；外部 `$ref` 被拒绝。

## 页面结果

- 标准 REST CRUD Resource 可生成 ready ResourcePage Proposal。
- 任意可执行同步 operation 至少可生成 basic OperationPage Proposal。
- `x-execution: task` 或受控 task semantic 可生成 TaskPage Proposal。
- `x-approval` 只声明治理要求；审批等待态和审批后的同步/任务结果由 Server 生成，不是独立页面类型。
- report capability 需要真实数据集/指标语义，信息不足时 needs_review。

用户在 Page Studio 显式接受并发布 Proposal；运行 Console 不直接读取 OpenAPI 或 tags 生成菜单。

详见 [Descriptor v2](../../architecture/openapi-sdk-descriptor-v2.md) 和 [Dashboard Page 模型](../../architecture/dashboard-page-model.md)。
