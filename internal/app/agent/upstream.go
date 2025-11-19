package agent

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	serverv1 "github.com/cuihairu/croupier/pkg/pb/croupier/server/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// UpstreamClient manages the connection to the central Croupier Server.
type UpstreamClient struct {
	serverAddr string
	agentID    string
	store      *agentlocal.LocalStore
	client     serverv1.ControlServiceClient
	conn       *grpc.ClientConn
}

// NewUpstreamClient creates a new upstream client.
func NewUpstreamClient(serverAddr, agentID string, store *agentlocal.LocalStore) *UpstreamClient {
	return &UpstreamClient{
		serverAddr: serverAddr,
		agentID:    agentID,
		store:      store,
	}
}

// Start begins the upstream synchronization process.
func (c *UpstreamClient) Start(ctx context.Context) error {
	if c.serverAddr == "" {
		slog.Info("upstream server address not configured, skipping upstream connection")
		return nil
	}

	slog.Info("connecting to upstream server", "addr", c.serverAddr)
	conn, err := grpc.Dial(c.serverAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(5*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to upstream server: %w", err)
	}
	c.conn = conn
	c.client = serverv1.NewControlServiceClient(conn)

	// Initial sync
	if err := c.sync(ctx); err != nil {
		slog.Error("initial sync failed", "error", err)
	}

	// Register update callback
	c.store.OnUpdate(func() {
		// Debounce updates slightly? For now, just sync.
		// Use a detached context or the background context since the callback might be async
		if err := c.sync(context.Background()); err != nil {
			slog.Error("sync failed", "error", err)
		}
	})

	// Heartbeat loop
	go c.heartbeatLoop(ctx)

	return nil
}

func (c *UpstreamClient) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := c.client.Heartbeat(ctx, &serverv1.HeartbeatRequest{AgentId: c.agentID}); err != nil {
				slog.Error("heartbeat failed", "error", err)
			}
		}
	}
}

func (c *UpstreamClient) sync(ctx context.Context) error {
	// Snapshot local store
	localData := c.store.List()

	// Convert to FunctionDescriptors
	var funcs []*serverv1.FunctionDescriptor
	for fid := range localData {
		funcs = append(funcs, &serverv1.FunctionDescriptor{
			Id:      fid,
			Enabled: true,
		})
	}

	req := &serverv1.RegisterRequest{
		AgentId:   c.agentID,
		Functions: funcs,
		// TODO: Populate other fields like GameID, Env from config if available
	}

	_, err := c.client.Register(ctx, req)
	if err != nil {
		return err
	}
	slog.Info("synced with upstream server", "functions", len(funcs))
	return nil
}

func (c *UpstreamClient) Stop() {
	if c.conn != nil {
		c.conn.Close()
	}
}
