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

// JobEvent represents a job execution event.
type JobEvent struct {
	JobID     string
	EventType string // "progress", "log", "done", "error"
	Data      []byte
}

// MockGRPCClient is a mock implementation of gRPC client for testing.
type MockGRPCClient struct {
	mu            sync.RWMutex
	invokeFunc    func(ctx context.Context, req *InvokeRequest) (*InvokeResponse, error)
	startJobFunc  func(ctx context.Context, req *InvokeRequest) (string, error)
	cancelJobFunc func(ctx context.Context, jobID string) error
	streamEvents  []*JobEvent
	calls         []string
	err           error
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

// SetStartJobFunc sets a custom start job function.
func (m *MockGRPCClient) SetStartJobFunc(f func(ctx context.Context, req *InvokeRequest) (string, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startJobFunc = f
}

// SetStreamEvents sets the events to return from stream.
func (m *MockGRPCClient) SetStreamEvents(events []*JobEvent) {
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

// StartJob starts a mock job.
func (m *MockGRPCClient) StartJob(ctx context.Context, req *InvokeRequest) (string, error) {
	m.mu.Lock()
	m.calls = append(m.calls, "StartJob:"+req.FunctionID)
	startJobFunc := m.startJobFunc
	err := m.err
	m.mu.Unlock()

	if err != nil {
		return "", err
	}
	if startJobFunc != nil {
		return startJobFunc(ctx, req)
	}
	return "mock-job-id-12345", nil
}

// CancelJob cancels a mock job.
func (m *MockGRPCClient) CancelJob(ctx context.Context, jobID string) error {
	m.mu.Lock()
	m.calls = append(m.calls, "CancelJob:"+jobID)
	cancelJobFunc := m.cancelJobFunc
	err := m.err
	m.mu.Unlock()

	if err != nil {
		return err
	}
	if cancelJobFunc != nil {
		return cancelJobFunc(ctx, jobID)
	}
	return nil
}

// StreamJob returns mock job events.
func (m *MockGRPCClient) StreamJob(ctx context.Context, jobID string) ([]*JobEvent, error) {
	m.mu.Lock()
	m.calls = append(m.calls, "StreamJob:"+jobID)
	events := m.streamEvents
	err := m.err
	m.mu.Unlock()

	if err != nil {
		return nil, err
	}
	if events != nil {
		return events, nil
	}
	return []*JobEvent{
		{JobID: jobID, EventType: "progress", Data: []byte(`{"percent":50}`)},
		{JobID: jobID, EventType: "done", Data: []byte(`{"result":"completed"}`)},
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
