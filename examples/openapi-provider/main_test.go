package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	agentcore "github.com/cuihairu/croupier/internal/app/agent"
)

const testTimeout = 5 * time.Second

// fakeAgent 实现 embeddedAgent：记录配置调用，Run 按需阻塞到 ctx 取消或立即返回。
type fakeAgent struct {
	entered chan struct{}
	done    chan struct{}
	wait    bool
	runErr  error

	onceEntered sync.Once
	onceDone    sync.Once

	mu      sync.Mutex
	addr    string
	kind    string
	meta    agentcore.UpstreamMetadata
	gotMeta bool
}

func newFakeAgent(wait bool, runErr error) *fakeAgent {
	return &fakeAgent{
		entered: make(chan struct{}),
		done:    make(chan struct{}),
		wait:    wait,
		runErr:  runErr,
	}
}

func (f *fakeAgent) SetLocalAddr(addr string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.addr = addr
}

func (f *fakeAgent) SetUpstreamTransportKind(kind string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.kind = kind
}

func (f *fakeAgent) WithUpstreamMetadata(meta agentcore.UpstreamMetadata) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.meta = meta
	f.gotMeta = true
}

func (f *fakeAgent) Run(ctx context.Context) error {
	f.onceEntered.Do(func() { close(f.entered) })
	defer f.onceDone.Do(func() { close(f.done) })
	if f.wait {
		<-ctx.Done()
	}
	return f.runErr
}

func (f *fakeAgent) snapshot() (addr, kind string, meta agentcore.UpstreamMetadata, ok bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.addr, f.kind, f.meta, f.gotMeta
}

// unusedFactory 返回的工厂：仅应被不会走到 Agent 分支的用例使用。
func unusedFactory(t *testing.T) agentFactory {
	t.Helper()
	return func(serverAddr, agentID, configDir string) embeddedAgent {
		t.Errorf("agent factory must not be called here (server=%q agent=%q dir=%q)", serverAddr, agentID, configDir)
		return newFakeAgent(true, nil)
	}
}

// TestMain_ExitCodeOnUnknownFlag 覆盖 main 本身：替换 os.Exit 注入点，
// 用非法 flag 断言退出码 2（与原 flag.ExitOnError 行为一致）。
func TestMain_ExitCodeOnUnknownFlag(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	oldExit := exit
	defer func() { exit = oldExit }()

	var code int
	exit = func(c int) { code = c }
	os.Args = []string{"openapi-provider", "-definitely-not-a-flag"}

	main()

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestRunMain_HelpFlag(t *testing.T) {
	var buf bytes.Buffer
	code := runMain("openapi-provider", []string{"-h"}, &buf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	out := buf.String()
	if !strings.Contains(out, "Usage of openapi-provider:") {
		t.Fatalf("usage header missing in %q", out)
	}
	for _, name := range []string{"-server", "-http", "-game-id", "-env", "-agent-id", "-local-addr", "-config-dir", "-no-agent"} {
		if !strings.Contains(out, name) {
			t.Fatalf("flag %s missing in usage output %q", name, out)
		}
	}
}

func TestRunMain_UnknownFlag(t *testing.T) {
	var buf bytes.Buffer
	code := runMain("openapi-provider", []string{"-bogus"}, &buf)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(buf.String(), "flag provided but not defined: -bogus") {
		t.Fatalf("error text missing in %q", buf.String())
	}
}

// TestRunMain_ListenFailure 覆盖运行期错误路径：HTTP 端口被占用 → run 返回
// error → 退出码 1（同时覆盖 signal.NotifyContext / defer stop / exitCode(err)）。
func TestRunMain_ListenFailure(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	code := runMain("openapi-provider", []string{"-http", ln.Addr().String(), "-no-agent"}, io.Discard)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}

func TestExitCode(t *testing.T) {
	if got := exitCode(nil); got != 0 {
		t.Fatalf("exitCode(nil) = %d, want 0", got)
	}
	if got := exitCode(errors.New("boom")); got != 1 {
		t.Fatalf("exitCode(err) = %d, want 1", got)
	}
}

// TestNewFlagSet_Declarations 锁定 CLI 契约：flag 名与默认值不得漂移。
func TestNewFlagSet_Declarations(t *testing.T) {
	fs, cfg := newFlagSet("openapi-provider", io.Discard)
	wantDefaults := map[string]string{
		"server":     "127.0.0.1:19090",
		"http":       "127.0.0.1:8091",
		"game-id":    "default",
		"env":        "dev",
		"agent-id":   "openapi-demo-agent",
		"local-addr": "127.0.0.1:19091",
		"config-dir": "",
		"no-agent":   "false",
	}
	gotDefaults := map[string]string{}
	fs.VisitAll(func(f *flag.Flag) {
		gotDefaults[f.Name] = f.DefValue
		if f.Usage == "" {
			t.Errorf("flag %s has empty usage", f.Name)
		}
	})
	if len(gotDefaults) != len(wantDefaults) {
		t.Fatalf("flags = %v, want %v", gotDefaults, wantDefaults)
	}
	for name, want := range wantDefaults {
		if gotDefaults[name] != want {
			t.Errorf("flag %s default = %q, want %q", name, gotDefaults[name], want)
		}
	}

	if err := fs.Parse([]string{"-http", "10.0.0.1:9000", "-game-id", "g2", "-env", "qa", "-no-agent"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.httpAddr != "10.0.0.1:9000" || cfg.gameID != "g2" || cfg.env != "qa" || !cfg.noAgent {
		t.Fatalf("cfg = %+v, want parsed values applied", *cfg)
	}
	if cfg.serverAddr != "127.0.0.1:19090" || cfg.agentID != "openapi-demo-agent" || cfg.configDir != "" {
		t.Fatalf("cfg = %+v, want untouched defaults for unset flags", *cfg)
	}
}

// TestDefaultNewAgent 覆盖生产 Agent 构造（仅构造对象，不连接）。
func TestDefaultNewAgent(t *testing.T) {
	a := defaultNewAgent("127.0.0.1:19090", "openapi-demo-agent", t.TempDir())
	if a == nil {
		t.Fatal("defaultNewAgent returned nil")
	}
	if _, ok := a.(*agentcore.App); !ok {
		t.Fatalf("type = %T, want *agentcore.App", a)
	}
	a.SetLocalAddr("127.0.0.1:0")
	a.SetUpstreamTransportKind("tcp")
}

// waitForHTTP 轮询直到服务可用；若 run 提前退出则用其错误直接失败。
func waitForHTTP(t *testing.T, url string, errCh <-chan error) {
	t.Helper()
	deadline := time.Now().Add(testTimeout)
	for {
		select {
		case err := <-errCh:
			t.Fatalf("run exited early: %v", err)
		default:
		}
		resp, err := http.Get(url)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", url)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func runInGoroutine(t *testing.T, ctx context.Context, cfg runConfig, factory agentFactory) <-chan error {
	t.Helper()
	errCh := make(chan error, 1)
	go func() { errCh <- run(ctx, cfg, factory) }()
	return errCh
}

func waitRunResult(t *testing.T, errCh <-chan error) error {
	t.Helper()
	select {
	case err := <-errCh:
		return err
	case <-time.After(testTimeout):
		t.Fatal("run did not return in time")
		return nil
	}
}

// TestRun_NoAgentServesHTTP 覆盖 -no-agent 生命周期：起服务 → 请求可用 →
// ctx 结束后优雅关闭（run 返回 nil）。
func TestRun_NoAgentServesHTTP(t *testing.T) {
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("probe listen: %v", err)
	}
	addr := probe.Addr().String()
	probe.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := runInGoroutine(t, ctx, runConfig{httpAddr: addr, noAgent: true}, unusedFactory(t))

	baseURL := "http://" + addr
	waitForHTTP(t, baseURL+"/players", errCh)

	resp, err := http.Get(baseURL + "/openapi.json")
	if err != nil {
		t.Fatalf("get openapi.json: %v", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
		t.Fatalf("openapi.json: status=%d content-type=%q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}

	cancel()
	if err := waitRunResult(t, errCh); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
}

// TestRun_AgentPath 覆盖内嵌 Agent 分支：providers.yaml 落盘、Agent 配置注入、
// ctx 结束后关停；Agent 本体用假实现（避免端口绑定与上游重连）。
func TestRun_AgentPath(t *testing.T) {
	fake := newFakeAgent(true, nil)
	var gotServer, gotAgentID, gotDir string
	factory := func(serverAddr, agentID, configDir string) embeddedAgent {
		gotServer, gotAgentID, gotDir = serverAddr, agentID, configDir
		return fake
	}

	dir := t.TempDir()
	cfg := runConfig{
		serverAddr: "127.0.0.1:19090",
		httpAddr:   "127.0.0.1:0",
		gameID:     "demo",
		env:        "qa",
		agentID:    "demo-agent",
		localAddr:  "127.0.0.1:0",
		configDir:  dir,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := runInGoroutine(t, ctx, cfg, factory)

	select {
	case <-fake.entered:
	case <-time.After(testTimeout):
		t.Fatal("agent Run was not started")
	}

	raw, err := os.ReadFile(filepath.Join(dir, "providers.yaml"))
	if err != nil {
		t.Fatalf("read providers.yaml: %v", err)
	}
	yaml := string(raw)
	for _, want := range []string{
		"providers:",
		"type: openapi",
		"enabled: true",
		`game_id: "demo"`,
		`env: "qa"`,
		`timeout: "5s"`,
	} {
		if !strings.Contains(yaml, want) {
			t.Errorf("providers.yaml missing %q:\n%s", want, yaml)
		}
	}
	baseURL := regexp.MustCompile(`baseUrl: "([^"]+)"`).FindStringSubmatch(yaml)
	if baseURL == nil || !strings.HasPrefix(baseURL[1], "http://127.0.0.1:") {
		t.Fatalf("baseUrl missing or malformed in:\n%s", yaml)
	}
	specURL := regexp.MustCompile(`openapiSpec: "([^"]+)"`).FindStringSubmatch(yaml)
	if specURL == nil || specURL[1] != baseURL[1]+"/openapi.json" {
		t.Fatalf("openapiSpec = %v, want baseUrl+/openapi.json in:\n%s", specURL, yaml)
	}

	// providers.yaml 指向本进程真实 API：契约可被拉取（Agent 注册函数的来源）
	resp, err := http.Get(specURL[1])
	if err != nil {
		t.Fatalf("get openapi spec: %v", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("openapi spec status = %d, want 200", resp.StatusCode)
	}

	if gotServer != "127.0.0.1:19090" || gotAgentID != "demo-agent" || gotDir != dir {
		t.Fatalf("factory args = (%q, %q, %q), want (127.0.0.1:19090, demo-agent, %s)", gotServer, gotAgentID, gotDir, dir)
	}
	addr, kind, meta, ok := fake.snapshot()
	if !ok {
		t.Fatal("WithUpstreamMetadata was not called")
	}
	if addr != "127.0.0.1:0" || kind != "tcp" {
		t.Fatalf("agent config = addr %q kind %q, want 127.0.0.1:0/tcp", addr, kind)
	}
	if meta.GameID != "demo" || meta.Env != "qa" || meta.Version != "openapi-demo-1.0" ||
		meta.DialTimeout != 5*time.Second || meta.RequestTimeout != 10*time.Second || meta.HeartbeatInterval != 2 {
		t.Fatalf("metadata = %+v, want demo/qa/openapi-demo-1.0 with 5s/10s/2", meta)
	}

	cancel()
	if err := waitRunResult(t, errCh); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	select {
	case <-fake.done:
	case <-time.After(testTimeout):
		t.Fatal("agent goroutine did not finish after cancel")
	}
}

// TestRun_ConfigDirFailures 覆盖 run 内配置目录/写文件失败分支
// （原实现为 slog.Error + os.Exit(1)，现为返回 error → 退出码 1）。
func TestRun_ConfigDirFailures(t *testing.T) {
	t.Run("mkdtemp fails", func(t *testing.T) {
		t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
		cfg := runConfig{httpAddr: "127.0.0.1:0", configDir: ""}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		errCh := runInGoroutine(t, ctx, cfg, unusedFactory(t))
		if err := waitRunResult(t, errCh); err == nil {
			t.Fatal("want error when temp config dir cannot be created")
		}
	})

	t.Run("mkdirall fails", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "not-a-dir")
		if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
		cfg := runConfig{httpAddr: "127.0.0.1:0", configDir: filepath.Join(file, "child")}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		errCh := runInGoroutine(t, ctx, cfg, unusedFactory(t))
		if err := waitRunResult(t, errCh); err == nil {
			t.Fatal("want error when config dir cannot be prepared")
		}
	})

	t.Run("write providers.yaml fails", func(t *testing.T) {
		dir := t.TempDir()
		// providers.yaml 是目录 → WriteFile 必然失败（EISDIR）
		if err := os.Mkdir(filepath.Join(dir, "providers.yaml"), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		cfg := runConfig{httpAddr: "127.0.0.1:0", configDir: dir}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		errCh := runInGoroutine(t, ctx, cfg, unusedFactory(t))
		if err := waitRunResult(t, errCh); err == nil {
			t.Fatal("want error when providers.yaml cannot be written")
		}
	})
}

// TestResolveConfigDir 覆盖配置目录解析的全部分支（空值建临时目录 / 已有目录 /
// 建临时目录失败 / MkdirAll 失败）。
func TestResolveConfigDir(t *testing.T) {
	t.Run("empty creates temp dir", func(t *testing.T) {
		dir, err := resolveConfigDir("")
		if err != nil {
			t.Fatalf("resolveConfigDir: %v", err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(dir) })
		if dir == "" || filepath.Dir(dir) != os.TempDir() {
			t.Fatalf("dir = %q, want a fresh dir under %s", dir, os.TempDir())
		}
		if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
			t.Fatalf("stat(%q) = %v, want an existing directory", dir, err)
		}
	})

	t.Run("existing dir passes through", func(t *testing.T) {
		want := t.TempDir()
		got, err := resolveConfigDir(want)
		if err != nil {
			t.Fatalf("resolveConfigDir: %v", err)
		}
		if got != want {
			t.Fatalf("dir = %q, want %q", got, want)
		}
	})

	t.Run("mkdtemp fails", func(t *testing.T) {
		t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
		dir, err := resolveConfigDir("")
		if err == nil {
			t.Fatal("want error when TMPDIR does not exist")
		}
		if dir != "" {
			t.Fatalf("dir = %q, want empty on error", dir)
		}
	})

	t.Run("mkdirall fails", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "not-a-dir")
		if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
		dir, err := resolveConfigDir(filepath.Join(file, "child"))
		if err == nil {
			t.Fatal("want error when target path is under a regular file")
		}
		if dir != "" {
			t.Fatalf("dir = %q, want empty on error", dir)
		}
	})
}

// failingListener 让 Serve 立刻以非 ErrServerClosed 错误返回，
// 覆盖 serveHTTP 的错误日志分支（Accept 故障是进程级边界，无法从外部稳定注入）。
type failingListener struct {
	err error
}

func (l failingListener) Accept() (net.Conn, error) { return nil, l.err }
func (l failingListener) Close() error              { return nil }
func (l failingListener) Addr() net.Addr            { return dummyAddr{} }

type dummyAddr struct{}

func (dummyAddr) Network() string { return "tcp" }
func (dummyAddr) String() string  { return "127.0.0.1:0" }

func TestServeHTTP_UnexpectedAcceptError(t *testing.T) {
	srv := &http.Server{Handler: http.NewServeMux()}
	done := make(chan struct{})
	go func() {
		serveHTTP(srv, failingListener{err: errors.New("accept boom")})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(testTimeout):
		t.Fatal("serveHTTP did not return on listener error")
	}
}

func TestServeHTTP_ServerClosedIsNormal(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := &http.Server{Handler: http.NewServeMux()}
	done := make(chan struct{})
	go func() {
		serveHTTP(srv, ln)
		close(done)
	}()
	if err := srv.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	select {
	case <-done:
	case <-time.After(testTimeout):
		t.Fatal("serveHTTP did not return after server close")
	}
}

// TestRunAgent 覆盖内嵌 Agent 的三种收尾形态：正常返回、ctx 有效时提前失败、
// ctx 取消后的失败（生产中信号关停路径由 run 的 <-ctx.Done 驱动）。
func TestRunAgent(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		runAgent(context.Background(), newFakeAgent(false, nil))
	})

	t.Run("error with live ctx", func(t *testing.T) {
		runAgent(context.Background(), newFakeAgent(false, errors.New("agent boom")))
	})

	t.Run("error after cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		runAgent(ctx, newFakeAgent(false, errors.New("agent boom")))
	})
}
