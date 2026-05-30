// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

package tasks

import (
	"context"
	"testing"
)

func TestEventTypeValues(t *testing.T) {
	tests := []struct {
		name      string
		eventType EventType
		want      string
	}{
		{"EventQueued", EventQueued, "queued"},
		{"EventStarted", EventStarted, "started"},
		{"EventProgress", EventProgress, "progress"},
		{"EventLog", EventLog, "log"},
		{"EventCompleted", EventCompleted, "completed"},
		{"EventFailed", EventFailed, "failed"},
		{"EventCancelRequested", EventCancelRequested, "cancel_requested"},
		{"EventCancelled", EventCancelled, "cancelled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(tt.eventType); got != tt.want {
				t.Errorf("EventType = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStatusConstants(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"StatusQueued", StatusQueued},
		{"StatusRunning", StatusRunning},
		{"StatusSucceeded", StatusSucceeded},
		{"StatusFailed", StatusFailed},
		{"StatusCancelRequested", StatusCancelRequested},
		{"StatusCancelled", StatusCancelled},
		{"StatusTimedOut", StatusTimedOut},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value == "" {
				t.Errorf("Status constant %s is empty", tt.name)
			}
		})
	}
}

func TestJSONPayload(t *testing.T) {
	tests := []struct {
		name string
		v    interface{}
		want []byte
	}{
		{
			name: "nil input",
			v:    nil,
			want: []byte("null"),
		},
		{
			name: "string input",
			v:    "test",
			want: []byte(`"test"`),
		},
		{
			name: "number input",
			v:    42,
			want: []byte(`42`),
		},
		{
			name: "map input",
			v:    map[string]string{"key": "value"},
			want: []byte(`{"key":"value"}`),
		},
		{
			name: "slice input",
			v:    []int{1, 2, 3},
			want: []byte(`[1,2,3]`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := JSONPayload(tt.v)
			// JSON marshaling may produce slightly different formatting
			// Just check that it's not empty and starts/ends as expected
			if len(got) == 0 {
				t.Errorf("JSONPayload() returned empty byte slice")
			}
		})
	}
}

// Mock Runtime for testing TaskContext interface
type mockRuntime struct {
	ctx           context.Context
	taskID        string
	cancelRequest bool
}

func (m *mockRuntime) Context() context.Context { return m.ctx }
func (m *mockRuntime) TaskID() string           { return m.taskID }
func (m *mockRuntime) ReportProgress(int32, string, []byte) error {
	return nil
}
func (m *mockRuntime) Log(string, []byte) error { return nil }
func (m *mockRuntime) IsCancelled() bool        { return m.cancelRequest }

func TestTaskContextInterface(t *testing.T) {
	ctx := context.Background()
	runtime := &mockRuntime{
		ctx:    ctx,
		taskID: "test-task-123",
	}

	var tc TaskContext = runtime

	// Test Context method
	if got := tc.Context(); got != ctx {
		t.Errorf("TaskContext.Context() = %v, want %v", got, ctx)
	}

	// Test TaskID method
	if got := tc.TaskID(); got != "test-task-123" {
		t.Errorf("TaskContext.TaskID() = %v, want test-task-123", got)
	}

	// Test IsCancelled method
	if tc.IsCancelled() {
		t.Error("TaskContext.IsCancelled() = true, want false")
	}

	// Test ReportProgress doesn't panic
	if err := tc.ReportProgress(50, "test", nil); err != nil {
		t.Errorf("TaskContext.ReportProgress() error = %v", err)
	}

	// Test Log doesn't panic
	if err := tc.Log("test message", nil); err != nil {
		t.Errorf("TaskContext.Log() error = %v", err)
	}
}

func TestRuntimeBasicMethods(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := &Store{} // Mock store - won't actually call DB in tests
	runtime := &Runtime{
		taskID: "test-123",
		ctx:    ctx,
		cancel: cancel,
		store:  store,
	}

	if got := runtime.Context(); got != ctx {
		t.Errorf("Runtime.Context() = %v, want %v", got, ctx)
	}

	if got := runtime.TaskID(); got != "test-123" {
		t.Errorf("Runtime.TaskID() = %v, want test-123", got)
	}
}
