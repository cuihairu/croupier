package otelrec

import (
	"context"
	"errors"
	"testing"
)

// Noop 必须满足 Recorder 且所有调用不 panic（零开销路径契约）。
func TestNoopRecorder_ImplementsContract(t *testing.T) {
	var _ Recorder = NoopRecorder{}
	r := Discard()
	ctx := context.Background()
	r.Start(ctx, map[string]string{"a": "b"})
	r.Stop(ctx, errors.New("x"))
	r.Connected(ctx)
	r.Disconnected(ctx, errors.New("y"))
	r.Count(ctx, "evt", 1, nil)
	r.Observe(ctx, "evt", 2.5, map[string]string{"k": "v"})
}
