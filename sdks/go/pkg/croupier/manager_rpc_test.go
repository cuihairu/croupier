// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

package croupier

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	agentv1 "github.com/cuihairu/croupier/sdks/go/pkg/pb/croupier/agent/v1"
	sdkv1 "github.com/cuihairu/croupier/sdks/go/pkg/pb/croupier/sdk/v1"
	"google.golang.org/protobuf/proto"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier/protocol"
)

// TestInvokeHandler_Success tests successful function invocation
func TestInvokeHandler_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	handler := newTestRPCHandler(t)

	// Register a test function
	testFunctionCalled := false
	handler.manager.handlers["test.function"] = func(ctx context.Context, payload []byte) ([]byte, error) {
		testFunctionCalled = true
		return []byte(`{"result":"success"}`), nil
	}

	req := &sdkv1.InvokeRequest{
		FunctionId: "test.function",
		Payload:    []byte(`{"input":"data"}`),
	}
	reqBody, _ := proto.Marshal(req)

	respBody, err := handler.invoke(ctx, protocol.MsgInvokeRequest, 12345, reqBody)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !testFunctionCalled {
		t.Error("test function was not called")
	}

	resp := &sdkv1.InvokeResponse{}
	if err := proto.Unmarshal(respBody, resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if string(resp.Payload) != `{"result":"success"}` {
		t.Errorf("expected success result, got: %s", string(resp.Payload))
	}
}

// TestInvokeHandler_FunctionNotFound tests error when function is not registered
func TestInvokeHandler_FunctionNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	handler := newTestRPCHandler(t)

	req := &sdkv1.InvokeRequest{
		FunctionId: "unknown.function",
		Payload:    []byte(`{}`),
	}
	reqBody, _ := proto.Marshal(req)

	_, err := handler.invoke(ctx, protocol.MsgInvokeRequest, 12345, reqBody)
	if err == nil {
		t.Error("expected error for unknown function")
	}

	expectedErr := "function not found"
	if err.Error() == "" || !contains(err.Error(), expectedErr) {
		t.Errorf("expected error containing %q, got: %v", expectedErr, err)
	}
}

// TestInvokeHandler_InvalidPayload tests error handling for invalid protobuf
func TestInvokeHandler_InvalidPayload(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	handler := newTestRPCHandler(t)

	invalidBody := []byte{0xFF, 0xFF, 0xFF}

	_, err := handler.invoke(ctx, protocol.MsgInvokeRequest, 12345, invalidBody)
	if err == nil {
		t.Error("expected error for invalid payload")
	}
}

// TestInvokeHandler_HandlerError tests error propagation from handler
func TestInvokeHandler_HandlerError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	handler := newTestRPCHandler(t)

	handler.manager.handlers["error.function"] = func(ctx context.Context, payload []byte) ([]byte, error) {
		return nil, fmt.Errorf("handler error")
	}

	req := &sdkv1.InvokeRequest{
		FunctionId: "error.function",
		Payload:    []byte(`{}`),
	}
	reqBody, _ := proto.Marshal(req)

	_, err := handler.invoke(ctx, protocol.MsgInvokeRequest, 12345, reqBody)
	if err == nil {
		t.Error("expected error from handler")
	}
}

// TestStartTaskHandler_TaskLifecycle tests task creation and execution
func TestStartTaskHandler_TaskLifecycle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	handler := newTestRPCHandler(t)

	executionStarted := make(chan struct{})
	executionDone := make(chan struct{})

	handler.manager.handlers["async.function"] = func(ctx context.Context, payload []byte) ([]byte, error) {
		close(executionStarted)
		<-ctx.Done() // Wait for cancellation or completion
		close(executionDone)
		return []byte(`{"result":"done"}`), nil
	}

	req := &sdkv1.InvokeRequest{
		FunctionId: "async.function",
		Payload:    []byte(`{"input":"data"}`),
	}
	reqBody, _ := proto.Marshal(req)

	respBody, err := handler.startTask(ctx, protocol.MsgStartTaskRequest, 12345, reqBody)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := &sdkv1.StartTaskResponse{}
	if err := proto.Unmarshal(respBody, resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.TaskId == "" {
		t.Error("expected non-empty task ID")
	}

	// Wait for execution to start
	select {
	case <-executionStarted:
	case <-time.After(time.Second):
		t.Error("task execution did not start")
	}

	// Verify task is stored
	handler.manager.tasksMutex.RLock()
	task, ok := handler.manager.tasks[resp.TaskId]
	handler.manager.tasksMutex.RUnlock()

	if !ok {
		t.Error("task not found in manager")
	}

	if task.Status != agentv1.TaskStatus_TASK_STATUS_RUNNING {
		t.Errorf("expected RUNNING status, got: %v", task.Status)
	}

	// Cancel the task
	if task.Cancel != nil {
		task.Cancel()
	}

	// Wait for execution to complete
	select {
	case <-executionDone:
	case <-time.After(time.Second):
		t.Error("task execution did not complete")
	}
}

// TestStartTaskHandler_FunctionNotFound tests error when function is not registered
func TestStartTaskHandler_FunctionNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	handler := newTestRPCHandler(t)

	req := &sdkv1.InvokeRequest{
		FunctionId: "unknown.function",
		Payload:    []byte(`{}`),
	}
	reqBody, _ := proto.Marshal(req)

	_, err := handler.startTask(ctx, protocol.MsgStartTaskRequest, 12345, reqBody)
	if err == nil {
		t.Error("expected error for unknown function")
	}
}

// TestStartTaskHandler_TaskCompletion tests task completion tracking
func TestStartTaskHandler_TaskCompletion(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	handler := newTestRPCHandler(t)

	handler.manager.handlers["quick.function"] = func(ctx context.Context, payload []byte) ([]byte, error) {
		return []byte(`{"result":"quick"}`), nil
	}

	req := &sdkv1.InvokeRequest{
		FunctionId: "quick.function",
		Payload:    []byte(`{}`),
	}
	reqBody, _ := proto.Marshal(req)

	respBody, err := handler.startTask(ctx, protocol.MsgStartTaskRequest, 12345, reqBody)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := &sdkv1.StartTaskResponse{}
	_ = proto.Unmarshal(respBody, resp)

	// Wait for task to complete
	time.Sleep(100 * time.Millisecond)

	handler.manager.tasksMutex.RLock()
	task, ok := handler.manager.tasks[resp.TaskId]
	handler.manager.tasksMutex.RUnlock()

	if !ok {
		t.Error("task not found after completion")
	}

	if task.Status != agentv1.TaskStatus_TASK_STATUS_SUCCEEDED {
		t.Errorf("expected SUCCEEDED status, got: %v", task.Status)
	}

	if string(task.Result) != `{"result":"quick"}` {
		t.Errorf("unexpected result: %s", string(task.Result))
	}
}

// TestStartTaskHandler_TaskFailure tests task failure tracking
func TestStartTaskHandler_TaskFailure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	handler := newTestRPCHandler(t)

	handler.manager.handlers["failing.function"] = func(ctx context.Context, payload []byte) ([]byte, error) {
		return nil, fmt.Errorf("execution failed")
	}

	req := &sdkv1.InvokeRequest{
		FunctionId: "failing.function",
		Payload:    []byte(`{}`),
	}
	reqBody, _ := proto.Marshal(req)

	respBody, err := handler.startTask(ctx, protocol.MsgStartTaskRequest, 12345, reqBody)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := &sdkv1.StartTaskResponse{}
	_ = proto.Unmarshal(respBody, resp)

	// Wait for task to fail
	time.Sleep(100 * time.Millisecond)

	handler.manager.tasksMutex.RLock()
	task, ok := handler.manager.tasks[resp.TaskId]
	handler.manager.tasksMutex.RUnlock()

	if !ok {
		t.Error("task not found after failure")
	}

	if task.Status != agentv1.TaskStatus_TASK_STATUS_FAILED {
		t.Errorf("expected FAILED status, got: %v", task.Status)
	}

	if task.Error == "" {
		t.Error("expected error message")
	}
}

// TestCancelTaskHandler_CancelRunningTask tests cancelling a running task
func TestCancelTaskHandler_CancelRunningTask(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	handler := newTestRPCHandler(t)

	taskRunning := make(chan struct{})
	taskDone := make(chan struct{})

	handler.manager.handlers["long.function"] = func(ctx context.Context, payload []byte) ([]byte, error) {
		close(taskRunning)
		<-ctx.Done()
		close(taskDone)
		return nil, ctx.Err()
	}

	// First start a task
	req := &sdkv1.InvokeRequest{
		FunctionId: "long.function",
		Payload:    []byte(`{}`),
	}
	reqBody, _ := proto.Marshal(req)

	startResp, err := handler.startTask(ctx, protocol.MsgStartTaskRequest, 12345, reqBody)
	if err != nil {
		t.Fatalf("failed to start task: %v", err)
	}

	startRespMsg := &sdkv1.StartTaskResponse{}
	_ = proto.Unmarshal(startResp, startRespMsg)

	// Wait for task to start running
	select {
	case <-taskRunning:
	case <-time.After(time.Second):
		t.Fatal("task did not start")
	}

	// Now cancel the task
	cancelReq := &sdkv1.CancelTaskRequest{
		TaskId: startRespMsg.TaskId,
	}
	cancelReqBody, _ := proto.Marshal(cancelReq)

	cancelResp, err := handler.cancelTask(ctx, protocol.MsgCancelTaskRequest, 12346, cancelReqBody)
	if err != nil {
		t.Fatalf("failed to cancel task: %v", err)
	}

	resp := &sdkv1.InvokeResponse{}
	_ = proto.Unmarshal(cancelResp, resp)

	// Verify cancellation response
	expectedPayload := fmt.Sprintf(`{"taskId":"%s","cancelled":true}`, startRespMsg.TaskId)
	if string(resp.Payload) != expectedPayload {
		t.Errorf("unexpected payload: %s", string(resp.Payload))
	}

	// Wait for task to be cancelled
	select {
	case <-taskDone:
	case <-time.After(time.Second):
		t.Error("task was not cancelled")
	}

	// Verify task status
	handler.manager.tasksMutex.RLock()
	task := handler.manager.tasks[startRespMsg.TaskId]
	handler.manager.tasksMutex.RUnlock()

	if task.Status != agentv1.TaskStatus_TASK_STATUS_CANCEL_REQUESTED {
		t.Errorf("expected CANCEL_REQUESTED status, got: %v", task.Status)
	}
}

// TestCancelTaskHandler_CancelUnknownTask tests cancelling non-existent task
func TestCancelTaskHandler_CancelUnknownTask(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	handler := newTestRPCHandler(t)

	cancelReq := &sdkv1.CancelTaskRequest{
		TaskId: "unknown-task-id",
	}
	cancelReqBody, _ := proto.Marshal(cancelReq)

	respBody, err := handler.cancelTask(ctx, protocol.MsgCancelTaskRequest, 12346, cancelReqBody)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := &sdkv1.InvokeResponse{}
	_ = proto.Unmarshal(respBody, resp)

	// Should still succeed, but with cancelled:false
	if !contains(string(resp.Payload), `"cancelled":false`) {
		t.Errorf("expected cancelled:false in payload, got: %s", string(resp.Payload))
	}
}

// TestCancelTaskHandler_ConcurrentCancels tests concurrent cancellation requests
func TestCancelTaskHandler_ConcurrentCancels(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	handler := newTestRPCHandler(t)

	blockingChan := make(chan struct{})

	handler.manager.handlers["blocking.function"] = func(ctx context.Context, payload []byte) ([]byte, error) {
		<-blockingChan
		return nil, nil
	}

	// Start a task
	req := &sdkv1.InvokeRequest{
		FunctionId: "blocking.function",
		Payload:    []byte(`{}`),
	}
	reqBody, _ := proto.Marshal(req)

	startResp, _ := handler.startTask(ctx, protocol.MsgStartTaskRequest, 12345, reqBody)
	startRespMsg := &sdkv1.StartTaskResponse{}
	_ = proto.Unmarshal(startResp, startRespMsg)

	// Give task time to start
	time.Sleep(50 * time.Millisecond)

	var wg sync.WaitGroup
	numCancels := 5

	// Issue multiple concurrent cancel requests
	for i := 0; i < numCancels; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cancelReq := &sdkv1.CancelTaskRequest{
				TaskId: startRespMsg.TaskId,
			}
			cancelReqBody, _ := proto.Marshal(cancelReq)
			_, _ = handler.cancelTask(ctx, protocol.MsgCancelTaskRequest, uint32(i), cancelReqBody)
		}()
	}

	wg.Wait()

	// Unblock the task
	close(blockingChan)
	time.Sleep(50 * time.Millisecond)
}

// TestStreamTaskHandler_StreamExistingTask tests streaming task events
func TestStreamTaskHandler_StreamExistingTask(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	handler := newTestRPCHandler(t)

	// Create a task directly
	taskID := "test-task-123"
	task := &Task{
		ID:       taskID,
		Status:   agentv1.TaskStatus_TASK_STATUS_RUNNING,
		Progress: 50,
	}

	handler.manager.tasksMutex.Lock()
	handler.manager.tasks[taskID] = task
	handler.manager.tasksMutex.Unlock()

	streamReq := &sdkv1.TaskStreamRequest{
		TaskId: taskID,
	}
	streamReqBody, _ := proto.Marshal(streamReq)

	respBody, err := handler.streamTask(ctx, protocol.MsgStreamTaskRequest, 12347, streamReqBody)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := &sdkv1.InvokeResponse{}
	_ = proto.Unmarshal(respBody, resp)

	// Verify event payload
	event := &sdkv1.TaskEvent{}
	if err := proto.Unmarshal(resp.Payload, event); err != nil {
		t.Fatalf("failed to unmarshal event: %v", err)
	}

	if event.TaskId != taskID {
		t.Errorf("expected task ID %s, got %s", taskID, event.TaskId)
	}

	if event.Progress != 50 {
		t.Errorf("expected progress 50, got %d", event.Progress)
	}
}

// TestStreamTaskHandler_TaskNotFound tests streaming non-existent task
func TestStreamTaskHandler_TaskNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	handler := newTestRPCHandler(t)

	streamReq := &sdkv1.TaskStreamRequest{
		TaskId: "non-existent-task",
	}
	streamReqBody, _ := proto.Marshal(streamReq)

	_, err := handler.streamTask(ctx, protocol.MsgStreamTaskRequest, 12347, streamReqBody)
	if err == nil {
		t.Error("expected error for non-existent task")
	}

	expectedErr := "task not found"
	if !contains(err.Error(), expectedErr) {
		t.Errorf("expected error containing %q, got: %v", expectedErr, err)
	}
}

// TestStreamTaskHandler_ProgressUpdate tests progress updates during streaming
func TestStreamTaskHandler_ProgressUpdate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	handler := newTestRPCHandler(t)

	taskID := "progress-task-123"
	task := &Task{
		ID:       taskID,
		Status:   agentv1.TaskStatus_TASK_STATUS_RUNNING,
		Progress: 75,
	}

	handler.manager.tasksMutex.Lock()
	handler.manager.tasks[taskID] = task
	handler.manager.tasksMutex.Unlock()

	streamReq := &sdkv1.TaskStreamRequest{
		TaskId: taskID,
	}
	streamReqBody, _ := proto.Marshal(streamReq)

	respBody, err := handler.streamTask(ctx, protocol.MsgStreamTaskRequest, 12347, streamReqBody)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := &sdkv1.InvokeResponse{}
	_ = proto.Unmarshal(respBody, resp)

	event := &sdkv1.TaskEvent{}
	_ = proto.Unmarshal(resp.Payload, event)

	if event.Progress != 75 {
		t.Errorf("expected progress 75, got %d", event.Progress)
	}
}

// TestTCPRPCHandler_UnknownMessage tests handling of unknown message types
func TestTCPRPCHandler_UnknownMessage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	handler := newTestRPCHandler(t)

	unknownMsgID := uint32(0xDEADBEEF)
	body := []byte(`{}`)

	_, err := handler.Handle(ctx, unknownMsgID, 12345, body)
	if err == nil {
		t.Error("expected error for unknown message type")
	}

	expectedErr := "unknown message ID"
	if !contains(err.Error(), expectedErr) {
		t.Errorf("expected error containing %q, got: %v", expectedErr, err)
	}
}

// TestTCPRPCHandler_MessageRouting tests routing to correct handler
func TestTCPRPCHandler_MessageRouting(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	handler := newTestRPCHandler(t)

	handler.manager.handlers["test.function"] = func(ctx context.Context, payload []byte) ([]byte, error) {
		return []byte(`{"routed":true}`), nil
	}

	// Test routing for InvokeRequest
	req := &sdkv1.InvokeRequest{
		FunctionId: "test.function",
		Payload:    []byte(`{}`),
	}
	reqBody, _ := proto.Marshal(req)

	respBody, err := handler.Handle(ctx, protocol.MsgInvokeRequest, 12345, reqBody)
	if err != nil {
		t.Fatalf("InvokeRequest routing failed: %v", err)
	}

	resp := &sdkv1.InvokeResponse{}
	_ = proto.Unmarshal(respBody, resp)

	if !contains(string(resp.Payload), `"routed":true`) {
		t.Error("message was not routed correctly")
	}

	// Test routing for StartTaskRequest
	startResp, err := handler.Handle(ctx, protocol.MsgStartTaskRequest, 12346, reqBody)
	if err != nil {
		t.Fatalf("StartTaskRequest routing failed: %v", err)
	}

	startRespMsg := &sdkv1.StartTaskResponse{}
	_ = proto.Unmarshal(startResp, startRespMsg)

	if startRespMsg.TaskId == "" {
		t.Error("StartTaskRequest did not return task ID")
	}
}

// TestTask_ConcurrentAccess tests concurrent task access
func TestTask_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	handler := newTestRPCHandler(t)

	handler.manager.handlers["concurrent.function"] = func(ctx context.Context, payload []byte) ([]byte, error) {
		time.Sleep(50 * time.Millisecond)
		return []byte(`{"done":true}`), nil
	}

	var wg sync.WaitGroup
	numTasks := 10

	for i := 0; i < numTasks; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := &sdkv1.InvokeRequest{
				FunctionId: "concurrent.function",
				Payload:    []byte(`{}`),
			}
			reqBody, _ := proto.Marshal(req)
			_, _ = handler.startTask(ctx, protocol.MsgStartTaskRequest, uint32(i), reqBody)
		}()
	}

	wg.Wait()

	// Verify all tasks were created
	handler.manager.tasksMutex.RLock()
	taskCount := len(handler.manager.tasks)
	handler.manager.tasksMutex.RUnlock()

	if taskCount < numTasks {
		t.Errorf("expected at least %d tasks, got %d", numTasks, taskCount)
	}
}

// Helper functions

func newTestRPCHandler(t *testing.T) *tcpRPCHandler {
	t.Helper()

	manager := &TCPManager{
		handlers: make(map[string]FunctionHandler),
		tasks:    make(map[string]*Task),
	}
	return newTCPRPCHandler(manager)
}

// contains checks if substr is contained in s
func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
