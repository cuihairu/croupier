package healthprobe

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startProbeListener 起 mock listener（私有握手演示用），返回地址；测试结束关闭。
func startProbeListener(t *testing.T, handler func(conn net.Conn)) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go handler(conn)
		}
	}()
	return ln.Addr().String(), func() { _ = ln.Close() }
}

func TestNewProtocolCheckerValidation(t *testing.T) {
	_, err := NewProtocolChecker("", []StepSpec{{Name: "a"}}, 0)
	assert.ErrorContains(t, err, "address required")
	_, err = NewProtocolChecker("127.0.0.1:1", nil, 0)
	assert.ErrorContains(t, err, "at least one step")
	_, err = NewProtocolChecker("127.0.0.1:1", []StepSpec{{Send: []byte("x")}}, 0)
	assert.ErrorContains(t, err, "name required")
	_, err = NewProtocolChecker("127.0.0.1:1", []StepSpec{{Name: "a", Expect: StepExpect{Regexp: "["}}}, 0)
	assert.ErrorContains(t, err, "invalid regexp")

	chk, err := NewProtocolChecker("127.0.0.1:1", []StepSpec{{Name: "a", Timeout: time.Second}}, 3*time.Second)
	require.NoError(t, err)
	assert.Equal(t, 3*time.Second, chk.DialTimeout)
	// 构造期拷贝步骤切片，外部后续改动不影响已构造的探测。
	steps := []StepSpec{{Name: "a"}}
	chk, err = NewProtocolChecker("127.0.0.1:1", steps, 0)
	require.NoError(t, err)
	steps[0].Name = "mutated"
	assert.Equal(t, "a", chk.Steps[0].Name)
}

// TestNewProtocolCheckerHexFields 配置面十六进制字段归一（sendHex/containsHex/
// prefixHex）：解码进字节字段、双写报错、坏 hex 构造期报。
func TestNewProtocolCheckerHexFields(t *testing.T) {
	chk, err := NewProtocolChecker("127.0.0.1:1", []StepSpec{
		{Name: "hs", SendHex: "48454c4f", Expect: StepExpect{PrefixHex: "57454c"}},
	}, 0)
	require.NoError(t, err)
	assert.Equal(t, []byte("HELO"), chk.Steps[0].Send)
	assert.Equal(t, "", chk.Steps[0].SendHex) // 归一后不留双源
	assert.Equal(t, []byte("WEL"), chk.Steps[0].Expect.Prefix)
	assert.Equal(t, "", chk.Steps[0].Expect.PrefixHex)

	_, err = NewProtocolChecker("127.0.0.1:1", []StepSpec{
		{Name: "hs", Send: []byte("A"), SendHex: "41"},
	}, 0)
	assert.ErrorContains(t, err, "send and sendHex are mutually exclusive")
	_, err = NewProtocolChecker("127.0.0.1:1", []StepSpec{
		{Name: "hs", Expect: StepExpect{Contains: []byte("A"), ContainsHex: "41"}},
	}, 0)
	assert.ErrorContains(t, err, "contains and containsHex are mutually exclusive")
	_, err = NewProtocolChecker("127.0.0.1:1", []StepSpec{
		{Name: "hs", SendHex: "zz"},
	}, 0)
	assert.ErrorContains(t, err, "invalid sendHex")
}

// TestProtocolCheckerPrivateHandshake §5.5.4 L2 验收主场景：mock listener 演示
// 私有握手——发握手包→期望前缀→发登录→期望成功码，配置零码接入。
func TestProtocolCheckerPrivateHandshake(t *testing.T) {
	addr, stop := startProbeListener(t, func(conn net.Conn) {
		defer func() { _ = conn.Close() }()
		buf := make([]byte, 4)
		if _, err := io.ReadFull(conn, buf); err != nil {
			return
		}
		if string(buf) != "HELO" {
			return
		}
		_, _ = conn.Write([]byte("WELCOME-1.0"))
		line := make([]byte, 9) // "LOGIN u1\n"
		if _, err := io.ReadFull(conn, line); err != nil {
			return
		}
		_, _ = conn.Write([]byte("AUTH OK"))
	})
	defer stop()

	chk, err := NewProtocolChecker(addr, []StepSpec{
		{Name: "handshake", Send: []byte("HELO"), Expect: StepExpect{Prefix: []byte("WEL")}},
		{Name: "login", Send: []byte("LOGIN u1\n"), Expect: StepExpect{Contains: []byte("OK")}},
	}, 0)
	require.NoError(t, err)
	require.NoError(t, chk.Check(context.Background()))

	d := chk.Details()
	assert.Equal(t, "protocol", d["probeType"])
	assert.Equal(t, "done", d["phase"])
	assert.Equal(t, "2/2", d["step"])
	assert.NotContains(t, d, "reconnect")
}

// TestProtocolCheckerStepFailureMarksPhase 协议失败：error 串带阶段标记，Details
// 记 phase/step；经 Probe 进窗口后 LastReason 同样可辨（§5.5.3 protocol-failed）。
func TestProtocolCheckerStepFailureMarksPhase(t *testing.T) {
	addr, stop := startProbeListener(t, func(conn net.Conn) {
		defer func() { _ = conn.Close() }()
		buf := make([]byte, 4)
		if _, err := io.ReadFull(conn, buf); err != nil {
			return
		}
		_, _ = conn.Write([]byte("WELCOME")) // 步骤 1 通过
		line := make([]byte, 9)
		if _, err := io.ReadFull(conn, line); err != nil {
			return
		}
		_, _ = conn.Write([]byte("REJECTED")) // 步骤 2：不含期望的 OK
	})
	defer stop()

	chk, err := NewProtocolChecker(addr, []StepSpec{
		{Name: "handshake", Send: []byte("HELO"), Expect: StepExpect{Prefix: []byte("WEL")}},
		{Name: "login", Send: []byte("LOGIN u1\n"), Expect: StepExpect{Contains: []byte("OK")}, Timeout: 300 * time.Millisecond},
	}, 0)
	require.NoError(t, err)
	err = chk.Check(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "protocol failed at step 2 (login)")
	assert.Contains(t, err.Error(), "REJECTED") // 现场字节进诊断串
	d := chk.Details()
	assert.Equal(t, "login", d["phase"])
	assert.Equal(t, "2/2", d["step"])

	// 经 Probe：failThreshold=1 直接开窗，LastReason 带阶段标记。
	var windows []Window
	tc := TargetConfig{Name: "gw", Type: TargetTypeProtocol, Address: addr,
		Steps: chk.Steps, FailThreshold: 1}
	p, err := ProbeFromTarget(tc, "demo", "prod", func(ev WindowEvent) { windows = append(windows, ev.Window) }, nil)
	require.NoError(t, err)
	res := p.ProbeOnce(context.Background())
	require.False(t, res.OK)
	require.Len(t, windows, 1)
	assert.Contains(t, windows[0].LastReason, "protocol failed at step 2 (login)")
	assert.Equal(t, "login", res.Details["phase"])
}

// TestProtocolCheckerDialFailureDistinct 与上例对偶：dial 失败的 LastReason 不带
// 「protocol failed」前缀，两类故障可区分（§5.5.3 验收）。
func TestProtocolCheckerDialFailureDistinct(t *testing.T) {
	chk, err := NewProtocolChecker("127.0.0.1:1", []StepSpec{{Name: "handshake", Send: []byte("HELO")}}, 0)
	require.NoError(t, err)
	err = chk.Check(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dial tcp")
	assert.NotContains(t, err.Error(), "protocol failed")
	assert.Equal(t, "dial", chk.Details()["phase"])

	var windows []Window
	tc := TargetConfig{Name: "dead", Type: TargetTypeProtocol, Address: "127.0.0.1:1",
		Steps: []StepSpec{{Name: "handshake", Send: []byte("HELO")}}, FailThreshold: 1}
	p, err := ProbeFromTarget(tc, "demo", "prod", func(ev WindowEvent) { windows = append(windows, ev.Window) }, nil)
	require.NoError(t, err)
	res := p.ProbeOnce(context.Background())
	require.False(t, res.OK)
	require.Len(t, windows, 1)
	assert.NotContains(t, windows[0].LastReason, "protocol failed")
	assert.Equal(t, "dial", res.Details["phase"])
}

// TestProtocolCheckerDialRetryOnce 规则 4：建连瞬时错误单次重试，计数进 Details。
func TestProtocolCheckerDialRetryOnce(t *testing.T) {
	var attempts atomic.Int32
	chk := &ProtocolChecker{
		Address: "127.0.0.1:0",
		Steps:   []StepSpec{{Name: "only"}},
		dialer: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if attempts.Add(1) == 1 {
				return nil, errors.New("connection reset")
			}
			c1, c2 := net.Pipe()
			go func() { _ = c1.Close() }() // 立即 EOF：无响应步骤将失败
			return c2, nil
		},
	}
	// 步骤无 Send 且管道立即 EOF → 无响应失败，但重试计数必须已记录。
	err := chk.Check(context.Background())
	require.Error(t, err)
	d := chk.Details()
	assert.Equal(t, "1", d["reconnect"])
	assert.EqualValues(t, 2, attempts.Load())

	// 重试后成功：reconnect=1 且 Check 通过。
	var attempts2 atomic.Int32
	chk2 := &ProtocolChecker{
		Address: "127.0.0.1:0",
		Steps:   []StepSpec{{Name: "only"}},
		dialer: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if attempts2.Add(1) == 1 {
				return nil, errors.New("connection reset")
			}
			c1, c2 := net.Pipe()
			go func() {
				_, _ = c1.Write([]byte("hi"))
				_ = c1.Close()
			}()
			return c2, nil
		},
	}
	require.NoError(t, chk2.Check(context.Background()))
	assert.Equal(t, "1", chk2.Details()["reconnect"])
	assert.EqualValues(t, 2, attempts2.Load())
}

// TestProtocolCheckerNoRetryMidHandshake 握手半途不重试（非幂等）：步骤失败时
// 不得出现重拨。
func TestProtocolCheckerNoRetryMidHandshake(t *testing.T) {
	addr, stop := startProbeListener(t, func(conn net.Conn) {
		defer func() { _ = conn.Close() }()
		// 收到后不回应，等对端超时。
		buf := make([]byte, 4)
		_, _ = io.ReadFull(conn, buf)
		time.Sleep(2 * time.Second)
	})
	defer stop()

	var dials atomic.Int32
	chk := &ProtocolChecker{
		Address: addr,
		Steps:   []StepSpec{{Name: "handshake", Send: []byte("HELO"), Timeout: 200 * time.Millisecond}},
		dialer: func(ctx context.Context, network, addr string) (net.Conn, error) {
			dials.Add(1)
			return net.Dial(network, addr)
		},
	}
	err := chk.Check(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "protocol failed at step 1 (handshake)")
	assert.EqualValues(t, 1, dials.Load()) // 半途失败不重拨
	assert.NotContains(t, chk.Details(), "reconnect")
}

// TestProtocolCheckerFreshConnectionPerCheck 规则 1：每次 Check 新建连接。
func TestProtocolCheckerFreshConnectionPerCheck(t *testing.T) {
	var conns atomic.Int32
	addr, stop := startProbeListener(t, func(conn net.Conn) {
		conns.Add(1)
		defer func() { _ = conn.Close() }()
		_, _ = conn.Write([]byte("ok"))
	})
	defer stop()

	chk, err := NewProtocolChecker(addr, []StepSpec{{Name: "only", Expect: StepExpect{Contains: []byte("ok")}}}, 0)
	require.NoError(t, err)
	require.NoError(t, chk.Check(context.Background()))
	require.NoError(t, chk.Check(context.Background()))
	assert.EqualValues(t, 2, conns.Load())
}

func TestProtocolCheckerExpectVariants(t *testing.T) {
	t.Run("empty expect any response", func(t *testing.T) {
		addr, stop := startProbeListener(t, func(conn net.Conn) {
			defer func() { _ = conn.Close() }()
			_, _ = conn.Write([]byte{0x00, 0xff})
		})
		defer stop()
		chk, err := NewProtocolChecker(addr, []StepSpec{{Name: "only", Timeout: time.Second}}, 0)
		require.NoError(t, err)
		assert.NoError(t, chk.Check(context.Background()))
	})
	t.Run("regexp full buffer", func(t *testing.T) {
		addr, stop := startProbeListener(t, func(conn net.Conn) {
			defer func() { _ = conn.Close() }()
			_, _ = conn.Write([]byte("v=1.2.3 status=green"))
		})
		defer stop()
		chk, err := NewProtocolChecker(addr, []StepSpec{
			{Name: "only", Expect: StepExpect{Regexp: `status=(green|yellow)`}, Timeout: time.Second},
		}, 0)
		require.NoError(t, err)
		assert.NoError(t, chk.Check(context.Background()))
	})
	t.Run("regexp no match then timeout", func(t *testing.T) {
		addr, stop := startProbeListener(t, func(conn net.Conn) {
			defer func() { _ = conn.Close() }()
			_, _ = conn.Write([]byte("status=red"))
			time.Sleep(500 * time.Millisecond) // 保持连接：不关，等对端读超时
		})
		defer stop()
		chk, err := NewProtocolChecker(addr, []StepSpec{
			{Name: "only", Expect: StepExpect{Regexp: `status=green`}, Timeout: 200 * time.Millisecond},
		}, 0)
		require.NoError(t, err)
		err = chk.Check(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "timeout waiting for match")
	})
	t.Run("partial data then closed", func(t *testing.T) {
		addr, stop := startProbeListener(t, func(conn net.Conn) {
			_, _ = conn.Write([]byte("status=red"))
			_ = conn.Close() // 收到部分数据即断开：与「空缓冲 EOF」区分
		})
		defer stop()
		chk, err := NewProtocolChecker(addr, []StepSpec{
			{Name: "only", Expect: StepExpect{Regexp: `status=green`}, Timeout: time.Second},
		}, 0)
		require.NoError(t, err)
		err = chk.Check(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "connection closed while waiting")
	})
	t.Run("multi-form AND", func(t *testing.T) {
		addr, stop := startProbeListener(t, func(conn net.Conn) {
			defer func() { _ = conn.Close() }()
			_, _ = conn.Write([]byte("PREFIX ok"))
		})
		defer stop()
		chk, err := NewProtocolChecker(addr, []StepSpec{
			{Name: "only", Expect: StepExpect{Prefix: []byte("PREFIX"), Contains: []byte("ok"), Regexp: `^PREFIX\s+ok$`}, Timeout: time.Second},
		}, 0)
		require.NoError(t, err)
		assert.NoError(t, chk.Check(context.Background()))

		// Contains 不满足 → 等到超时失败。
		chk, err = NewProtocolChecker(addr, []StepSpec{
			{Name: "only", Expect: StepExpect{Prefix: []byte("PREFIX"), Contains: []byte("great")}, Timeout: 200 * time.Millisecond},
		}, 0)
		require.NoError(t, err)
		assert.Error(t, chk.Check(context.Background()))
	})
	t.Run("closed without response", func(t *testing.T) {
		addr, stop := startProbeListener(t, func(conn net.Conn) {
			_ = conn.Close() // 直接断开
		})
		defer stop()
		chk, err := NewProtocolChecker(addr, []StepSpec{{Name: "only", Timeout: time.Second}}, 0)
		require.NoError(t, err)
		err = chk.Check(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "connection closed without response")
	})
	t.Run("timeout without response", func(t *testing.T) {
		addr, stop := startProbeListener(t, func(conn net.Conn) {
			defer func() { _ = conn.Close() }()
			time.Sleep(time.Second)
		})
		defer stop()
		chk, err := NewProtocolChecker(addr, []StepSpec{{Name: "only", Timeout: 150 * time.Millisecond}}, 0)
		require.NoError(t, err)
		err = chk.Check(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "timeout waiting for response")
	})
}

// TestNewCheckerProtocolTarget 工厂收口：steps 声明式与 protocol 适配器两条子路径。
func TestNewCheckerProtocolTarget(t *testing.T) {
	chk, err := NewChecker(TargetConfig{Name: "gw", Type: TargetTypeProtocol,
		Address: "127.0.0.1:7777",
		Steps:   []StepSpec{{Name: "handshake", Send: []byte("HELO"), Expect: StepExpect{Prefix: []byte("WEL")}}}})
	require.NoError(t, err)
	pc, ok := chk.(*ProtocolChecker)
	require.True(t, ok)
	require.Len(t, pc.Steps, 1)
	assert.Equal(t, "handshake", pc.Steps[0].Name)

	// 缺地址 / 缺步骤与适配器名。
	_, err = NewChecker(TargetConfig{Name: "gw", Type: TargetTypeProtocol, Steps: []StepSpec{{Name: "a"}}})
	assert.ErrorContains(t, err, "requires address")
	_, err = NewChecker(TargetConfig{Name: "gw", Type: TargetTypeProtocol, Address: "127.0.0.1:1"})
	assert.ErrorContains(t, err, "requires steps or protocol")

	// steps 与 protocol 互斥。
	_, err = NewChecker(TargetConfig{Name: "gw", Type: TargetTypeProtocol, Address: "127.0.0.1:1",
		Protocol: "testgame", Steps: []StepSpec{{Name: "a"}}})
	assert.ErrorContains(t, err, "mutually exclusive")

	// 未注册的适配器名。
	_, err = NewChecker(TargetConfig{Name: "gw", Type: TargetTypeProtocol, Address: "127.0.0.1:1", Protocol: "no-such-game"})
	assert.ErrorContains(t, err, "unknown game protocol")

	// 注册（包内测试用假适配器）→ 工厂解析到注册器产物。
	registerTestGameOnce.Do(func() {
		RegisterGameProtocol("TestGame", func(tc TargetConfig) (Checker, error) {
			return CheckerFunc(func(ctx context.Context) error { return errors.New("testgame stub") }), nil
		})
	})
	chk, err = NewChecker(TargetConfig{Name: "gw2", Type: TargetTypeProtocol, Address: "127.0.0.1:1", Protocol: "testgame"})
	require.NoError(t, err)
	_, isFunc := chk.(CheckerFunc)
	assert.True(t, isFunc)
	assert.Contains(t, RegisteredGameProtocols(), "testgame")
}

var registerTestGameOnce sync.Once

func TestRegisterGameProtocolPanicsOnDuplicate(t *testing.T) {
	assert.PanicsWithValue(t, "healthprobe: RegisterGameProtocol called twice for dup-probe-game", func() {
		RegisterGameProtocol("dup-probe-game", func(tc TargetConfig) (Checker, error) { return nil, nil })
		RegisterGameProtocol("dup-probe-game", func(tc TargetConfig) (Checker, error) { return nil, nil })
	})
	assert.Panics(t, func() { RegisterGameProtocol("", func(tc TargetConfig) (Checker, error) { return nil, nil }) })
	assert.Panics(t, func() { RegisterGameProtocol("nil-maker", nil) })
}
