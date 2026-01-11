// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"
	"fmt"
	"sync"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	globalAgentOpsClient *AgentOpsClient
	globalAgentOpsOnce   sync.Once
)

// InitAgentOpsClient initializes the global agent ops client.
func InitAgentOpsClient(registry *reg.Store) {
	globalAgentOpsOnce.Do(func() {
		globalAgentOpsClient = NewAgentOpsClient(registry)
	})
}

// GetAgentOpsClient returns the global agent ops client.
func GetAgentOpsClient() *AgentOpsClient {
	return globalAgentOpsClient
}

// AgentOpsClient manages gRPC connections to agent OpsService.
type AgentOpsClient struct {
	mu       sync.RWMutex
	clients  map[string]opsv1.OpsServiceClient // agent_id -> client
	conns    map[string]*grpc.ClientConn       // agent_id -> connection
	registry *reg.Store
}

// NewAgentOpsClient creates a new agent ops client manager.
func NewAgentOpsClient(registry *reg.Store) *AgentOpsClient {
	return &AgentOpsClient{
		clients:  make(map[string]opsv1.OpsServiceClient),
		conns:    make(map[string]*grpc.ClientConn),
		registry: registry,
	}
}

// GetClient returns an OpsService client for the specified agent.
// It creates a new connection if one doesn't exist.
func (m *AgentOpsClient) GetClient(ctx context.Context, agentID string) (opsv1.OpsServiceClient, error) {
	m.mu.RLock()
	client, ok := m.clients[agentID]
	m.mu.RUnlock()

	if ok {
		return client, nil
	}

	// Need to create new connection
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if client, ok := m.clients[agentID]; ok {
		return client, nil
	}

	// Get agent address from registry
	if m.registry == nil {
		return nil, fmt.Errorf("registry not available")
	}

	m.registry.Mu().RLock()
	agent := m.registry.AgentsUnsafe()[agentID]
	m.registry.Mu().RUnlock()

	if agent == nil {
		return nil, fmt.Errorf("agent %q not found", agentID)
	}

	if agent.RPCAddr == "" {
		return nil, fmt.Errorf("agent %q has no RPC address", agentID)
	}

	// Create gRPC connection with timeout
	conn, err := grpc.DialContext(ctx, agent.RPCAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to agent %q: %w", agentID, err)
	}

	client = opsv1.NewOpsServiceClient(conn)
	m.clients[agentID] = client
	m.conns[agentID] = conn

	return client, nil
}

// Close closes the connection for a specific agent.
func (m *AgentOpsClient) Close(agentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	conn, ok := m.conns[agentID]
	if !ok {
		return nil
	}

	delete(m.clients, agentID)
	delete(m.conns, agentID)

	return conn.Close()
}

// CloseAll closes all agent connections.
func (m *AgentOpsClient) CloseAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var firstErr error
	for agentID, conn := range m.conns {
		if err := conn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		delete(m.clients, agentID)
		delete(m.conns, agentID)
	}

	return firstErr
}

// Remove removes a client from the cache (e.g., after agent disconnects).
func (m *AgentOpsClient) Remove(agentID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.clients, agentID)
	delete(m.conns, agentID)
}
