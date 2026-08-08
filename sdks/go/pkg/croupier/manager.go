// Package croupier provides a Go SDK for Croupier game function registration and execution.
package croupier

import (
	"context"
)

// Manager handles communication with the agent
type Manager interface {
	// Connect establishes connection to the agent
	Connect(ctx context.Context) error

	// Disconnect closes the connection
	Disconnect()

	// RegisterWithAgent registers functions with the agent
	RegisterWithAgent(ctx context.Context, serviceID, serviceVersion string, functions []ProviderFunctionDescriptor) (string, error)

	// IsConnected returns true if connected to agent
	IsConnected() bool

	// SetOnDisconnect sets a callback invoked when connection is lost.
	SetOnDisconnect(fn func())
}

// ManagerConfig holds configuration for creating a Manager
type ManagerConfig struct {
	// AgentAddr is the address of the Croupier agent
	AgentAddr string

	// ControlAddr is the optional control-plane address for manifest upload
	ControlAddr string

	// TimeoutSeconds is the timeout for RPC calls
	TimeoutSeconds int

	// HeartbeatInterval is the heartbeat interval in seconds
	HeartbeatInterval int

	// Insecure disables TLS
	Insecure bool

	// CAFile is the path to the CA certificate
	CAFile string

	// CertFile is the path to the client certificate
	CertFile string

	// KeyFile is the path to the client key
	KeyFile string

	// ServerName is the server name for TLS verification
	ServerName string

	// ProviderLang is the language reported via ProviderMeta
	ProviderLang string

	// ProviderSDK is the SDK identifier reported via ProviderMeta
	ProviderSDK string

	// InsecureSkipVerify skips TLS verification (not recommended)
	InsecureSkipVerify bool
}

// NewManager creates a new Manager using TCP transport
func NewManager(config ManagerConfig, handlers map[string]FunctionHandler) (Manager, error) {
	clientConfig := ClientConfig{
		AgentAddr:          config.AgentAddr,
		ControlAddr:        config.ControlAddr,
		TimeoutSeconds:     config.TimeoutSeconds,
		HeartbeatInterval:  config.HeartbeatInterval,
		Insecure:           config.Insecure,
		CAFile:             config.CAFile,
		CertFile:           config.CertFile,
		KeyFile:            config.KeyFile,
		ServerName:         config.ServerName,
		ProviderLang:       config.ProviderLang,
		ProviderSDK:        config.ProviderSDK,
		InsecureSkipVerify: config.InsecureSkipVerify,
	}
	return NewTCPManager(clientConfig, handlers)
}
