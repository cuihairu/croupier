// 覆盖目标：MockGRPCClient.CancelTask 的 SetError 提前返回分支
// （grpc_client.go:121-123）。
package mocks

import (
	"context"
	"errors"
	"testing"
)

func TestMockGRPCClient_CancelTaskSetErrorReturn(t *testing.T) {
	client := NewMockGRPCClient()
	expectedErr := errors.New("connection closed")
	client.SetError(expectedErr)

	if err := client.CancelTask(context.Background(), "task-7"); err != expectedErr {
		t.Errorf("CancelTask() error = %v, want %v", err, expectedErr)
	}

	calls := client.GetCalls()
	if len(calls) != 1 || calls[0] != "CancelTask:task-7" {
		t.Errorf("GetCalls() = %v, want [CancelTask:task-7]", calls)
	}
}
