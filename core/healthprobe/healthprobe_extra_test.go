package healthprobe

// 分散分支补测：HTTP/TCP checker 构造默认值与失败面、HeartbeatFile 落盘
// 故障分支、Manager nil 守卫与 start 后 Add 确定性感跑、ProbePolicy 全校验
// 分支与 EffectiveInterval 逐层 continue/兜底、Probe.Run/RunDynamic 兜底
// 分支、ProtocolChecker 探测期故障分支（stub conn + 注入 dialer）。

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

func TestCheckerConstructorsDefaultTimeouts(t *testing.T) {
	assert.Equal(t, 5*time.Second, NewHTTPChecker("http://x", 0).Client.Timeout)
	assert.Equal(t, 3*time.Second, NewTCPChecker("127.0.0.1:1", 0).DialTimeout)

	// HTTP 请求装配失败（坏 URL，请求发出前即报）
	err := NewHTTPChecker("://bad", time.Second).Check(context.Background())
	require.Error(t, err)

	// TCP 拨号失败
	err = NewTCPChecker("127.0.0.1:1", 50*time.Millisecond).Check(context.Background())
	require.Error(t, err)
}

func TestHeartbeatFileFailureBranches(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o644))

	// 父目录建不出来
	err := NewHeartbeatFile(filepath.Join(blocker, "hb")).Beat()
	require.ErrorContains(t, err, "create heartbeat dir")

	// 路径是目录 → 写入失败
	dir := filepath.Join(t.TempDir(), "as-dir")
	require.NoError(t, os.Mkdir(dir, 0o755))
	err = NewHeartbeatFile(dir).Beat()
	require.ErrorContains(t, err, "write heartbeat")

	// 文件不存在 → LastBeat/Age 均不可用
	hb := NewHeartbeatFile(filepath.Join(t.TempDir(), "never-beat"))
	_, ok := hb.LastBeat()
	assert.False(t, ok)
	_, ok = hb.Age()
	assert.False(t, ok)
}

func TestManagerNilGuardsAndConstructFailures(t *testing.T) {
	var nilManager *Manager
	require.ErrorContains(t, nilManager.Add(httpTarget("x")), "nil manager")
	require.ErrorContains(t, nilManager.Remove("x"), "nil manager")
	assert.Nil(t, nilManager.Probes())

	_, err := NewManager(nil, "", "", nil, nil)
	require.ErrorContains(t, err, "no targets configured")

	// 目标校验失败 → 整体报错不产半装配
	_, err = NewManager([]TargetConfig{{Name: "  ", Category: TargetCategoryGame, Type: TargetTypeTCP, Address: "x:1"}}, "", "", nil, nil)
	require.Error(t, err)
}

func TestManagerAddAfterStartDeterministic(t *testing.T) {
	// 先等首个目标真的跑起来，再 Add 第二个 —— startNow 分支（立即起跑）
	// 与「已起跑条目跳过」分支确定性感发（原用例靠 goroutine 竞速，覆盖漂移）。
	var probes atomic.Int32
	m := NewDynamicManager("", "", nil, func(Result) { probes.Add(1) })
	m.Add(httpTarget("first"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { m.StartDynamic(ctx, nil); close(done) }()

	require.Eventually(t, func() bool { return probes.Load() >= 1 },
		2*time.Second, 10*time.Millisecond)
	require.NoError(t, m.Add(httpTarget("second")))
	require.Eventually(t, func() bool { return probes.Load() >= 2 },
		2*time.Second, 10*time.Millisecond, "post-start add must run")
	assert.Len(t, m.Probes(), 2)

	// Remove 未命中已有条目 → 走 continue 分支后 not found
	require.ErrorContains(t, m.Remove("ghost"), "not found")

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("manager start did not exit after ctx cancel")
	}
}

func TestProbePolicyValidateAllBranches(t *testing.T) {
	// nil 策略校验通过
	var nilPolicy *ProbePolicy
	require.NoError(t, nilPolicy.Validate())
	assert.Equal(t, DefaultInterval, nilPolicy.BaseInterval())
	assert.Equal(t, DefaultInterval, nilPolicy.EffectiveInterval("t", time.Now()))
	assert.True(t, nilPolicy.AutoBoostEnabled())

	p := &ProbePolicy{Interval: "5x"}
	require.ErrorContains(t, p.Validate(), "probe policy interval")

	p = &ProbePolicy{Targets: []ProbePolicyTarget{{Target: "  "}}}
	require.ErrorContains(t, p.Validate(), "target name is required")

	p = &ProbePolicy{Targets: []ProbePolicyTarget{
		{Target: "a", Interval: "30s"}, {Target: "a", Interval: "30s"},
	}}
	require.ErrorContains(t, p.Validate(), `duplicate target "a"`)

	p = &ProbePolicy{Targets: []ProbePolicyTarget{{Target: "a", Interval: "bad"}}}
	require.ErrorContains(t, p.Validate(), "probe policy targets[a] interval")

	p = &ProbePolicy{Windows: []ProbePolicyWindow{{Start: "25:00", End: "26:00"}}}
	require.ErrorContains(t, p.Validate(), "windows[0] start")

	p = &ProbePolicy{Windows: []ProbePolicyWindow{{Start: "08:00", End: "bad"}}}
	require.ErrorContains(t, p.Validate(), "windows[0] end")

	p = &ProbePolicy{Windows: []ProbePolicyWindow{{Start: "09:00", End: "08:00"}}}
	require.ErrorContains(t, p.Validate(), "end must be after start")

	p = &ProbePolicy{Windows: []ProbePolicyWindow{{Start: "08:00", End: "09:00", Interval: "xx"}}}
	require.ErrorContains(t, p.Validate(), "windows[0] interval")

	p = &ProbePolicy{Events: []ProbePolicyEvent{{At: "not-rfc3339"}}}
	require.ErrorContains(t, p.Validate(), "must be RFC3339")

	p = &ProbePolicy{Events: []ProbePolicyEvent{{At: "2026-01-01T00:00:00Z", Duration: "bad"}}}
	require.ErrorContains(t, p.Validate(), "events[0] duration")

	p = &ProbePolicy{Events: []ProbePolicyEvent{{At: "2026-01-01T00:00:00Z", Duration: "1h", Interval: "bad"}}}
	require.ErrorContains(t, p.Validate(), "events[0] interval")
}

func TestProbePolicyEffectiveIntervalLayers(t *testing.T) {
	now := time.Date(2026, 11, 9, 10, 30, 0, 0, time.UTC)
	inWindowAt := "2026-11-09T10:00:00Z"

	// 事件层命中（duration 覆盖 now）
	p := &ProbePolicy{Events: []ProbePolicyEvent{{At: inWindowAt, Duration: "1h", Interval: "2s"}}}
	assert.Equal(t, 2*time.Second, p.EffectiveInterval("t", now))

	// 事件 At 坏 → continue 落窗口层
	p = &ProbePolicy{
		Events:  []ProbePolicyEvent{{At: "bad", Duration: "1h", Interval: "2s"}},
		Windows: []ProbePolicyWindow{{Start: "10:00", End: "11:00", Interval: "4s"}},
	}
	assert.Equal(t, 4*time.Second, p.EffectiveInterval("t", now))

	// 事件 duration 坏 → continue 落窗口层
	p = &ProbePolicy{
		Events:  []ProbePolicyEvent{{At: inWindowAt, Duration: "bad", Interval: "2s"}},
		Windows: []ProbePolicyWindow{{Start: "10:00", End: "11:00", Interval: "6s"}},
	}
	assert.Equal(t, 6*time.Second, p.EffectiveInterval("t", now))

	// 事件 interval 坏 → policyDurationOr 兜底 DefaultInterval
	p = &ProbePolicy{Events: []ProbePolicyEvent{{At: inWindowAt, Duration: "1h", Interval: "nope"}}}
	assert.Equal(t, DefaultInterval, p.EffectiveInterval("t", now))

	// 窗口 start 坏 → continue → base
	p = &ProbePolicy{Windows: []ProbePolicyWindow{{Start: "bad", End: "11:00", Interval: "4s"}}}
	assert.Equal(t, DefaultInterval, p.EffectiveInterval("t", now))

	// 窗口 end 坏 → continue → base
	p = &ProbePolicy{Windows: []ProbePolicyWindow{{Start: "10:00", End: "bad", Interval: "4s"}}}
	assert.Equal(t, DefaultInterval, p.EffectiveInterval("t", now))

	// 窗口命中但不覆盖 now → base；BaseInterval 自身坏 → 防御性兜底
	p = &ProbePolicy{Interval: "broken"}
	assert.Equal(t, DefaultInterval, p.BaseInterval())
}

func TestProbeTargetGetterAndFallbackLoops(t *testing.T) {
	pr := New("db-primary", SemanticsReadiness, CheckerFunc(func(context.Context) error { return nil }))
	assert.Equal(t, "db-primary", pr.Target())

	// 无窗可用 → CurrentWindow miss 分支
	_, open := pr.CurrentWindow()
	assert.False(t, open)

	// 探测失败 → Result.Err 落值
	p := New("x", SemanticsLiveness, CheckerFunc(func(context.Context) error {
		return errors.New("boom")
	}))
	res := p.ProbeOnce(context.Background())
	require.False(t, res.OK)
	assert.Contains(t, res.Err, "boom")
}

func TestProbeRunAndDynamicFallbacks(t *testing.T) {
	var runs atomic.Int32
	p := New("run-x", SemanticsLiveness, CheckerFunc(func(context.Context) error {
		runs.Add(1)
		return nil
	}))

	// Run（固定间隔闭包）→ 首探即跑，cancel 退出
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { p.Run(ctx, time.Hour); close(done) }()
	require.Eventually(t, func() bool { return runs.Load() >= 1 }, 2*time.Second, 5*time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit after cancel")
	}

	// RunDynamic：intervalFn 为 nil / 返回 <=0 都兜底 DefaultInterval
	var runs2 atomic.Int32
	p2 := New("run-y", SemanticsLiveness, CheckerFunc(func(context.Context) error {
		runs2.Add(1)
		return nil
	}))
	ctx2, cancel2 := context.WithCancel(context.Background())
	done2 := make(chan struct{})
	go func() { p2.RunDynamic(ctx2, func(time.Time) time.Duration { return 0 }); close(done2) }()
	require.Eventually(t, func() bool { return runs2.Load() >= 1 }, 2*time.Second, 5*time.Millisecond)
	cancel2()
	select {
	case <-done2:
	case <-time.After(2 * time.Second):
		t.Fatal("RunDynamic did not exit after cancel")
	}
}

func TestDBCheckerOpenUnknownDriverFails(t *testing.T) {
	// 未知 driver → sql.Open 即报（延迟连接的 driver 系列在 Open 不报错，
	// 这里取的是 Open 面的早失败分支）
	c := &DBChecker{Driver: "no-such-driver", DSN: "x"}
	err := c.CheckWith(context.Background(), nil)
	require.ErrorContains(t, err, "db open")
}

func TestCheckerContextDeadlineTightens(t *testing.T) {
	// 上层 ctx deadline 更早 → 以更紧者为准（TCP 侧与 protocol 步进侧同语义）
	deadlined, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Nanosecond))
	defer cancel()
	require.Error(t, NewTCPChecker("127.0.0.1:1", time.Second).Check(deadlined))

	// protocol 步进侧：ctx deadline 紧于 step timeout，正常回包即通过（不等待）
	conn := &stubProtoConn{reads: [][]byte{[]byte("ok")}}
	c := handBuiltProtocol(conn, StepSpec{Name: "a"})
	require.NoError(t, c.Check(deadlined))
}

func TestICMPPingOnceWithoutContextDeadline(t *testing.T) {
	// 直接驱动 pingOnce（无 ctx deadline → 回退 checker timeout 分支）
	c := NewICMPChecker("127.0.0.1", 200*time.Millisecond, 1)
	c.open = func(string, string) (net.PacketConn, error) {
		return &fakeICMPConn{
			reply: func(req []byte) ([]byte, error) {
				return echoReplyOf(t, icmpProtocolIPv4, req, nil, ipv4ReplyType()), nil
			},
		}, nil
	}
	ip, err := net.ResolveIPAddr("ip", "127.0.0.1")
	require.NoError(t, err)
	require.NoError(t, c.pingOnce(context.Background(), ip))
}

// ipv4ReplyType 应答回声类型（测试文件内小助手，避免散写转换）。
func ipv4ReplyType() icmp.Type { return ipv4.ICMPTypeEchoReply }

// failTimeline AppendWindow 恒失败的 timeline 替身（打点失败不炸探测链）。
type failTimeline struct{}

func (failTimeline) AppendWindow(WindowEvent) error { return errors.New("disk full") }

func TestProbeTimelineAppendFailureWarnsOnly(t *testing.T) {
	var runs atomic.Int32
	p := New("tl-x", SemanticsReadiness, CheckerFunc(func(context.Context) error {
		if runs.Add(1) == 1 {
			return errors.New("down")
		}
		return nil
	}), WithThresholds(1, 1), WithTimeline(failTimeline{}))

	res := p.ProbeOnce(context.Background())
	require.False(t, res.OK) // 窗口开启
	res = p.ProbeOnce(context.Background())
	require.True(t, res.OK) // 闭窗；两窗口事件都走 append 失败分支
	assert.GreaterOrEqual(t, runs.Load(), int32(2))
}

func TestProbeRunClosureAndNilIntervalFallback(t *testing.T) {
	// checker 内部 cancel：保证 ProbeOnce 后 intervalFn 必被执行（无竞速）
	ctx, cancel := context.WithCancel(context.Background())
	var runs atomic.Int32
	p := New("run-z", SemanticsLiveness, CheckerFunc(func(context.Context) error {
		if runs.Add(1) == 1 {
			cancel()
		}
		return nil
	}))
	p.Run(ctx, time.Hour) // 固定间隔闭包至少执行一次后退出

	// RunDynamic nil intervalFn → DefaultInterval 兜底闭包
	ctx2, cancel2 := context.WithCancel(context.Background())
	var runs2 atomic.Int32
	p2 := New("run-w", SemanticsLiveness, CheckerFunc(func(context.Context) error {
		if runs2.Add(1) == 1 {
			cancel2()
		}
		return nil
	}))
	p2.RunDynamic(ctx2, nil)
}

// containsHex 成功归一路径（send/prefix 已有既有用例覆盖）
func TestProtocolCheckerContainsHexNormalize(t *testing.T) {
	chk, err := NewProtocolChecker("127.0.0.1:1", []StepSpec{
		{Name: "hs", Expect: StepExpect{ContainsHex: "4f4b"}},
	}, 0)
	require.NoError(t, err)
	assert.Equal(t, []byte("OK"), chk.Steps[0].Expect.Contains)
	assert.Empty(t, chk.Steps[0].Expect.ContainsHex)
}

func TestFileTimelineFailureBranches(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o644))

	// 父目录建不出
	_, err := OpenFileTimeline(filepath.Join(blocker, "tl.jsonl"))
	require.ErrorContains(t, err, "create timeline dir")

	// 打不开（目标是目录）
	dir := filepath.Join(t.TempDir(), "as-dir")
	require.NoError(t, os.Mkdir(dir, 0o755))
	_, err = OpenFileTimeline(dir)
	require.ErrorContains(t, err, "open timeline file")

	// 正常链路：写一条 → Close → 二次 Close 幂等 → Append 报已关闭
	path := filepath.Join(t.TempDir(), "tl.jsonl")
	tl, err := OpenFileTimeline(path)
	require.NoError(t, err)
	require.NoError(t, tl.AppendWindow(WindowEvent{Kind: EventUnavailable}))
	require.NoError(t, tl.Close())
	require.NoError(t, tl.Close())
	require.ErrorContains(t, tl.AppendWindow(WindowEvent{}), "timeline closed")

	// LoadTimeline：坏行报 decode 错误并保留已解析前缀；空行跳过
	bad := filepath.Join(t.TempDir(), "bad.jsonl")
	require.NoError(t, os.WriteFile(bad, []byte("{\"kind\":\"unavailable\"}\n\n{bad}\n"), 0o644))
	events, err := LoadTimeline(bad)
	require.ErrorContains(t, err, "decode timeline line")
	require.Len(t, events, 1)
}

func TestProcessCheckerFindProcessFailure(t *testing.T) {
	// unix FindProcess 对不存在的 pid 也成功；负 pid 走 FindProcess 错误分支
	// （不可达的环境跳过）。
	if _, err := os.FindProcess(-1); err != nil {
		err = NewProcessChecker(-1).Check(context.Background())
		require.ErrorContains(t, err, "find process")
	}
}

// ---- protocol.go ----

func TestProtocolCheckerNormalizeRestConflicts(t *testing.T) {
	// contains/prefix 双写冲突 + 坏 hex + 构造期坏 regexp
	_, err := NewProtocolChecker("127.0.0.1:1", []StepSpec{
		{Name: "a", Expect: StepExpect{Contains: []byte("A"), ContainsHex: "41"}},
	}, 0)
	require.ErrorContains(t, err, "contains and containsHex are mutually exclusive")

	_, err = NewProtocolChecker("127.0.0.1:1", []StepSpec{
		{Name: "a", Expect: StepExpect{Prefix: []byte("A"), PrefixHex: "41"}},
	}, 0)
	require.ErrorContains(t, err, "prefix and prefixHex are mutually exclusive")

	_, err = NewProtocolChecker("127.0.0.1:1", []StepSpec{
		{Name: "a", Expect: StepExpect{ContainsHex: "zz"}},
	}, 0)
	require.ErrorContains(t, err, "invalid containsHex")

	_, err = NewProtocolChecker("127.0.0.1:1", []StepSpec{
		{Name: "a", Expect: StepExpect{PrefixHex: "zz"}},
	}, 0)
	require.ErrorContains(t, err, "invalid prefixHex")

	_, err = NewProtocolChecker("127.0.0.1:1", []StepSpec{
		{Name: "a", Expect: StepExpect{Regexp: "("}},
	}, 0)
	require.ErrorContains(t, err, "invalid regexp")
}

// stubProtoConn net.Conn 替身：SetDeadline/Write 可注入故障，Read 按序列
// 返回预置数据后挂起到 deadline。
type stubProtoConn struct {
	reads        [][]byte
	readErr      error // reads 耗尽后返回（非超时/EOF 类读错误走 "read:" 分支）
	deadlineErr  error
	writeErr     error
	defaultDelay time.Duration
}

func (c *stubProtoConn) Read(p []byte) (int, error) {
	if len(c.reads) > 0 {
		r := c.reads[0]
		c.reads = c.reads[1:] // 消费式：残余数据只回放一次，否则匹配循环永转
		return copy(p, r), nil
	}
	if c.readErr != nil {
		return 0, c.readErr
	}
	time.Sleep(c.defaultDelay)
	return 0, os.ErrDeadlineExceeded
}

func (c *stubProtoConn) Write(b []byte) (int, error) {
	if c.writeErr != nil {
		return 0, c.writeErr
	}
	return len(b), nil
}

func (c *stubProtoConn) SetDeadline(time.Time) error {
	return c.deadlineErr
}

func (c *stubProtoConn) SetReadDeadline(time.Time) error  { return nil }
func (c *stubProtoConn) SetWriteDeadline(time.Time) error { return nil }
func (c *stubProtoConn) Close() error                     { return nil }
func (c *stubProtoConn) LocalAddr() net.Addr              { return nil }
func (c *stubProtoConn) RemoteAddr() net.Addr             { return nil }

func handBuiltProtocol(conn net.Conn, steps ...StepSpec) *ProtocolChecker {
	return &ProtocolChecker{Address: "127.0.0.1:1", Steps: steps, dialer: func(context.Context, string, string) (net.Conn, error) {
		return conn, nil
	}}
}

func TestProtocolCheckerRunStepFaults(t *testing.T) {
	ctx := context.Background()

	// SetDeadline 失败
	c := handBuiltProtocol(&stubProtoConn{deadlineErr: errors.New("bad fd")},
		StepSpec{Name: "a"})
	require.ErrorContains(t, c.Check(ctx), "set deadline")

	// Send 写失败
	c = handBuiltProtocol(&stubProtoConn{writeErr: errors.New("broken pipe")},
		StepSpec{Name: "a", Send: []byte("ping")})
	require.ErrorContains(t, c.Check(ctx), "write 4 bytes")

	// 探测期非法 regexp（绕过构造期的手装 checker）
	c = handBuiltProtocol(&stubProtoConn{},
		StepSpec{Name: "a", Expect: StepExpect{Regexp: "["}})
	require.ErrorContains(t, c.Check(ctx), "invalid regexp")

	// Contains 不匹配 → 超时；>64 字节现场被截断
	long := make([]byte, 100)
	for i := range long {
		long[i] = 'x'
	}
	c = handBuiltProtocol(&stubProtoConn{reads: [][]byte{long}, defaultDelay: time.Second},
		StepSpec{Name: "a", Expect: StepExpect{Contains: []byte("needle")}, Timeout: 80 * time.Millisecond})
	err := c.Check(ctx)
	require.ErrorContains(t, err, "timeout waiting for match")
	require.Contains(t, err.Error(), string(long[:64]))
	require.NotContains(t, err.Error(), string(long))

	// Prefix 不匹配 → 同样等到超时
	c = handBuiltProtocol(&stubProtoConn{reads: [][]byte{[]byte("hello")}, defaultDelay: time.Second},
		StepSpec{Name: "a", Expect: StepExpect{Prefix: []byte("bye")}, Timeout: 80 * time.Millisecond})
	require.ErrorContains(t, c.Check(ctx), "timeout waiting for match")

	// 非超时/EOF 类读错误 → "read:" 分支透传
	c = handBuiltProtocol(&stubProtoConn{reads: [][]byte{[]byte("x")}, readErr: errors.New("conn reset")},
		StepSpec{Name: "a", Expect: StepExpect{Contains: []byte("needle")}})
	require.ErrorContains(t, c.Check(ctx), "read: conn reset")
}

func TestProtocolCheckerDialTimeoutDefault(t *testing.T) {
	c := &ProtocolChecker{}
	assert.Equal(t, DefaultDialTimeout, c.dialTimeout())
	c.DialTimeout = time.Second
	assert.Equal(t, time.Second, c.dialTimeout())
}

func TestProtocolCheckerDialCancelledNoRetry(t *testing.T) {
	// 上层取消不算瞬时错误：失败后不重试（retried=false）
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	var calls int
	c := &ProtocolChecker{Address: "x", Steps: []StepSpec{{Name: "a"}},
		dialer: func(context.Context, string, string) (net.Conn, error) {
			calls++
			return nil, errors.New("refused")
		}}
	err := c.Check(cancelled)
	require.Error(t, err)
	require.Equal(t, 1, calls, "cancelled dial must not retry")
	require.Contains(t, c.Details()["phase"], "dial")
}
