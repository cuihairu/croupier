package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const (
	MetadataTraceID     = "traceId"
	MetadataTraceparent = "traceparent"
	MetadataTracestate  = "tracestate"
)

// InjectContext writes W3C trace context and trace_id into metadata.
func InjectContext(ctx context.Context, metadata map[string]string) map[string]string {
	if metadata == nil {
		metadata = map[string]string{}
	}
	otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(metadata))
	if sc := trace.SpanContextFromContext(ctx); sc.HasTraceID() {
		metadata[MetadataTraceID] = sc.TraceID().String()
	}
	return metadata
}

// ExtractContext reads W3C trace context from metadata.
func ExtractContext(ctx context.Context, metadata map[string]string) context.Context {
	if metadata == nil {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(metadata))
}

// TraceIDFromContext returns the active trace ID, if any.
func TraceIDFromContext(ctx context.Context) string {
	if sc := trace.SpanContextFromContext(ctx); sc.HasTraceID() {
		return sc.TraceID().String()
	}
	return ""
}

// TraceIDFromMetadata returns the propagated trace ID without parsing it.
func TraceIDFromMetadata(metadata map[string]string) string {
	if metadata == nil {
		return ""
	}
	return metadata[MetadataTraceID]
}
