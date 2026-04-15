package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cuihairu/croupier/internal/agent"
	"github.com/cuihairu/croupier/internal/core/extension/externalfunc"
	extensionsync "github.com/cuihairu/croupier/internal/core/extension/sync"
	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	"github.com/cuihairu/croupier/internal/platform/tlsutil"
	transportcore "github.com/cuihairu/croupier/internal/transport"
	tcptr "github.com/cuihairu/croupier/internal/transport/tcp"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"google.golang.org/protobuf/types/known/emptypb"
)

// App assembles TCP-based services for Agent process.
type App struct {
	store            *agentlocal.LocalStore
	jobs             *jobIndex
	upstream         *UpstreamClient
	extensionRuntime *ExtensionRuntime
	extensionDrivers *ExtensionDriverRuntime
	extensionRoutes  map[string]extensionFunctionRoute
	extensionMu      sync.RWMutex
	extensionPuller  *ExtensionSyncPuller
	outTLS           *tlsutil.ClientTLSConfig
	providerManager  *ProviderManager
	configDir        string

	// TCP local server
	localHandler *agent.LocalHandler
	localServer  transportcore.Server
	localAddr    string

	// Ops module (optional)
	opsConfig *OpsConfig
	opsServer *OpsServer
	agentID   string
	version   string
}

func New(serverAddr, agentID string) *App {
	store := agentlocal.NewLocalStore()
	app := &App{
		store:            store,
		jobs:             newJobIndex(),
		upstream:         NewUpstreamClient(serverAddr, agentID, store, nil),
		extensionRuntime: NewExtensionRuntime(),
		extensionDrivers: NewExtensionDriverRuntime(),
		extensionRoutes:  map[string]extensionFunctionRoute{},
		configDir:        getConfigDir(),
		agentID:          agentID,
		localAddr:        ":19091",
	}
	app.upstream.SetDynamicLabelsProvider(app.extensionRuntimeDynamicLabels)
	return app
}

func NewWithConfigDir(serverAddr, agentID, configDir string) *App {
	store := agentlocal.NewLocalStore()
	app := &App{
		store:            store,
		jobs:             newJobIndex(),
		upstream:         NewUpstreamClient(serverAddr, agentID, store, nil),
		extensionRuntime: NewExtensionRuntime(),
		extensionDrivers: NewExtensionDriverRuntime(),
		extensionRoutes:  map[string]extensionFunctionRoute{},
		configDir:        configDir,
		agentID:          agentID,
		localAddr:        ":19091",
	}
	app.upstream.SetDynamicLabelsProvider(app.extensionRuntimeDynamicLabels)
	return app
}

func getConfigDir() string {
	// Check CROUPIER_CONFIG_DIR first
	if dir := os.Getenv("CROUPIER_CONFIG_DIR"); dir != "" {
		return dir
	}
	// Default to ./configs relative to working directory
	return "./configs"
}

// StartLocalServer starts the TCP local server for SDK connections.
func (a *App) StartLocalServer() error {
	// Initialize provider manager
	a.providerManager = NewProviderManager(a.store, a.configDir, nil)
	if a.extensionDrivers != nil {
		a.extensionDrivers.SetOpenAPICaller(func(ctx context.Context, provider, method string, request []byte) ([]byte, error) {
			if a.providerManager == nil {
				return nil, errors.New("provider manager not initialized")
			}
			return a.providerManager.Call(ctx, provider+"."+method, request)
		})
	}

	// Create local handler (business logic, TCP transport)
	a.localHandler = agent.NewLocalHandler(a.store, a.configDir, a.agentID, slog.Default())

	// Set provider manager - wrap it to implement the interface
	pmWrapper := &providerManagerWrapper{pm: a.providerManager, app: a}
	a.localHandler.SetProviderManager(pmWrapper)

	// Set TLS config for outbound connections
	if a.outTLS != nil {
		a.localHandler.SetTLSConfig(a.outTLS)
	}

	// Initialize and set ops server wrapper
	if a.opsConfig == nil {
		a.opsConfig = DefaultOpsConfig()
	}
	a.opsServer = NewOpsServer(a.opsConfig, a.agentID, a.version, nil)
	opsWrapper := &opsServerWrapper{ops: a.opsServer}
	a.localHandler.SetOpsServer(opsWrapper)

	// Start TCP server with local handler
	tcpServer, err := tcptr.NewServer(&tcptr.Config{
		Address:     a.localAddr,
		Insecure:    true,
		RecvTimeout: time.Second,
		SendTimeout: 10 * time.Second,
	}, a.localHandler)
	if err != nil {
		return fmt.Errorf("failed to create TCP local server: %w", err)
	}
	a.localServer = tcpServer
	go func() {
		if err := tcpServer.Serve(context.Background()); err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("agent tcp local server stopped", "error", err)
		}
	}()

	return nil
}

// GetLocalServerAddr returns the local TCP server address.
func (a *App) GetLocalServerAddr() string {
	if a == nil {
		return ""
	}
	if a.localServer != nil {
		return a.localServer.Addr()
	}
	return ""
}

// SetLocalAddr sets the local server address.
func (a *App) SetLocalAddr(addr string) {
	if a == nil {
		return
	}
	a.localAddr = addr
}

// Run starts the agent's background processes (upstream sync and local server).
func (a *App) Run(ctx context.Context) error {
	// Start local TCP server for SDK connections
	if err := a.StartLocalServer(); err != nil {
		return fmt.Errorf("failed to start local server: %w", err)
	}

	// Load providers before starting upstream
	if a.providerManager != nil {
		if err := a.providerManager.Load(ctx); err != nil {
			return err
		}
	}

	a.startMaintenance(ctx)
	a.startExtensionSync(ctx)
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

func (a *App) startExtensionSync(ctx context.Context) {
	if a == nil || a.extensionRuntime == nil {
		return
	}
	baseURL := strings.TrimSpace(os.Getenv("CROUPIER_EXTENSION_SYNC_API"))
	if baseURL == "" {
		return
	}
	interval := parseDurationEnv("CROUPIER_EXTENSION_SYNC_INTERVAL", 30*time.Second)
	a.extensionPuller = NewExtensionSyncPuller(baseURL, a.agentID, interval, a.extensionRuntime)
	a.extensionPuller.Start(ctx)
}

// Stop shuts down background upstream connection and local TCP server.
func (a *App) Stop() {
	if a == nil {
		return
	}
	// Stop local TCP server
	if a.localServer != nil {
		_ = a.localServer.Close()
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

// SetUpstreamTransportKind sets the control-plane transport kind for server connections.
func (a *App) SetUpstreamTransportKind(kind string) {
	if a == nil || a.upstream == nil {
		return
	}
	a.upstream.SetTransportKind(kind)
}

// OnConnected sets a callback to be invoked when the agent successfully connects to the server.
// The callback is called after successful registration.
func (a *App) OnConnected(callback func()) {
	if a != nil && a.upstream != nil {
		a.upstream.OnConnected(callback)
	}
}

// OnDisconnected sets a callback to be invoked when the agent disconnects from the server.
// The callback is called with the error that caused the disconnection.
func (a *App) OnDisconnected(callback func(error)) {
	if a != nil && a.upstream != nil {
		a.upstream.OnDisconnected(callback)
	}
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

func (a *App) ExtensionRuntime() *ExtensionRuntime {
	if a == nil {
		return nil
	}
	return a.extensionRuntime
}

func (a *App) ApplyExtensionSyncPayload(payload *extensionsync.AgentSyncPayload) (*ExtensionRuntimeApplyResult, error) {
	if a == nil || a.extensionRuntime == nil {
		return nil, errors.New("extension runtime not initialized")
	}
	result, err := a.extensionRuntime.ApplyPayload(payload)
	if err != nil {
		a.extensionRuntime.RecordError(err)
		return nil, err
	}
	if err := a.syncExtensionsFromRuntime(context.Background()); err != nil {
		a.extensionRuntime.RecordError(err)
		return nil, err
	}
	return result, nil
}

func (a *App) ApplyExtensionSyncPayloadJSON(raw []byte) (*ExtensionRuntimeApplyResult, error) {
	if a == nil || a.extensionRuntime == nil {
		return nil, errors.New("extension runtime not initialized")
	}
	var payload extensionsync.AgentSyncPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		a.extensionRuntime.RecordError(err)
		return nil, err
	}
	result, err := a.extensionRuntime.ApplyPayload(&payload)
	if err != nil {
		a.extensionRuntime.RecordError(err)
		return nil, err
	}
	if err := a.syncExtensionsFromRuntime(context.Background()); err != nil {
		a.extensionRuntime.RecordError(err)
		return nil, err
	}
	return result, nil
}

func (a *App) ReconcileExtensions() (*ExtensionRuntimeApplyResult, error) {
	if a == nil || a.extensionRuntime == nil {
		return nil, errors.New("extension runtime not initialized")
	}
	result, err := a.extensionRuntime.Reload()
	if err != nil {
		a.extensionRuntime.RecordError(err)
		return nil, err
	}
	if err := a.syncExtensionsFromRuntime(context.Background()); err != nil {
		a.extensionRuntime.RecordError(err)
		return nil, err
	}
	return result, nil
}

func (a *App) PullExtensionSyncOnce(ctx context.Context) error {
	if a == nil || a.extensionPuller == nil {
		return errors.New("extension sync puller not initialized")
	}
	if err := a.extensionPuller.PullOnce(ctx); err != nil {
		a.extensionRuntime.RecordError(err)
		return err
	}
	if err := a.syncExtensionsFromRuntime(ctx); err != nil {
		a.extensionRuntime.RecordError(err)
		return err
	}
	return nil
}

func (a *App) syncExtensionsFromRuntime(ctx context.Context) error {
	if a == nil || a.extensionRuntime == nil {
		return nil
	}
	snap := a.extensionRuntime.Snapshot()
	if a.providerManager != nil {
		entries := buildExtensionProviderEntries(snap)
		if err := a.providerManager.SyncExtensionProviders(ctx, entries); err != nil {
			return err
		}
	}
	if a.extensionDrivers != nil {
		if _, err := a.extensionDrivers.Sync(ctx, snap); err != nil {
			return err
		}
	}
	a.syncExtensionFunctionsFromRuntime()
	return nil
}

func (a *App) syncExtensionFunctionsFromRuntime() {
	if a == nil || a.store == nil || a.extensionRuntime == nil {
		return
	}
	snap := a.extensionRuntime.Snapshot()
	routes := map[string]extensionFunctionRoute{}
	for _, item := range snap.Installations {
		providerID := "extension:" + strconv.FormatUint(uint64(item.InstallationID), 10)
		funcs := discoverExtensionFunctions(item)
		a.store.Register(providerID, "", item.ReleaseVersion, funcs)
		for _, fn := range funcs {
			if fn == nil || strings.TrimSpace(fn.GetId()) == "" {
				continue
			}
			routes[strings.TrimSpace(fn.GetId())] = extensionFunctionRoute{
				InstallationID: item.InstallationID,
				Driver:         resolveFunctionDriver(item, fn.GetId()),
			}
		}
	}
	a.extensionMu.Lock()
	a.extensionRoutes = routes
	a.extensionMu.Unlock()
}

func discoverExtensionFunctions(item RuntimeInstallation) []*sdkv1.LocalFunctionDescriptor {
	out := make([]*sdkv1.LocalFunctionDescriptor, 0)
	seen := map[string]bool{}
	pushWithEntity := func(id, operation, category string, tags []string, entity string) {
		fid := strings.TrimSpace(id)
		if fid == "" || seen[fid] {
			return
		}
		seen[fid] = true
		out = append(out, &sdkv1.LocalFunctionDescriptor{
			Id:          fid,
			Version:     item.ReleaseVersion,
			Category:    firstNonEmpty(category, "extension"),
			Risk:        "unknown",
			Entity:      firstNonEmpty(entity, item.ExtensionID),
			Operation:   firstNonEmpty(operation, "custom"),
			Tags:        append([]string{"extension", item.ExtensionID}, tags...),
			Summary:     "Extension function",
			Description: "Discovered from extension runtime binding",
		})
	}
	push := func(id, operation, category string, tags []string) {
		pushWithEntity(id, operation, category, tags, "")
	}
	for _, f := range discoverExternalPlatformFunctions(item) {
		pushWithEntity(
			f.FunctionID,
			f.Operation,
			"external-platform",
			[]string{"external-platform", f.Provider, "capability:" + f.Capability},
			f.Provider,
		)
	}

	for _, b := range item.Bindings {
		bt := strings.ToLower(strings.TrimSpace(b.BindingType))
		key := strings.TrimSpace(b.BindingKey)
		switch bt {
		case "function":
			op := valueString(b.Spec, "operation")
			cat := valueString(b.Spec, "category")
			push(firstNonEmpty(key, valueString(b.Spec, "function_id"), valueString(b.Spec, "id")), op, cat, nil)
		case "capability":
			capID := firstNonEmpty(key, valueString(b.Spec, "capability"), valueString(b.Spec, "id"))
			ops := valueStringSlice(b.Spec, "operations")
			if len(ops) == 0 {
				push(capID, "custom", "capability", []string{"capability"})
				continue
			}
			for _, op := range ops {
				push(capID+"."+sanitizeNodeKey(op), op, "capability", []string{"capability"})
			}
		case "operation":
			capID := firstNonEmpty(valueString(b.Spec, "capability"), key)
			op := firstNonEmpty(valueString(b.Spec, "operation"), valueString(b.Spec, "name"), "custom")
			if capID != "" {
				push(capID+"."+sanitizeNodeKey(op), op, "capability", []string{"capability"})
			}
		}
	}
	return out
}

type externalPlatformFunction struct {
	Provider   string
	Capability string
	Operation  string
	FunctionID string
}

func discoverExternalPlatformFunctions(item RuntimeInstallation) []externalPlatformFunction {
	if !externalfunc.IsExternalPlatformExtensionID(item.ExtensionID) {
		return nil
	}
	out := make([]externalPlatformFunction, 0)
	seen := map[string]struct{}{}
	bindings := make([]externalfunc.Binding, 0, len(item.Bindings))
	for _, b := range item.Bindings {
		bindings = append(bindings, externalfunc.Binding{
			BindingType: b.BindingType,
			BindingKey:  b.BindingKey,
			Spec:        b.Spec,
		})
	}
	for provider, operations := range externalfunc.DiscoverProviderOperations(bindings) {
		for _, opKey := range operations {
			fid := externalfunc.BuildFunctionID(provider, opKey)
			if _, exists := seen[fid]; exists {
				continue
			}
			seen[fid] = struct{}{}
			out = append(out, externalPlatformFunction{
				Provider:   provider,
				Capability: externalfunc.Capability(provider),
				Operation:  opKey,
				FunctionID: fid,
			})
		}
	}
	return out
}

func buildExtensionProviderEntries(snapshot ExtensionRuntimeSnapshot) map[string]ProviderEntry {
	out := map[string]ProviderEntry{}
	for _, item := range snapshot.Installations {
		if !externalfunc.IsExternalPlatformExtensionID(item.ExtensionID) {
			continue
		}
		for _, b := range item.Bindings {
			bt := strings.ToLower(strings.TrimSpace(b.BindingType))
			if bt != "provider" && bt != "openapi" {
				continue
			}
			parsed, ok := externalfunc.ParseProviderBinding(b.BindingKey, b.Spec)
			if !ok {
				continue
			}
			name := sanitizeNodeKey(parsed.Provider)
			if name == "" {
				continue
			}
			cfg := map[string]interface{}{}
			for k, v := range parsed.Config {
				cfg[k] = v
			}
			if len(parsed.Operations) > 0 {
				if _, exists := cfg["methods"]; !exists {
					methods := make([]map[string]any, 0, len(parsed.Operations))
					for _, opKey := range parsed.Operations {
						methods = append(methods, map[string]any{
							"name":   opKey,
							"path":   "/" + opKey,
							"method": "POST",
						})
					}
					if len(methods) > 0 {
						cfg["methods"] = methods
					}
				}
			}
			out[name] = ProviderEntry{
				Enabled: parsed.Enabled,
				Type:    parsed.Type,
				Config:  cfg,
			}
		}
	}
	return out
}

func valueString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func valueStringSlice(m map[string]any, key string) []string {
	if m == nil {
		return nil
	}
	raw, ok := m[key]
	if !ok || raw == nil {
		return nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		s := strings.TrimSpace(fmt.Sprint(item))
		if s == "" || s == "<nil>" {
			continue
		}
		out = append(out, s)
	}
	return out
}

func (a *App) extensionRuntimeDynamicLabels() map[string]string {
	if a == nil || a.extensionRuntime == nil {
		return nil
	}
	snap := a.extensionRuntime.Snapshot()
	enabled := 0
	bindings := 0
	for _, item := range snap.Installations {
		if item.Enabled {
			enabled++
		}
		bindings += len(item.Bindings)
	}
	labels := map[string]string{
		"extensions.runtime.installations": strconv.Itoa(len(snap.Installations)),
		"extensions.runtime.enabled":       strconv.Itoa(enabled),
		"extensions.runtime.bindings":      strconv.Itoa(bindings),
		"extensions.runtime.status":        firstNonEmpty(snap.LastApplyStatus, "unknown"),
		"extensions.runtime.applied_at":    strconv.FormatInt(snap.AppliedAt, 10),
		"extensions.runtime.last_error_at": strconv.FormatInt(snap.LastErrorAt, 10),
		"extensions.runtime.last_failed":   strconv.Itoa(snap.LastFailed),
	}
	if snap.LastError != "" {
		labels["extensions.runtime.last_error"] = truncateString(snap.LastError, 160)
	}
	if a.extensionDrivers != nil {
		stats := a.extensionDrivers.LastResult()
		labels["extensions.runtime.driver_init"] = strconv.Itoa(stats.Initialized)
		labels["extensions.runtime.driver_reload"] = strconv.Itoa(stats.Reloaded)
		labels["extensions.runtime.driver_stop"] = strconv.Itoa(stats.Stopped)
		labels["extensions.runtime.driver_failed"] = strconv.Itoa(stats.Failed)
	}
	return labels
}

func truncateString(s string, max int) string {
	if max <= 0 {
		return ""
	}
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

type extensionFunctionRoute struct {
	InstallationID uint
	Driver         string
}

func resolveFunctionDriver(item RuntimeInstallation, functionID string) string {
	fid := strings.TrimSpace(functionID)
	if _, _, ok := externalfunc.ParseFunctionID(fid); ok {
		return "openapi-driver"
	}
	for _, b := range item.Bindings {
		key := strings.TrimSpace(b.BindingKey)
		if key != "" && key != fid {
			continue
		}
		if d := valueString(b.Spec, "driver"); d != "" {
			return d
		}
		names := resolveDriverNames(RuntimeInstallation{Bindings: []RuntimeBinding{b}})
		if len(names) > 0 {
			return names[0]
		}
	}
	return "workflow-driver"
}

func (a *App) hasExtensionFunction(functionID string) bool {
	if a == nil {
		return false
	}
	key := strings.TrimSpace(functionID)
	if key == "" {
		return false
	}
	a.extensionMu.RLock()
	defer a.extensionMu.RUnlock()
	_, ok := a.extensionRoutes[key]
	return ok
}

func (a *App) invokeExtensionFunction(ctx context.Context, functionID string, payload []byte) ([]byte, error) {
	if a == nil || a.extensionDrivers == nil {
		return nil, errors.New("extension drivers not initialized")
	}
	key := strings.TrimSpace(functionID)
	if key == "" {
		return nil, errors.New("function id is required")
	}
	a.extensionMu.RLock()
	route, ok := a.extensionRoutes[key]
	a.extensionMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("extension function not found: %s", key)
	}
	driver := firstNonEmpty(route.Driver, "workflow-driver")
	if driver != "openapi-driver" {
		if out, handled, err := invokeExternalPlatformFunction(ctx, key, payload, func(ctx context.Context, provider, method string, request []byte) ([]byte, error) {
			if a.providerManager == nil {
				return nil, errors.New("provider manager not initialized")
			}
			return a.providerManager.Call(ctx, provider+"."+method, request)
		}); handled {
			return out, err
		}
	}
	return a.extensionDrivers.Invoke(ctx, driver, key, payload)
}

// opsServerWrapper wraps OpsServer to implement agent.OpsServerWrapper interface
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

func (w *opsServerWrapper) ListServicesJSON(ctx context.Context, jsonReq []byte) ([]byte, error) {
	return w.ops.ListServicesJSON(ctx, jsonReq)
}

func (w *opsServerWrapper) GetServiceStatusJSON(ctx context.Context, jsonReq []byte) ([]byte, error) {
	return w.ops.GetServiceStatusJSON(ctx, jsonReq)
}

func (w *opsServerWrapper) ListCronJobsJSON(ctx context.Context) ([]byte, error) {
	return w.ops.ListCronJobsJSON(ctx)
}

// providerManagerWrapper wraps ProviderManager to implement agent.ProviderManager interface
type providerManagerWrapper struct {
	pm  *ProviderManager
	app *App
}

func (w *providerManagerWrapper) IsPlatformFunction(functionID string) bool {
	if w != nil && w.app != nil && w.app.hasExtensionFunction(functionID) {
		return true
	}
	if w == nil || w.pm == nil {
		return false
	}
	return w.pm.IsPlatformFunction(functionID)
}

func (w *providerManagerWrapper) Call(ctx context.Context, functionID string, request []byte) ([]byte, error) {
	if w != nil && w.app != nil && w.app.hasExtensionFunction(functionID) {
		return w.app.invokeExtensionFunction(ctx, functionID, request)
	}
	if w == nil || w.pm == nil {
		return nil, errors.New("provider manager not initialized")
	}
	return w.pm.Call(ctx, functionID, request)
}
