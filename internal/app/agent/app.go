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
	// Provider 握手鉴权（CR-FIX-11）：cmd 配置经 SetProviderAuth 注入，
	// startLocalServerSafely 建监听时透传 TCPLocalListenerConfig，与
	// LocalHandler.SetProviderAuth 同源。
	providerAuthRequire bool
	providerAuthToken   string
	providerManager     *ProviderManager
	providerSessions    *sidecar.ProviderSessionStore
	telemetry           *telemetry.GameTelemetryService
	// rec 是 agent 观测面（OBS-AGENT-001）：启停/连接状态/事件计数。缺省
	// noop（零开销）；cmd 入口经 SetRecorder 注入真实 OTel 装配。
	rec                otelrec.Recorder
	userOnConnected    func()
	userOnDisconnected func(error)
	configDir          string

	// TCP local server
	localHandler *agent.LocalHandler
	// localServerMu 守护 localServer 的发布与读取：StartLocalServer 在 Run
	// 调用方派生的 goroutine 内赋值，而 Stop/GetLocalServerAddr 可能在启动
	// 完成前被调用（SIGTERM 早到、测试快速收尾），无同步的读/写构成数据
	// 竞争（2026-09-22 -race 实证）。
	localServerMu sync.Mutex
	localServer   transportcore.Server
	localAddr     string

	// Ops module (optional)
	opsMu     sync.Mutex
	opsConfig *OpsConfig
	opsServer *OpsServer
	agentID   string
	version   string

	// probeManager 声明式探针装配（R2）。为 nil = 未配置探测（默认）。
	probeManager *ProbeManager

	// funcHost 函数适配插件宿主（todo.md:35）。为 nil = 未配置（默认）。
	funcHost *funcplugin.Host

	// serveLocal 是本地 listener Serve 的注入点：生产为 nil（走真实
	// tcpServer.Serve），测试注入返回非 Canceled 错误的实现以驱动
	// StartLocalServer 的错误日志分支（listener 在函数内部构造，无法
	// 从外部触发其 Accept 故障路径）。
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

// SetRecorder 装配 agent 观测面（OBS-AGENT-001）：nil=noop（默认关闭零开销
// 路径）；真实 OTel 实现由 cmd 入口注入。必须在 Run 前调用（装配链在
// armRecorderChaining 里把 recorder 与用户连接回调串成一条上游链）。
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

// armRecorderChaining 把上游连接/断开事件接到 recorder（再透传用户回调）。
// 幂等：重复调用只是重写同一个上游槽位（闭包读当前 a.rec/a.userOn*）。
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
	// 托管进程快照随 metrics 上报捎带（supervisor S1）
	a.upstream.WithSupervisorSampler(opsServer)

	// OPS-005 日志采集：配置了 logFiles 则装配 tailer，折入 metrics 上报。
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
	// provider keepalive 探针（间隔可配：CROUPIER_AGENTLOCAL_PROBE_INTERVAL，
	// 默认 5s）：pong 经 SDK 事件循环回出，卡死的 provider 在 1~2 个周期
	// 内被摘除——调用路由不再选到半死会话（连接在但处理不动的僵尸）。
	if probeInterval := parseDurationEnv("CROUPIER_AGENTLOCAL_PROBE_INTERVAL", 5*time.Second); probeInterval > 0 {
		probe := agent.NewProviderKeepalive(providerSessions, probeInterval, slog.Default())
		go probe.Run(context.Background())
	}
	tcpServer.SetOnConnect(func(session *sidecar.ProviderSession) {
		// 事件/吞吐计数面（OBS-AGENT-001）：provider 注册是 SDK 侧可用性
		// 的原子事件，按 service 维度计数。
		a.rec.Count(context.Background(), "events", 1, map[string]string{"op": "provider_register", "service": session.ServiceID})
		addr := session.Conn().RemoteAddr()
		// 平台固定键 + 会话携带的用户实例元数据（保留键已在帧解析时剥离，
		// 见 agent.MergeUserProviderMetadata）。
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
		// 覆盖边界说明：Serve 仅在 Accept 返回非超时错误且未走 Close 路径
		// 时返回错误（如进程 fd 耗尽 EMFILE），listener 由 StartLocalServer
		// 内部构造、生产无法注入该故障；正规 Stop() 路径 Serve 恒返回 nil，
		// ctx 为 Background 也排除 context.Canceled。此日志分支保留为生产
		// 异常兜底，经 serveLocal 注入点测试驱动。
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

// SetProviderAuth 配置 Provider 握手鉴权（CR-FIX-11）：require=true 且
// token 为空视为配置错误（fail-closed：所有握手被拒）；调用方应在配置
// 校验阶段拦截此组合。
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
