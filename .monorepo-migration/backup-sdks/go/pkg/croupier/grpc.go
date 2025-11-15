package croupier

import (
	"context"
)

// GRPCManager handles gRPC communication with the agent
type GRPCManager interface {
	// Connect establishes connection to the agent
	Connect(ctx context.Context) error

	// Disconnect closes the connection
	Disconnect()

	// RegisterWithAgent registers functions with the agent
	RegisterWithAgent(ctx context.Context, serviceID, serviceVersion string, functions []LocalFunctionDescriptor) (string, error)

	// StartServer starts the local gRPC server
	StartServer(ctx context.Context) error

	// GetLocalAddress returns the local server address
	GetLocalAddress() string

	// IsConnected returns true if connected to agent
	IsConnected() bool
}

// GRPCConfig holds configuration for gRPC manager
type GRPCConfig struct {
	AgentAddr      string
	LocalListen    string
	TimeoutSeconds int
	Insecure       bool
	CAFile         string
	CertFile       string
	KeyFile        string
}