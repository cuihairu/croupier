package croupier

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// grpcManager implements GRPCManager interface
type grpcManager struct {
	config   GRPCConfig
	handlers map[string]FunctionHandler

	// Connection state
	mu        sync.RWMutex
	conn      *grpc.ClientConn
	connected bool
	localAddr string

	// Local server
	server   *grpc.Server
	listener net.Listener

	// Session management
	sessionID string
}

// NewGRPCManager creates a new gRPC manager
func NewGRPCManager(config GRPCConfig, handlers map[string]FunctionHandler) (GRPCManager, error) {
	return &grpcManager{
		config:   config,
		handlers: handlers,
	}, nil
}

// Connect implements GRPCManager.Connect
func (g *grpcManager) Connect(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.connected {
		return nil
	}

	// Create gRPC connection options
	var opts []grpc.DialOption

	if g.config.Insecure {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		// Setup TLS credentials
		creds, err := g.createTLSCredentials()
		if err != nil {
			return fmt.Errorf("failed to create TLS credentials: %w", err)
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	}

	// Set timeout
	if g.config.TimeoutSeconds > 0 {
		timeout := time.Duration(g.config.TimeoutSeconds) * time.Second
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Establish connection
	conn, err := grpc.DialContext(ctx, g.config.AgentAddr, opts...)
	if err != nil {
		return fmt.Errorf("failed to connect to agent at %s: %w", g.config.AgentAddr, err)
	}

	g.conn = conn
	g.connected = true

	fmt.Printf("✅ Successfully connected to Agent: %s\n", g.config.AgentAddr)
	return nil
}

// Disconnect implements GRPCManager.Disconnect
func (g *grpcManager) Disconnect() {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.connected = false

	// Stop local server
	if g.server != nil {
		g.server.GracefulStop()
		g.server = nil
	}

	if g.listener != nil {
		g.listener.Close()
		g.listener = nil
	}

	// Close agent connection
	if g.conn != nil {
		g.conn.Close()
		g.conn = nil
	}

	fmt.Println("📴 Disconnected from Agent")
}

// RegisterWithAgent implements GRPCManager.RegisterWithAgent
func (g *grpcManager) RegisterWithAgent(ctx context.Context, serviceID, serviceVersion string, functions []LocalFunctionDescriptor) (string, error) {
	if !g.connected {
		return "", fmt.Errorf("not connected to agent")
	}

	// This is a mock implementation - in a real implementation, this would:
	// 1. Use generated gRPC client stub from local.proto
	// 2. Call the LocalControlService.RegisterLocal RPC
	// 3. Handle the actual proto message marshaling

	sessionID := fmt.Sprintf("mock_session_%s_%d", serviceID, time.Now().Unix())

	fmt.Printf("📡 Registering service with Agent:\n")
	fmt.Printf("   Service ID: %s\n", serviceID)
	fmt.Printf("   Version: %s\n", serviceVersion)
	fmt.Printf("   Local Address: %s\n", g.localAddr)
	fmt.Printf("   Functions: %d\n", len(functions))
	for _, fn := range functions {
		fmt.Printf("     - %s (v%s)\n", fn.ID, fn.Version)
	}
	fmt.Printf("   Session ID: %s\n", sessionID)

	g.sessionID = sessionID
	return sessionID, nil
}

// StartServer implements GRPCManager.StartServer
func (g *grpcManager) StartServer(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Parse listen address
	listenAddr := g.config.LocalListen
	if listenAddr == "" {
		listenAddr = ":0" // Auto-assign port
	}

	// Create listener
	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", listenAddr, err)
	}

	g.listener = lis
	g.localAddr = lis.Addr().String()

	// Create gRPC server
	var opts []grpc.ServerOption

	if !g.config.Insecure {
		// Setup TLS credentials for server
		creds, err := g.createServerTLSCredentials()
		if err != nil {
			lis.Close()
			return fmt.Errorf("failed to create server TLS credentials: %w", err)
		}
		opts = append(opts, grpc.Creds(creds))
	}

	g.server = grpc.NewServer(opts...)

	// Register function service (placeholder)
	// In a real implementation, register generated service here.

	fmt.Printf("🚀 Local gRPC server started on: %s\n", g.localAddr)

	// Start serving in a goroutine
	go func() {
		if err := g.server.Serve(lis); err != nil {
			log.Printf("gRPC server error: %v", err)
		}
	}()

	return nil
}

// GetLocalAddress implements GRPCManager.GetLocalAddress
func (g *grpcManager) GetLocalAddress() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.localAddr
}

// IsConnected implements GRPCManager.IsConnected
func (g *grpcManager) IsConnected() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.connected
}

// createTLSCredentials creates TLS credentials for client connection
func (g *grpcManager) createTLSCredentials() (credentials.TransportCredentials, error) {
	if g.config.CAFile == "" && g.config.CertFile == "" && g.config.KeyFile == "" {
		// Use system root CAs
		return credentials.NewTLS(nil), nil
	}

	// Load custom certificates
	// This is a simplified implementation - real implementation would
	// handle CA files, client certificates, etc.
	return credentials.NewTLS(nil), nil
}

// createServerTLSCredentials creates TLS credentials for server
func (g *grpcManager) createServerTLSCredentials() (credentials.TransportCredentials, error) {
	// This would load server certificates in a real implementation
	return credentials.NewTLS(nil), nil
}

