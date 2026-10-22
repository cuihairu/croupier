# SDK OpenTelemetry Trace 传播详解

状态：已评审（2026-08-21）——平台侧已完成，SDK 侧分期实施
关联：`docs/analytics/opentelemetry-integration.md`（平台侧集成指南）、`docs/architecture/sdk-wire-protocol.md`（trace 字段协议层约定）、`configs/telemetry.example.yaml`（配置样例）

## 结论速览

| 决策项                           | 结论                                                    |
| -------------------------------- | ------------------------------------------------------- |
| 平台侧 OTel 埋点                 | 保留（已实现并验收，J-001.02）                          |
| SDK 六语言 trace 传播            | 值得做，分两期：Go/Python/JS 一期；Java/C#/C++ 二期按需 |
| SDK 内置 exporter / 业务自动埋点 | **不做**——业务观测归游戏方自己的 OTel 体系              |

## Trace 是什么、解决什么问题

一次 GM 操作（例如"封禁玩家"）会穿过四层进程：

```
Dashboard/脚本 ──HTTP──> Server ──TCP──> Agent ──本地调用──> Provider(游戏进程)
```

任何一层都可能出问题：权限拒绝、负载均衡选错 Agent、Provider 处理超时……
如果没有 trace，排障时只能各层分别翻日志，靠时间戳人工对齐。

**Trace 把一次请求在所有层产生的记录用同一个 ID（trace_id）串联。**
每层再各自上报 span（起始时间、耗时、错误、属性）到 OTel 后端
（Jaeger/Grafana Tempo/商业 APM），排障时打开一条瀑布图即可看到每一跳。

Croupier 的实现遵循两个业界标准，使用者无需学习私有概念：

- **W3C Trace Context**（`traceparent` 头）：跨进程传播 trace 的标准格式
- **OpenTelemetry（OTel）**：span 生成与上报的事实标准 SDK

## 端到端链路全景

```
调用方                     Server                        Agent                  Provider(SDK)
─────────────────────────────────────────────────────────────────────────────────────────────
                      function.invoke (SpanKind=Server)
                           │ trace 开始（或延续外部 traceparent）
                           │ telemetry.InjectContext(ctx, metadata)
                           │   metadata["traceparent"] = "00-<traceId>-<spanId>-01"
                           │   metadata["traceId"]     = "<traceId>"   ← 冗余明文，便于弱端读取
                           ▼
                      function.dispatch.invoke (SpanKind=Client)
                           │ ExtractContext 延续 trace → 选 Agent → TCP 转发
                           ▼
                      agent.invoke (SpanKind=Server)
                           │ ExtractContext 延续同 trace
                           │ metadata 再次 InjectContext
                           ▼
                      sdk.invoke / function.{id}（规划中，SpanKind=Server）
                           │ SDK 提取 traceparent 为父上下文（尚未实现，见"SDK 现状"）

响应回程：InvokeResponse.traceId = "<traceId>"（调用方可拿去 Jaeger 查询）
审计落库：AuditContext.TraceID 同值，与审计记录永久关联
```

要点：

- **每个进程内 span 自动父子相连**：`function.dispatch.invoke` 的 parent 是
  `function.invoke`，`agent.invoke` 的 parent 是 `function.dispatch.invoke`，
  在 Jaeger 中呈现为一条瀑布
- **跨进程靠 metadata 携带 `traceparent`**：不是魔法，就是请求 metadata map
  里多了一个标准格式的字符串
- **调用方带 trace 进来时平台延续之**：外部系统（游戏自己的服务）若已在
  HTTP 头携带 `traceparent`，Server 会将其作为整条链路的根，而非另起一条

## Span 目录

| Span 名称                  | 进程   | SpanKind | 代码位置                                       | 关键属性                                           |
| -------------------------- | ------ | -------- | ---------------------------------------------- | -------------------------------------------------- |
| `function.invoke`          | Server | Server   | `internal/api/function/helpers.go:251`         | `function.id` / `function.mode` / `function.route` |
| `function.dispatch.invoke` | Server | Client   | `internal/platform/dispatch/dispatcher.go:253` | `function.id`                                      |
| `agent.invoke`             | Agent  | Server   | `internal/agent/local_handler.go:197`          | `agent.id` / `function.id` 等                      |
| `agent.task.execute`       | Agent  | Consumer | `internal/agent/local_handler.go:411`          | `agent.id` / 任务属性                              |

SpanKind 语义：`Server` = 承接入站请求，`Client` = 发起出站调用。在 Jaeger
的视图中这决定了跨进程连线的画法。

## Metadata 协议字段

定义于 `internal/telemetry/context.go`，随 invoke 请求的 metadata map 下发：

| 字段          | 格式                                                        | 用途                                          |
| ------------- | ----------------------------------------------------------- | --------------------------------------------- |
| `traceparent` | W3C：`00-{32位hex traceId}-{16位hex spanId}-{2位hex flags}` | 标准跨进程传播字段，otel SDK 可直接 Extract   |
| `tracestate`  | W3C：厂商扩展键值对（通常为空）                             | 透传保留                                      |
| `traceId`     | 32 位 hex 明文                                              | 冗余字段：不解析 `traceparent` 的简单端直接读 |

W3C `traceparent` 示例：

```
traceparent: 00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01
              │  │──────────── traceId ───────────││─ spanId ─││└ flags(采样位)
              └─ version
```

写 SDK 传播时的两个入口函数：

```go
// 下发方向（Server/Agent 已实现）：把当前 ctx 的 trace 上下文写入 metadata
metadata = telemetry.InjectContext(ctx, metadata)

// 接收方向（SDK 需要实现的）：从 metadata 恢复带父 trace 的 ctx
ctx = telemetry.ExtractContext(ctx, metadata)
```

不引入 otel 库也能实现——手动拼/读 `traceparent` 字符串即可（约 30 行）。

## trace 的生命周期

### 1. 生成（根 span）

- 调用方**不带** `traceparent`：Server 的 `function.invoke` 自动开新 trace
  （零侵入：所有现有调用天然有 trace，前提是开启了 tracing 导出）
- 调用方**带** `traceparent`（如游戏侧已有 OTel）：平台延续该 trace，
  全链路并入调用方的观测体系

### 2. 透出（调用方可见）

同步调用的 HTTP 响应体携带 `traceId`（`internal/api/function/dto.go:176`）：

```json
{
  "result": { ... },
  "traceId": "0af7651916cd43dd8448eb211c80319c"
}
```

各语言 SDK 的响应 DTO 均保留该字段（Go：`types.go` `TraceID`；C++：
`GetTaskStatus` 解析 `traceId`）。

### 3. 关联（审计）

审计记录（`internal/audit/audit.go:182` `AuditContext.TraceID`）与 trace 同 ID。
排查一次危险操作时，可在审计页拿到 traceId 再去 Jaeger 还原完整执行过程——
审计回答"谁在何时做了什么"，trace 回答"每一步实际发生了什么"。

### 4. 消费（Dashboard 跳转）

- 函数调用页（`web/src/pages/Functions/Invoke/InvocationResponse.tsx`）：响应含
  traceId 时展示"在 Jaeger 中打开"链接（`${jaegerUrl}/trace/${traceId}`）
- 链路追踪页（`/traces`）：不做 trace 列表/详情查询（避免重复造 Jaeger），
  仅提供 traceId 输入 + 跳转；支持 `?traceId=...` 从其他页面带参跳入

跳转地址来自运维配置（见下节），需配置后按钮才可用。

## 配置指南

### Server/Agent 侧（决定"有没有 trace"）

```yaml
# configs/telemetry.example.yaml
telemetry:
  service_name: croupier-server # OTEL_SERVICE_NAME
  collector_url: http://otel-collector:4318 # OTEL_EXPORTER_OTLP_ENDPOINT（OTLP HTTP）
  enable_tracing: true # OTEL_ENABLE_TRACING
  sampling_ratio: 1.0 # OTEL_SAMPLING_RATIO（1.0 = 全采样）
```

或等价环境变量：

```bash
OTEL_SERVICE_NAME=croupier-server
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318
OTEL_ENABLE_TRACING=true
```

未开启 tracing（或 collector 不可达）时：**传播字段照常注入 metadata**
（`traceparent`/`traceId` 仍存在），但 span 不会被导出，响应 `traceId`
为空。属部署配置，非代码缺口。

### Dashboard 跳转（决定"能不能一键跳"）

```bash
CROUPIER_JAEGER_URL=https://jaeger.company.com          # 调用页/链路页跳转按钮
CROUPIER_GRAFANA_EXPLORE_URL=https://grafana.company.com/explore  # Grafana 入口
```

### 本地体验（Jaeger all-in-one）

```bash
docker run --rm -p 16686:16686 -p 4318:4318 jaegertracing/all-in-one:latest
# Server 配置 OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 后发起一次调用
# 打开 http://localhost:16686，Service 选 croupier-server 即可看到 span 瀑布
```

## SDK 现状（六语言）

| 语言   | 响应透出 traceId             | 请求注入 traceparent                        | Provider 提取延续                                                                        | 备注                    |
| ------ | ---------------------------- | ------------------------------------------- | ---------------------------------------------------------------------------------------- | ----------------------- |
| Go     | ✅（DTO `TraceID`）          | ❌                                          | ✅ 一期（ctx 注入 `WithTraceMetadata` + `TraceParentFromContext`/`TraceIDFromContext`）  | 一期                    |
| Python | ✅（invoker `trace_id`）     | ❌                                          | ✅ 一期（context JSON 随 metadata 透传 + `croupier.trace` 读取辅助）                     | 一期                    |
| JS     | ✅（`InvokeResult.traceId`） | ❌                                          | ✅ 一期（context JSON 随 metadata 透传 + `traceParentFromContext`/`traceIdFromContext`） | 一期                    |
| Java   | ❌                           | ❌                                          | ❌                                                                                       | 二期按需                |
| C#     | ❌                           | ❌                                          | ❌                                                                                       | 二期按需                |
| C++    | 部分（task 状态）            | 部分（`traceId` 明文注入 metadata，非 W3C） | ❌                                                                                       | `InvokeOptions.traceId` |

一期口径说明：Provider 侧"提取延续"当前为**无 otel 依赖的传播**——trace
字段进入 handler 上下文（Go 为 context value，Python/JS 为 context JSON
字段）供游戏方读取与日志关联；以 otel API 创建服务端 span（`sdk.invoke`）
仍属后续增量（需引入各语言 otel 库依赖，见下节"为什么 SDK 只做传播"）。

平台侧不依赖 SDK 是否实现传播：metadata 无 `traceparent` 时各层自动开新
trace，行为与现状完全一致（**零侵入**）。

## 为什么 SDK 只做传播、不做导出

1. **协议已就绪**：平台在 invoke 的 metadata（HTTP 调用方对应响应体）携带 W3C `traceparent`，SDK 只需读/写该字段
2. **职责边界**：SDK 是 RPC 库，不是观测代理。游戏业务的 metrics/spans 归游戏方自己的 OTel 体系
3. **成本可控**：传播用各语言官方 otel SDK 的 propagator，无 exporter 依赖、无后台线程

## SDK 实现要求（每语言）

### L3 Invoker（HTTP 调用方）

| 项               | 要求                                                                         |
| ---------------- | ---------------------------------------------------------------------------- |
| 响应透出         | 调用结果结构含 `traceId`（Go 已有），文档标注"用于 Jaeger/Grafana 查询"      |
| 请求注入（可选） | 调用方上下文已有 trace 时，向请求注入 `traceparent` 头；平台侧会延续该 trace |
| 不依赖 otel      | 不引入 otel 库也可实现——手动拼/读 `traceparent` 字符串即可（约 30 行）       |

### Provider（函数宿主，tcp_manager / local server）

| 项          | 要求                                                                         |
| ----------- | ---------------------------------------------------------------------------- |
| 提取        | invoke metadata 含 `traceparent` 时提取为父上下文                            |
| 延续        | 以该上下文创建服务端 span（`function.{id}` / `sdk.invoke`），SpanKind=Server |
| 无 trace 时 | 正常开新 trace，行为与现状完全一致（**零侵入**）                             |
| 导出        | SDK 不导出；若游戏方自配 OTLP exporter，span 自然进入其观测体系              |

### 验收口径

- 集成测试：带 traceparent 的 invoke 在 provider 侧 span.parent 是 server 侧 span（可用本地 jaeger all-in-one 断言）
- 不配置任何 OTel 的现有用户：行为不变（回归六语言既有合同测试）

## 分期

| 期   | 语言           | 预估                                    |
| ---- | -------------- | --------------------------------------- |
| 一期 | Go、Python、JS | 约 4 人日（Go 1.5 + Python 1.5 + JS 1） |
| 二期 | Java、C#、C++  | 约 5 人日，按接入需求排期               |

## 常见问题

**Q：响应里 `traceId` 为空？**
未开启 `telemetry.enable_tracing`，或 OTLP collector 不可达。传播字段仍在
metadata 里，但导出关闭时 span 上下文不落 traceId。

**Q：采样率怎么控制？**
`telemetry.sampling_ratio`（0.0–1.0）。注意被采掉的请求响应仍可能有
traceId（头采样位决定），但 Jaeger 里查不到完整 span。

**Q：SDK 还没实现传播，现在排障怎么办？**
平台三层 span（function.invoke → function.dispatch.invoke → agent.invoke）
已经可用，能覆盖"权限拒绝/选错 Agent/超时"等绝大多数平台侧问题；Provider
内部耗时暂时只能看 SDK 日志。

**Q：能把 trace 接入我们自己的 APM 吗？**
可以。标准 OTLP 输出，任何兼容 OTLP 的后端（Jaeger、Tempo、SkyWalking、
Datadog 等）都能直接接收；也可接自建 Collector 做二次处理。

## 对使用者的影响

- **不配置 OTel**：零变化（metadata 无 traceparent 时平台自动开新 trace）
- **配置了**：调用响应多一个 `traceId`；排查跨层问题时把 traceId 贴进 Jaeger 即可——目标用户既有习惯
