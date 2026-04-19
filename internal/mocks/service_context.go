package mocks

import (
	"sync"
	"time"
)

// MockConfig holds mock configuration values.
type MockConfig struct {
	AgentID           string
	GameID            string
	Env               string
	LocalAddr         string
	ServerAddr        string
	HeartbeatInterval int64
}

// DefaultMockConfig returns a default mock configuration.
func DefaultMockConfig() *MockConfig {
	return &MockConfig{
		AgentID:           "test-agent-001",
		GameID:            "test-game",
		Env:               "development",
		LocalAddr:         "localhost:19090",
		ServerAddr:        "localhost:8443",
		HeartbeatInterval: 30,
	}
}

// MockServiceContext is a mock implementation of ServiceContext for testing.
type MockServiceContext struct {
	mu            sync.RWMutex
	Config        *MockConfig
	startTime     time.Time
	registeredAt  time.Time
	lastHeartbeat time.Time
	activeTasks   int
	functions     map[string]interface{}
	isRunning     bool
}

// NewMockServiceContext creates a new mock service context.
func NewMockServiceContext() *MockServiceContext {
	return &MockServiceContext{
		Config:    DefaultMockConfig(),
		startTime: time.Now(),
		functions: make(map[string]interface{}),
		isRunning: true,
	}
}

// SetConfig sets the mock configuration.
func (m *MockServiceContext) SetConfig(config *MockConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Config = config
}

// SetRunning sets the running state.
func (m *MockServiceContext) SetRunning(running bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.isRunning = running
}

// IsRunning returns the running state.
func (m *MockServiceContext) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isRunning
}

// SetActiveTasks sets the number of active tasks.
func (m *MockServiceContext) SetActiveTasks(count int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activeTasks = count
}

// GetActiveTasks returns the number of active tasks.
func (m *MockServiceContext) GetActiveTasks() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeTasks
}

// AddFunction adds a function to the mock context.
func (m *MockServiceContext) AddFunction(id string, handler interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.functions[id] = handler
}

// GetFunctions returns all registered functions.
func (m *MockServiceContext) GetFunctions() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]interface{})
	for k, v := range m.functions {
		result[k] = v
	}
	return result
}

// FunctionCount returns the number of registered functions.
func (m *MockServiceContext) FunctionCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.functions)
}

// SetRegisteredAt sets the registration timestamp.
func (m *MockServiceContext) SetRegisteredAt(t time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.registeredAt = t
}

// GetRegisteredAt returns the registration timestamp.
func (m *MockServiceContext) GetRegisteredAt() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.registeredAt
}

// SetLastHeartbeat sets the last heartbeat timestamp.
func (m *MockServiceContext) SetLastHeartbeat(t time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastHeartbeat = t
}

// GetLastHeartbeat returns the last heartbeat timestamp.
func (m *MockServiceContext) GetLastHeartbeat() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastHeartbeat
}

// Uptime returns the mock uptime duration.
func (m *MockServiceContext) Uptime() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return time.Since(m.startTime)
}

// SetStartTime sets the start time for uptime calculation.
func (m *MockServiceContext) SetStartTime(t time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startTime = t
}
