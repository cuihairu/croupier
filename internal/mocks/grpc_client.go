package mocks

import (
	"context"
	"sync"
)

// InvokeRequest represents a function invocation request.
type InvokeRequest struct {
	FunctionID     string
	Payload        []byte
	IdempotencyKey string
	Timeout        int64
}

// InvokeResponse represents a function invocation response.
type InvokeResponse struct {
	Success bool
	Result  []byte
	Error   string
}

// TaskEvent represents a task execution event.
type TaskEvent struct {
	TaskID    string
	EventType string // "progress", "log", "done", "error"
	Data      []byte
}

// MockGRPCClient is a mock implementation of gRPC client for testing.
type MockGRPCClient struct {
	mu             sync.RWMutex
	invokeFunc     func(ctx context.Context, req *InvokeRequest) (*InvokeResponse, error)
	startTaskFunc  func(ctx context.Context, req *InvokeRequest) (string, error)
	cancelTaskFunc func(ctx context.Context, taskID string) error
	streamEvents   []*TaskEvent
	calls          []string
	err            error
}

// NewMockGRPCClient creates a new mock gRPC client.
func NewMockGRPCClient() *MockGRPCClient {
	return &MockGRPCClient{
		calls: make([]string, 0),
	}
}

// SetError configures the mock to return an error.
func (m *MockGRPCClient) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

// SetInvokeFunc sets a custom invoke function.
func (m *MockGRPCClient) SetInvokeFunc(f func(ctx context.Context, req *InvokeRequest) (*InvokeResponse, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.invokeFunc = f
}

// SetStartTaskFunc sets a custom start task function.
func (m *MockGRPCClient) SetStartTaskFunc(f func(ctx context.Context, req *InvokeRequest) (string, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startTaskFunc = f
}

// SetStreamEvents sets the events to return from stream.
func (m *MockGRPCClient) SetStreamEvents(events []*TaskEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.streamEvents = events
}

// Invoke performs a mock function invocation.
func (m *MockGRPCClient) Invoke(ctx context.Context, req *InvokeRequest) (*InvokeResponse, error) {
	m.mu.Lock()
	m.calls = append(m.calls, "Invoke:"+req.FunctionID)
	invokeFunc := m.invokeFunc
	err := m.err
	m.mu.Unlock()

	if err != nil {
		return nil, err
	}
	if invokeFunc != nil {
		return invokeFunc(ctx, req)
	}
	return &InvokeResponse{
		Success: true,
		Result:  []byte(`{"status":"ok"}`),
	}, nil
}

// StartTask starts a mock task.
func (m *MockGRPCClient) StartTask(ctx context.Context, req *InvokeRequest) (string, error) {
	m.mu.Lock()
	m.calls = append(m.calls, "StartTask:"+req.FunctionID)
	startTaskFunc := m.startTaskFunc
	err := m.err
	m.mu.Unlock()

	if err != nil {
		return "", err
	}
	if startTaskFunc != nil {
		return startTaskFunc(ctx, req)
	}
	return "mock-task-id-12345", nil
}

// CancelTask cancels a mock task.
func (m *MockGRPCClient) CancelTask(ctx context.Context, taskID string) error {
	m.mu.Lock()
	m.calls = append(m.calls, "CancelTask:"+taskID)
	cancelTaskFunc := m.cancelTaskFunc
	err := m.err
	m.mu.Unlock()

	if err != nil {
		return err
	}
	if cancelTaskFunc != nil {
		return cancelTaskFunc(ctx, taskID)
	}
	return nil
}

// StreamTask returns mock task events.
func (m *MockGRPCClient) StreamTask(ctx context.Context, taskID string) ([]*TaskEvent, error) {
	m.mu.Lock()
	m.calls = append(m.calls, "StreamTask:"+taskID)
	events := m.streamEvents
	err := m.err
	m.mu.Unlock()

	if err != nil {
		return nil, err
	}
	if events != nil {
		return events, nil
	}
	return []*TaskEvent{
		{TaskID: taskID, EventType: "progress", Data: []byte(`{"percent":50}`)},
		{TaskID: taskID, EventType: "done", Data: []byte(`{"result":"completed"}`)},
	}, nil
}

// GetCalls returns all recorded calls.
func (m *MockGRPCClient) GetCalls() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]string, len(m.calls))
	copy(result, m.calls)
	return result
}

// ClearCalls clears all recorded calls.
func (m *MockGRPCClient) ClearCalls() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = make([]string, 0)
}
