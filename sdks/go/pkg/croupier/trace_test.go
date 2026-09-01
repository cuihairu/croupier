package croupier

import (
	"context"
	"testing"
)

func TestTraceMetadataRoundTrip(t *testing.T) {
	ctx := WithTraceMetadata(context.Background(), map[string]string{
		"traceparent": " 00-abc-def-01 ",
		"trace_id":    " abc ",
		"other":       "ignored",
	})
	if got := TraceParentFromContext(ctx); got != "00-abc-def-01" {
		t.Fatalf("traceparent = %q", got)
	}
	if got := TraceIDFromContext(ctx); got != "abc" {
		t.Fatalf("trace_id = %q", got)
	}

	// 无 trace 字段 → 原样返回（零侵入）
	plain := context.Background()
	same := WithTraceMetadata(plain, map[string]string{"k": "v"})
	if same != plain {
		t.Fatal("ctx without trace fields must be returned unchanged")
	}
	if TraceParentFromContext(plain) != "" || TraceIDFromContext(plain) != "" {
		t.Fatal("empty ctx must yield empty trace values")
	}
}
