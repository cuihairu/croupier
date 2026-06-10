package tasks

import (
	"context"
	"testing"
)

func TestRuntime_TaskID(t *testing.T) {
	r := &Runtime{taskID: "t1"}
	if r.TaskID() != "t1" {
		t.Errorf("TaskID = %q, want %q", r.TaskID(), "t1")
	}
}

func TestRuntime_Context(t *testing.T) {
	ctx := context.WithValue(context.Background(), "key", "val")
	r := &Runtime{ctx: ctx}
	if r.Context().Value("key") != "val" {
		t.Error("Context should return the stored context")
	}
}

func TestRuntime_IsCancelled(t *testing.T) {
	r := &Runtime{}
	if r.IsCancelled() {
		t.Error("should not be cancelled initially")
	}
	r.cancelRequested = true
	if !r.IsCancelled() {
		t.Error("should be cancelled after setting flag")
	}
}

func TestRuntime_RequestCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := &Runtime{ctx: ctx, cancel: cancel}

	r.RequestCancel()

	if !r.IsCancelled() {
		t.Error("should be cancelled after RequestCancel")
	}
	if ctx.Err() != context.Canceled {
		t.Error("context should be canceled")
	}
}

func TestRuntime_RequestCancel_NilCancel(t *testing.T) {
	r := &Runtime{}
	// Should not panic with nil cancel
	r.RequestCancel()
	if !r.IsCancelled() {
		t.Error("should be cancelled")
	}
}

func TestJSONPayload_Extra(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		b := JSONPayload(nil)
		if string(b) != "null" {
			t.Errorf("expected null, got %s", string(b))
		}
	})

	t.Run("string", func(t *testing.T) {
		b := JSONPayload("hello")
		if string(b) != `"hello"` {
			t.Errorf("expected \"hello\", got %s", string(b))
		}
	})

	t.Run("map", func(t *testing.T) {
		b := JSONPayload(map[string]int{"a": 1})
		if len(b) == 0 {
			t.Error("expected non-empty JSON")
		}
	})

	t.Run("unmarshalable", func(t *testing.T) {
		// channels can't be marshaled to JSON
		b := JSONPayload(make(chan int))
		if string(b) != "null" {
			t.Errorf("expected null for unmarshalable, got %s", string(b))
		}
	})
}
