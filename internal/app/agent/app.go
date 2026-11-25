package agent

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/cuihairu/croupier/agents/sidecar"
	"github.com/cuihairu/croupier/agents/sidecar/funcplugin"
	corelogs "github.com/cuihairu/croupier/core/logs"
	"github.com/cuihairu/croupier/core/otelrec"
	transportcore "github.com/cuihairu/croupier/core/transport"
	"github.com/cuihairu/croupier/internal/agent"
	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	"github.com/cuihairu/croupier/internal/platform/tlsutil"
	"github.com/cuihairu/croupier/internal/telemetry"
)

// App assembles TCP-based services for Agent process.
type App struct {
	store            *agentlocal.LocalStore
	tasks            *taskIndex
	upstream         *UpstreamClient
	extensionRuntime *ExtensionRuntime
	extensionDrivers *ExtensionDriverRuntime
	extensionRoutes  map[string]extensionFunctionRoute
	extensionMu      sync.RWMutex
	extensionPuller  *ExtensionSyncPuller
	outTLS           *tlsutil.ClientTLSConfig
	localTLS         *tls.Config
	// Provider handshake auth (CR-FIX-11): cmd config is injected via
	// SetProviderAuth, and startLocalServerSafely passes it through to
	// TCPLocalListenerConfig when building the listener; same source as
	// LocalHandler.SetProviderAuth.
	providerAuthRequire bool
	providerAuthToken   string
	providerManager     *ProviderManager
	providerSessions    *sidecar.ProviderSessionStore
	telemetry           *telemetry.GameTelemetryService
	// rec is the agent observability surface (OBS-AGENT-001):
	// start/stop, connection state, event counts. Defaults to noop
	// (zero overhead); the cmd entrypoint injects the real OTel assembly
	// via SetRecorder.
	rec                otelrec.Recorder
	userOnConnected    func()
	userOnDisconnected func(error)
	configDir          string

	// TCP local server
	localHandler *agent.LocalHandler
	// localServerMu guards publication of and reads from localServer:
	// StartLocalServer assigns it inside the goroutine spawned by the Run
	// caller, while Stop/GetLocalServerAddr may run before startup finishes
	// (early SIGTERM, fast test teardown); unsynchronized reads/writes are a
	// data race (demonstrated by -race on 2026-09-22).
	localServerMu sync.Mutex
	localServer   transportcore.Server
	localAddr     string

	// Ops module (optional)
	opsMu     sync.Mutex
	opsConfig *OpsConfig
	opsServer *OpsServer
	agentID   string
	version   string

	// probeManager is the declarative probe assembly (R2). nil = no probing
	// configured (default).
	probeManager *ProbeManager

	// funcHost is the host for function adapter plugins (todo.md:35). nil =
	// not configured (default).
	funcHost *funcplugin.Host

	// serveLocal is the injection point for the local listener's Serve:
	// production leaves it nil (real tcpServer.Serve); tests inject an
	// implementation returning a non-Canceled error to drive StartLocalServer's
	// error-logging branch (the listener is built inside the function, so its
	// Accept failure path cannot be triggered externally).
	serveLocal func(server *agent.TCPLocalListener) error
	serveErrMu sync.Mutex
	serveErr   chan error
}

func (a *App) WithTelemetry(service *telemetry.GameTelemetryService) {
	if a == nil {
		return
	}
	a.telemetry = service
}

// TelemetryService returns the wired telemetry service (nil when the OTel
// master switch is off and nothing was wired, todo.md:31).
func (a *App) TelemetryService() *telemetry.GameTelemetryService {
	if a == nil {
		return nil
	}
	return a.telemetry
}

// SetRecorder wires the agent observability surface (OBS-AGENT-001): nil =
// noop (the default off, zero-overhead path); the real OTel implementation is
// injected by the cmd entrypoint. It must be called before Run (the wiring
// chains the recorder and the user connection callbacks into one upstream
// chain inside armRecorderChaining).
func (a *App) SetRecorder(rec otelrec.Recorder) {
	if a == nil {
		return
	}
	if rec == nil {
		rec = otelrec.Discard()
	}
	a.rec = rec
	a.armRecorderChaining()
}

// armRecorderChaining wires upstream connect/disconnect events into the
// recorder (then forwards to the user callbacks). Idempotent: repeated calls
// just overwrite the same upstream slot (the closures read the current
// a.rec/a.userOn*).
func (a *App) armRecorderChaining() {
	if a == nil || a.upstream == nil {
		return
	}
	a.upstream.OnConnected(func() {
		a.rec.Connected(context.Background())
		if a.userOnConnected != nil {
			a.userOnConnected()
		}
	})
	a.upstream.OnDisconnected(func(err error) {
		a.rec.Disconnected(context.Background(), err)
		if a.userOnDisconnected != nil {
			a.userOnDisconnected(err)
		}
	})
}

func New(serverAddr, agentID string) *App {
	store := agentlocal.NewLocalStore()
	app := &App{
		store:            store,
		tasks:            newTaskIndex(),
		upstream:         NewUpstreamClient(serverAddr, agentID, store, nil),
		extensionRuntime: NewExtensionRuntime(),
		extensionDrivers: NewExtensionDriverRuntime(),
		extensionRoutes:  map[string]extensionFunctionRoute{},
		configDir:        getConfigDir(),
		agentID:          agentID,
		localAddr:        ":19091",
		rec:              otelrec.Discard(),
	}
	app.upstream.SetDynamicLabelsProvider(app.extensionRuntimeDynamicLabels)
	return app
}

func NewWithConfigDir(serverAddr, agentID, configDir string) *App {
	store := agentlocal.NewLocalStore()
	app := &App{
		store:            store,
		tasks:            newTaskIndex(),
		upstream:         NewUpstreamClient(serverAddr, agentID, store, nil),
		extensionRuntime: NewExtensionRuntime(),
		extensionDrivers: NewExtensionDriverRuntime(),
		extensionRoutes:  map[string]extensionFunctionRoute{},
		configDir:        configDir,
		agentID:          agentID,
		localAddr:        ":19091",
		rec:              otelrec.Discard(),
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
	a.localHandler.SetTaskEventReporter(a.upstream)
	a.upstream.SetLocalHandler(a.localHandler)

	// Initialize and set ops server wrapper
	if a.opsConfig == nil {
		a.opsConfig = DefaultOpsConfig()
	}
	a.opsMu.Lock()
	a.opsServer = NewOpsServer(a.opsConfig, a.agentID, a.version, nil)
	opsServer := a.opsServer
	a.opsMu.Unlock()
	opsWrapper := &opsServerWrapper{ops: opsServer}
	a.localHandler.SetOpsServer(opsWrapper)
	// Managed-process snapshots piggyback on the metrics report (supervisor S1)
	a.upstream.WithSupervisorSampler(opsServer)

	// OPS-005 log collection: when logFiles are configured, assemble the tailer
	// and fold it into the metrics report.
	if a.opsConfig != nil && len(a.opsConfig.LogFiles) > 0 {
		tailer := corelogs.NewTailer(
			a.opsConfig.LogFiles,
			a.opsConfig.LogMaxLineBytes,
			a.opsConfig.LogMaxLinesPerCycle,
		)
		a.upstream.WithLogTailer(tailer)
	}

	// Keep each SDK Provider's established TCP session so invocations can be
	// forwarded over that session instead of dialing a provider-supplied address.
	providerSessions := sidecar.NewProviderSessionStore()
	a.localHandler.SetProviderSessionStore(providerSessions)
	a.providerSessions = providerSessions
	tcpServer, err := agent.NewTCPLocalListener(&agent.TCPLocalListenerConfig{
		Address: a.localAddr,
		// Go SDK providers heartbeat every 60 seconds by default. The listener
		// must leave enough idle time for that heartbeat and normal scheduling
		// jitter, otherwise healthy providers are removed before their first
		// heartbeat and Server reports no live agent for their functions.
		RecvTimeout: 2 * time.Minute,
		SendTimeout: 30 * time.Second,
		TLSConfig:   a.localTLS,

		RequireProviderAuth: a.providerAuthRequire,
		ProviderAuthToken:   a.providerAuthToken,
	}, providerSessions, slog.Default())
	if err != nil {
		return fmt.Errorf("failed to create TCP local server: %w", err)
	}
	tcpServer.SetLocalHandler(a.localHandler)
	// Provider keepalive probe (interval configurable via
	// CROUPIER_AGENTLOCAL_PROBE_INTERVAL, default 5s): the pong is returned
	// through the SDK event loop, so a stuck provider is removed within 1-2
	// cycles — invocation routing no longer picks half-dead sessions
	// (zombies whose connection is alive but whose processing is stalled).
	if probeInterval := parseDurationEnv("CROUPIER_AGENTLOCAL_PROBE_INTERVAL", 5*time.Second); probeInterval > 0 {
		probe := agent.NewProviderKeepalive(providerSessions, probeInterval, slog.Default())
		go probe.Run(context.Background())
	}
	tcpServer.SetOnConnect(func(session *sidecar.ProviderSession) {
		// Event/throughput counting surface (OBS-AGENT-001): provider
		// registration is the atomic event of SDK-side availability, counted
		// per service.
		a.rec.Count(context.Background(), "events", 1, map[string]string{"op": "provider_register", "service": session.ServiceID})
		addr := session.Conn().RemoteAddr()
		// Platform fixed keys plus the user instance metadata carried by the
		// session (reserved keys are already stripped during frame parsing,
		// see agent.MergeUserProviderMetadata).
		a.store.Register(session.SessionID, session.ServiceID, addr, session.Version, session.Functions,
			agent.MergeProviderInstanceMetadata(session, a.upstream.GameID(), a.upstream.Env()))
	})
	tcpServer.SetOnDisconnect(func(session *sidecar.ProviderSession) {
		a.rec.Count(context.Background(), "events", 1, map[string]string{"op": "provider_unregister", "service": session.ServiceID})
		a.store.RemoveProvider(session.SessionID)
	})
	a.localServerMu.Lock()
	a.localServer = tcpServer
	a.localServerMu.Unlock()
	serve := a.serveLocal
	if serve == nil {
		// Coverage boundary: Serve only returns an error when Accept yields a
		// non-timeout error without taking the Close path (e.g. process fd
		// exhaustion, EMFILE); the listener is built inside StartLocalServer,
		// so that failure cannot be injected in production. The normal Stop()
		// path always returns nil from Serve, and a Background ctx also rules
		// out context.Canceled. This log branch is kept as a production
		// exception fallback, driven in tests through the serveLocal injection
		// point.
		serve = func(server *agent.TCPLocalListener) error {
			return server.Serve(context.Background())
		}
	}
	a.serveErrMu.Lock()
	if a.serveErr == nil {
		a.serveErr = make(chan error, 1)
	}
	serveErr := a.serveErr
	a.serveErrMu.Unlock()
	go func() {
		if err := serve(tcpServer); err != nil && !errors.Is(err, context.Canceled) {
			serveErr <- err
			return
		}
		a.localServerMu.Lock()
		if a.localServer != nil {
			a.localServer = nil
		}
		a.localServerMu.Unlock()
	}()

	return nil
}

// GetLocalServerAddr returns the local TCP server address.
func (a *App) GetLocalServerAddr() string {
	if a == nil {
		return ""
	}
	a.localServerMu.Lock()
	ls := a.localServer
	a.localServerMu.Unlock()
	if ls != nil {
		return ls.Addr()
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

// SetProviderAuth configures Provider handshake auth (CR-FIX-11): require=true
// with an empty token is treated as a configuration error (fail-closed: all
// handshakes are rejected); callers should block that combination at config
// validation time.
func (a *App) SetProviderAuth(require bool, token string) {
	if a == nil {
		return
	}
	a.providerAuthRequire = require
	a.providerAuthToken = token
	if a.localHandler != nil {
		a.localHandler.SetProviderAuth(require, token)
	}
}

// Run starts the agent's background processes (upstream sync and local server).
func (a *App) Run(ctx context.Context) error {
	if err := a.startLocalServerSafely(); err != nil {
		a.Stop()
		return err
	}

	if err := a.startAgentServices(ctx); err != nil {
		a.Stop()
		return err
	}

	select {
	case err := <-a.currentServeErr():
		a.Stop()
		return fmt.Errorf("agent local server failed: %w", err)
	default:
	}
	return nil
}
