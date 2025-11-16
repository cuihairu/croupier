Provider Manifest（语言无关）— 设计说明

目标
- 用一份语言无关的 JSON 清单宣告 Provider 能力（函数与实体/虚拟对象及其操作）。
- 基于清单驱动参数校验（JSON Schema）、UI 表单生成（hints）、权限/限流语义、以及传输映射（可选 Proto FQN）。
- 同时支持 in‑proc（Go 内嵌）与 out‑of‑proc（Python/Node 等独立进程）两种 SDK 形态。

核心概念
- provider：提供者元信息（id、version、language、sdk 版本等）。
- function：具名能力（建议命名为 `entity.operation`），包含请求/响应（JSON‑Schema 或 Proto FQN）、权限、语义（rate_limit/concurrency/idempotent）、传输映射、UI 提示。
- entity（实体/虚拟对象）：业务对象类型（可“虚拟”，仅有上下文/生命周期），含对象 schema 及一组操作（create/get/update/delete/custom…）。每个操作独立声明参数、权限、目标定位方式（如何找到某个对象实例）。
- context（隐式）：`game_id`、`env`、`actor`、`trace_id`、headers 等由网关注入，不进入 payload schema。

Manifest 文件
- JSON 文档（建议用 `docs/providers-manifest.schema.json` 校验）。
- 请求/响应可引用 JSON‑Schema（推荐）或 Proto FQN。
- 与 `schema/*.json`（各参数 Schema）以及可选的 FDS（.desc）一起打包分发。

注册流程（语言无关 SPI）
- Provider 进程加载 manifest 后，通过 ControlService 上报能力（可新增 RPC，或给现有 Register 扩展字段）：
  - 上报 `provider` 元信息、`functions[]`、`entities[]`，以及内嵌或外链的 JSON‑Schema（也可用内容哈希 + 上传端点）。
- Server 接收后合并为统一 descriptors，暴露在 `/api/descriptors`，供 UI/RBAC/校验使用。
- 调用链路使用 FunctionService（gRPC）；载荷默认 JSON；指定 `transport.proto` 时，Server/Edge 可用 FDS 做 JSON↔Proto 转换。

参数定义与校验
- 首选 JSON‑Schema：
  - 约束丰富：`required`、`min/max`、`enum`、`format`（email/hostname/ip/uri/date-time/color/json…）、`oneOf/anyOf` 等。
  - UI 提示：使用自定义扩展 `x-ui`（widget/options/placeholder/order）与 `x-mask`（敏感字段）。
- 可选 Proto 映射：设置 `transport.proto.request_fqn/response_fqn` 并随包提供 `.desc`。
- 参数来源控制：字段级 `x-source: body|query|path|header|meta`（meta 从 context 读取）。

虚拟对象（实体）
- `entities[]` 定义：`id`、`title`、`color`、`schema`（JSON‑Schema/Proto）与 `operations[]`。
- 操作字段：
  - `op`：如 `create`、`get`、`update`、`delete`、或 `custom_*`。
  - `target`：如何定位对象（如 `{ "field": "session_id" }` 或 `{ "jsonpath": "$.session.id" }`），create/list 可不需要。
  - `request/response`：Schema 或 Proto FQN。
  - `auth.require`：默认权限建议（如 `session:create`）。

错误模型
- Provider 处理器需返回带类型的错误：`invalid_argument`、`not_found`、`already_exists`、`precondition_failed`、`rate_limited`、`deadline_exceeded`、`unavailable`、`unauthorized`、`forbidden`、`internal`。
- Server/Edge 将其映射为 HTTP 状态码与“是否可重试”建议。

路由与语义
- `semantics.rate_limit`：如 `100/s`、`1000/m`，或对象 `{ value: 100, window: "1s" }`。
- `semantics.concurrency`：并发上限（整数）。
- `semantics.idempotent`：是否幂等。
- 负载均衡建议（可选）：`routing.hash_key`（字段名或 JSONPath），用于一致性哈希。

打包
- provider.tgz（或目录）包含：
  - `manifest.json`（本文件）
  - `schema/*.json`（引用的 JSON‑Schema）
  - 可选：`descriptors.fds`（FileDescriptorSet）
  - 可选：`ui/*`（UI 附加资源）

Manifest 示例（节选）
```json
{
  "provider": { "id": "player", "version": "1.2.0", "lang": "python", "sdk": "croupier-py@0.3.0" },
  "functions": [
    {
      "id": "player.ban",
      "request": { "json_schema": "schema/ban_request.json" },
      "response": { "json_schema": "schema/ban_response.json" },
      "auth": { "require": ["player:ban"] },
      "semantics": { "idempotent": true, "rate_limit": "100/s", "concurrency": 10 },
      "transport": { "proto": { "request_fqn": "croupier.player.v1.BanRequest", "response_fqn": "croupier.player.v1.BanResponse" } },
      "ui": { "category": "player", "risk": "medium" }
    }
  ],
  "entities": [
    {
      "id": "session",
      "title": "Session",
      "color": "#1677ff",
      "schema": { "json_schema": "schema/session.json" },
      "operations": [
        {
          "op": "create",
          "request": { "json_schema": "schema/create_session_request.json" },
          "response": { "json_schema": "schema/session.json" },
          "auth": { "require": ["session:create"] }
        },
        {
          "op": "close",
          "target": { "field": "session_id" },
          "request": { "json_schema": "schema/close_request.json" },
          "response": { "json_schema": "schema/empty.json" },
          "auth": { "require": ["session:close"] }
        }
      ]
    }
  ]
}
```

Proto‑First 生成
- 计划扩展 `tools/protoc-gen-croupier` 支持 `emit_manifest=true`：
  - 解析方法/消息及自定义注解，生成 `manifest.json` 与 `schema/*.json`。
  - 将 RPC 映射为 `functions[]`，消息映射为 JSON‑Schema。
  - 允许通过自定义 option 标注 `auth.require`、`semantics.*`、`entity/op/target`、`ui`。

控制面集成
- 扩展 ControlService（新增 RPC 或字段）以接收 Provider 能力载荷（压缩后的 manifest JSON + 可选嵌入的 schema/fds）。
- Server 合并为统一 descriptors 并暴露 `/api/descriptors`，供 UI 与校验使用。

注意
- JSON 文件尽量使用 ASCII；颜色按 `#1677ff` 六位十六进制。
- `json_schema` 的路径相对 manifest 或包根目录。
