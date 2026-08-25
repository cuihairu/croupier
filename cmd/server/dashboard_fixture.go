package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	agentcore "github.com/cuihairu/croupier/internal/app/agent"
	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/db/router"
	"github.com/cuihairu/croupier/internal/handler"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/dispatch"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/server"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// FixtureSDKFunction mirrors croupier.FunctionDescriptor from the Go SDK. The
// server module cannot import the SDK (both vendor the same generated
// protobuf files, which conflict in one process), so the fixture SDK runs as
// a genuine separate process built from sdks/go/cmd/e2eprovider.
type FixtureSDKFunction struct {
	ID                string   `json:"id"`
	Version           string   `json:"version,omitempty"`
	Tags              []string `json:"tags,omitempty"`
	Summary           string   `json:"summary,omitempty"`
	Description       string   `json:"description,omitempty"`
	OperationID       string   `json:"operationId,omitempty"`
	Deprecated        bool     `json:"deprecated,omitempty"`
	InputSchema       string   `json:"inputSchema,omitempty"`
	OutputSchema      string   `json:"outputSchema,omitempty"`
	Resource          string   `json:"resource,omitempty"`
	Operation         string   `json:"operation,omitempty"`
	Capability        string   `json:"capability,omitempty"`
	Execution         string   `json:"execution,omitempty"`
	ApprovalRequired  bool     `json:"approvalRequired,omitempty"`
	ApprovalPolicyKey string   `json:"approvalPolicyKey,omitempty"`
	Risk              string   `json:"risk,omitempty"`
	Permission        string   `json:"permission,omitempty"`
	Enabled           bool     `json:"enabled"`
}

// DashboardFixtureOptions configures the real-dashboard E2E fixture named
// "real-dashboard". The fixture boots a genuine Server, a genuine Agent
// connected over the control TCP transport, a genuine Go SDK provider and a
// deterministic /players OpenAPI provider, all scoped to a dedicated
// (gameID, env) pair.
type DashboardFixtureOptions struct {
	// BaseDir holds the sqlite database, agent config and other state. When
	// empty a temporary directory is created and owned by the fixture.
	BaseDir string
	// GameID and Env define the clean scope this fixture owns.
	GameID string
	Env    string
	// Addrs accept "host:port"; port 0 picks a free port. Empty values use
	// loopback with a free port.
	HTTPAddr       string
	ControlAddr    string
	AgentLocalAddr string
	ProviderAddr   string
	FixtureAddr    string
	// BootstrapDir seeds admins/roles; defaults to the repository configs dir.
	BootstrapDir string
	// SDKFunctions is the initial deterministic SDK function set. When nil,
	// DefaultFixtureSDKFunctions() is used.
	SDKFunctions []FixtureSDKFunction
}

// DashboardFixture is a running real-dashboard fixture instance.
type DashboardFixture struct {
	GameID string
	Env    string

	HTTPAddr       string
	ControlAddr    string
	AgentLocalAddr string
	ProviderAddr   string
	FixtureAddr    string
	BaseDir        string

	ownsBaseDir bool
	cfg         config.Config
	svcCtx      *svc.ServiceContext
	control     *controlRuntime
	httpSrv     *http.Server
	fixtureSrv  *http.Server
	agent       *agentcore.App
	provider    *playersProvider

	scopeGameCreated       bool
	scopeBindingCreated    bool
	scopeAdminID           uint
	scopeAdminPrevious     model.LastScope
	scopeAdminScopeUpdated bool

	mu       sync.Mutex
	sdkCmd   *exec.Cmd
	sdkFns   []FixtureSDKFunction
	sdkCalls []fixtureSDKCall

	rootCtx    context.Context
	rootCancel context.CancelFunc
	closed     bool
}

// fixtureSDKCall records one SDK function invocation reported by the
// e2eprovider process.
type fixtureSDKCall struct {
	FunctionID string          `json:"functionId"`
	Payload    json.RawMessage `json:"payload,omitempty"`
}

// DefaultFixtureSDKFunctions returns the deterministic SDK function set:
// `mail.send` carries only id + input/output schema (unannotated), which the
// platform must degrade into a standalone Operation proposal. `mail.wait` is
// an equally unannotated cancellation canary used by real L3 lifecycle tests.
func DefaultFixtureSDKFunctions() []FixtureSDKFunction {
	return []FixtureSDKFunction{
		{
			ID:           "mail.send",
			Version:      "1.0.0",
			Summary:      "Send an in-game mail",
			InputSchema:  `{"type":"object","properties":{"player_id":{"type":"string"},"title":{"type":"string"},"content":{"type":"string"}},"required":["player_id","title"]}`,
			OutputSchema: `{"type":"object","properties":{"success":{"type":"boolean"},"mail_id":{"type":"string"}}}`,
			Enabled:      true,
		},
		{
			ID:           "mail.wait",
			Version:      "1.0.0",
			Summary:      "Wait until completion or cancellation",
			InputSchema:  `{"type":"object","properties":{"wait_ms":{"type":"integer","minimum":1}},"required":["wait_ms"]}`,
			OutputSchema: `{"type":"object","properties":{"success":{"type":"boolean"},"waited_ms":{"type":"integer"}}}`,
			Enabled:      true,
		},
	}
}

func fixtureFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func fixtureAddrWithPort(addr string) (string, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		port, err := fixtureFreePort()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("127.0.0.1:%d", port), nil
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", err
	}
	if port == "0" {
		free, err := fixtureFreePort()
		if err != nil {
			return "", err
		}
		port = fmt.Sprintf("%d", free)
	}
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port), nil
}

func defaultFixtureBootstrapDir() string {
	// cmd/server -> repository root -> configs
	wd, err := os.Getwd()
	if err != nil {
		return "configs"
	}
	candidates := []string{
		filepath.Join(wd, "configs"),
		filepath.Join(wd, "..", "..", "configs"),
	}
	for _, dir := range candidates {
		if _, err := os.Stat(filepath.Join(dir, "admins.json")); err == nil {
			abs, err := filepath.Abs(dir)
			if err == nil {
				return abs
			}
			return dir
		}
	}
	return "configs"
}

// StartDashboardFixture boots the named "real-dashboard" fixture.
func StartDashboardFixture(ctx context.Context, opts DashboardFixtureOptions) (*DashboardFixture, error) {
	f := &DashboardFixture{
		GameID: strings.TrimSpace(opts.GameID),
		Env:    strings.TrimSpace(opts.Env),
	}
	if f.GameID == "" {
		f.GameID = "e2e-game"
	}
	if f.Env == "" {
		f.Env = "e2e"
	}

	var err error
	f.BaseDir = strings.TrimSpace(opts.BaseDir)
	if f.BaseDir == "" {
		f.BaseDir, err = os.MkdirTemp("", "croupier-real-dashboard-fixture-")
		if err != nil {
			return nil, err
		}
		f.ownsBaseDir = true
	} else if err := os.MkdirAll(f.BaseDir, 0o755); err != nil {
		return nil, err
	}

	if f.HTTPAddr, err = fixtureAddrWithPort(opts.HTTPAddr); err != nil {
		return nil, err
	}
	if f.ControlAddr, err = fixtureAddrWithPort(opts.ControlAddr); err != nil {
		return nil, err
	}
	if f.AgentLocalAddr, err = fixtureAddrWithPort(opts.AgentLocalAddr); err != nil {
		return nil, err
	}
	if f.ProviderAddr, err = fixtureAddrWithPort(opts.ProviderAddr); err != nil {
		return nil, err
	}
	if f.FixtureAddr, err = fixtureAddrWithPort(opts.FixtureAddr); err != nil {
		return nil, err
	}

	rootCtx, rootCancel := context.WithCancel(context.Background())
	f.rootCtx = rootCtx
	f.rootCancel = rootCancel

	cleanupOnError := func() {
		rootCancel()
		if f.ownsBaseDir {
			_ = os.RemoveAll(f.BaseDir)
		}
	}

	if err := f.startServer(rootCtx, opts); err != nil {
		cleanupOnError()
		return nil, fmt.Errorf("start fixture server: %w", err)
	}
	if err := f.startProvider(); err != nil {
		cleanupOnError()
		return nil, fmt.Errorf("start players provider: %w", err)
	}
	if err := f.startAgent(rootCtx); err != nil {
		cleanupOnError()
		return nil, fmt.Errorf("start fixture agent: %w", err)
	}

	f.sdkFns = opts.SDKFunctions
	if f.sdkFns == nil {
		f.sdkFns = DefaultFixtureSDKFunctions()
	}
	if err := f.startSDK(rootCtx, f.sdkFns); err != nil {
		cleanupOnError()
		return nil, fmt.Errorf("start fixture sdk: %w", err)
	}
	if err := f.startFixtureAPI(); err != nil {
		cleanupOnError()
		return nil, fmt.Errorf("start fixture api: %w", err)
	}
	_ = ctx // startup waits are driven by WaitReady
	return f, nil
}

func (f *DashboardFixture) startServer(ctx context.Context, opts DashboardFixtureOptions) error {
	bootstrapDir := strings.TrimSpace(opts.BootstrapDir)
	if bootstrapDir == "" {
		bootstrapDir = defaultFixtureBootstrapDir()
	}

	host, portStr, err := net.SplitHostPort(f.HTTPAddr)
	if err != nil {
		return err
	}
	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		return err
	}

	cfg := config.Config{}
	cfg.Server.Host = host
	cfg.Server.Port = port
	cfg.Server.Mode = "test"
	cfg.Server.Timeout = 600000
	cfg.Database.Driver = "sqlite"
	cfg.Database.DataSource = filepath.Join(f.BaseDir, "croupier-server.db")
	cfg.Control.Addr = f.ControlAddr
	cfg.Auth.JWTSecret = "real-dashboard-fixture-secret"
	cfg.BootstrapData.BaseDir = bootstrapDir
	cfg.Storage.Driver = "file"
	cfg.Storage.BaseDir = filepath.Join(f.BaseDir, "uploads")
	applyRuntimeDefaults(&cfg)
	f.cfg = cfg

	svcCtx := svc.NewServiceContext(cfg)
	wireDashboardRegistrationPipeline(svcCtx)
	telemetrySvc, err := svc.NewTelemetryService(cfg, "croupier-server", slog.Default())
	if err != nil {
		return fmt.Errorf("initialize fixture telemetry: %w", err)
	}
	svcCtx.Telemetry = telemetrySvc
	f.svcCtx = svcCtx
	if err := f.ensureUIScope(ctx); err != nil {
		return err
	}

	sessionStore := server.NewAgentSessionStore()
	sessionResolver := server.NewSessionResolverAdapter(sessionStore)
	svcCtx.AgentSessionResolver = sessionResolver
	if svcCtx.Dispatcher != nil {
		svcCtx.Dispatcher.SetSessionResolver(sessionResolver)
		taskRunModel := model.NewTaskRunModel(svcCtx.DB)
		taskEventModel := model.NewTaskEventModel(svcCtx.DB)
		svcCtx.Dispatcher.SetTaskEventQuery(dispatch.NewTaskEventQueryAdapter(taskEventModel, taskRunModel))
		svcCtx.Dispatcher.SetTaskRunWriter(dispatch.NewTaskRunWriterAdapter(taskRunModel))
	}
	f.control = startControlServer(ctx, &cfg, svcCtx, sessionStore)
	go startRegistryCleanup(ctx, svcCtx)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(cors.Default())
	r.RedirectTrailingSlash = false
	r.RedirectFixedPath = false
	r.Use(svc.NewAuthMiddleware(svcCtx))
	handler.RegisterHandlers(r, svcCtx)

	ln, err := net.Listen("tcp", f.HTTPAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", f.HTTPAddr, err)
	}
	f.httpSrv = &http.Server{Handler: wrapHTTPHandler(svcCtx, r)}
	go func() {
		if err := f.httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Default().Error("fixture http server stopped", "error", err)
		}
	}()
	return nil
}

// ensureUIScope makes the fixture's dedicated registration scope selectable
// by the real dashboard. Registration data alone is intentionally not enough:
// the UI only exposes scopes backed by games + game_envs metadata.
func (f *DashboardFixture) ensureUIScope(ctx context.Context) error {
	if f.svcCtx == nil || f.svcCtx.GameModel == nil || f.svcCtx.AdminModel == nil {
		return fmt.Errorf("fixture scope models are unavailable")
	}

	game, err := f.svcCtx.GameModel.FindByGameIDString(ctx, f.GameID)
	if err != nil {
		game = &model.Game{
			GameID:      f.GameID,
			Name:        f.GameID,
			AliasName:   "Real Dashboard E2E",
			Description: "Isolated game scope owned by the real-dashboard fixture.",
			Enabled:     true,
			Status:      "test",
			Color:       "#1677ff",
		}
		if err := game.SetEnvs([]model.GameEnv{{
			Env:         f.Env,
			Description: "Real dashboard E2E environment",
			Color:       "#1677ff",
		}}); err != nil {
			return fmt.Errorf("encode fixture game environments: %w", err)
		}
		if err := f.svcCtx.GameModel.Create(ctx, game); err != nil {
			return fmt.Errorf("create fixture game %s: %w", f.GameID, err)
		}
		f.scopeGameCreated = true
	}

	binding, err := f.svcCtx.GameModel.FindEnvBinding(ctx, f.GameID, f.Env)
	if err != nil {
		return fmt.Errorf("find fixture environment binding: %w", err)
	}
	if binding == nil {
		databaseName := router.DefaultGameDBName(f.GameID, f.Env)
		if f.svcCtx.Router != nil {
			databaseName = f.svcCtx.Router.NameForGame(f.GameID, f.Env)
		}
		if err := f.svcCtx.GameModel.AddEnvBinding(
			ctx,
			f.GameID,
			f.Env,
			databaseName,
			"Real dashboard E2E environment",
			"#1677ff",
		); err != nil {
			return fmt.Errorf("create fixture environment binding: %w", err)
		}
		f.scopeBindingCreated = true
	}

	admin, err := f.svcCtx.AdminModel.FindByUsername(ctx, "admin")
	if err != nil {
		return fmt.Errorf("find fixture admin: %w", err)
	}
	f.scopeAdminID = admin.ID
	f.scopeAdminPrevious = model.LastScope{GameID: admin.LastGameID, Env: admin.LastEnv}
	if err := f.svcCtx.AdminModel.UpdateLastScope(ctx, admin.ID, f.GameID, f.Env); err != nil {
		return fmt.Errorf("select fixture scope for admin: %w", err)
	}
	f.scopeAdminScopeUpdated = true
	return nil
}

func (f *DashboardFixture) startProvider() error {
	f.provider = newPlayersProvider()
	ln, err := net.Listen("tcp", f.ProviderAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", f.ProviderAddr, err)
	}
	go func() {
		if err := http.Serve(ln, f.provider.handler()); err != nil && err != http.ErrServerClosed {
			slog.Default().Error("fixture provider stopped", "error", err)
		}
	}()
	return nil
}

func (f *DashboardFixture) startAgent(ctx context.Context) error {
	agentDir := filepath.Join(f.BaseDir, "agent")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		return err
	}
	providersYAML := fmt.Sprintf(`providers:
  players:
    enabled: true
    type: openapi
    game_id: %q
    env: %q
    config:
      baseUrl: "http://%s"
      openapiSpec: "http://%s/openapi.json"
      timeout: "5s"
`, f.GameID, f.Env, f.ProviderAddr, f.ProviderAddr)
	if err := os.WriteFile(filepath.Join(agentDir, "providers.yaml"), []byte(providersYAML), 0o644); err != nil {
		return err
	}

	agent := agentcore.NewWithConfigDir(f.ControlAddr, "real-dashboard-agent", agentDir)
	agent.SetLocalAddr(f.AgentLocalAddr)
	agent.SetUpstreamTransportKind("tcp")
	agent.WithUpstreamMetadata(agentcore.UpstreamMetadata{
		GameID:            f.GameID,
		Env:               f.Env,
		Version:           "fixture",
		DialTimeout:       5 * time.Second,
		RequestTimeout:    10 * time.Second,
		HeartbeatInterval: 2,
	})
	go func() {
		if err := agent.Run(ctx); err != nil && ctx.Err() == nil {
			slog.Default().Error("fixture agent stopped", "error", err)
		}
	}()
	f.agent = agent
	return nil
}

func fixtureSDKDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for _, candidate := range []string{
		filepath.Join(wd, "sdks", "go"),
		filepath.Join(wd, "..", "..", "sdks", "go"),
	} {
		if _, err := os.Stat(filepath.Join(candidate, "go.mod")); err == nil {
			abs, err := filepath.Abs(candidate)
			if err != nil {
				return "", err
			}
			return abs, nil
		}
	}
	return "", fmt.Errorf("sdks/go module not found from working directory %s", wd)
}

// startSDK spawns the genuine Go SDK provider process
// (sdks/go/cmd/e2eprovider) connected to the fixture agent.
func (f *DashboardFixture) startSDK(ctx context.Context, functions []FixtureSDKFunction) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.startSDKLocked(ctx, functions)
}

func (f *DashboardFixture) startSDKLocked(ctx context.Context, functions []FixtureSDKFunction) error {
	raw, err := json.Marshal(functions)
	if err != nil {
		return err
	}
	reportURL := "http://" + f.FixtureAddr
	args := []string{
		"-agent-addr", f.AgentLocalAddr,
		"-game-id", f.GameID,
		"-env", f.Env,
		"-functions", string(raw),
		"-report-url", reportURL,
	}
	var cmd *exec.Cmd
	if bin := strings.TrimSpace(os.Getenv("CROUPIER_E2E_PROVIDER_BIN")); bin != "" {
		cmd = exec.CommandContext(ctx, bin, args...)
	} else {
		sdkDir, err := fixtureSDKDir()
		if err != nil {
			return err
		}
		// Build once and run the binary directly: `go run` wraps the child
		// and would not forward Process.Kill to the actual provider process.
		bin := filepath.Join(f.BaseDir, "e2eprovider")
		if _, err := os.Stat(bin); os.IsNotExist(err) {
			build := exec.Command("go", "build", "-o", bin, "./cmd/e2eprovider")
			build.Dir = sdkDir
			if out, err := build.CombinedOutput(); err != nil {
				return fmt.Errorf("build e2eprovider: %w: %s", err, out)
			}
		}
		cmd = exec.CommandContext(ctx, bin, args...)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start e2eprovider: %w", err)
	}
	go func() { _ = cmd.Wait() }()
	f.sdkCmd = cmd
	return nil
}

// ReplaceSDKFunctions swaps the SDK function set at runtime by restarting the
// SDK provider process against the same agent. Used by contract-change E2E
// scenarios to re-register a published function with a changed schema or
// governance.
func (f *DashboardFixture) ReplaceSDKFunctions(ctx context.Context, functions []FixtureSDKFunction) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopSDKLocked()
	f.sdkFns = functions
	providerCtx := f.rootCtx
	if providerCtx == nil {
		providerCtx = context.WithoutCancel(ctx)
	}
	return f.startSDKLocked(providerCtx, functions)
}

func (f *DashboardFixture) stopSDKLocked() {
	if f.sdkCmd != nil && f.sdkCmd.Process != nil {
		_ = f.sdkCmd.Process.Kill()
		f.sdkCmd = nil
	}
}

// SDKFunctions returns the current fixture SDK function set.
func (f *DashboardFixture) SDKFunctions() []FixtureSDKFunction {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]FixtureSDKFunction, len(f.sdkFns))
	copy(out, f.sdkFns)
	return out
}

// SDKCalls returns the invocations reported by the SDK provider process.
func (f *DashboardFixture) SDKCalls() []fixtureSDKCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]fixtureSDKCall, len(f.sdkCalls))
	copy(out, f.sdkCalls)
	return out
}

// ServiceContext exposes the fixture server service context for assertions.
func (f *DashboardFixture) ServiceContext() *svc.ServiceContext { return f.svcCtx }

// DB exposes the fixture server database for scoped assertions.
func (f *DashboardFixture) DB() *gorm.DB {
	if f.svcCtx == nil {
		return nil
	}
	return f.svcCtx.DB
}

// WaitReady polls until the agent session and the initial SDK function
// contracts have materialized in the fixture scope.
func (f *DashboardFixture) WaitReady(ctx context.Context) error {
	deadline := time.Now().Add(60 * time.Second)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("fixture did not become ready in time")
		}
		if f.ready() {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func (f *DashboardFixture) ready() bool {
	if f.svcCtx == nil || f.svcCtx.RegistryStore == nil {
		return false
	}
	agents := f.svcCtx.RegistryStore.AgentsUnsafe()
	agent, ok := agents["real-dashboard-agent"]
	if !ok || agent == nil {
		return false
	}
	for _, desc := range f.SDKFunctions() {
		if _, ok := agent.Functions[desc.ID]; !ok {
			return false
		}
	}
	db := f.DB()
	if db == nil {
		return false
	}
	for _, desc := range f.SDKFunctions() {
		var count int64
		if err := db.Model(&model.FunctionContract{}).
			Where("game_id = ? AND env = ? AND function_id = ?", f.GameID, f.Env, desc.ID).
			Count(&count).Error; err != nil || count == 0 {
			return false
		}
	}
	return true
}

// CleanupScope removes only this fixture scope's derived rows. It never
// touches rows of other (game_id, env) scopes or shared/meta tables.
func (f *DashboardFixture) CleanupScope(ctx context.Context) error {
	db := f.DB()
	if db == nil {
		return nil
	}
	scope := func(q *gorm.DB) *gorm.DB {
		return q.Where("game_id = ? AND env = ?", f.GameID, f.Env)
	}
	// Version-history tables are keyed by parent id, not scope columns, so
	// delete them before their parents.
	if err := db.WithContext(ctx).
		Where("semantics_id IN (?)", scope(db.Model(&model.CapabilitySemantics{})).Select("id")).
		Delete(&model.CapabilitySemanticVersion{}).Error; err != nil {
		return fmt.Errorf("cleanup capability semantic versions: %w", err)
	}
	if err := db.WithContext(ctx).
		Where("proposal_id IN (?)", scope(db.Model(&model.PageProposal{})).Select("id")).
		Delete(&model.PageProposalVersion{}).Error; err != nil {
		return fmt.Errorf("cleanup page proposal versions: %w", err)
	}
	scoped := []interface{}{
		&model.FunctionContract{},
		&model.ResourceCapability{},
		&model.CapabilitySemantics{},
		&model.PageProposal{},
		&model.BlockedProposalIssue{},
		&model.PageSpec{},
		&model.PublishedPageSpec{},
		&model.PageVersion{},
		&model.OpenAPISource{},
		&model.OpenAPISourceBinding{},
		&reg.AgentSessionDB{},
		&reg.AgentRegistrationOperationDB{},
	}
	for _, m := range scoped {
		if err := scope(db.WithContext(ctx).Unscoped()).Delete(m).Error; err != nil {
			return fmt.Errorf("cleanup %T: %w", m, err)
		}
	}
	if f.scopeAdminScopeUpdated && f.svcCtx.AdminModel != nil {
		if err := f.svcCtx.AdminModel.UpdateLastScope(
			ctx,
			f.scopeAdminID,
			f.scopeAdminPrevious.GameID,
			f.scopeAdminPrevious.Env,
		); err != nil {
			return fmt.Errorf("restore fixture admin scope: %w", err)
		}
		f.scopeAdminScopeUpdated = false
	}
	if f.scopeBindingCreated && f.svcCtx.GameModel != nil {
		if err := f.svcCtx.GameModel.RemoveEnvBinding(ctx, f.GameID, f.Env); err != nil {
			return fmt.Errorf("cleanup fixture environment binding: %w", err)
		}
		f.scopeBindingCreated = false
	}
	if f.scopeGameCreated {
		if err := db.WithContext(ctx).Unscoped().Where("game_id = ?", f.GameID).Delete(&model.Game{}).Error; err != nil {
			return fmt.Errorf("cleanup fixture game: %w", err)
		}
		f.scopeGameCreated = false
	}
	return nil
}

// Close stops SDK, agent, servers and removes fixture-owned state.
func (f *DashboardFixture) Close(ctx context.Context) error {
	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return nil
	}
	f.closed = true
	f.stopSDKLocked()
	f.mu.Unlock()
	if f.agent != nil {
		f.agent.Stop()
	}
	if f.control != nil {
		if f.control.tcpListener != nil {
			_ = f.control.tcpListener.Close()
		}
		if f.control.controlService != nil {
			f.control.controlService.Stop()
		}
	}
	if f.httpSrv != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = f.httpSrv.Shutdown(shutdownCtx)
		cancel()
	}
	if f.fixtureSrv != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = f.fixtureSrv.Shutdown(shutdownCtx)
		cancel()
	}
	if f.rootCancel != nil {
		f.rootCancel()
	}
	if f.svcCtx != nil && f.svcCtx.Telemetry != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = f.svcCtx.Telemetry.Shutdown(shutdownCtx)
		cancel()
	}
	if f.svcCtx != nil && f.svcCtx.Router != nil {
		_ = f.svcCtx.Router.Close()
	}
	if f.ownsBaseDir {
		_ = os.RemoveAll(f.BaseDir)
	}
	return nil
}

// --- fixture control API ---

type fixtureHealthResponse struct {
	Status    string   `json:"status"`
	GameID    string   `json:"gameId"`
	Env       string   `json:"env"`
	Server    string   `json:"server"`
	Provider  string   `json:"provider"`
	Agent     bool     `json:"agentConnected"`
	Functions []string `json:"functions"`
}

func (f *DashboardFixture) startFixtureAPI() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/__fixture__/health", func(w http.ResponseWriter, r *http.Request) {
		resp := fixtureHealthResponse{
			Status:   "ok",
			GameID:   f.GameID,
			Env:      f.Env,
			Server:   "http://" + f.HTTPAddr,
			Provider: "http://" + f.ProviderAddr,
		}
		if f.svcCtx != nil && f.svcCtx.RegistryStore != nil {
			if agent, ok := f.svcCtx.RegistryStore.AgentsUnsafe()["real-dashboard-agent"]; ok && agent != nil {
				resp.Agent = true
				for id := range agent.Functions {
					resp.Functions = append(resp.Functions, id)
				}
			}
		}
		writeFixtureJSON(w, http.StatusOK, resp)
	})
	mux.HandleFunc("/__fixture__/sdk/functions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeFixtureJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
			return
		}
		var req struct {
			Functions []FixtureSDKFunction `json:"functions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeFixtureJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json", "message": err.Error()})
			return
		}
		if err := f.ReplaceSDKFunctions(r.Context(), req.Functions); err != nil {
			writeFixtureJSON(w, http.StatusInternalServerError, map[string]string{"error": "sdk_replace_failed", "message": err.Error()})
			return
		}
		writeFixtureJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "count": len(req.Functions)})
	})
	mux.HandleFunc("/__fixture__/sdk/calls", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeFixtureJSON(w, http.StatusOK, map[string]interface{}{"calls": f.SDKCalls()})
		case http.MethodPost:
			var call fixtureSDKCall
			if err := json.NewDecoder(r.Body).Decode(&call); err != nil {
				writeFixtureJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json", "message": err.Error()})
				return
			}
			f.mu.Lock()
			f.sdkCalls = append(f.sdkCalls, call)
			f.mu.Unlock()
			writeFixtureJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		case http.MethodDelete:
			f.mu.Lock()
			f.sdkCalls = nil
			f.mu.Unlock()
			writeFixtureJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		default:
			writeFixtureJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		}
	})
	mux.HandleFunc("/__fixture__/audit/page-execute", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeFixtureJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
			return
		}
		if f.DB() == nil {
			writeFixtureJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database_unavailable"})
			return
		}
		var stored audit.AuditModel
		if err := f.DB().WithContext(r.Context()).
			Where("event_type = ?", string(audit.EventPageExecute)).
			Order("id DESC").
			First(&stored).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				writeFixtureJSON(w, http.StatusNotFound, map[string]string{"error": "page_execute_audit_not_found"})
				return
			}
			writeFixtureJSON(w, http.StatusInternalServerError, map[string]string{"error": "audit_query_failed", "message": err.Error()})
			return
		}
		record, err := stored.ToRecord()
		if err != nil {
			writeFixtureJSON(w, http.StatusInternalServerError, map[string]string{"error": "audit_decode_failed", "message": err.Error()})
			return
		}
		writeFixtureJSON(w, http.StatusOK, map[string]interface{}{
			"eventType": record.EventType,
			"outcome":   record.Outcome,
			"gameId":    record.Resource.GameID,
			"env":       record.Resource.Environment,
			"details":   record.Details,
		})
	})
	mux.HandleFunc("/__fixture__/provider/calls", func(w http.ResponseWriter, r *http.Request) {
		writeFixtureJSON(w, http.StatusOK, map[string]interface{}{"calls": f.provider.calls()})
	})
	mux.HandleFunc("/__fixture__/provider/reset", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeFixtureJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
			return
		}
		f.provider.reset()
		writeFixtureJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	ln, err := net.Listen("tcp", f.FixtureAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", f.FixtureAddr, err)
	}
	f.fixtureSrv = &http.Server{Handler: mux}
	go func() {
		if err := f.fixtureSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Default().Error("fixture api stopped", "error", err)
		}
	}()
	return nil
}

func writeFixtureJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
