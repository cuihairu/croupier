# OpenAPI 请求契约

> **状态**：当前 —— 本文是 OpenAPI 操作（operation）统一请求表示的**唯一事实源**
> （SUI-001）。它定义规范数据结构、提取规则、面向操作者的输入投影、与旧平铺输入的
> 兼容边界，以及执行期到上游 HTTP 请求的映射。下游条目（SUI-002–005，以及 OpenAPI→UI
> 支持矩阵 SUI-047）以本文为准实现；任何偏离必须先回写本文。
>
> 相关：[OpenAPI / SDK Descriptor v2](./openapi-sdk-descriptor-v2.md)（注册字段与
> JSON Schema 支持子集）、[PageSpec 协议](./pagespec-protocol.md)（操作者侧表单与
> Selector AST 输入映射）。

## 动机

一个 OpenAPI operation 的输入由两种不同形态构成，而当前这两条推导路径各自为政：

1. **寻址参数** —— 固定位置参数，声明在 `in: path | query | header | cookie` 下，
   各自有序列化方式。
2. **请求体** —— 带媒体类型与 schema 的载荷，其根可能是 object、array 或标量。

当前有两条代码路径推导操作者侧输入 schema：

- **上传管线**（`internal/api/openapi` 的 `openAPIRequestSchema`）只把 `path`/`query`
  参数与 `application/json` object 体的属性压平成一个 object：它静默丢弃 `header`/`cookie`
  参数、让同名 body 属性覆盖参数、丢弃 body 根级约束，并在存在任意参数时丢弃根数组/
  标量 body；
- **运行时发现**（`internal/drivers/openapi` 的 `extractInputSchema` /
  `extractInputSchemaKin`）把**所有位置**的参数压平，并把非 object 体挂在 `body` 属性下
  —— 同一操作产生不同形态。

两条路径没有共享测试锁定，而执行器（`Provider.Call`）直接拒绝非 object 的请求 JSON，
因此根数组 body 永远送达不到上游。

本文统一表示，使两条路径推导出同一份规范契约，操作者侧输入成为该契约的确定性投影，
执行器消费同一契约构建实际 HTTP 请求。

## 规范表示

规范表示是**每 operation 一份可 JSON 序列化的请求契约**。权威 Go 类型位于
`internal/dashboard/spec`（`request_contract.go`）；JSON 键名遵循契约命名约定
（`lowerCamelCase`）。

### `OperationRequestContract`

```json
{
  "parameters": [
    {
      "name": "id",
      "in": "path",
      "required": true,
      "deprecated": false,
      "description": "player id",
      "serialization": { "style": "simple", "explode": false },
      "schema": { "type": "string" }
    }
  ],
  "body": {
    "mediaType": "application/json",
    "required": true,
    "schema": { "type": "object", "properties": { "name": { "type": "string" } } }
  },
  "diagnostics": []
}
```

| 字段 | 类型 | 含义 |
|---|---|---|
| `parameters` | `RequestParameterSpec[]` | 固定位置参数，声明顺序（operation 级在前、path-item 级在后；operation 级参数覆盖同 `(in, name)` 的 path-item 级参数）。 |
| `body` | `RequestBodySpec \| null` | 请求体槽位，operation 声明了请求体时存在。 |
| `diagnostics` | `Diagnostic[]` | 提取期诊断（同名冲突、非平铺形态）。见 [兼容边界](#兼容边界)。 |

### `RequestParameterSpec`

| 字段 | 类型 | 含义 |
|---|---|---|
| `name` | `string` | 参数名。 |
| `in` | `path \| query \| header \| cookie` | 固定位置。 |
| `required` | `bool` | 是否必填。 |
| `deprecated` | `bool` | 是否已废弃。 |
| `description` | `string` | 人类可读描述。 |
| `serialization` | `RequestSerialization` | 该位置的 OpenAPI `style` / `explode`（缺省取规范默认：path `simple`、query `form`、header `simple`、cookie `form`）。 |
| `schema` | `JSON Schema` | 参数 schema（对文档 `$ref` 内联后）。 |

### `RequestBodySpec`

| 字段 | 类型 | 含义 |
|---|---|---|
| `mediaType` | `string` | 载荷媒体类型，按执行器支持能力选择（SUI-007）：`application/json` 与 RFC-6839 `application/*+json` 后缀类型可执行；`application/json` 优先，其次 `+json` 后缀，其余字典序。见 [媒体类型选择](#媒体类型选择sui-007)。 |
| `required` | `bool` | 请求体是否必填。 |
| `schema` | `JSON Schema` | **完整** body schema，原样保留，含根 `type`（`object`/`array`/标量）与全部根级约束（`minProperties`、`additionalProperties` 等）。 |

## 提取规则

提取是单一函数、两个前端，二者必须一致：

- **kin 前端** —— 操作 `*openapi3.Operation`（上传管线与驱动 kin 路径使用）；
- **map 前端** —— 操作 `map[string]interface{}` 表示（驱动的 Swagger 2.0 兼容回落
  与配置文件 provider 使用）。

同一份 3.x 文档，两条前端产出相同的 `OperationRequestContract`；一致性由对拍测试锁定。
规则：

1. **参数**：先收集 operation 自身 `parameters`，再收集 path-item 级 `parameters`；
   同 `(in, name)` 的 operation 级参数替换 path-item 级。保持顺序（operation 级按声明
   顺序在前，未被覆盖的 path-item 级在后）。
2. **参数 schema**：对文档做 `$ref` 内联；无 schema 的参数兜底 `{ "type": "string" }`。
3. **序列化**：从参数推导 `style` / `explode`；缺省回退到该位置的 OpenAPI 默认
   （`path`→`simple`、`query`→`form`、`header`→`simple`、`cookie`→`form`）。
4. **请求体**：取执行器可执行的 content 条目（SUI-007；见
   [媒体类型选择](#媒体类型选择sui-007)）；body schema 原样保留（含根数组、
   标量与根级约束）。
5. **诊断**：见 [兼容边界](#兼容边界)。

## 面向操作者的输入投影

函数契约的输入 schema（操作者在表单里填写的内容）是请求契约的确定性投影。三种模式：

| 模式 | 条件 | 输入 schema |
|---|---|---|
| `flat` | 无 body，或可平铺 object body 且无参数同名冲突 | `{ "type": "object", "properties": { <参数> + <body 属性> }, "required": [...] }` |
| `structured` | body 不可平铺（根数组/标量，或带不可保留根级约束的 object），**或**参数名与 body 属性同名 | `{ "type": "object", "properties": { "params": { <参数> }, "body": <body schema> }, "required": [...] }` |
| `body_only` | 无参数且声明了 body | body schema 原样。 |

**可平铺** body：`application/json` 的 object schema，其根键全部落在
`{ type, properties, required, description, title, default, example }` 内。其余
（`minProperties`、`additionalProperties`、`patternProperties`、`allOf`/`oneOf`/`anyOf`、
非 object 根）都不可平铺。

**参数同名冲突**：(a) 同名参数出现在不同位置，或 (b) 参数名等于某个可平铺 body 属性名。

`flat` 投影对所有旧代码无损处理过的情形**字节稳定**：参数（含旧上传代码丢弃的 `header`/`cookie`）
按声明顺序成为属性，body 属性随后，`required` 去重并排序。空输入（无参数、无 body）投影为
`""`，向调用方发出「回退到运行时注册 schema」的信号，与旧上传路径完全一致。

`structured` 投影按命名空间隔离，天然无冲突：寻址参数在 `params` 下、整个 body 在 `body`
下。存在至少一个参数时 `params` 必填；`body.required` 为真时 `body` 必填。执行器从
`params.*` 读寻址值、从 `body` 原样读载荷。

`body_only` 投影用于无寻址参数且声明了 body 的情形，使所有 body 约束原样保留。

## 兼容边界

旧的**平铺**接口在投影模式为 `flat` 且参数位置为 `path`/`query` 的 operation 上被精确保留
（旧上传代码无损支持的那类）。这就是「已有无歧义对象输入维持原接口」的保证：重读已持久化
契约（旧绑定重读）与重新导入同一文档（重新导入）都重推导出完全相同的 schema。

无法由平铺投影无损表达的 operation **不被静默降级** —— 提取会输出 operation 级诊断
（`SeverityWarning`）并带字段级指针：

| 代码 | 条件 | 字段 |
|---|---|---|
| `openapi_request_parameter_collision` | 两个参数跨位置同名，或参数名等于某个可平铺 body 属性名 | `$.paths.<p>.<method>.parameters`（冲突的参数/属性名在 message 中） |
| `openapi_request_body_shape_nonflat` | body 根是数组/标量，或 object body 携带平铺可保留集合之外的根级约束 | `$.paths.<p>.<method>.requestBody` |
| `openapi_request_body_no_executable_media` | 声明的请求体媒体类型全部超出执行器能力（SUI-007） | `$.paths.<p>.<method>.requestBody`（message 中列出声明的媒体类型与可执行集合） |

投影仍产出可用的输入 schema（`structured` 或 `body_only`）—— 诊断是信息性的，不会让上传
失败。没有任何内容被丢弃：body 保持其形态挂在 `body` 槽位，每个参数保持其位置。

## 执行期映射

执行器（`internal/drivers/openapi` 的 `Provider.Call`）消费请求契约构建上游 HTTP 请求：

- `path` 参数替换 URL 模板中的占位符，按 `style`（`simple` / `label` / `matrix`）编码；
- `query` 参数追加到查询串，按 `style`（`form` / `spaceDelimited` / `pipeDelimited` /
  `deepObject`）与 `explode` 编码；
- `header` 参数设置请求头，按 `simple` 编码；
- `cookie` 参数设置 `Cookie` 头，按 `form` 编码（仅原始值）；
- body 按 `body.mediaType` 序列化（执行器能力选择，SUI-007）；`body` 值原样
  序列化（因此根数组/标量以该 JSON 类型送达），以所选媒体类型设置
  `Content-Type`，且**不**按已消费参数过滤。存量契约记录不可执行媒体类型时
  以提取侧诊断同文拒绝——JSON 载荷绝不在错标 `Content-Type` 下送出。

### 参数序列化（SUI-006）

执行器按 OpenAPI style/explode 规则序列化每个参数（参数值百分号编码，分隔符按
OpenAPI 附录保持字面）。公开声明、可执行的矩阵：

| 位置 | `style` | `explode` | 值形状 | 线上形态 |
|---|---|---|---|---|
| `path` | `simple` | 均支持 | 原始值 | `/users/{id}` → `/users/5` |
| `path` | `simple` | 均支持 | 数组 | `/users/1,2,3` |
| `path` | `simple` | `false` | 对象 | `/users/id,1,tag,alice` |
| `path` | `simple` | `true` | 对象 | `/users/id=1,tag=alice` |
| `path` | `label` | 均支持 | 原始值/数组/对象 | `/users/.5`（`.1,2` / `.1.2` / `.id=1.tag=alice`） |
| `path` | `matrix` | 均支持 | 原始值/数组/对象 | `/users/;id=5`（`;id=1;id=2` / `;id=1,tag=alice`） |
| `query` | `form` | `false` | 数组/对象 | `?list=1,2`（`?u=id,1,tag,alice`） |
| `query` | `form` | `true` | 数组/对象 | `?list=1&list=2`（`?id=1&tag=alice`） |
| `query` | `spaceDelimited` | `false` | 数组/对象 | `?s=1+2`（`?u=id+1+tag+alice`） |
| `query` | `pipeDelimited` | `false` | 数组/对象 | `?p=1\|2`（`?u=id\|1\|tag\|alice`） |
| `query` | `deepObject` | `true` | 对象 | `?m[key]=1&m[k2]=2` |
| `header` | `simple` | 均支持 | 原始值/数组/对象 | `X: 5`（`X: 1,2,3` / `X: id=1,tag=alice`） |
| `cookie` | `form` | 均支持 | 原始值 | `Cookie: token=8` |

`explode` 按位置取默认：`path`/`header` 默认 `false`，`query`/`cookie` 默认 `true`。
不可执行的组合（style 不属于该位置——`path` 里的 `form`、`query` 里的 `simple` 等；
`deepObject` 配 `explode=false`；`deepObject` 作用于非对象值；`cookie` 收到数组/对象值）
在两个提取前端都以诊断 `openapi_parameter_serialization_unsupported` 记录（severity
`error`，`field` = 参数名），执行器以同一原因拒绝请求，绝不静默错序列化。
`spaceDelimited` 分隔符以 `+` 发送（表单编码的空格，线上合法，解码后即空格连接列表）；
`|` 与 `,` 分隔符保持字面。

### 媒体类型选择（SUI-007）

请求体媒体类型从声明 content 条目中按**执行器能力**选择，而非仅按声明顺序。
执行器序列化 JSON 载荷，故可执行媒体类型恰为 JSON 媒体类型：`application/json`
与 RFC-6839 结构化语法后缀 `application/*+json`（`application/vnd.api+json`、
`application/hal+json` 等）。媒体类型参数（`charset`、`boundary` 等）与大小写忽略。

选择确定性，故**多媒体类型文档重复导入恒推导出同一 body 槽位**：

1. 可执行条目中 `application/json` 优先；
2. 其次 `application/*+json` 后缀条目，同 rank 按字典序；
3. 声明但不可执行的条目（`application/x-www-form-urlencoded`、
   `multipart/form-data`、`application/xml` 等）被跳过。

当操作声明了请求体但**无一媒体类型可执行**时，不设 body 槽位，提案以诊断
`openapi_request_body_no_executable_media`（severity `warning`，`field` =
`body`）记录原因；message 列出声明的媒体类型与可执行集合。诊断是信息性的——
上传仍成功，操作者据此看到为何不提供输入 body。这取代了旧行为（不可执行
媒体类型静默成为 body 槽位并以错标 JSON 送上上游）。Swagger 2.0 `consumes`
同规则：第一个可执行条目（声明顺序）获胜，全部不可执行时记录同诊断而非
body 槽位。

**输出 schema** 从声明的成功响应派生：200 优先，其余 2xx 升序，每个优先
`application/json`（仅当无 2xx 声明 JSON 媒体时才回退非 JSON 媒体）。运行时
驱动与上传管线都确定性地推导——map 迭代顺序绝不改变结果，故同一文档重复
导入得出同一输出 schema。

到达 `Call` 的输入 JSON 即投影输出。执行器从契约识别模式（而不是嗅探输入）：

- `flat` —— 从顶层键读寻址值；body 是移除已消费参数后的剩余部分；
- `structured` —— 从 `params.*` 读寻址值、从 `body` 读载荷（原样，可为数组/标量）；
- `body_only` —— 整个输入即载荷，原样发送。

对旧的配置文件 provider（显式 `parameters` / `request_body` 映射），契约从配置映射合成，
执行器保持旧行为。

## 已知边界

- `form-urlencoded` 与 `multipart` body 已声明但尚未执行（SUI-012 / SUI-013）。
  自 SUI-007 起被媒体类型选择**跳过**（不可执行）——仅声明此类媒体类型的操作
  得到无 body 槽位加 `openapi_request_body_no_executable_media` 诊断，而非
  静默错标 JSON 载荷；仍记入支持矩阵（SUI-047）。
- Swagger 2.0 文档只走 map 前端：`in: body` 参数成为 body 槽位，`in: formData` 参数
  被提升进 body object 属性。它们不参与 kin/map 对拍测试（kin 拒载 2.0）。
  其 `consumes` 媒体类型遵循 SUI-007 能力规则（第一个可执行条目；全部不可
  执行记录诊断）。
- 执行器对不可执行的序列化组合（见「参数序列化（SUI-006）」）与存量契约上的
  不可执行 body 媒体类型以错误拒绝，错误信息与提取侧诊断同文；绝不发送错
  序列化或错标的请求。
- 参数值嵌套超过一层（对象数组、含数组值的对象）不可序列化：值按标量处理，非标量
  项报错而不是送上变形的线上形态。
- 声明了 content 条目但 schema 缺失的条目不产生 body 槽位也不发诊断（旧
  「无 schema 即无 body」口径）；`openapi_request_body_no_executable_media`
  仅在声明了 schema 且无一媒体类型可执行时发出。
