# OpenAPI 3.x 请求契约

Source 按 operation 存储 `request`，Provider 对同一份文档派生同一个结构。它描述 HTTP 请求。页面布局、组件及样式继续通过数据库中的 UI 配置管理。

## 规范表示

- `parameters[]` 按位置和名称区分参数，保留 `in`、`name`、`required`、`schema`、`style`、`explode`、`allowReserved` 与 `allowEmptyValue`。使用 `content` 声明的参数记录各媒体类型及 schema，采用独立的序列化方式。
- operation 的同位置同名参数覆盖 path-item 的声明。名称保留原拼写，HTTP header 名称比较忽略大小写。JSON Pointer 指向采用的声明，引用保留目标地址。
- query 和 cookie 默认 `form`，path 和 header 默认 `simple`。`form` 默认 `explode: true`，其他 style 默认 `false`。参见 [OpenAPI 3.0.4 Parameter Object](https://spec.openapis.org/oas/v3.0.4.html#parameter-object)。
- `body` 保留请求体是否必填、声明位置及每个 `content[mediaType].schema`。schema 保留根类型及根级约束。JSON 值的缺省与显式 `null` 分开表达。执行器发送 `application/json` 请求体，并在请求体类型配置为表单时发送 `application/x-www-form-urlencoded`（SUI-012）：值经 `url.Values.Encode` 转义（空格为 `+`）、列表值保持同一 key 多个 entry、嵌套对象用展开式 `key[subkey]=value`；配置为 multipart 时发送 `multipart/form-data`（SUI-013，文件字段为 `{filename, content}` 上传对象、`content` 为 data URL 或原始 base64，上游收到解码后的字节）。但 Web Upload 控件仍产出文件 **URL** 而非文件字节，生成页面尚无法交付序列化所需载荷，故 multipart 在兼容性分级中保持 `manual`（SUI-047 矩阵跟踪）。
- Source 将契约随 operation 快照存储。旧 Source 行可以从已保存的 OpenAPI 文档重新派生。引用解析和独立 schema 的可执行边界由后续任务覆盖。

## 调用输入与兼容

无歧义的对象可以继续使用已有顶层字段。平铺只适用于名称唯一且能保留业务约束的对象。当参数发生同名冲突，或合并会丢失根级约束、请求体存在性等语义时，新的契约使用按位置分组的输入，如 `{"path":{"id":"p1"},"query":{"filter":"active"},"body":{"filter":2}}`。根数组和标量放在 `body`，可同时携带独立的 `query`。投影选择由请求契约决定，字段日志与诊断携带 operation ID 和声明位置。

显式 Provider 配置和 Swagger 2.0 继续使用既有映射。新派生的兼容诊断需在上传、重读及重新导入时可查。参数序列化组合、媒体类型协商、复杂引用及默认 UI 生成状态由后续任务覆盖。

[共享 fixture](https://github.com/cuihairu/croupier/blob/main/testdata/openapi/sui-request.yaml) 包含 path-item 继承、operation 覆盖、必填 header、cookie、query/body 同名，以及带 query 的根数组和标量。
