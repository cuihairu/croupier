package agent

import (
	"context"
	"fmt"
	"log/slog"
	"os"
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
	gameID     string
	env        string
	version    string
	rpcAddr    string
}

// NewUpstreamClient creates a new upstream client.
func NewUpstreamClient(serverAddr, agentID string, store *agentlocal.LocalStore, meta *UpstreamMetadata) *UpstreamClient {
	if meta == nil {
		meta = &UpstreamMetadata{
			GameID:  firstNonEmpty(os.Getenv("CROUPIER_GAME_ID"), os.Getenv("GAME_ID")),
			Env:     firstNonEmpty(os.Getenv("CROUPIER_ENV"), os.Getenv("ENV")),
			Version: firstNonEmpty(os.Getenv("CROUPIER_AGENT_VERSION"), os.Getenv("AGENT_VERSION")),
			RPCAddr: os.Getenv("CROUPIER_AGENT_RPC_ADDR"),
		}
	}
	client := &UpstreamClient{
		serverAddr: serverAddr,
		agentID:    agentID,
		store:      store,
	}
	if meta != nil {
		client.gameID = meta.GameID
		client.env = meta.Env
		client.version = meta.Version
		client.rpcAddr = meta.RPCAddr
	}
	return client
}

// UpstreamMetadata captures optional metadata for registering with server.
type UpstreamMetadata struct {
	GameID  string
	Env     string
	Version string
	RPCAddr string
}

// WithMetadata applies metadata updates for the next sync.
func (c *UpstreamClient) WithMetadata(meta UpstreamMetadata) {
	c.gameID = meta.GameID
	c.env = meta.Env
	c.version = meta.Version
	c.rpcAddr = meta.RPCAddr
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
	versionSnapshot := c.store.FunctionVersions()

	// Convert to FunctionDescriptors
	var funcs []*serverv1.FunctionDescriptor
	for fid, instances := range localData {
		desc := &serverv1.FunctionDescriptor{
			Id:      fid,
			Enabled: len(instances) > 0,
			Version: pickVersion(versionSnapshot[fid]),
		}
		funcs = append(funcs, desc)
	}

	req := &serverv1.RegisterRequest{
		AgentId:   c.agentID,
		Version:   c.version,
		RpcAddr:   c.rpcAddr,
		GameId:    c.gameID,
		Env:       c.env,
		Functions: funcs,
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

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func pickVersion(versions map[string]string) string {
	best := ""
	for _, ver := range versions {
		if ver == "" {
			continue
		}
		if best == "" || ver > best {
			best = ver
		}
	}
	return best
}
