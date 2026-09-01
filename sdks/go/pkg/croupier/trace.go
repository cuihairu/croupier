package croupier

import (
	"context"
	"strings"
)

// OTel trace 传播（一期·无依赖版）：平台在 invoke metadata 携带 W3C
// traceparent 与冗余明文 trace_id（见 docs/architecture/sdk-otel-propagation.md）。
// SDK 职责仅是把这些值提取进 handler 上下文——游戏方据此做日志关联，
// 或自行接入其 OTel 体系（SDK 不内置 exporter/自动埋点）。

// metadata 约定键（与平台 telemetry.InjectContext 一致）。
const (
	MetadataTraceparent = "traceparent"
	MetadataTraceID     = "trace_id"
)

type traceContextKey struct{}

type traceContext struct {
	traceparent string
	traceID     string
}

// WithTraceMetadata 把 invoke metadata 中的 trace 字段写入 ctx（无则原样返回）。
func WithTraceMetadata(ctx context.Context, meta map[string]string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	tp := strings.TrimSpace(meta[MetadataTraceparent])
	tid := strings.TrimSpace(meta[MetadataTraceID])
	if tp == "" && tid == "" {
		return ctx
	}
	return context.WithValue(ctx, traceContextKey{}, traceContext{traceparent: tp, traceID: tid})
}

// TraceParentFromContext 返回当前请求的 W3C traceparent（无则空串）。
func TraceParentFromContext(ctx context.Context) string {
	if tc, ok := ctx.Value(traceContextKey{}).(traceContext); ok {
		return tc.traceparent
	}
	return ""
}

// TraceIDFromContext 返回当前请求的 trace_id 明文（无则空串）。
// 可直接拼进游戏方日志/错误信息，去 Jaeger/Grafana 查询整条链路。
func TraceIDFromContext(ctx context.Context) string {
	if tc, ok := ctx.Value(traceContextKey{}).(traceContext); ok {
		return tc.traceID
	}
	return ""
}
