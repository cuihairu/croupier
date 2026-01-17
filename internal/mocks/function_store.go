package mocks

import (
	"sync"
)

// FunctionInfo represents a registered function's metadata.
type FunctionInfo struct {
	FunctionID   string
	DisplayName  string
	Tags         []string
	InputSchema  string
	OutputSchema string
}

// MockFunctionStore is a mock implementation of function storage for testing.
type MockFunctionStore struct {
	mu        sync.RWMutex
	functions map[string]*FunctionInfo
	err       error
}

// NewMockFunctionStore creates a new mock function store.
func NewMockFunctionStore() *MockFunctionStore {
	return &MockFunctionStore{
		functions: make(map[string]*FunctionInfo),
	}
}

// SetError configures the mock to return an error.
func (m *MockFunctionStore) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

// AddFunction adds a function to the mock store.
func (m *MockFunctionStore) AddFunction(info *FunctionInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.functions[info.FunctionID] = info
}

// GetFunction retrieves a function from the mock store.
func (m *MockFunctionStore) GetFunction(functionID string) (*FunctionInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.err != nil {
		return nil, m.err
	}
	info, ok := m.functions[functionID]
	if !ok {
		return nil, nil
	}
	return info, nil
}

// ListFunctions returns all functions in the mock store.
func (m *MockFunctionStore) ListFunctions() ([]*FunctionInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.err != nil {
		return nil, m.err
	}
	result := make([]*FunctionInfo, 0, len(m.functions))
	for _, info := range m.functions {
		result = append(result, info)
	}
	return result, nil
}

// Clear removes all functions from the mock store.
func (m *MockFunctionStore) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.functions = make(map[string]*FunctionInfo)
	m.err = nil
}
