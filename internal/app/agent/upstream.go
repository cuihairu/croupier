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

	"github.com/cuihairu/croupier/internal/nng"
	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	"github.com/cuihairu/croupier/internal/platform/tlsutil"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
)

// UpstreamClient manages the connection to the central Croupier Server.
type UpstreamClient struct {
	serverAddr string
	agentID    string
	store      *agentlocal.LocalStore
	nngClient  *nng.Client
	updateCh   chan struct{}
	gameID     string
	env        string
	version    string
	rpcAddr    string
	region     string
	zone       string
	labels     map[string]string
	tlsCfg     *tlsutil.ClientTLSConfig

	// Timeouts (from config, with defaults)
	dialTimeout       time.Duration
	requestTimeout    time.Duration
	heartbeatInterval time.Duration

	// Metrics reporting
	metricsCollector *MetricsCollector
	metricsInterval  time.Duration
	metricsEnabled   bool
	metricsMu        sync.Mutex
	metricsOnce      sync.Once

	// Connection callbacks
	onConnected    func()      // Called when successfully connected to server
	onDisconnected func(error) // Called when disconnected from server
}

func (c *UpstreamClient) Connected() bool {
	return c != nil && c.nngClient != nil && c.nngClient.Connected()
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
		client.region = meta.Region
		client.zone = meta.Zone
		if meta.Labels != nil {
			client.labels = make(map[string]string, len(meta.Labels))
			for k, v := range meta.Labels {
				client.labels[k] = v
			}
		}
	}

	return client
}

func (c *UpstreamClient) SetTLSConfig(cfg *tlsutil.ClientTLSConfig) {
	c.tlsCfg = cfg
}

// UpstreamMetadata captures optional metadata for registering with server.
type UpstreamMetadata struct {
	GameID            string
	Env               string
	Version           string
	RPCAddr           string
	Region            string            // region/zone info (e.g. "us-west-1")
	Zone              string            // availability zone (e.g. "us-west-1a")
	Labels            map[string]string // system metadata (os, arch, hostname, etc.)
	DialTimeout       time.Duration     // Connection timeout (default 10s)
	RequestTimeout    time.Duration     // Request timeout (default 10s)
	HeartbeatInterval time.Duration     // Heartbeat interval (default 30s)
}

// WithMetadata applies metadata updates for the next sync.
func (c *UpstreamClient) WithMetadata(meta UpstreamMetadata) {
	c.gameID = meta.GameID
	c.env = meta.Env
	c.version = meta.Version
	c.rpcAddr = meta.RPCAddr
	c.region = meta.Region
	c.zone = meta.Zone
	if meta.Labels != nil {
		c.labels = meta.Labels
	}
	if meta.DialTimeout > 0 {
		c.dialTimeout = meta.DialTimeout
	}
	if meta.RequestTimeout > 0 {
		c.requestTimeout = meta.RequestTimeout
	}
	if meta.HeartbeatInterval > 0 {
		c.heartbeatInterval = meta.HeartbeatInterval
	}
}

// OnConnected sets a callback function to be called when successfully connected to server.
// The callback is invoked after successful registration with the server.
func (c *UpstreamClient) OnConnected(callback func()) {
	c.onConnected = callback
}

// OnDisconnected sets a callback function to be called when disconnected from server.
// The callback is invoked with the error that caused the disconnection.
func (c *UpstreamClient) OnDisconnected(callback func(error)) {
	c.onDisconnected = callback
}

// dialServer establishes a new NNG connection to the upstream server and registers.
// It closes any existing connection before establishing a new one.
// On successful connection, it automatically calls register.
func (c *UpstreamClient) dialServer(ctx context.Context) error {
	// Close existing NNG connection if any
	if c.nngClient != nil {
		c.nngClient.Close()
		c.nngClient = nil
	}

	// Create and connect NNG client
	c.nngClient = nng.NewClient(c.serverAddr)
	c.nngClient.SetLogger(slog.Default())

	if err := c.nngClient.Dial(); err != nil {
		c.nngClient = nil
		return fmt.Errorf("failed to connect to upstream server via NNG: %w", err)
	}

	// 连接成功后立即注册
	if err := c.syncOnce(ctx); err != nil {
		c.nngClient.Close()
		c.nngClient = nil
		return fmt.Errorf("failed to register after connection: %w", err)
	}

	return nil
}

// Start begins the upstream synchronization process.
func (c *UpstreamClient) Start(ctx context.Context) error {
	if c.serverAddr == "" {
		slog.Info("upstream server address not configured, skipping upstream connection")
		return nil
	}

	slog.Info("connecting to upstream server", "addr", c.serverAddr, "tls", c.tlsCfg != nil)

	// 初始连接尝试（dialServer 内部会自动注册）
	if err := c.dialServer(ctx); err != nil {
		slog.Warn("⚠️  failed to connect to upstream server, will keep retrying in background...", "addr", c.serverAddr, "error", err)
		// 启动后台重连 goroutine
		go c.reconnectLoop(ctx, true)
	} else {
		slog.Info("✅ upstream connected and registered successfully")
		// Call connection callback if set
		if c.onConnected != nil {
			c.onConnected()
		}
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

// reconnectLoop 持续重试连接上游服务器，直到成功或上下文取消
// needDial 为 true 表示需要先建立连接（内部会自动注册）
func (c *UpstreamClient) reconnectLoop(ctx context.Context, needDial bool) {
	const retryInterval = 5 * time.Second
	var attemptCount int

	for {
		if ctx.Err() != nil {
			slog.Info("upstream reconnect cancelled, exiting retry loop")
			return
		}

		// 建立连接（dialServer 内部会自动注册）
		if needDial {
			attemptCount++
			slog.Info("⏳ waiting for upstream server to be ready...", "attempt", attemptCount, "addr", c.serverAddr)

			if err := c.dialServer(ctx); err != nil {
				slog.Debug("upstream dial attempt failed", "attempt", attemptCount, "error", err)
				time.Sleep(retryInterval)
				continue
			}

			// dialServer 成功 = 连接并注册成功
			slog.Info("✅ upstream reconnected and registered successfully", "attempt", attemptCount)

			// Call connection callback if set
			if c.onConnected != nil {
				c.onConnected()
			}

			return
		}
	}
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
	interval := c.heartbeatInterval
	if interval <= 0 {
		interval = 3 * time.Second // 默认 3 秒
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// 断线重连：连续心跳失败次数阈值（达到阈值时尝试重新注册）
	const maxHeartbeatFailures = 2 // 连续失败 2 次就重连（约 6 秒）
	consecutiveFailures := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 检查客户端是否可用，未连接时跳过本次心跳
			if c.nngClient == nil || !c.nngClient.Connected() {
				slog.Debug("heartbeat skipped: client not connected")
				continue
			}
			// 心跳调用设置超时，避免 server 关闭后一直阻塞
			hbCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			_, err := c.nngClient.Heartbeat(hbCtx, &agentv1.HeartbeatRequest{AgentId: c.agentID})
			cancel()
			if err != nil {
				consecutiveFailures++
				slog.Error("heartbeat failed", "error", err, "consecutive_failures", consecutiveFailures)
				// 连续失败达到阈值，尝试重新连接（内部会自动注册）
				if consecutiveFailures >= maxHeartbeatFailures {
					slog.Warn("❌ heartbeat failed, attempting re-connect and register...", "failures", consecutiveFailures)
					// 重新建立连接（dialServer 内部会自动注册）
					if dialErr := c.dialServer(ctx); dialErr != nil {
						slog.Error("re-connect dial failed", "error", dialErr)
						// 连接失败，继续尝试心跳
						continue
					}
					// dialServer 成功 = 连接并注册成功
					slog.Info("✅ re-connected and registered successfully")
					consecutiveFailures = 0 // 重置计数器
				}
			} else {
				// 心跳成功，重置计数器
				if consecutiveFailures > 0 {
					slog.Info("heartbeat recovered, re-registering to ensure session is active...", "previous_failures", consecutiveFailures)
					// 立即重新注册，确保注册信息是最新的
					if syncErr := c.syncWithRetry(ctx, 1); syncErr != nil {
						slog.Warn("re-register after recovery failed", "error", syncErr)
					} else {
						slog.Info("✅ re-registered successfully after recovery")
					}
					consecutiveFailures = 0
				}
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
		requestTimeout := c.requestTimeout
		if requestTimeout <= 0 {
			requestTimeout = 10 * time.Second
		}
		syncCtx, cancel := context.WithTimeout(ctx, requestTimeout)
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
	if c.nngClient == nil || !c.nngClient.Connected() {
		return fmt.Errorf("NNG client not connected")
	}

	// Snapshot local store
	localData := c.store.List()
	versionSnapshot := c.store.FunctionVersions()
	metaSnapshot := c.store.FunctionMetadata()

	// Convert to FunctionDescriptors
	var funcs []*agentv1.FunctionDescriptor
	for fid, instances := range localData {
		desc := &agentv1.FunctionDescriptor{
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

	req := &agentv1.RegisterRequest{
		AgentId:   c.agentID,
		Version:   c.version,
		RpcAddr:   c.rpcAddr,
		GameId:    c.gameID,
		Env:       c.env,
		Region:    c.region,
		Zone:      c.zone,
		Labels:    c.labels,
		Functions: funcs,
		Processes: processes,
	}

	_, err := c.nngClient.Register(ctx, req)
	if err != nil {
		return err
	}
	slog.Info("synced with upstream server via NNG", "functions", len(funcs))

	// Call connection callback if set
	if c.onConnected != nil {
		c.onConnected()
	}

	return nil
}

func buildProcesses(localData map[string][]agentlocal.Instance, versionSnapshot map[string]map[string]string) []*agentv1.AgentProcess {
	byServiceID := map[string]*agentv1.AgentProcess{}
	fnSeen := map[string]map[string]struct{}{} // service_id -> function_id set

	for fid, instances := range localData {
		fid = strings.TrimSpace(fid)
		if fid == "" {
			continue
		}
		for _, inst := range instances {
			sid := strings.TrimSpace(inst.ProviderID)
			if sid == "" {
				continue
			}
			p := byServiceID[sid]
			if p == nil {
				p = &agentv1.AgentProcess{
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

	out := make([]*agentv1.AgentProcess, 0, len(byServiceID))
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
	if c.nngClient == nil || !c.nngClient.Connected() {
		return fmt.Errorf("upstream client not connected")
	}
	return c.syncWithRetry(ctx, 3)
}

// Heartbeat sends a single heartbeat to the control server.
func (c *UpstreamClient) Heartbeat(ctx context.Context) error {
	if c == nil {
		return fmt.Errorf("upstream client is nil")
	}
	if c.nngClient == nil || !c.nngClient.Connected() {
		return fmt.Errorf("upstream client not connected")
	}
	_, err := c.nngClient.Heartbeat(ctx, &agentv1.HeartbeatRequest{AgentId: c.agentID})
	return err
}

func (c *UpstreamClient) Stop() {
	if c.nngClient != nil {
		c.nngClient.Close()
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
// Note: Metrics reporting via NNG is not yet implemented.
// This method collects metrics but does not send them.
func (c *UpstreamClient) reportMetrics(ctx context.Context) {
	c.metricsOnce.Do(func() {
		if c.metricsCollector == nil {
			c.metricsCollector = NewMetricsCollector(c.agentID)
		}
	})

	// Collect metrics (logging for now)
	_ = c.metricsCollector.Collect(ctx)
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
// Note: Metrics reporting via NNG is not yet implemented.
func (c *UpstreamClient) ReportMetricsOnce(ctx context.Context) error {
	if c == nil {
		return fmt.Errorf("upstream client is nil")
	}

	c.metricsOnce.Do(func() {
		if c.metricsCollector == nil {
			c.metricsCollector = NewMetricsCollector(c.agentID)
		}
	})

	// Collect metrics (not implemented for NNG yet)
	_ = c.metricsCollector.Collect(ctx)
	return fmt.Errorf("metrics reporting via NNG not yet implemented")
}
