# SDK OpenTelemetry 传播方案

状态：已评审（2026-08-21）——平台侧已完成，SDK 侧分期实施
关联：`docs/analytics/opentelemetry-integration.md`（平台侧集成指南）、`docs/architecture/sdk-wire-protocol.md`（trace 字段协议层约定）

## 结论速览

| 决策项                           | 结论                                                    |
| -------------------------------- | ------------------------------------------------------- |
| 平台侧 OTel 埋点                 | 保留（已实现并验收，J-001.02）                          |
| SDK 六语言 trace 传播            | 值得做，分两期：Go/Python/JS 一期；Java/C#/C++ 二期按需 |
| SDK 内置 exporter / 业务自动埋点 | **不做**——业务观测归游戏方自己的 OTel 体系              |

## 现状（代码事实）

### 平台侧（已完成）

全链路 span 与 W3C traceparent 透传已实现：

```
调用方 ──HTTP──> Server(function.invoke span)
                 │  metadata 注入 traceparent（internal/telemetry/context.go InjectContext）
                 ▼ TCP
              Agent(agent.invoke span, ExtractContext 延续同 trace)
                 │  metadata 再注入
                 ▼
              Provider(SDK 函数进程)
```

- Server：`internal/api/function/helpers.go`（function.invoke span + 响应 `traceId` 字段）
- Dispatcher：`internal/platform/dispatch/dispatcher.go`（ExtractContext/InjectContext 于 metadata map）
- Agent：`internal/agent/local_handler.go`（agent.invoke，SpanKind=Server）
- 审计关联：audit 记录含 `trace_id`，Dashboard 调用页 traceId 可跳 Jaeger

### SDK 侧（六语言均为零）

- SDK 不产生、不消费 trace；Go SDK 仅在响应 DTO 透出 `traceId` 字段
- 生产环境需开启 `OTEL_ENABLED` + `OTEL_EXPORTER_OTLP_ENDPOINT`（未开启时响应 traceId 为空，属部署配置，非代码缺口）

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

## 对使用者的影响

- **不配置 OTel**：零变化（metadata 无 traceparent 时平台自动开新 trace）
- **配置了**：调用响应多一个 `traceId`；排查跨层问题时把 traceId 贴进 Jaeger 即可——目标用户既有习惯
