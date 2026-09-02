// 覆盖目标：SetStartTaskFunc / SetStreamEvents / ClearCalls / SetConfig，
// 以及 CancelTask 的 cancelTaskFunc 分支与 StreamTask 的自定义事件分支。
package mocks

import (
	"context"
	"errors"
	"testing"
)

func TestMockGRPCClient_SetStartTaskFunc(t *testing.T) {
	client := NewMockGRPCClient()

	client.SetStartTaskFunc(func(ctx context.Context, req *InvokeRequest) (string, error) {
		return "custom-task-id", nil
	})

	taskID, err := client.StartTask(context.Background(), &InvokeRequest{FunctionID: "fn"})
	if err != nil {
		t.Fatalf("StartTask() error = %v", err)
	}
	if taskID != "custom-task-id" {
		t.Errorf("StartTask() = %q, want %q", taskID, "custom-task-id")
	}

	calls := client.GetCalls()
	if len(calls) != 1 || calls[0] != "StartTask:fn" {
		t.Errorf("GetCalls() = %v, want [StartTask:fn]", calls)
	}
}

func TestMockGRPCClient_SetStartTaskFuncError(t *testing.T) {
	client := NewMockGRPCClient()
	expectedErr := errors.New("start failed")

	client.SetStartTaskFunc(func(ctx context.Context, req *InvokeRequest) (string, error) {
		return "", expectedErr
	})

	_, err := client.StartTask(context.Background(), &InvokeRequest{FunctionID: "fn"})
	if err != expectedErr {
		t.Errorf("StartTask() error = %v, want %v", err, expectedErr)
	}
}

func TestMockGRPCClient_SetStreamEvents(t *testing.T) {
	client := NewMockGRPCClient()

	events := []*TaskEvent{
		{TaskID: "task-1", EventType: "log", Data: []byte(`{"line":"x"}`)},
	}
	client.SetStreamEvents(events)

	got, err := client.StreamTask(context.Background(), "task-1")
	if err != nil {
		t.Fatalf("StreamTask() error = %v", err)
	}
	if len(got) != 1 || got[0].EventType != "log" {
		t.Errorf("StreamTask() = %+v, want one log event", got)
	}
}

func TestMockGRPCClient_StreamTaskError(t *testing.T) {
	client := NewMockGRPCClient()
	expectedErr := errors.New("stream broken")
	client.SetError(expectedErr)

	_, err := client.StreamTask(context.Background(), "task-1")
	if err != expectedErr {
		t.Errorf("StreamTask() error = %v, want %v", err, expectedErr)
	}
}

func TestMockGRPCClient_CancelTaskError(t *testing.T) {
	client := NewMockGRPCClient()
	expectedErr := errors.New("cancel failed")

	// cancelTaskFunc 无公开 setter，同包测试直接注入以覆盖自定义分支
	client.mu.Lock()
	client.cancelTaskFunc = func(ctx context.Context, taskID string) error {
		return expectedErr
	}
	client.mu.Unlock()

	if err := client.CancelTask(context.Background(), "task-9"); err != expectedErr {
		t.Errorf("CancelTask() error = %v, want %v", err, expectedErr)
	}
}

func TestMockGRPCClient_ClearCalls(t *testing.T) {
	client := NewMockGRPCClient()

	_, _ = client.Invoke(context.Background(), &InvokeRequest{FunctionID: "a"})
	_, _ = client.StartTask(context.Background(), &InvokeRequest{FunctionID: "b"})
	if len(client.GetCalls()) != 2 {
		t.Fatalf("GetCalls() = %v, want 2 entries", client.GetCalls())
	}

	client.ClearCalls()
	if calls := client.GetCalls(); len(calls) != 0 {
		t.Errorf("GetCalls() after ClearCalls() = %v, want empty", calls)
	}
}

func TestMockServiceContext_SetConfig(t *testing.T) {
	ctx := NewMockServiceContext()

	if ctx.Config.AgentID != "test-agent-001" {
		t.Errorf("default Config.AgentID = %q, want test-agent-001", ctx.Config.AgentID)
	}

	newConfig := &MockConfig{
		AgentID: "agent-x",
		GameID:  "game-y",
		Env:     "prod",
	}
	ctx.SetConfig(newConfig)

	if ctx.Config != newConfig {
		t.Errorf("SetConfig() did not replace Config: %+v", ctx.Config)
	}
	if ctx.Config.AgentID != "agent-x" || ctx.Config.Env != "prod" {
		t.Errorf("Config = %+v, want AgentID=agent-x Env=prod", ctx.Config)
	}
}
