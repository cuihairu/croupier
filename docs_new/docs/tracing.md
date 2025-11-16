# Tracing & Trace ID Propagation

Croupier propagates a lightweight `trace_id` across the Server → Agent → Adapter chain. This helps correlate UI/API calls, audit events, and downstream requests.

Flow
- Server (HTTP): generates a random `trace_id` per request and records it in audit; forwards it via `InvokeRequest.metadata["trace_id"]`.
- Agent/Server routing: preserves request metadata (trace_id/game_id/env) on RPC to function handlers.
- Adapters (HTTP/Prom): add `X-Trace-Id`, `X-Game-Id`, `X-Env` headers to outbound HTTP requests when not already present.

Headers
- `X-Trace-Id`: correlates downstream REST requests with the originating GM action.
- `X-Game-Id`, `X-Env`: scope hints for multi-game environments (optional).

UI
- `/gm/audit`（或相关页面）中可查看 `trace_id` 字段；服务端日志也会打印 `trace_id`。

Future Work
- OTLP exporter for distributed tracing (Jaeger/Tempo/etc.): planned. The existing `trace_id` can be embedded into spans once enabled.

