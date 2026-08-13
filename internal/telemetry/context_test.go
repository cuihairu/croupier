package telemetry

import (
	"context"
	"testing"
)

func TestInjectContext_NilMetadata(t *testing.T) {
	ctx := context.Background()
	result := InjectContext(ctx, nil)

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// Result is a valid map even if empty (no span context)
	t.Logf("metadata keys: %v", result)
}

func TestInjectContext_ExistingMetadata(t *testing.T) {
	ctx := context.Background()
	metadata := map[string]string{
		"existing-key": "existing-value",
	}
	result := InjectContext(ctx, metadata)

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result["existing-key"] != "existing-value" {
		t.Errorf("expected existing key to be preserved, got %q", result["existing-key"])
	}
}

func TestExtractContext_NilMetadata(t *testing.T) {
	ctx := context.Background()
	result := ExtractContext(ctx, nil)

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// Should return same context
	if result != ctx {
		t.Error("expected same context when metadata is nil")
	}
}

func TestExtractContext_EmptyMetadata(t *testing.T) {
	ctx := context.Background()
	result := ExtractContext(ctx, map[string]string{})

	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestTraceIDFromContext_NoSpan(t *testing.T) {
	ctx := context.Background()
	result := TraceIDFromContext(ctx)

	if result != "" {
		t.Errorf("expected empty trace ID, got %q", result)
	}
}

func TestTraceIDFromMetadata_NilMetadata(t *testing.T) {
	result := TraceIDFromMetadata(nil)

	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestTraceIDFromMetadata_EmptyMetadata(t *testing.T) {
	result := TraceIDFromMetadata(map[string]string{})

	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestTraceIDFromMetadata_WithTraceID(t *testing.T) {
	metadata := map[string]string{
		MetadataTraceID: "abc123def456",
	}
	result := TraceIDFromMetadata(metadata)

	if result != "abc123def456" {
		t.Errorf("expected abc123def456, got %q", result)
	}
}

func TestInjectAndExtractRoundTrip(t *testing.T) {
	ctx := context.Background()

	// Inject
	metadata := InjectContext(ctx, nil)

	// Extract
	extracted := ExtractContext(ctx, metadata)

	if extracted == nil {
		t.Fatal("expected non-nil context after extract")
	}

	// The extracted context should be usable
	extractedTraceID := TraceIDFromContext(extracted)
	if extractedTraceID == "" {
		t.Log("no trace ID in extracted context (expected for non-span context)")
	}
}

func TestMetadataConstants(t *testing.T) {
	if MetadataTraceID != "trace_id" {
		t.Errorf("expected trace_id, got %q", MetadataTraceID)
	}
	if MetadataTraceparent != "traceparent" {
		t.Errorf("expected traceparent, got %q", MetadataTraceparent)
	}
	if MetadataTracestate != "tracestate" {
		t.Errorf("expected tracestate, got %q", MetadataTracestate)
	}
}
