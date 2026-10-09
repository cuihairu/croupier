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

	"github.com/cuihairu/croupier/core/register"
	"github.com/cuihairu/croupier/core/report"
	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	"github.com/cuihairu/croupier/internal/platform/tlsutil"
	transportcore "github.com/cuihairu/croupier/internal/transport"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
)

// UpstreamClient manages the connection to the central Croupier Server.
type UpstreamClient struct {
	serverAddr string
	agentID    string
	store      *agentlocal.LocalStore
	// reg 承载注册/心跳/重连循环（上收 core/register，K2）；连接快照经
	// reg.Conn() 取回，扩展发送（任务/指标）经 reporter。
	reg      *register.Client
	reporter *report.Reporter
	updateCh chan struct{}
	gameID   string
	env      string
	version  string
	region   string
	zone     string
	labels   map[string]string
	tlsCfg   *tlsutil.ClientTLSConfig

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
	// supervisorSampler 由装配阶段经 WithSupervisorSampler 注入，
	// 传给懒创建的 metricsCollector。
	supervisorSampler SupervisorSampler

	// Connection callbacks
	onConnected    func()      // Called when successfully connected to server
	onDisconnected func(error) // Called when disconnected from server
	dynamicLabels  func() map[string]string
	transportKind  string

	// localHandler processes inbound requests from Server (e.g., Invoke, StartTask)
	// when using MuxConn-based TCP transport.
	localHandler transportcore.Handler
}

func (c *UpstreamClient) Connected() bool {
	if c == nil {
		return false
	}
	cl := c.currentClient()
	return cl != nil && cl.Connected()
}

func (c *UpstreamClient) GameID() string {
	if c == nil {
		return ""
	}
	return c.gameID
}

func (c *UpstreamClient) Env() string {
	if c == nil {
		return ""
	}
	return c.env
}

func (c *UpstreamClient) SendTaskEvent(ctx context.Context, event *sdkv1.TaskEvent) error {
	if c == nil {
		return fmt.Errorf("upstream client is nil")
	}
	return c.reporter.SendTaskEvent(ctx, event)
}

func (c *UpstreamClient) ReportTaskEvent(ctx context.Context, event *sdkv1.TaskEvent) error {
	return c.SendTaskEvent(ctx, event)
}

// NewUpstreamClient creates a new upstream client.
func NewUpstreamClient(serverAddr, agentID string, store *agentlocal.LocalStore, meta *UpstreamMetadata) *UpstreamClient {
	if meta == nil {
		meta = &UpstreamMetadata{
			GameID:  firstNonEmpty(os.Getenv("CROUPIER_GAME_ID"), os.Getenv("GAME_ID")),
			Env:     firstNonEmpty(os.Getenv("CROUPIER_ENV"), os.Getenv("ENV")),
			Version: firstNonEmpty(os.Getenv("CROUPIER_AGENT_VERSION"), os.Getenv("AGENT_VERSION")),
		}
	}
	client := &UpstreamClient{
		serverAddr:    serverAddr,
		agentID:       agentID,
		store:         store,
		transportKind: "tcp",
	}
	// 注册/心跳/重连上收 core/register（K2）：传输构造与 payload 组装留在
	// 业务侧（闭包晚绑定字段，SetTLSConfig/SetLocalHandler/WithMetadata 生效）。
	client.reg = register.New(register.Config{
		AgentID:     agentID,
		Dial:        client.dialUpstream,
		Payload:     client.buildRegisterPayload,
		OnConnected: func() { client.fireOnConnected() },
	})
	client.reporter = report.New(func() report.Conn {
		if cc := client.currentClient(); cc != nil {
			return cc
		}
		return nil
	})
	if meta != nil {
		client.gameID = meta.GameID
		client.env = meta.Env
		client.version = meta.Version
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

func (c *UpstreamClient) SetTransportKind(kind string) {
	// Validate: only "tcp" is supported after 旧传输 removal
	kind = strings.TrimSpace(strings.ToLower(kind))
	if kind == "" {
		kind = "tcp" // default to tcp if empty
	}
	if kind != "tcp" {
		panic(fmt.Sprintf("unsupported transport kind: %q (only 'tcp' is supported)", kind))
	}
	c.transportKind = kind
}

// SetLocalHandler sets the handler for inbound requests from Server.
// This is used when the upstream connection uses MuxConn (bidirectional TCP).
func (c *UpstreamClient) SetLocalHandler(h transportcore.Handler) {
	c.localHandler = h
}

func (c *UpstreamClient) SetDynamicLabelsProvider(fn func() map[string]string) {
	c.dynamicLabels = fn
}

// UpstreamMetadata captures optional metadata for registering with server.
type UpstreamMetadata struct {
	GameID            string
	Env               string
	Version           string
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
		c.reg.SetHeartbeatInterval(meta.HeartbeatInterval)
	}
	if meta.RequestTimeout > 0 {
		c.reg.SetRequestTimeout(meta.RequestTimeout)
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

// dialUpstream 建立上游连接（业务侧传输构造：带本地 handler 走 MuxConn
// 双向通道，否则简单 TCP 客户端），供 core/register Dialer 调用。
func (c *UpstreamClient) dialUpstream(ctx context.Context) (register.Conn, error) {
	if c.localHandler != nil {
		return newMuxControlClient(c.serverAddr, c.localHandler, c.tlsCfg)
	}
	return newControlClient(c.transportKind, c.serverAddr, c.tlsCfg)
}

// dialServer 建立连接并注册（转发 core/register，重连/心跳自愈共用）。
func (c *UpstreamClient) dialServer(ctx context.Context) error {
	return c.reg.DialAndRegister(ctx)
}

// fireOnConnected 触发业务侧连接回调（core 每次注册成功后调用）。
func (c *UpstreamClient) fireOnConnected() {
	if c != nil && c.onConnected != nil {
		c.onConnected()
	}
}

// notifyUpdate 是 store 变更回调：向 updateCh 发送去抖通知，channel 满
// （已有一条待处理通知）时丢弃——updateLoop 按 debounce 周期消费，旧通知
// 未被消费前新变更无需重复通知。
func (c *UpstreamClient) notifyUpdate() {
	select {
	case c.updateCh <- struct{}{}:
	default:
	}
}

// Start begins the upstream synchronization process.
// 注册/心跳/重连循环上收 core/register：初始连接注册、失败后台重连、
// 心跳自愈全部由 reg.Start 驱动；本方法只补业务面（函数同步去抖循环与
// 指标上报循环）。
func (c *UpstreamClient) Start(ctx context.Context) error {
	if c.serverAddr == "" {
		slog.Info("upstream server address not configured, skipping upstream connection")
		return nil
	}

	slog.Info("connecting to upstream server", "addr", c.serverAddr, "tls", c.tlsCfg != nil)

	c.reg.Start(ctx)
	if c.reg.Connected() {
		slog.Info("✅ upstream connected and registered successfully")
		c.fireOnConnected() // 保持既有行为：初始成功路径回调两次（注册一次+Start 一次）
	}

	// Register update callback
	c.updateCh = make(chan struct{}, 1)
	c.store.OnUpdate(c.notifyUpdate)
	go c.updateLoop(ctx, 500*time.Millisecond)

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

// reconnectLoop 后台重连循环（转发 core/register；needDial 参数为历史签名
// 兼容位，core 语义恒为「先建连再注册」）。
func (c *UpstreamClient) reconnectLoop(ctx context.Context, needDial bool) {
	_ = needDial
	c.reg.RunReconnectLoop(ctx)
}

// stopAndResetTimer 安全重置去抖定时器。Go 1.23 起 time.Timer 的 channel
// 为 unbuffered 且官方保证 Stop/Reset 后不会再收到过期触发值（stale
// value），经典的「Stop 返回 false 时先排空 timer.C」排水模式已无必要，
// 原排水 select 恒走 default（本仓库 go 1.26），作为死代码删除。
func stopAndResetTimer(timer *time.Timer, debounce time.Duration) {
	timer.Stop()
	timer.Reset(debounce)
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
				stopAndResetTimer(timer, debounce)
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

// heartbeatLoop 心跳自愈循环（转发 core/register：断连主动重连、失败达
// 阈值重连、恢复后重注册；间隔默认 3s，经 WithMetadata 调整）。
func (c *UpstreamClient) heartbeatLoop(ctx context.Context) {
	c.reg.RunHeartbeatLoop(ctx)
}

// syncWithRetry 带退避重试注册（转发 core/register，attempts<=0 按 1 次）。
func (c *UpstreamClient) syncWithRetry(ctx context.Context, attempts int) error {
	return c.reg.SyncAttempts(ctx, attempts)
}

// buildRegisterPayload 组装 RegisterRequest（业务面：本地函数库快照→
// FunctionDescriptor→AgentProcess 清单）。连接校验与 Register 调用在
// core/register（注册前重新取连接快照，覆盖组装期间连接被置空的窗口）。
func (c *UpstreamClient) buildRegisterPayload(_ context.Context) (*agentv1.RegisterRequest, error) {
	// Snapshot local store
	localData := c.store.List()
	versionSnapshot := c.store.FunctionVersions()
	metaSnapshot := c.store.FunctionMetadata()

	slog.Info("[upstream] syncing functions", "function_count", len(localData), "agent_id", c.agentID)

	// Convert to FunctionDescriptors with complete information
	var funcs []*agentv1.FunctionDescriptor
	for fid, instances := range localData {
		meta := metaSnapshot[fid]
		desc := &agentv1.FunctionDescriptor{
			Id:      fid,
			Enabled: len(instances) > 0,
			Version: pickVersion(versionSnapshot[fid]),
		}
		// Copy all metadata fields if available
		if meta != nil {
			desc.Resource = meta.Resource
			desc.Risk = meta.Risk
			desc.Operation = meta.Operation
			desc.Capability = meta.Capability
			desc.Execution = meta.Execution
			desc.ApprovalRequired = meta.ApprovalRequired
			desc.ApprovalPolicyKey = meta.ApprovalPolicyKey
			desc.Permission = meta.Permission
			desc.InputSchema = meta.InputSchema
			desc.OutputSchema = meta.OutputSchema
			// Ensure tags is never nil
			if meta.Tags != nil {
				desc.Tags = meta.Tags
			} else {
				desc.Tags = []string{}
			}
			desc.Summary = meta.Summary
			desc.Description = meta.Description
			desc.Deprecated = meta.Deprecated
		}
		funcs = append(funcs, desc)
	}

	providers := buildProviders(localData, versionSnapshot)

	req := &agentv1.RegisterRequest{
		AgentId:   c.agentID,
		Version:   c.version,
		GameId:    c.gameID,
		Env:       c.env,
		Region:    c.region,
		Zone:      c.zone,
		Labels:    c.composeLabels(),
		Functions: funcs,
		Processes: providers,
	}
	return req, nil
}

// syncOnce 单次注册（转发 core/register）。
func (c *UpstreamClient) syncOnce(ctx context.Context) error {
	return c.reg.SyncOnce(ctx)
}

func buildProviders(localData map[string][]agentlocal.Instance, versionSnapshot map[string]map[string]string) []*agentv1.AgentProcess {
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
				// 从 Instance.Metadata 填充 SDK 信息（参考 Nacos metadata）
				if inst.Metadata != nil {
					p.SdkLanguage = strings.TrimSpace(inst.Metadata["sdkLanguage"])
					p.SdkVersion = strings.TrimSpace(inst.Metadata["sdkVersion"])
					p.SdkName = strings.TrimSpace(inst.Metadata["sdkName"])
					p.GameId = strings.TrimSpace(inst.Metadata["gameId"])
					p.Env = strings.TrimSpace(inst.Metadata["env"])
					// 用户自定义实例元数据（serverId 等多 KV）原样上报（保留键
					// 已在合并时剥离），供服务端 SDK 分布页展示/搜索。
					p.Metadata = agentlocal.UserMetadata(inst.Metadata)
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
	return c.reg.Sync(ctx)
}

// Heartbeat sends a single heartbeat to the control server.
func (c *UpstreamClient) Heartbeat(ctx context.Context) error {
	if c == nil {
		return fmt.Errorf("upstream client is nil")
	}
	hbClient := c.currentClient()
	if hbClient == nil {
		return fmt.Errorf("upstream client not connected")
	}
	_, err := hbClient.Heartbeat(ctx, &agentv1.HeartbeatRequest{AgentId: c.agentID})
	return err
}

// Stop 关闭当前上游连接（core/register 持有连接状态）。
func (c *UpstreamClient) Stop() {
	if c == nil {
		return
	}
	c.reg.Stop()
}

// currentClient 返回当前上游连接快照（core/register 持有；扩展发送断言回
// 业务侧完整 controlClient 面）。
func (c *UpstreamClient) currentClient() controlClient {
	if c == nil {
		return nil
	}
	if conn := c.reg.Conn(); conn != nil {
		if cc, ok := conn.(controlClient); ok {
			return cc
		}
	}
	return nil
}

// setClient 注入当前连接（测试/高级装配；正常路径经 core 重连建立）。
func (c *UpstreamClient) setClient(cl controlClient) {
	if cl == nil {
		c.reg.SetConn(nil)
		return
	}
	c.reg.SetConn(cl)
}

// setOwnerInstance 记录最近注册响应中的集群实例 ID（测试/高级装配；正常
// 路径来自注册响应）。
func (c *UpstreamClient) setOwnerInstance(v string) {
	c.reg.SetOwnerInstance(v)
}

// ownerInstance 返回最近注册响应中的集群实例 ID（心跳携带，三方对账）。
func (c *UpstreamClient) ownerInstance() string {
	return c.reg.OwnerInstance()
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

// SendMetricEvent serialises a MetricsReport and pushes it to the server as
// a one-way MetricEvent. Returns an error if the client is nil, not connected,
// or the underlying transport rejects the send.
func (c *UpstreamClient) SendMetricEvent(ctx context.Context, metricsReport *opsv1.MetricsReport) error {
	if c == nil {
		return fmt.Errorf("upstream client is nil")
	}
	return c.reporter.SendMetrics(ctx, metricsReport)
}

// reportMetrics collects a single snapshot and pushes it upstream. Errors
// are logged but not returned because this runs from the periodic ticker
// loop where a single failed cycle should not stop reporting.
func (c *UpstreamClient) reportMetrics(ctx context.Context) {
	c.metricsOnce.Do(func() {
		if c.metricsCollector == nil {
			c.metricsCollector = NewMetricsCollector(c.agentID)
		}
		c.metricsCollector.WithSampler(c.supervisorSampler)
	})

	report := c.metricsCollector.Collect(ctx)
	if err := c.SendMetricEvent(ctx, report); err != nil {
		slog.Debug("metrics report failed", "agent_id", c.agentID, "err", err)
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
	c.metricsCollector.WithSampler(c.supervisorSampler)
}

// WithSupervisorSampler attaches a supervised-process sampler that is folded
// into every metrics report. Call during assembly, before the reporting loop
// starts.
func (c *UpstreamClient) WithSupervisorSampler(s SupervisorSampler) {
	if c == nil {
		return
	}
	c.metricsMu.Lock()
	defer c.metricsMu.Unlock()

	c.supervisorSampler = s
	if c.metricsCollector != nil {
		c.metricsCollector.WithSampler(s)
	}
}

// ReportMetricsOnce collects the current metrics snapshot and pushes it
// upstream as a single MetricEvent. Use this for manual / on-demand reporting;
// the periodic loop driven by startMetricsLoop calls reportMetrics directly.
func (c *UpstreamClient) ReportMetricsOnce(ctx context.Context) error {
	if c == nil {
		return fmt.Errorf("upstream client is nil")
	}

	c.metricsOnce.Do(func() {
		if c.metricsCollector == nil {
			c.metricsCollector = NewMetricsCollector(c.agentID)
		}
		c.metricsCollector.WithSampler(c.supervisorSampler)
	})

	report := c.metricsCollector.Collect(ctx)
	return c.SendMetricEvent(ctx, report)
}

// toTitle converts a string to title case (first letter uppercase)
func sanitizeNodeKey(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "/", "_")
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			out = append(out, r)
		}
	}
	return strings.Trim(string(out), "._-")
}

func defaultAgentMenuPath(fid, entity string) string {
	if e := sanitizeNodeKey(entity); e != "" {
		return "/game/entities/" + e
	}
	return "/game/functions/invoke?fid=" + fid
}

// operationToVerbs converts an operation type to permission verbs
func operationToVerbs(operation string) []string {
	switch operation {
	case "create":
		return []string{"create", "write"}
	case "read", "get", "list":
		return []string{"read", "view"}
	case "update", "edit":
		return []string{"update", "edit", "write"}
	case "delete", "remove":
		return []string{"delete", "remove"}
	case "invoke", "execute":
		return []string{"invoke", "execute"}
	case "custom":
		return []string{"invoke"}
	default:
		return []string{"invoke"}
	}
}

func (c *UpstreamClient) composeLabels() map[string]string {
	if c == nil {
		return nil
	}
	base := cloneLabels(c.labels)
	if c.dynamicLabels == nil {
		return base
	}
	dynamic := c.dynamicLabels()
	if len(dynamic) == 0 {
		return base
	}
	if base == nil {
		base = map[string]string{}
	}
	for k, v := range dynamic {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		base[key] = strings.TrimSpace(v)
	}
	return base
}

func cloneLabels(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}
