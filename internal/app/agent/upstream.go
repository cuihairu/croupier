package agent

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	"github.com/cuihairu/croupier/internal/platform/tlsutil"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
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
	opsClient  opsv1.OpsServiceClient
	conn       *grpc.ClientConn
	updateCh   chan struct{}
	gameID     string
	env        string
	version    string
	rpcAddr    string
	tlsCfg     *tlsutil.ClientTLSConfig

	// Metrics reporting
	metricsCollector *MetricsCollector
	metricsInterval  time.Duration
	metricsEnabled   bool
	metricsMu        sync.Mutex
	metricsOnce      sync.Once
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

	return client
}

func (c *UpstreamClient) SetTLSConfig(cfg *tlsutil.ClientTLSConfig) {
	c.tlsCfg = cfg
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

	slog.Info("connecting to upstream server", "addr", c.serverAddr, "tls", c.tlsCfg != nil)

	var dialOpts []grpc.DialOption
	if c.tlsCfg != nil {
		cfg := *c.tlsCfg
		if strings.TrimSpace(cfg.ServerName) == "" {
			cfg.ServerName = hostFromTarget(c.serverAddr)
		}
		creds, err := tlsutil.ClientTLSFromConfig(cfg)
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
	c.opsClient = opsv1.NewOpsServiceClient(conn)

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

	// Metrics reporting loop (if enabled)
	c.metricsMu.Lock()
	if c.metricsEnabled {
		c.metricsMu.Unlock()
		go c.metricsLoop(ctx)
	} else {
		c.metricsMu.Unlock()
	}

	return nil
}

func hostFromTarget(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(target)
	if err == nil {
		return strings.Trim(host, "[]")
	}
	// best-effort: handle "host" or "[ipv6]" without port
	return strings.Trim(strings.TrimPrefix(target, "["), "]")
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
	metaSnapshot := c.store.FunctionMetadata()

	// Convert to FunctionDescriptors
	var funcs []*serverv1.FunctionDescriptor
	for fid, instances := range localData {
		desc := &serverv1.FunctionDescriptor{
			Id:      fid,
			Enabled: len(instances) > 0,
			Version: pickVersion(versionSnapshot[fid]),
		}
		// Copy schema fields from metadata if available
		if meta := metaSnapshot[fid]; meta != nil {
			desc.InputSchema = meta.InputSchema
			desc.OutputSchema = meta.OutputSchema
		}
		funcs = append(funcs, desc)
	}

	processes := buildProcesses(localData, versionSnapshot)

	req := &serverv1.RegisterRequest{
		AgentId:   c.agentID,
		Version:   c.version,
		RpcAddr:   c.rpcAddr,
		GameId:    c.gameID,
		Env:       c.env,
		Functions: funcs,
		Processes: processes,
	}

	_, err := c.client.Register(ctx, req)
	if err != nil {
		return err
	}
	slog.Info("synced with upstream server", "functions", len(funcs))
	return nil
}

func buildProcesses(localData map[string][]agentlocal.Instance, versionSnapshot map[string]map[string]string) []*serverv1.AgentProcess {
	byServiceID := map[string]*serverv1.AgentProcess{}
	fnSeen := map[string]map[string]struct{}{} // service_id -> function_id set

	for fid, instances := range localData {
		fid = strings.TrimSpace(fid)
		if fid == "" {
			continue
		}
		for _, inst := range instances {
			sid := strings.TrimSpace(inst.ServiceID)
			if sid == "" {
				continue
			}
			p := byServiceID[sid]
			if p == nil {
				p = &serverv1.AgentProcess{
					ServiceId: sid,
					Addr:      strings.TrimSpace(inst.Addr),
					Version:   strings.TrimSpace(inst.Version),
				}
				byServiceID[sid] = p
				fnSeen[sid] = map[string]struct{}{}
			}

			// best-effort: keep the freshest addr/version/last_seen.
			if p.Addr == "" && inst.Addr != "" {
				p.Addr = strings.TrimSpace(inst.Addr)
			}
			if p.Version == "" && inst.Version != "" {
				p.Version = strings.TrimSpace(inst.Version)
			}
			seenUnix := inst.LastSeen.Unix()
			if seenUnix > p.LastSeenUnix {
				p.LastSeenUnix = seenUnix
			}

			if _, ok := fnSeen[sid][fid]; ok {
				continue
			}
			fnSeen[sid][fid] = struct{}{}
			p.FunctionIds = append(p.FunctionIds, fid)

			// If service version isn't set, try function-version snapshot as fallback.
			if strings.TrimSpace(p.Version) == "" {
				if ver := pickVersion(versionSnapshot[fid]); ver != "" {
					p.Version = ver
				}
			}
		}
	}

	out := make([]*serverv1.AgentProcess, 0, len(byServiceID))
	for _, p := range byServiceID {
		out = append(out, p)
	}
	return out
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

// metricsLoop periodically reports metrics to the upstream server.
func (c *UpstreamClient) metricsLoop(ctx context.Context) {
	interval := c.metricsInterval
	if interval <= 0 {
		interval = 30 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Report immediately on start
	c.reportMetrics(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.reportMetrics(ctx)
		}
	}
}

// reportMetrics sends a single metrics report to the upstream server.
func (c *UpstreamClient) reportMetrics(ctx context.Context) {
	if c.opsClient == nil {
		return
	}

	c.metricsOnce.Do(func() {
		if c.metricsCollector == nil {
			c.metricsCollector = NewMetricsCollector(c.agentID)
		}
	})

	report := c.metricsCollector.Collect(ctx)

	if _, err := c.opsClient.ReportMetrics(ctx, report); err != nil {
		slog.Debug("failed to report metrics", "error", err)
	}
}

// WithMetricsReporting enables and configures periodic metrics reporting.
// interval: reporting interval (default 30s if 0)
func (c *UpstreamClient) WithMetricsReporting(interval time.Duration) {
	if c == nil {
		return
	}
	c.metricsMu.Lock()
	defer c.metricsMu.Unlock()

	c.metricsEnabled = true
	c.metricsInterval = interval
	if c.metricsCollector == nil {
		c.metricsCollector = NewMetricsCollector(c.agentID)
	}
}

// ReportMetricsOnce sends a single metrics report (for manual trigger).
func (c *UpstreamClient) ReportMetricsOnce(ctx context.Context) error {
	if c == nil || c.opsClient == nil {
		return fmt.Errorf("upstream client not connected")
	}

	c.metricsOnce.Do(func() {
		if c.metricsCollector == nil {
			c.metricsCollector = NewMetricsCollector(c.agentID)
		}
	})

	report := c.metricsCollector.Collect(ctx)
	_, err := c.opsClient.ReportMetrics(ctx, report)
	return err
}
