package runner

// runner.go 补测：fetchAllowlist 闭包全分支（无连接 / 线上拉取：空版本保持
// 现状、已发布取载荷、wire 错误透传）、applyAllowlistCenter 校验与热换分支、
// Run 的审计/存档/SSH 装配失败早退（audit-first 不降级）、注册上线成功路径
// （conn 存储 + Connected）、Hub 句柄与会话 observer 打点、tlsFromCfg 两侧、
// newAllowlistPuller 的 apply seam 与 OnApply。

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/controlconn"
	"github.com/cuihairu/croupier/agents/devops/config"
	"github.com/cuihairu/croupier/agents/devops/session"
	"github.com/cuihairu/croupier/core/configsync"
	"github.com/cuihairu/croupier/core/execlog"
	"github.com/cuihairu/croupier/core/transport/tcp"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

const validAllowlistJSON = `[{"host":"h1","ports":[22],"protocols":["ssh"]}]`

// runnerServer 注册/心跳/白名单帧级应答服务；ver() 返回当前发布的白名单
// 版本（"" = 未发布），failAllowlist 置真后服务端收到白名单拉取即断连。
type allowServerState struct {
	ver           atomic.Value // string
	failAllowlist atomic.Bool
}

func runnerServer(t *testing.T, state *allowServerState) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		for {
			frame, err := tcp.ReadFrame(conn)
			if err != nil {
				return
			}
			_, msgID, reqID, _, err := protocol.ParseMessageFromBody(frame)
			if err != nil {
				return
			}
			var resp proto.Message
			respID := msgID
			switch msgID {
			case protocol.MsgRegisterRequest:
				resp, respID = &agentv1.RegisterResponse{SessionId: "sess-runner", ExpireAt: 1}, protocol.MsgRegisterResponse
			case protocol.MsgHeartbeatRequest:
				resp, respID = &agentv1.HeartbeatResponse{}, protocol.MsgHeartbeatResponse
			case protocol.MsgDevopsAllowlistConfigRequest:
				if state.failAllowlist.Load() {
					return // 断连模拟 wire 错误
				}
				respID = protocol.MsgDevopsAllowlistConfigResponse
				if v, _ := state.ver.Load().(string); v == "" {
					resp = &opsv1.DevopsAllowlistConfigResponse{}
				} else {
					resp = &opsv1.DevopsAllowlistConfigResponse{Version: v, AllowlistJson: validAllowlistJSON}
				}
			default:
				return
			}
			out, err := proto.Marshal(resp)
			if err != nil {
				return
			}
			if err := tcp.WriteFrame(conn, protocol.NewMessageBody(respID, reqID, out)); err != nil {
				return
			}
		}
	}()
	return ln.Addr().String()
}

func TestFetchAllowlistWithoutConn(t *testing.T) {
	r := New(testConfig(false), nil)
	_, err := r.fetchAllowlist(context.Background())
	require.ErrorContains(t, err, "control conn not ready")
}

func TestFetchAllowlistLiveConnBranches(t *testing.T) {
	state := &allowServerState{}
	state.ver.Store("")
	addr := runnerServer(t, state)
	cfg := testConfig(false)
	cfg.Agent.ServerAddr = addr
	r := New(cfg, nil)

	conn, err := controlconn.Dial(addr, nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	r.conn.Store(conn)
	ctx := context.Background()

	// 空版本 = 服务端未发布 → nil 载荷，Puller 保持现状
	got, err := r.fetchAllowlist(ctx)
	require.NoError(t, err)
	require.Nil(t, got)

	// 发布 v7 → 取载荷
	state.ver.Store("7")
	got, err = r.fetchAllowlist(ctx)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "7", got.Version)
	require.Contains(t, string(got.Payload), "h1")

	// wire 错误（服务端断连）→ 透传
	state.failAllowlist.Store(true)
	_, err = r.fetchAllowlist(ctx)
	require.Error(t, err)
}

// stubRunnerTarget/stubRunnerAudit Broker 装配的最小替身（SwapAllowlist 不触
// Open/审计面）。
type stubRunnerTarget struct{}

func (stubRunnerTarget) Name() string { return "stub" }
func (stubRunnerTarget) Open(context.Context, session.SessionRequest) (session.Session, error) {
	return nil, errors.New("stub target")
}

type stubRunnerAudit struct{}

func (stubRunnerAudit) Admit(*execlog.Record) error  { return nil }
func (stubRunnerAudit) Record(*execlog.Record) error { return nil }

func TestApplyAllowlistCenterBranches(t *testing.T) {
	r := New(testConfig(false), nil)
	ctx := context.Background()

	// 坏 JSON
	require.ErrorContains(t, r.applyAllowlistCenter(ctx, []byte("{bad")), "parse allowlist json")
	// 空名单
	require.ErrorContains(t, r.applyAllowlistCenter(ctx, []byte("[]")), "must not be empty")
	// 缺 host
	require.ErrorContains(t, r.applyAllowlistCenter(ctx,
		[]byte(`[{"host":"","ports":[22],"protocols":["ssh"]}]`)), "host is required")

	// broker 未装配
	require.ErrorContains(t, r.applyAllowlistCenter(ctx, []byte(validAllowlistJSON)), "broker not wired")

	// 装配后整体热换
	allow := session.NewAllowlist(testConfig(false).Devops.Allowlist)
	r.broker = session.NewBroker(allow, stubRunnerAudit{}, stubRunnerTarget{}, nil,
		"devops", "d1", "demo", "prod")
	require.NoError(t, r.applyAllowlistCenter(ctx, []byte(validAllowlistJSON)))
}

func TestNewAllowlistPullerKeepsInjectedApplyAndFiresOnApply(t *testing.T) {
	r := New(testConfig(false), nil)
	r.fetchAllowlist = func(context.Context) (*configsync.Versioned[[]byte], error) {
		return &configsync.Versioned[[]byte]{Version: "7", Payload: []byte(validAllowlistJSON)}, nil
	}
	applied := make(chan []byte, 1)
	r.applyAllowlist = func(_ context.Context, payload []byte) error {
		applied <- payload
		return nil
	}

	puller := r.newAllowlistPuller() // 注入的 apply 不被中心实现覆盖
	ok, err := puller.PullOnce(context.Background())
	require.NoError(t, err)
	require.True(t, ok)
	select {
	case payload := <-applied:
		require.Contains(t, string(payload), "h1")
	case <-time.After(2 * time.Second):
		t.Fatal("apply hook not invoked")
	}
}

func TestRunAssemblyFailuresExitWithError(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o644))

	// 审计通道打不开 → 返回错误交监管，不降级为无审计放行
	cfg := testConfig(true)
	cfg.Devops.SessionDir = t.TempDir()
	cfg.Devops.ExeclogDir = filepath.Join(blocker, "exec")
	require.Error(t, New(cfg, nil).Run(context.Background()))

	// 存档目录打不开
	cfg = testConfig(true)
	cfg.Devops.SessionDir = filepath.Join(blocker, "sessions")
	cfg.Devops.ExeclogDir = t.TempDir()
	cfg.Devops.SSH = config.SSH{User: "ops", PasswordFile: mkPasswordFile(t), InsecureHostKey: true}
	require.Error(t, New(cfg, nil).Run(context.Background()))

	// SSH 凭据缺失 → 不起会话面
	cfg = testConfig(true)
	cfg.Devops.SessionDir = t.TempDir()
	cfg.Devops.ExeclogDir = t.TempDir()
	require.ErrorContains(t, New(cfg, nil).Run(context.Background()), "devops ssh channel")
}

func mkPasswordFile(t *testing.T) string {
	t.Helper()
	pw := filepath.Join(t.TempDir(), "pw")
	require.NoError(t, os.WriteFile(pw, []byte("testpass\n"), 0o600))
	return pw
}

func TestRunRegistrationStoresConnAndFetches(t *testing.T) {
	state := &allowServerState{}
	state.ver.Store("")
	cfg := testConfig(false)
	cfg.Agent.ServerAddr = runnerServer(t, state)
	r := New(cfg, nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()

	waitFor(t, "conn registered", func() bool {
		c := r.conn.Load()
		return c != nil && c.Connected()
	})
	// 注册成功后的 conn 直接可拉白名单（空版本保持现状）
	got, err := r.fetchAllowlist(context.Background())
	require.NoError(t, err)
	require.Nil(t, got)

	cancel()
	require.NoError(t, <-done)
}

func TestRunEnabledWiresHubAndObserver(t *testing.T) {
	cfg := testConfig(true)
	cfg.Devops.SessionDir = t.TempDir()
	cfg.Devops.ExeclogDir = t.TempDir()
	cfg.Devops.SSH = config.SSH{User: "ops", PasswordFile: mkPasswordFile(t), InsecureHostKey: true}
	r := New(cfg, nil)
	counts := newCaptureCounts()
	r.SetRecorder(counts)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()

	// allowlist_apply/ready 在 Hub 装配之后打出 —— 以此为同步点读 Hub
	waitForCount(t, counts, "allowlist_apply", "ready")
	require.NotNil(t, r.Hub(), "hub not wired when enabled")

	// 白名单外开会被拒 → observer 打 session/fail
	_, err := r.Broker().OpenSession(ctx, session.SessionRequest{
		Protocol: session.ProtocolSSH, Host: "not-allowed", Port: 22, Operator: "ops",
	})
	require.Error(t, err)
	waitForCount(t, counts, "session", "fail")

	cancel()
	require.NoError(t, <-done)
}

func TestTLSFromCfg(t *testing.T) {
	require.Nil(t, tlsFromCfg(testConfig(false)))

	cfg := testConfig(false)
	cfg.OutboundTLS = config.OutboundTLS{
		Enabled:            true,
		CertFile:           "c.crt",
		KeyFile:            "c.key",
		CAFile:             "ca.crt",
		ServerName:         "control.internal",
		InsecureSkipVerify: true,
	}
	got := tlsFromCfg(cfg)
	require.NotNil(t, got)
	require.True(t, got.Enabled)
	require.Equal(t, "c.crt", got.CertFile)
	require.Equal(t, "c.key", got.KeyFile)
	require.Equal(t, "ca.crt", got.CAFile)
	require.Equal(t, "control.internal", got.ServerName)
	require.True(t, got.InsecureSkipVerify)
}

// captureCounts 记录 Count 打点的 recorder（channel 传递，跨 goroutine 无 race）。
type captureCounts struct {
	counts chan countHit
}

type countHit struct {
	name string
	tags map[string]string
}

func newCaptureCounts() *captureCounts { return &captureCounts{counts: make(chan countHit, 16)} }

func (c *captureCounts) Start(context.Context, map[string]string)                    {}
func (c *captureCounts) Stop(context.Context, error)                                 {}
func (c *captureCounts) Connected(context.Context)                                   {}
func (c *captureCounts) Disconnected(context.Context, error)                         {}
func (c *captureCounts) Observe(context.Context, string, float64, map[string]string) {}

func (c *captureCounts) Count(_ context.Context, name string, _ int64, tags map[string]string) {
	select {
	case c.counts <- countHit{name: name, tags: tags}:
	default:
	}
}

func waitForCount(t *testing.T, c *captureCounts, name, outcome string) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case hit := <-c.counts:
			if hit.name == name && hit.tags["outcome"] == outcome {
				return
			}
		case <-deadline:
			t.Fatalf("timeout waiting for count %s(outcome=%s)", name, outcome)
		}
	}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", what)
}
