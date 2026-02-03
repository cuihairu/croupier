package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/cuihairu/croupier/internal/nng"
	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	"github.com/cuihairu/croupier/internal/platform/tlsutil"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"google.golang.org/protobuf/types/known/emptypb"
)

// App assembles NNG-based services for Agent process.
type App struct {
	store           *agentlocal.LocalStore
	jobs            *jobIndex
	upstream        *UpstreamClient
	outTLS          *tlsutil.ClientTLSConfig
	platformManager *PlatformManager
	configDir       string

	// NNG server (replaces gRPC)
	nngServer *nng.AgentServer
	nngAddr   string

	// Ops module (optional)
	opsConfig *OpsConfig
	opsServer *OpsServer
	agentID   string
	version   string
}

func New(serverAddr, agentID string) *App {
	store := agentlocal.NewLocalStore()
	return &App{
		store:     store,
		jobs:      newJobIndex(),
		upstream:  NewUpstreamClient(serverAddr, agentID, store, nil),
		configDir: getConfigDir(),
		agentID:   agentID,
		nngAddr:   ":19091", // Default NNG Agent local service port
	}
}

func NewWithConfigDir(serverAddr, agentID, configDir string) *App {
	store := agentlocal.NewLocalStore()
	return &App{
		store:     store,
		jobs:      newJobIndex(),
		upstream:  NewUpstreamClient(serverAddr, agentID, store, nil),
		configDir: configDir,
		agentID:   agentID,
		nngAddr:   ":19091", // Default NNG Agent local service port
	}
}

func getConfigDir() string {
	// Check CROUPIER_CONFIG_DIR first
	if dir := os.Getenv("CROUPIER_CONFIG_DIR"); dir != "" {
		return dir
	}
	// Default to ./configs relative to working directory
	return "./configs"
}

// StartNNGServer starts the NNG server for local services.
// Replaces RegisterGRPC for NNG-based communication.
func (a *App) StartNNGServer() error {
	// Initialize platform manager
	a.platformManager = NewPlatformManager(a.store, a.configDir, nil)

	// Create NNG server
	a.nngServer = nng.NewAgentServer(a.nngAddr, a.store)

	// Set platform manager - wrap it to implement the interface
	pmWrapper := &platformManagerWrapper{pm: a.platformManager}
	a.nngServer.SetPlatformManager(pmWrapper)

	// Set TLS config for outbound connections
	if a.outTLS != nil {
		a.nngServer.SetTLSConfig(a.outTLS)
	}

	// Initialize and set ops server wrapper
	if a.opsConfig == nil {
		a.opsConfig = DefaultOpsConfig()
	}
	a.opsServer = NewOpsServer(a.opsConfig, a.agentID, a.version, nil)

	// Wrap ops server for NNG
	opsWrapper := &opsServerWrapper{ops: a.opsServer}
	a.nngServer.SetOpsServer(opsWrapper)

	// Start NNG server
	if err := a.nngServer.Start(); err != nil {
		return fmt.Errorf("failed to start NNG server: %w", err)
	}

	return nil
}

// GetNNGServerAddr returns the NNG server address
func (a *App) GetNNGServerAddr() string {
	if a == nil || a.nngServer == nil {
		return ""
	}
	return a.nngServer.GetAddr()
}

// SetNNGAddr sets the NNG server address
func (a *App) SetNNGAddr(addr string) {
	if a == nil {
		return
	}
	a.nngAddr = addr
}

// Run starts the agent's background processes (upstream sync and NNG server).
func (a *App) Run(ctx context.Context) error {
	// Start NNG server for local services
	if err := a.StartNNGServer(); err != nil {
		return fmt.Errorf("failed to start NNG server: %w", err)
	}

	// Load platforms before starting upstream
	if a.platformManager != nil {
		if err := a.platformManager.Load(ctx); err != nil {
			return err
		}
	}

	a.startMaintenance(ctx)
	return a.upstream.Start(ctx)
}

func (a *App) startMaintenance(ctx context.Context) {
	if a == nil || a.store == nil {
		return
	}

	pruneInterval := parseDurationEnv("CROUPIER_AGENTLOCAL_PRUNE_INTERVAL", 30*time.Second)
	maxAge := parseDurationEnv("CROUPIER_AGENTLOCAL_MAX_AGE", 2*time.Minute)
	jobResultMaxAge := parseDurationEnv("CROUPIER_AGENTLOCAL_JOBRESULT_MAX_AGE", 10*time.Minute)
	if pruneInterval <= 0 || maxAge <= 0 {
		return
	}

	go func() {
		ticker := time.NewTicker(pruneInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				a.store.Prune(maxAge)
				if jobResultMaxAge > 0 {
					a.store.CleanupOldJobResults(jobResultMaxAge)
				}
			}
		}
	}()
}

func parseDurationEnv(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

// Stop shuts down background upstream connection and NNG server.
func (a *App) Stop() {
	if a == nil {
		return
	}
	// Stop NNG server
	if a.nngServer != nil {
		a.nngServer.Stop()
	}
	// Stop upstream connection
	if a.upstream != nil {
		a.upstream.Stop()
	}
}

// WithUpstreamMetadata updates metadata fields propagated to the control server.
func (a *App) WithUpstreamMetadata(meta UpstreamMetadata) {
	if a == nil || a.upstream == nil {
		return
	}
	a.upstream.WithMetadata(meta)
}

func (a *App) WithUpstreamTLSConfig(cfg *tlsutil.ClientTLSConfig) {
	if a == nil || a.upstream == nil {
		return
	}
	a.upstream.SetTLSConfig(cfg)
}

func (a *App) WithOutboundTLSConfig(cfg *tlsutil.ClientTLSConfig) {
	if a == nil {
		return
	}
	a.outTLS = cfg
}

// Store exposes the local instance registry.
func (a *App) Store() *agentlocal.LocalStore {
	if a == nil {
		return nil
	}
	return a.store
}

// SyncUpstream forces a best-effort upstream register call.
func (a *App) SyncUpstream(ctx context.Context) error {
	if a == nil || a.upstream == nil {
		return errors.New("agent upstream not initialized")
	}
	return a.upstream.Sync(ctx)
}

// HeartbeatUpstream triggers a best-effort upstream heartbeat call.
func (a *App) HeartbeatUpstream(ctx context.Context) error {
	if a == nil || a.upstream == nil {
		return errors.New("agent upstream not initialized")
	}
	return a.upstream.Heartbeat(ctx)
}

// WithOpsConfig sets the ops module configuration.
// Call this before RegisterGRPC to configure ops capabilities.
func (a *App) WithOpsConfig(cfg *OpsConfig) {
	if a == nil {
		return
	}
	a.opsConfig = cfg

	// Enable metrics reporting if ops is configured
	if cfg != nil && cfg.MetricsEnabled {
		a.upstream.WithMetricsReporting(cfg.MetricsInterval)
	}
}

// WithVersion sets the agent version for reporting.
func (a *App) WithVersion(version string) {
	if a == nil {
		return
	}
	a.version = version
}

// OpsServer returns the ops server instance (for testing or direct access).
func (a *App) OpsServer() *OpsServer {
	if a == nil {
		return nil
	}
	return a.opsServer
}

// MetricsCollector returns a new metrics collector for this agent.
func (a *App) MetricsCollector() *MetricsCollector {
	if a == nil {
		return nil
	}
	return NewMetricsCollector(a.agentID)
}

// Jobs returns the job index for tracking active jobs.
func (a *App) Jobs() *jobIndex {
	if a == nil {
		return nil
	}
	return a.jobs
}

// ActiveJobCount returns the number of currently active jobs.
func (a *App) ActiveJobCount() int {
	if a == nil || a.jobs == nil {
		return 0
	}
	return a.jobs.Len()
}

// opsServerWrapper wraps OpsServer to implement nng.OpsServerWrapper interface
type opsServerWrapper struct {
	ops *OpsServer
}

func (w *opsServerWrapper) GetSystemInfo(ctx context.Context, req *emptypb.Empty) (*opsv1.SystemInfo, error) {
	return w.ops.GetSystemInfo(ctx, req)
}

func (w *opsServerWrapper) ListProcesses(ctx context.Context, req *emptypb.Empty) (*opsv1.ListProcessesResponse, error) {
	return w.ops.ListProcesses(ctx, req)
}

func (w *opsServerWrapper) ReportMetrics(ctx context.Context, req *opsv1.MetricsReport) (*emptypb.Empty, error) {
	return w.ops.ReportMetrics(ctx, req)
}

func (w *opsServerWrapper) RestartProcess(ctx context.Context, req *opsv1.RestartProcessRequest) (*opsv1.RestartProcessResponse, error) {
	return w.ops.RestartProcess(ctx, req)
}

func (w *opsServerWrapper) StopProcess(ctx context.Context, req *opsv1.StopProcessRequest) (*opsv1.StopProcessResponse, error) {
	return w.ops.StopProcess(ctx, req)
}

func (w *opsServerWrapper) StartProcess(ctx context.Context, req *opsv1.StartProcessRequest) (*opsv1.StartProcessResponse, error) {
	return w.ops.StartProcess(ctx, req)
}

func (w *opsServerWrapper) ExecuteCommand(ctx context.Context, req *opsv1.ExecuteCommandRequest) (*opsv1.ExecuteCommandResponse, error) {
	return w.ops.ExecuteCommand(ctx, req)
}

// platformManagerWrapper wraps PlatformManager to implement nng.PlatformManager interface
type platformManagerWrapper struct {
	pm *PlatformManager
}

func (w *platformManagerWrapper) IsPlatformFunction(functionID string) bool {
	return w.pm.IsPlatformFunction(functionID)
}

func (w *platformManagerWrapper) Call(ctx context.Context, functionID string, request []byte) ([]byte, error) {
	return w.pm.Call(ctx, functionID, request)
}
