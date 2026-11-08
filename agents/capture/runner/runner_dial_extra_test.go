package runner

// runner.go 补测（dial 面）：fetchConfig/sendAlert 闭包线上往返（帧级应答服
// 务）、New 非法文件规则的防御性降级、gates 拉取 OnApply 的 appliedRevision
// 上报与重注册分支、consumeDurable ack 失败、位点 tick 周刷、nil-gates 的
// consume/sweep 直通、routeHit 队列满、sendHit 落盘/ack 清理/取消分支、
// Run 注册上线存储 conn、源循环退避 Stop、buildPayload 连接器能力标签、
// tlsFromCfg enabled 映射。
//
// 诚实缺口（不硬凑）：buildGateSet 各闸构造错误分支（ValidateGates 对同批
// 构造器先行 round-trip，构造期错误不可达）、readiness 窗口闭包与装配失败
// 分支（captureReadinessInterval=10s 起探过慢 / probeTarget 过滤保证目标
// 合法，NewManager 错误不可达）、journal 关闭失败（linux 文件 Close 不败）、
// writeFileAtomic 的 Write/Sync/Close 故障分支（临时文件无注入点）、spool
// 序列化失败（固定形状恒可 marshal）、源循环退避 Stop 兜底（v5 MaxElapsed-
// Time 15min 不可注入，生产语义为永不放弃仅封顶节奏）。

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/capture/config"
	"github.com/cuihairu/croupier/agents/capture/connector"
	"github.com/cuihairu/croupier/agents/capture/gate"
	"github.com/cuihairu/croupier/agents/controlconn"
	"github.com/cuihairu/croupier/core/configsync"
	"github.com/cuihairu/croupier/core/otelrec"
	"github.com/cuihairu/croupier/core/register"
	"github.com/cuihairu/croupier/core/transport/tcp"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// gatesServerState 帧级应答服务状态：ver() 为当前发布的闸规则版本（""=未
// 发布），failGates 置真后收到闸拉取即断连，regs 收注册请求。
type gatesServerState struct {
	ver       atomic.Value // string
	failGates atomic.Bool
	regs      chan struct{}
}

func newGatesServerState() *gatesServerState {
	return &gatesServerState{regs: make(chan struct{}, 4)}
}

// gatesServer 注册/心跳/闸规则/告警帧级应答服务（与 devops runnerServer 同型）。
func gatesServer(t *testing.T, state *gatesServerState) string {
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
				select {
				case state.regs <- struct{}{}:
				default:
				}
				resp, respID = &agentv1.RegisterResponse{SessionId: "sess-capture", ExpireAt: 1}, protocol.MsgRegisterResponse
			case protocol.MsgHeartbeatRequest:
				resp, respID = &agentv1.HeartbeatResponse{}, protocol.MsgHeartbeatResponse
			case protocol.MsgCaptureConfigRequest:
				if state.failGates.Load() {
					return // 断连模拟 wire 错误
				}
				respID = protocol.MsgCaptureConfigResponse
				if v, _ := state.ver.Load().(string); v == "" {
					resp = &opsv1.CaptureGatesConfigResponse{}
				} else {
					resp = &opsv1.CaptureGatesConfigResponse{Version: v, GatesJson: `{"event":{"ops":["insert","update","delete"]}}`}
				}
			case protocol.MsgAlertEventRequest:
				resp, respID = &opsv1.CaptureRuleHitAck{Accepted: true}, protocol.MsgAlertEventResponse
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

func TestFetchGatesAndAlertLiveConn(t *testing.T) {
	state := newGatesServerState()
	state.ver.Store("")
	addr := gatesServer(t, state)
	cfg := testConfig(t, false)
	cfg.Agent.ServerAddr = addr
	r := New(cfg, nil)

	conn, err := controlconn.Dial(addr, nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	r.conn.Store(conn)
	ctx := context.Background()

	// 空版本 = 服务端未发布 → nil 载荷，Puller 保持现状
	got, err := r.fetchConfig(ctx)
	require.NoError(t, err)
	require.Nil(t, got)

	// 发布 v7 → 取载荷
	state.ver.Store("7")
	got, err = r.fetchConfig(ctx)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "7", got.Version)
	require.Contains(t, string(got.Payload), "insert")

	// 告警上报闭包走存储的 conn 往返（先于 wire 断连用例）
	ack, err := r.sendAlert(ctx, &opsv1.CaptureRuleHitReport{AgentId: "capture-test", Rule: "reconcile.x"})
	require.NoError(t, err)
	require.True(t, ack.GetAccepted())

	// wire 错误（服务端断连）→ 透传
	state.failGates.Store(true)
	_, err = r.fetchConfig(ctx)
	require.Error(t, err)
}

func TestGatesPullerApplyTriggersReregister(t *testing.T) {
	r := New(testConfig(t, false), nil)
	r.fetchConfig = func(context.Context) (*configsync.Versioned[[]byte], error) {
		return &configsync.Versioned[[]byte]{Version: "7", Payload: []byte(`{}`)}, nil
	}
	// reg 句柄已装配 → OnApply 触发重注册（对死地址 SyncOnce 失败仅 Debug）。
	reg := register.New(register.Config{
		AgentID: "capture-test",
		Dial: func(context.Context) (register.Conn, error) {
			return nil, errors.New("conn down")
		},
		Payload: func(context.Context) (*agentv1.RegisterRequest, error) {
			return &agentv1.RegisterRequest{AgentId: "capture-test"}, nil
		},
		Logger: r.log,
	})
	r.reg.Store(reg)

	ok, err := r.newGatesPuller().PullOnce(context.Background())
	require.NoError(t, err)
	require.True(t, ok)
	v := r.appliedGatesRevision.Load()
	require.NotNil(t, v)
	require.Equal(t, "7", *v)
}

func TestNewInvalidGatesFallback(t *testing.T) {
	cfg := testConfig(t, false)
	// 文件规则非法（缺规则名）→ New 防御性降级全直通并计 PolicyApplyErrors
	// （config.Load 启动即校验，此处直构 cfg 才可达）。
	cfg.Capture.Gates.Reconcile.Rules = []gate.ReconcileRule{{
		ItemTable: "game.items", LedgerTable: "game.ledger", ItemKey: "k", LedgerKey: "k",
	}}
	r := New(cfg, nil)
	require.EqualValues(t, 1, r.Stats().PolicyApplyErrors)

	// 降级不阻碍后续中心规则热生效
	require.NoError(t, r.applyGates(context.Background(), []byte(`{}`)))
	require.EqualValues(t, 1, r.Stats().PolicyApplyErrors)
}

func TestConsumeDurableAckError(t *testing.T) {
	r := New(testConfig(t, false), nil)
	stub := &stubSource{
		events: []connector.ChangeEvent{{
			SourceID: "s1", SourceType: "mysql", Database: "game", Table: "items",
			Operation: connector.OpUpdate, Position: connector.Position{Kind: "gtid", Value: "g-1"},
		}},
		ackErr: errors.New("ack rejected"),
	}
	r.newConnector = func() connector.SourceConnector { return stub }
	err := r.runSourceOnce(context.Background())
	require.ErrorContains(t, err, "advance checkpoint")
	require.Equal(t, 1, stub.closeCnt)
}

func TestRunSourceOnceTickFlushesBookmark(t *testing.T) {
	cfg := testConfig(t, false)
	cfg.Capture.BookmarkFlushIntervalSec = 1
	r := New(cfg, nil)
	stub := &stubSource{block: true, cp: connector.Position{Kind: "gtid", Value: "g-tick"}}
	r.newConnector = func() connector.SourceConnector { return stub }

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.runSourceOnce(ctx) }()

	// tick 周刷落盘（位点 tick 与闸 sweep 同频）
	require.Eventually(t, func() bool {
		bm, err := r.store.Load()
		return err == nil && bm.Position != ""
	}, 3*time.Second, 20*time.Millisecond, "tick must flush bookmark")

	cancel()
	require.NoError(t, <-done)
}

func TestConsumeAndSweepNilGates(t *testing.T) {
	// gates 未装配（直构 Runner，绕开 New 的兜底装载）→ consume/sweep 直通。
	r := &Runner{log: slog.New(slog.NewTextHandler(io.Discard, nil)), rec: otelrec.Discard()}
	ev := connector.ChangeEvent{SourceID: "s1", SourceType: "mysql", Database: "game",
		Table: "items", Operation: connector.OpInsert}
	r.consume(ev)
	r.sweepGates(time.Now().UnixMilli())
}

func TestConsumeLineageGateBranch(t *testing.T) {
	r := New(testConfig(t, false), nil)
	lineageJSON := `{"lineage":{"rules":[{"name":"gw","table":"game.items","allowAccounts":["app"]}]}}`
	require.NoError(t, r.applyGates(context.Background(), []byte(lineageJSON)))
	// lineage 闸在位 → consume 走血缘评估臂（命中与否都进 recordHits）。
	ev := connector.ChangeEvent{SourceID: "s1", SourceType: "mysql", Database: "game",
		Table: "items", Operation: connector.OpInsert, Account: "app"}
	r.consume(ev)
	require.EqualValues(t, 1, r.Stats().Evaluated)
}

func TestRouteHitQueueFull(t *testing.T) {
	r := New(testConfig(t, false), nil)
	r.hitSpool = nil // 纯内存队列口径
	for i := 0; i < hitQueueSize; i++ {
		r.hitCh <- gate.Hit{Rule: "r"}
	}
	require.ErrorContains(t, r.routeHit(gate.Hit{Rule: "overflow"}), "capture alert queue full")
	<-r.hitCh
	require.NoError(t, r.routeHit(gate.Hit{Rule: "fits"}))
}

func TestSendHitFaultBranches(t *testing.T) {
	ctx := context.Background()
	blocker := filepath.Join(t.TempDir(), "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o644))

	// spool 落盘失败（父目录是普通文件）→ 计丢弃并透传
	r := New(testConfig(t, false), nil)
	r.hitSpool = newAlertSpool(filepath.Join(blocker, "spool.json"))
	err := r.sendHit(ctx, gate.Hit{Rule: "r1"})
	require.Error(t, err)
	require.EqualValues(t, 1, r.Stats().AlertDropped)

	// writer 缺失 → saveLocked 防御分支
	r2 := New(testConfig(t, false), nil)
	r2.hitSpool.writeFile = nil
	require.ErrorContains(t, r2.sendHit(ctx, gate.Hit{Rule: "r2"}), "alert spool writer unavailable")

	// ack 清理失败：队里预置一条保证 ack 时仍有 pending → writeFile 翻转失败
	// （预置在翻转装配前，闭包首调即 ack 清理落盘）。
	r3 := New(testConfig(t, false), nil)
	r3.hitSpool = newAlertSpool(filepath.Join(t.TempDir(), "spool.json"))
	require.NoError(t, r3.hitSpool.enqueue(gate.Hit{Rule: "pre-existing"}))
	var calls atomic.Int32
	r3.hitSpool.writeFile = func(path string, data []byte) error {
		if calls.Add(1) == 2 { // 1: enqueue 本条, 2: ack 清理
			return errors.New("disk full")
		}
		return writeFileAtomic(path, data)
	}
	sent := make(chan string, 1)
	r3.sendAlert = func(_ context.Context, rep *opsv1.CaptureRuleHitReport) (*opsv1.CaptureRuleHitAck, error) {
		sent <- rep.Rule
		return &opsv1.CaptureRuleHitAck{Accepted: true}, nil
	}
	require.NoError(t, r3.sendHit(ctx, gate.Hit{Rule: "r3"}), "ack 清理失败不影响上报成功")
	select {
	case rule := <-sent:
		require.Equal(t, "r3", rule)
	case <-time.After(2 * time.Second):
		t.Fatal("sendAlert not invoked")
	}

	// ctx 已取消：首试失败即 break，不再重试
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	attempts := 0
	r4 := New(testConfig(t, false), nil)
	r4.sendAlert = func(context.Context, *opsv1.CaptureRuleHitReport) (*opsv1.CaptureRuleHitAck, error) {
		attempts++
		return nil, errors.New("down")
	}
	require.Error(t, r4.sendHit(cancelled, gate.Hit{Rule: "r4"}))
	require.Equal(t, 1, attempts, "cancelled ctx must not retry")

	// 重试间隔等待期间 ctx 取消 → Sleep false → break（有界重试的中断路径）
	r5ctx, r5cancel := context.WithCancel(ctx)
	r5 := New(testConfig(t, false), nil)
	r5.hitRetryInterval = time.Hour
	failOnce := make(chan struct{})
	r5.sendAlert = func(context.Context, *opsv1.CaptureRuleHitReport) (*opsv1.CaptureRuleHitAck, error) {
		select {
		case failOnce <- struct{}{}:
		default:
		}
		return nil, errors.New("down")
	}
	done5 := make(chan error, 1)
	go func() { done5 <- r5.sendHit(r5ctx, gate.Hit{Rule: "r5"}) }()
	<-failOnce
	time.Sleep(50 * time.Millisecond) // 确保已进入重试等待
	r5cancel()
	select {
	case err := <-done5:
		require.Error(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("sendHit did not exit after ctx cancel during retry wait")
	}
}

func TestRunLiveServerRegistrationStoresConn(t *testing.T) {
	state := newGatesServerState()
	cfg := testConfig(t, false)
	cfg.Agent.ServerAddr = gatesServer(t, state)
	r := New(cfg, nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()

	// Run 内 Dial 闭包存储连接（capture 拉取/上报闭包共用），注册握手完成
	waitFor(t, func() bool {
		c := r.conn.Load()
		return c != nil && c.Connected()
	})
	select {
	case <-state.regs:
	case <-time.After(5 * time.Second):
		t.Fatal("server did not see register request")
	}

	cancel()
	require.NoError(t, <-done)
}

func TestRunCaptureLoopBackoffStop(t *testing.T) {
	cfg := testConfig(t, true)
	r := New(cfg, nil)
	// 源重开退避快档：第 4 次重开 NextBackOff 返回 Stop → 兜底 backoffMax。
	r.backoffInitial = time.Millisecond
	r.backoffMax = 3 * time.Millisecond
	stub := &stubSource{openErr: errors.New("no repl")}
	r.newConnector = func() connector.SourceConnector { return stub }

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()

	// 退避序列 1/1.5/2.25ms 微秒级走完 Stop 分支；宽限后取消退出
	time.Sleep(200 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("capture loop did not exit after cancel")
	}
}

func TestBuildPayloadConnectorCapabilityLabels(t *testing.T) {
	base := testConfig(t, true)
	base.Capture.MySQL = nil
	base.Capture.Options = map[string]string{"tables": "game.items"}

	base.Capture.SourceType = "mongodb"
	r := New(base, nil)
	req, err := r.buildPayload(context.Background())
	require.NoError(t, err)
	require.Equal(t, "game.items", req.Labels["captureTables"])
	require.NotEmpty(t, req.Labels["captureConnectorCapabilities"], "mongo caps label missing")

	base.Capture.SourceType = "postgres"
	r = New(base, nil)
	req, err = r.buildPayload(context.Background())
	require.NoError(t, err)
	require.Equal(t, "game.items", req.Labels["captureTables"])
	require.NotEmpty(t, req.Labels["captureConnectorCapabilities"], "postgres caps label missing")
}

func TestTLSFromCfgEnabled(t *testing.T) {
	cfg := testConfig(t, false)
	cfg.OutboundTLS = config.OutboundTLS{
		Enabled:            true,
		CertFile:           "c.crt",
		KeyFile:            "c.key",
		CAFile:             "ca.crt",
		ServerName:         "capture.internal",
		InsecureSkipVerify: true,
	}
	got := tlsFromCfg(cfg)
	require.NotNil(t, got)
	require.True(t, got.Enabled)
	require.Equal(t, "c.crt", got.CertFile)
	require.Equal(t, "c.key", got.KeyFile)
	require.Equal(t, "ca.crt", got.CAFile)
	require.Equal(t, "capture.internal", got.ServerName)
	require.True(t, got.InsecureSkipVerify)
}
