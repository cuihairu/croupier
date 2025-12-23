package agent

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	"github.com/cuihairu/croupier/internal/platform/tlsutil"
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
	updateCh   chan struct{}
	gameID     string
	env        string
	version    string
	rpcAddr    string
	// TLS configuration
	tlsEnabled bool
	certFile   string
	keyFile    string
	caFile     string
	serverName string
}

func (c *UpstreamClient) Connected() bool {
	return c != nil && c.client != nil && c.conn != nil
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

	// Read TLS configuration from environment
	client.tlsEnabled = os.Getenv("CROUPIER_SERVER_TLS_ENABLED") == "true"
	client.certFile = os.Getenv("CROUPIER_CLIENT_CERT_FILE")
	client.keyFile = os.Getenv("CROUPIER_CLIENT_KEY_FILE")
	client.caFile = os.Getenv("CROUPIER_CA_FILE")
	client.serverName = os.Getenv("CROUPIER_SERVER_NAME")

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

	slog.Info("connecting to upstream server", "addr", c.serverAddr, "tls", c.tlsEnabled)

	var dialOpts []grpc.DialOption
	if c.tlsEnabled {
		// Use TLS
		creds, err := tlsutil.ClientTLS(c.certFile, c.keyFile, c.caFile, c.serverName)
		if err != nil {
			return fmt.Errorf("failed to create TLS credentials: %w", err)
		}
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
	} else {
		// Use insecure connection
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	dialOpts = append(dialOpts, grpc.WithBlock())

	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(dialCtx, c.serverAddr, dialOpts...)
	if err != nil {
		return fmt.Errorf("failed to connect to upstream server: %w", err)
	}
	c.conn = conn
	c.client = serverv1.NewControlServiceClient(conn)

	// Initial sync
	if err := c.syncWithRetry(ctx, 3); err != nil {
		slog.Error("initial sync failed", "error", err)
	}

	// Register update callback
	c.updateCh = make(chan struct{}, 1)
	c.store.OnUpdate(func() {
		select {
		case c.updateCh <- struct{}{}:
		default:
		}
	})
	go c.updateLoop(ctx, 500*time.Millisecond)

	// Heartbeat loop
	go c.heartbeatLoop(ctx)

	return nil
}

func (c *UpstreamClient) updateLoop(ctx context.Context, debounce time.Duration) {
	var timer *time.Timer
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.updateCh:
			if timer == nil {
				timer = time.NewTimer(debounce)
			} else {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(debounce)
			}
		case <-func() <-chan time.Time {
			if timer == nil {
				return nil
			}
			return timer.C
		}():
			timer = nil
			if err := c.syncWithRetry(ctx, 3); err != nil {
				slog.Error("sync failed", "error", err)
			}
		}
	}
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

func (c *UpstreamClient) syncWithRetry(ctx context.Context, attempts int) error {
	if attempts <= 0 {
		attempts = 1
	}
	var lastErr error
	backoff := 200 * time.Millisecond
	for i := 0; i < attempts; i++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		syncCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := c.syncOnce(syncCtx)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		if backoff < 2*time.Second {
			backoff *= 2
		}
	}
	return lastErr
}

func (c *UpstreamClient) syncOnce(ctx context.Context) error {
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

// Sync forces a best-effort Register call to the control server.
func (c *UpstreamClient) Sync(ctx context.Context) error {
	if c == nil {
		return fmt.Errorf("upstream client is nil")
	}
	if c.client == nil {
		return fmt.Errorf("upstream client not connected")
	}
	return c.syncWithRetry(ctx, 3)
}

// Heartbeat sends a single heartbeat to the control server.
func (c *UpstreamClient) Heartbeat(ctx context.Context) error {
	if c == nil {
		return fmt.Errorf("upstream client is nil")
	}
	if c.client == nil {
		return fmt.Errorf("upstream client not connected")
	}
	_, err := c.client.Heartbeat(ctx, &serverv1.HeartbeatRequest{AgentId: c.agentID})
	return err
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
