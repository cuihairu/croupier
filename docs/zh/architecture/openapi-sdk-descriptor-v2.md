---
title: OpenAPI / SDK Descriptor v2
icon: file-code
order: 6
category:
  - 系统架构
tag:
  - OpenAPI
  - SDK
  - 函数注册
  - PageSpec
---

# OpenAPI / SDK Descriptor v2

> **状态**：Current -- 本文是函数注册与页面自动生成的权威契约，已由各 SDK parity 测试与注册边界 guard（`internal/function/registrationguard/`）守护。SDK/OpenAPI 只负责可执行能力，不负责页面设计。
>
> **六语言 demo 契约同源**：六语言 SDK demo（go/python/java/js/csharp/cpp）向同一 `(game_id, env)` 注册同名函数，registry 最新注册胜出——任一语言的契约槽位（schema/文案/approval）漂移都会在其他 demo 重启时覆盖正确值，表现为 `function_contract_versions` 反复 updated 与已发布页面 bindingFreshness 永久 stale（2026-09 三起线上实证）。修改任一 demo 的契约必须六语言同步；CI 由 `scripts/check-demo-contract-baseline.sh`（文案基线 + 调用 `scripts/check_demo_contract_parity.py` 十栏逐槽互比，基准 = Go demo）守护，单语言独有函数不参与比对。

## 目的

OpenAPI 和 SDK descriptor 负责描述 **可执行能力**，而不是描述页面。平台据此识别 CRUD 和非 CRUD 语义，再生成可追溯的 PageProposal。

```text
OpenAPI operation / SDK descriptor
  -> FunctionContract
  -> CapabilitySemantics
  -> PageProposal
  -> PageSpec
```

详细页面模型见 [Dashboard Resource/Page 模型](./dashboard-page-model.md)。

## 注册字段

所有 SDK 必须收敛到同一 FunctionContract：

| 字段                      | OpenAPI 来源                    | SDK 字段       | 说明                                                                                             |
| ------------------------- | ------------------------------- | -------------- | ------------------------------------------------------------------------------------------------ |
| `id`                      | `operationId`                   | `id`           | 稳定函数 ID                                                                                      |
| `version`                 | 文档/服务版本                   | `version`      | 函数契约版本                                                                                     |
| `summary` / `description` | 标准字段                        | 同名字段       | 目录和诊断说明，非菜单事实                                                                       |
| `inputSchema`             | request body schema             | `inputSchema`  | 该 operation 的[请求契约](./openapi-request-contract.md)面向操作者的投影；参数/body 划分见该文档 |
| `outputSchema`            | response schema                 | `outputSchema` | 响应 JSON Schema                                                                                 |
| `resourceKey`             | REST path 推导或 `x-resource`   | `resource`     | 稳定业务资源 key                                                                                 |
| `operationKey`            | REST operation 或 `x-operation` | `operation`    | 业务动作 key                                                                                     |
| `capability`              | REST 形态推导或 `x-capability`  | `capability`   | 受控业务语义，非 UI                                                                              |
| `execution`               | 标准响应/`x-execution`          | `execution`    | `sync` 或 `task`                                                                                 |
| `approval`                | `x-approval`                    | `approval`     | 是否需要审批及可选 policyKey，与 execution 正交                                                  |
| `risk` / `permission`     | `x-risk` / `x-permission`       | 同名字段       | 治理约束；risk 为 `safe/warning/high/danger`                                                     |
| `enabled` / `deprecated`  | 标准字段                        | 同名字段       | 目录与执行开关；disabled 合同持久化保留但不可执行、不可发布                                      |
| `tags`                    | 标准字段                        | `tags`         | 目录与检索用途，非菜单事实                                                                       |

### 命名约定

FunctionContract 对外暴露的 SDK / JSON 字段名统一使用 lowerCamelCase（如 `inputSchema`、`outputSchema`、`approvalRequired`）。proto 字段名按 protobuf 规范保留 snake_case，其生成的 `json_name` 即 lowerCamelCase；文档、示例与 SDK 对外表层一律以 lowerCamelCase 指代契约字段，禁止把 proto 字段名直接透传为 JSON/SDK 键名。语言原生标识符遵循各自惯例（如 Python kwargs 为 snake_case），但 SDK 序列化到 JSON/wire 时必须使用本表的 lowerCamelCase 契约键。

`capability` 只允许：

```text
collection_query | item_query | create | update | delete | action | task | report
```

它回答“函数在资源生命周期中做什么”，不回答“按钮显示在哪里”或“页面长什么样”。

`execution: task` 可与 `approval.required: true` 组合，表示审批通过后再创建任务；`approval` 不是 `execution` 的第三个枚举值。SDK/OpenAPI 只能声明治理要求，页面等待态、审批状态和结果展示仍由 Server 生成。

`id`、`resourceKey` 和 `operationKey` 必须是稳定小写 key，格式为 `[a-z0-9][a-z0-9._-]*`。不符合格式的 SDK 注册或 OpenAPI 导入必须返回结构化错误；平台不得靠运行时编码、截断或大小写归一化修复身份。

## OpenAPI REST 自动识别

当 OpenAPI 同时满足稳定 path 和 schema 条件时，Server 必须产生高置信度 CRUD 语义：

| OpenAPI 操作                    | 默认 capability              | 额外验证                                 |
| ------------------------------- | ---------------------------- | ---------------------------------------- |
| `GET /players`                  | `collection_query`           | 可识别 collection schema；分页为可选能力 |
| `POST /players`                 | `create`                     | request body 对应资源输入                |
| `GET /players/{playerId}`       | `item_query`                 | path parameter 是资源 identity 候选      |
| `PUT/PATCH /players/{playerId}` | `update`                     | request body 与 identity 可关联          |
| `DELETE /players/{playerId}`    | `delete`                     | identity 可关联                          |
| 其他 REST 操作                  | `action` / `task` / `report` | 需显式 execution 或可验证响应特征        |

推导出的语义必须记录来源和置信度。REST path 不满足规范、identity 不可验证或 schema 矛盾时，平台降级为 OperationPage Proposal 并给出诊断，不得猜测成 Resource CRUD。

高置信度的可判定条件（全部满足才允许标记 `ready`）：

- path 符合 `{resource}` 或 `{resource}/{id}` 两段形态，resource 段为稳定名词。
- identity 必须在 collection item 或 item query 的输出 JSON Schema 中唯一确定；item/update/delete/action 所需 input 由后续 typed selector 显式映射并校验。REST path parameter 只是候选，不得作为唯一 identity 证据。
- `collection_query` 的 output schema 可识别为数组，或含明确 items 字段的分页包装。
- 同一 `resourceKey` 聚合的生命周期函数版本兼容、digest 可追溯。

任一条件不满足即降级并记录 diagnostic，不按“最像”猜测。SDK 侧参加 Resource CRUD 组合的条件是：`resourceKey + capability` 显式提供，且资源 identity 可在 item output schema 中唯一确定，或由 Resource Catalog 版本化补充；item/update/delete/action 的 input 由 typed selector 显式验证。否则只生成 OperationPage Proposal。

## SDK 函数语义

SDK 没有 HTTP method/path，不能只凭函数名生成完整 CRUD 页面。SDK 可以提供 `resourceKey + capability`，这是能力注册的一部分，不是 UI 配置：

```ts
registerFunction({
  id: "player.update",
  resource: "player",
  operation: "update",
  capability: "update",
  inputSchema: PlayerUpdateSchema,
  outputSchema: PlayerSchema,
  risk: "warning",
});
```

SDK 若只提供函数 ID 和 JSON Schema，仍可正常注册并获得默认 OperationPage；若要参加 Resource CRUD 组合，必须提供受控 capability 或由平台管理员在 Resource Catalog 补充语义。这个要求不会让 SDK 设计菜单、列、Form、页面或 mapping。

## 注册输入边界

SDK 和 OpenAPI Source 导入边界只接受 FunctionContract 字段。页面展示和编排信息必须留在 PageProposal/PageSpec 或 Page Studio 中，包括：

- 表格列、分页绑定、详情布局、按钮位置、图表配置和任务事件绑定。
- 菜单、路由、分类显示名、页面标题、按钮文案和多语言页面 labels。
- 任意浏览器运行时 target、scope、secret、connector URL 或 HTTP header。
- 任意组件树、组件 props、页面 mapping 或布局 DSL。

导入器遇到上述页面字段必须返回结构化 diagnostics，不得静默丢弃或转换。

## JSON Schema 支持子集

生成器只对以下子集保证生成可直接发布的表单与列表：object/array/scalar 类型、`required`、`enum`、格式 hint（`date`、`date-time`）和本地 `$defs`/`$ref`。`oneOf`/`anyOf`/`discriminator`、远程 `$ref` 等更复杂的组合不影响函数注册，但表单与列生成必须降级为 `needs_review` 并记录 diagnostic，不得硬吞后生成不可用的默认页面。

方言在判定前先归一（SUI-009）：OpenAPI 3.0 `nullable: true` 与 3.1 `type: [T, "null"]` 都统一为 2020-12 风格 `type` 数组（`"null"` 恒在末位），主类型（非 null）驱动控件，两侧声明同一可空语义。多类型联合（>1 个非 null 类型）、条件 schema（`if`/`then`/`else`、`dependentSchemas`）、元组（`prefixItems`、`items` 为数组）不能直接成表单 —— 降级为 `needs_review` 并记 `openapi_schema_dialect_keyword` 诊断（需 JSON 编辑或手工绑定 / discriminator），绝不静默误渲染。Go 判定器与 web 镜像对同一批样例判定一致（两侧 parity 测试锁定）。

方向也在表单见到 schema 之前解析（SUI-010）：喂给 `inputSchema` 的请求 schema 已剔除 `readOnly` 字段（`required` 收缩跳过它们），响应 schema 已剔除 `writeOnly` 字段 —— 客户端绝不为服务端拥有的值提供输入，也绝不把客户端提供的密钥当作服务端输出来校验。`openapi_schema_readonly_dropped` / `openapi_schema_writeonly_dropped` 诊断记录剔除；详见[请求契约](./openapi-request-contract.md)文档。

### 服务端校验策略（SUI-016）

Console 执行与函数直接调用用同一个判定引擎（`internal/validation.ValidateJSONSchemaVerdict`）执行 input schema，因此策略一致；共享对照 fixture 表（`internal/validation/validationtest`）被两个入口逐条断言，入口间分歧表现为 fixture 失败而非静默漂移。策略：**缺少 schema** 放行（无契约可验；游戏服务端仍是权威运行时校验方）；**schema 无法编译、或 payload 不是合法 JSON** 按缺陷拒绝，不静默当作无契约（fail-closed，与 ARCH-027 同向）；**契约违规**上报字段路径（缺必填单独聚合）。未知字段遵循标准 JSON Schema 语义：未显式声明 `additionalProperties: false` 即放行；`additionalProperties` 为 schema 形式时对未知值做校验；无 `properties` 的对象即自由字典、接受任意键。两个入口保留各自的 HTTP 形态（直接调用违规映射 400，console 绑定 payload 映射 422）。

### 请求兼容性分级（SUI-011）

分级在归一后的请求契约上判定（SUI-008/009/010 之后），因此分级结果不会与表单实际渲染的内容不一致。每个 operation 得到且仅得到一个层级，每一个拉低层级的构造都带字段路径上报 —— 即请求契约内的 JSON pointer：

| 声明的构造                                                                      | 层级       | 诊断                                                                                | 修复方式                                                                                          |
| ------------------------------------------------------------------------------- | ---------- | ----------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------- |
| object / array / 标量请求体、本地 `$defs`/`$ref`、`required`、`enum`、格式 hint | `direct`   | —                                                                                   | —                                                                                                 |
| `allOf`（任意位置）                                                             | `degraded` | `openapi_schema_compatibility_degraded`                                             | 拍平不保证字段顺序，改用 JSON 编辑                                                                |
| `oneOf` / `anyOf`                                                               | `degraded` | `openapi_schema_compatibility_degraded`                                             | 选定分支后用 JSON 编辑                                                                            |
| 根为 `array` 或裸标量值的请求体                                                 | `degraded` | `openapi_schema_compatibility_degraded`                                             | 在 `body` 下用 JSON 编辑                                                                          |
| 以非 JSON content 媒体类型声明的参数                                            | `degraded` | `openapi_schema_compatibility_degraded`                                             | 改用 JSON schema 声明该参数                                                                       |
| `discriminator`                                                                 | `manual`   | `openapi_schema_compatibility_manual`                                               | 以 `application/json` 发送具体变体                                                                |
| 无法解析的 `$ref`（目标缺失、循环或远程）                                       | `manual`   | `openapi_schema_compatibility_manual`（另有 `openapi_schema_reference_unresolved`） | 在源文档修正引用                                                                                  |
| 请求体只声明 `application/x-www-form-urlencoded`                                | `degraded` | `openapi_schema_compatibility_degraded`                                             | 配置执行器以表单发送（`requestBody.type: form`，SUI-012）                                         |
| 请求体同时声明 JSON 与 `application/x-www-form-urlencoded`                      | `degraded` | `openapi_schema_compatibility_degraded`                                             | 去掉多余媒体类型或只发 JSON                                                                       |
| 请求体只声明 `multipart/form-data`（或其它生成页面无法交付载荷的媒体类型）      | `manual`   | `openapi_schema_compatibility_manual`                                               | 声明 `application/json`；multipart 在 Upload 控件交付文件字节（而非 URL）前保持 manual（SUI-013） |
| 请求体同时声明 JSON 与此类媒体类型                                              | `manual`   | `openapi_schema_compatibility_manual`                                               | 去掉多余媒体类型或只发 JSON                                                                       |

执行器始终发送 `application/json` 请求体，请求体类型配置为表单时发送 `application/x-www-form-urlencoded`（SUI-012），配置为 multipart 时发送 `multipart/form-data`（SUI-013），因此缺少 JSON 媒体类型只有在生成页面无法交付载荷时才属于无法发送：`manual` 的 operation 不得生成可直接执行的默认提案。form-urlencoded 请求体不在该列 —— 它可以发送，但生成的页面无法表达执行器使用哪种序列化方式，因此不进入可直接执行层级。multipart 请求体执行器本身可发送，但 Web Upload 控件仍产出文件 URL 而非文件字节，页面无法提供序列化所需载荷，在上传控件值契约扩展出上传对象之前该 operation 保持 `manual`。

自 SUI-026 起，已声明非对象根的页内 JSON 编辑入口已经存在：契约根为 `array`、裸标量或 `null` 的生成页面（或组合页区块）渲染 `JsonRootEditor`——与结构化表单同一 handle 契约、校验器与执行链，payload 原样提交——而不再没有可编辑入口。变体渲染现状（SUI-042 已落地）：对象根且显式 `discriminator` 的多态 schema，可解析分支按所选分支合并 schema 渲染（交互式选择 discriminator 值，预置分支键）；不可解析（分支缺失/引用断裂/嵌套 discriminator）不猜分支，回退 `JsonRootEditor`。无 discriminator 的 `allOf` / `oneOf` / `anyOf` 仍走 JSON 编辑路径。渲染端能力不改变分级——discriminator 在驱动侧仍判 `manual`，默认提案依旧强制 `needs_review`。层级原因持续展示在 OpenAPI Sources 的 `requestCompatibility` 列（按 `functionId` 关联的 `openapi_schema_compatibility*` 诊断）。

### 输入支持矩阵（SUI-047）

按六个维度归纳请求输入在生成页面的落点。三种状态与分级对齐：**可自动生成并执行**（`direct`：结构化表单控件渲染，默认提案可直接接受执行）、**可用 JSON 编辑**（`degraded`：页内 `JsonRootEditor` 原样提交，无需改源）、**需要人工补绑定**（`manual`：默认提案强制 `needs_review`，须修源或人工审阅放行）。分级从声明面判定；下表同时如实记录执行面的现源行为——三处执行面缺口（朴素序列化、分组信封未解包、cookie 不发送）为 2026-11-26 SUI-047 走查实证，重落地跟踪见 todo.md。

| 维度     | 形态                                                              | 状态             | 对照样例                                                                                                    | 现源执行行为与边界                                                                                                                                                                                                                                                             |
| -------- | ----------------------------------------------------------------- | ---------------- | ----------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 参数位置 | `path` / `query` / `header` 参数（平铺投影）                      | 可自动生成并执行 | `GET /players/{playerId}?verbose`（`playerId` integer required、`verbose` boolean）→ 平铺表单字段           | path 值替换 URL 模板占位符、query 拼接查询串、header 写请求头；值为 `%v` 字符串直拼——**无 style/explode 区分、无百分号编码**（SUI-006 序列化器在 1deb91baf 合并中丢失，见[请求契约](./openapi-request-contract.md)回归标注；含保留字符的值会破坏请求，重落地前按朴素形态使用） |
| 参数位置 | 同名冲突或平铺约束触发分组投影（`$.path` / `$.query` / … 子对象） | 需要人工补绑定   | `DELETE /invites/{token}` 与 query `token` 同名 → 分组信封 `{path:{token},query:{token}}`                   | 表单渲染正常（分组子对象），但**执行链无解包步骤**——执行器按顶层名平铺取值，实测 `/players/{id}` 喂分组信封得 `/players/`（path 替换为空、query 丢失）；该形态无执行面测试，绑定前需人工验证或改契约避让                                                                       |
| 参数位置 | `cookie` 参数                                                     | 需要人工补绑定   | `GET /session` 声明 `in: cookie` 的 `sid` → 表单有 `$.cookie.sid` 字段                                      | 执行器没有 cookie 发送分支——值**静默不传输**（[请求契约](./openapi-request-contract.md)曾声明 Cookie 头映射，与现源不符）；需要会话态的接口应改用 header/query 或由网关注入                                                                                                    |
| 根数组   | object 根请求体                                                   | 可自动生成并执行 | `POST /players` 体 `{"name":"…","level":5}` → 结构化表单                                                    | 属性逐字段渲染，校验与执行走同一 schema                                                                                                                                                                                                                                        |
| 根数组   | `array` / 裸标量 / `null` 根请求体                                | 可用 JSON 编辑   | `POST /inventory/batch` 体根为 `[{"sku":"A01","qty":2}]` → `JsonRootEditor`                                 | payload 原样提交（根数组/标量直达上游），同一 handle 契约与执行链（SUI-026）                                                                                                                                                                                                   |
| 引用     | 本地 `$ref` / `$defs`                                             | 可自动生成并执行 | `{"$ref":"#/$defs/PlayerInput"}` → 内联展开成表单                                                           | 归一化（SUI-008）内联解析后与字面 schema 同路                                                                                                                                                                                                                                  |
| 引用     | 缺失 / 循环 / 远程 `$ref`                                         | 需要人工补绑定   | `{"$ref":"https://cdn.example.com/common.yaml#/Player"}` → `openapi_schema_reference_unresolved` + `manual` | 须在源文档修正引用后重导；生成器不给此类 operation 出可直接执行的默认提案                                                                                                                                                                                                      |
| 日期时间 | `format: date` / `date-time` / `time`                             | 可自动生成并执行 | `createdAt: {type:string, format:date-time}` → DateTimeWidget                                               | `date` 提交 `YYYY-MM-DD`、`date-time` 提交带显式本地偏移的 RFC3339（SUI-022）；`x-widget` hint（DatePicker/TimePicker）同路                                                                                                                                                    |
| 日期时间 | 未覆盖的 format（`uuid` / `duration` / 自定义）                   | 可自动生成并执行 | `traceId: {type:string, format:uuid}` → 普通文本框                                                          | 按 string 直传；格式权威校验在游戏服务端（schema 未声明的 format 不参与分级）                                                                                                                                                                                                  |
| 组合类型 | 无组合（单一 object）                                             | 可自动生成并执行 | `{"type":"object","properties":{…}}`                                                                        | —                                                                                                                                                                                                                                                                              |
| 组合类型 | `allOf` / `oneOf` / `anyOf`（无 discriminator）                   | 可用 JSON 编辑   | `allOf:[{$ref:Base},{properties:{ext:…}}]`；`oneOf:[Cat,Dog]` → `JsonRootEditor`                            | 拍平不保证字段顺序（allOf）、分支未结构化（oneOf/anyOf）；选定分支后 JSON 原样提交                                                                                                                                                                                             |
| 组合类型 | `discriminator`（对象根 + 显式 discriminator）                    | 需要人工补绑定   | Pet：`oneOf:[Cat,Dog]` + `discriminator:{propertyName:petType}`                                             | 分级 `manual` → 默认提案 `needs_review`；渲染端已支持可解析分支的交互式选择（SUI-042：Select + 按分支合并投影，不可解析回退 `JsonRootEditor`），提案仍须人工审阅放行                                                                                                           |
| 媒体类型 | 仅 `application/json`                                             | 可自动生成并执行 | `content:{application/json:{schema:…}}`                                                                     | JSON 序列化，根数组/标量原样到达上游                                                                                                                                                                                                                                           |
| 媒体类型 | 仅 form-urlencoded / JSON 与 form 并存                            | 可用 JSON 编辑   | `content:{application/x-www-form-urlencoded:{schema:…}}`                                                    | 执行器可发（`requestBody.type: form`，SUI-012）；页面无法表达序列化方式选择，故不进 direct                                                                                                                                                                                     |
| 媒体类型 | 仅 multipart 或其它不可交付媒体 / 与 JSON 并存                    | 需要人工补绑定   | `content:{multipart/form-data:{schema:…}}`                                                                  | 执行器可序列化 multipart（SUI-013），但 Upload 控件交付文件 URL 而非文件字节，生成页面无法交付载荷；控件值契约扩展前保持 `manual`                                                                                                                                              |

状态的消费方式：`direct` 的 operation 生成可直接执行的默认提案；`degraded` 提案不强制 `needs_review`，页内 JSON 编辑即可执行；`manual` 强制 `needs_review`，原因展示在 OpenAPI Sources 的 `requestCompatibility` 列。参数位置维度的三个执行面缺口（朴素序列化 / 分组信封未解包 / cookie 不发送）是分级引擎当前不感知的——它们不改变声明分级，已在本表与[请求契约](./openapi-request-contract.md)如实标注，修复由 todo 重落地项跟踪。

## OpenAPI Source 与执行绑定

上传 OpenAPI 单请求完成「上传即成页」全链生成（[契约与绑定正交化设计](./ui-generation-upload-pipeline.md) D1/D2/D4）：解析校验后，每个 operation 直接落库为 FunctionContract 物料（无对应运行时函数的置 `executionState=unbound`），同事务触发组件模板重建与 PageProposal 生成，响应携带 `{ operations, contractsCreated, templatesUpdated, proposalsCreated, diagnostics }` 摘要。契约存在不再以运行时注册为前提。

```text
OpenAPI Source
  -> parse / validate / normalize
  -> FunctionContract（unbound 物料；已有 bound 契约不降级、重复上传幂等）
  -> 组件模板重建 + CapabilitySemantics / PageProposal
  -> 运行时注册同名函数自动翻转 bound / 编辑器抽屉人工绑定
```

执行绑定是契约的**正交属性**：`unbound` 契约照常进入模板与提案管线、可保存发布；仅执行被阻断——binding execute 返回结构化 `409 executor_unbound`，前端渲染「未绑定执行器」空态并引导绑定。agent/SDK 注册同 scope 同 functionId 的函数时自动将对应 unbound 契约置 `bound`；不同名运行时函数可在编辑器属性面板的绑定抽屉或 OpenAPI Sources 页人工绑定（不同名绑定成功后即时清理被取代的 unbound 行）。

绑定与未来的受控 http connector 都必须按 `game_id + env` 隔离，并经过权限、审计与 OTel。OpenAPI 文档不允许包含 Secret、任意内网 URL 或页面配置。

## 生成边界

Server 以 FunctionContract + CapabilitySemantics 生成 Proposal：

- 标准 CRUD 语义生成 ResourcePage Proposal。
- 可执行但缺少 Resource 语义的同步函数生成 `basic` OperationPage Proposal。
- `execution=task` 生成 TaskPage Proposal；真实 task 状态契约不完整时为 `needs_review`。`approval.required=true` 则先显示审批等待态，批准后显示同步结果或任务状态。
- `capability=report` 生成 ReportPage Proposal；数据集/指标语义不完整时为 `needs_review`。

生成器不可读取或写入 SDK/OpenAPI 中的页面 UI。PageProposal 的列、映射、导航、表单展示和动作位置都由 Server 按生成规则产生，随后由 Page Studio 管理。

## 函数变更

每次 FunctionContract 或 CapabilitySemantics 变化必须：

1. 保存新版本和摘要。
2. 重新计算受影响的 ResourceCapability 与 PageProposal。
3. 与 PageDraft/PublishedPageSpec 的 source digest 做 diff。
4. 标记 stale，阻断不安全执行。
5. 允许用户在 Page Studio 查看、合并、修复并重新发布。

不得以最新函数 descriptor 静默替换已发布页面。

## SDK 交付要求

- 所有 SDK 对 `FunctionContract` 的字段语义一致；不支持的字段必须明确报错或在能力矩阵标记为未支持。
- Go 的 `RegisterFromOpenAPI` 只是本地 handler 注册 helper；其他 SDK 不能假称有同等能力。
- Demo 必须至少覆盖一个 CRUD Resource、一个 Operation、一个 Task 和一个 Report contract，并通过 Server 的 Proposal 生成 E2E。
- SDK 绝不携带页面 UI 或 PageSpec。
