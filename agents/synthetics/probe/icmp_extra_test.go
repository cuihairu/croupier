package probe

// icmp.go 补测：经注入式 dial 替身覆盖 Run 聚合层（解析失败/取消/重试
// 耗尽取 lastErr/超时聚合）与 pingOnce 全分支（dial 权限错误 →
// CapabilityError、SetDeadline/WriteTo/ReadFrom 失败、超时、坏包跳过、
// 非 Echo 主体跳过、ID/Seq/载荷不匹配跳过、回声应答命中成功、类型不符
// 报错）、classifyICMPError 三分类、CapabilityError 两形态、
// NewICMPProbe 零值兜底、真到期驱动的超时聚合。
//
// 诚实跳过（结构性不可达）：pingOnce 的 Marshal 错误分支（:101，报文由
// 固定类型 + 随机载荷构成，Echo 主体恒可序列化）与 ctx.Deadline 无期限
// 分支（:105，Run 恒经 WithTimeout 派生 ctx，唯一调用点必带 deadline）。
//
// 已知生产缺陷（只加测试不改行为）：IPv6 回声应答的 received.Type 动态
// 类型为 ipv6.ICMPType，与 replyType（ipv4.ICMPType 转换值）的 ==
// 比较按动态类型判定永假——正确的 IPv6 应答也会走「network: ICMP …」
// 错误分支（TestICMPv6EchoReplyDefectPinned 钉住现状，修复须用户指令）。

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

// fakePacketConn 可注入失败与回放报文的 PacketConn 替身。
type fakePacketConn struct {
	deadlineErr error
	writeErr    error
	writeHook   func(p []byte)
	readErr     error
	readBufs    [][]byte
	// blockUntilDeadline 为真时 ReadFrom 睡到 SetDeadline 记录的期限再
	// 返回超时（模拟真实 socket 行为，驱动 Run 的真到期聚合分支）。
	blockUntilDeadline bool
	deadline           time.Time
}

func (c *fakePacketConn) ReadFrom(p []byte) (int, net.Addr, error) {
	if c.blockUntilDeadline {
		if wait := time.Until(c.deadline); wait > 0 {
			time.Sleep(wait)
		}
		return 0, nil, os.ErrDeadlineExceeded
	}
	if len(c.readBufs) > 0 {
		b := c.readBufs[0]
		c.readBufs = c.readBufs[1:]
		return copy(p, b), &net.IPAddr{IP: net.IPv4(1, 2, 3, 4)}, nil
	}
	if c.readErr != nil {
		return 0, nil, c.readErr
	}
	return 0, nil, os.ErrDeadlineExceeded
}

func (c *fakePacketConn) WriteTo(p []byte, _ net.Addr) (int, error) {
	if c.writeErr != nil {
		return 0, c.writeErr
	}
	if c.writeHook != nil {
		c.writeHook(p)
	}
	return len(p), nil
}

func (c *fakePacketConn) Close() error                     { return nil }
func (c *fakePacketConn) LocalAddr() net.Addr              { return nil }
func (c *fakePacketConn) SetDeadline(d time.Time) error    { c.deadline = d; return c.deadlineErr }
func (c *fakePacketConn) SetReadDeadline(time.Time) error  { return nil }
func (c *fakePacketConn) SetWriteDeadline(time.Time) error { return nil }

// echoReplyFrom 从捕获的请求报文构造回声应答（保 ID/Seq/载荷）。
func echoReplyFrom(t *testing.T, proto int, req []byte, mutate func(*icmp.Echo)) []byte {
	t.Helper()
	msg, err := icmp.ParseMessage(proto, req)
	require.NoError(t, err)
	echo, ok := msg.Body.(*icmp.Echo)
	require.True(t, ok, "captured probe must be an echo request")
	if mutate != nil {
		mutate(echo)
	}
	reply := &icmp.Message{Type: msg.Type, Code: 0, Body: echo}
	if proto == 58 {
		reply.Type = ipv6.ICMPTypeEchoReply
	} else {
		reply.Type = ipv4.ICMPTypeEchoReply
	}
	out, err := reply.Marshal(nil)
	require.NoError(t, err)
	return out
}

func TestCapabilityErrorVariants(t *testing.T) {
	capErr := &CapabilityError{Capability: "CAP_NET_RAW"}
	require.Equal(t, "capability: CAP_NET_RAW is required for ICMP", capErr.Error())
	require.Nil(t, capErr.Unwrap())

	wrapped := &CapabilityError{Capability: "CAP_NET_RAW", Err: os.ErrPermission}
	require.Contains(t, wrapped.Error(), "capability: CAP_NET_RAW is required for ICMP:")
	require.True(t, errors.Is(wrapped.Unwrap(), os.ErrPermission))
}

func TestClassifyICMPErrorFamilies(t *testing.T) {
	require.Contains(t, classifyICMPError(fmt.Errorf("socket: %w", os.ErrPermission)),
		"capability: CAP_NET_RAW is required for ICMP")
	require.Contains(t, classifyICMPError(errors.New("lookup xyz: no such host")), "dns: ")
	require.Contains(t, classifyICMPError(errors.New("connection refused")), "network: ")
}

func TestICMPRunResolveError(t *testing.T) {
	// 非法字面量（256 溢出八位组）→ 解析必失败（空串其实可解析为 nil）
	r := NewICMPProbe("256.256.256.256", time.Second, 1).Run(context.Background())
	require.False(t, r.OK)
	require.Contains(t, r.Error, "dns: ")
}

func TestNewICMPProbeDefaults(t *testing.T) {
	// 零值兜底：count/timeout 非正时取 1 次 / 1s
	p := NewICMPProbe("127.0.0.1", 0, 0)
	require.Equal(t, 1, p.Count)
	require.Equal(t, time.Second, p.Timeout)
}

func TestICMPRunAggregateTimeoutAtDeadline(t *testing.T) {
	// ReadFrom 真睡到期限 → ctx 实际到期 → Run 聚合层 timeout 文案
	// （即时返回超时错误的路径走的是 lastErr 分支，文案相同）
	p := NewICMPProbe("127.0.0.1", 100*time.Millisecond, 1)
	p.dial = func(string, string) (net.PacketConn, error) {
		return &fakePacketConn{blockUntilDeadline: true}, nil
	}
	r := p.Run(context.Background())
	require.False(t, r.OK)
	require.Contains(t, r.Error, "timeout: no ICMP reply within 100ms")
}

func TestICMPRunDialPermissionError(t *testing.T) {
	p := NewICMPProbe("127.0.0.1", time.Second, 1)
	p.dial = func(string, string) (net.PacketConn, error) {
		return nil, fmt.Errorf("socket: %w", os.ErrPermission)
	}
	r := p.Run(context.Background())
	require.False(t, r.OK)
	require.Contains(t, r.Error, "capability: CAP_NET_RAW is required for ICMP")
}

func TestICMPRunRetryExhaustedKeepsLastError(t *testing.T) {
	// 重试耗尽且 ctx 未到期 → 聚合 lastErr（非 timeout/cancelled 分支）
	p := NewICMPProbe("127.0.0.1", time.Second, 2)
	p.dial = func(string, string) (net.PacketConn, error) {
		return nil, errors.New("dial: connection refused")
	}
	r := p.Run(context.Background())
	require.False(t, r.OK)
	require.Contains(t, r.Error, "network: dial: connection refused")
}

func TestICMPRunContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p := NewICMPProbe("127.0.0.1", time.Second, 3)
	p.dial = func(string, string) (net.PacketConn, error) {
		return &fakePacketConn{}, nil
	}
	r := p.Run(ctx)
	require.False(t, r.OK)
	require.Contains(t, r.Error, "cancelled: ")
}

func TestICMPRunSuccessWithFakeConn(t *testing.T) {
	conn := &fakePacketConn{}
	conn.writeHook = func(p []byte) {
		conn.readBufs = append(conn.readBufs, echoReplyFrom(t, 1, p, nil))
	}
	p := NewICMPProbe("127.0.0.1", time.Second, 1)
	p.dial = func(string, string) (net.PacketConn, error) { return conn, nil }

	r := p.Run(context.Background())
	require.True(t, r.OK)
	require.Equal(t, 200, r.Status)
	require.Empty(t, r.Error)
}

func TestICMPRunIgnoresForeignReplies(t *testing.T) {
	// 回放序列：坏包 → 非 Echo 主体 → ID 不符 → 载荷不符 → 正确应答
	conn := &fakePacketConn{}
	conn.writeHook = func(p []byte) {
		reply, err := icmp.ParseMessage(1, p)
		require.NoError(t, err)
		echo := reply.Body.(*icmp.Echo)

		garbage := []byte{0xFF, 0x01, 0x02}
		dstUnreach, err := (&icmp.Message{
			Type: ipv4.ICMPTypeDestinationUnreachable, Code: 1,
			Body: &icmp.DstUnreach{Data: make([]byte, 8)},
		}).Marshal(nil)
		require.NoError(t, err)
		wrongID, err := (&icmp.Message{
			Type: ipv4.ICMPTypeEchoReply, Code: 0,
			Body: &icmp.Echo{ID: echo.ID + 1, Seq: echo.Seq, Data: echo.Data},
		}).Marshal(nil)
		require.NoError(t, err)
		wrongPayload, err := (&icmp.Message{
			Type: ipv4.ICMPTypeEchoReply, Code: 0,
			Body: &icmp.Echo{ID: echo.ID, Seq: echo.Seq, Data: make([]byte, 8)},
		}).Marshal(nil)
		require.NoError(t, err)
		conn.readBufs = append(conn.readBufs, garbage, dstUnreach, wrongID, wrongPayload,
			echoReplyFrom(t, 1, p, nil))
	}
	p := NewICMPProbe("127.0.0.1", time.Second, 1)
	p.dial = func(string, string) (net.PacketConn, error) { return conn, nil }

	r := p.Run(context.Background())
	require.True(t, r.OK, "foreign replies must be skipped, not fatal")
}

func TestICMPRunEchoRequestAsReplyErrors(t *testing.T) {
	// Echo 主体三元组全匹配但类型是请求不是应答 → 报 ICMP 类型错误
	conn := &fakePacketConn{}
	conn.writeHook = func(p []byte) {
		conn.readBufs = append(conn.readBufs, p) // 原样回显（请求类型）
	}
	p := NewICMPProbe("127.0.0.1", time.Second, 1)
	p.dial = func(string, string) (net.PacketConn, error) { return conn, nil }

	r := p.Run(context.Background())
	require.False(t, r.OK)
	require.Contains(t, r.Error, "network: ICMP echo code 0")
}

func TestICMPRunPingOnceFailureFamilies(t *testing.T) {
	newProbe := func(conn net.PacketConn) *ICMPProbe {
		p := NewICMPProbe("127.0.0.1", time.Second, 1)
		p.dial = func(string, string) (net.PacketConn, error) { return conn, nil }
		return p
	}

	// SetDeadline 失败
	p := newProbe(&fakePacketConn{deadlineErr: errors.New("fd gone")})
	r := p.Run(context.Background())
	require.Contains(t, r.Error, "network: fd gone")

	// WriteTo 失败
	p = newProbe(&fakePacketConn{writeErr: errors.New("send refused")})
	r = p.Run(context.Background())
	require.Contains(t, r.Error, "network: send send refused")

	// ReadFrom 一般错误
	p = newProbe(&fakePacketConn{readErr: errors.New("reset by peer")})
	r = p.Run(context.Background())
	require.Contains(t, r.Error, "network: reset by peer")

	// ReadFrom 超时（pingOnce 超时 + Run 聚合 timeout 文案）
	p = newProbe(&fakePacketConn{})
	r = p.Run(context.Background())
	require.Contains(t, r.Error, "timeout: no ICMP reply within ")
}

func TestICMPv6EchoReplyDefectPinned(t *testing.T) {
	// 缺陷钉住（只加测试不改行为）：IPv6 正确应答的类型比较按动态类型
	// 判定（ipv6.ICMPType vs ipv4.ICMPType）永假 → 正确应答也被判错误。
	conn := &fakePacketConn{}
	conn.writeHook = func(p []byte) {
		conn.readBufs = append(conn.readBufs, echoReplyFrom(t, 58, p, nil))
	}
	p := NewICMPProbe("::1", time.Second, 1)
	var dialedNetwork string
	p.dial = func(network, address string) (net.PacketConn, error) {
		dialedNetwork = network
		return conn, nil
	}

	r := p.Run(context.Background())
	require.Equal(t, "ip6:ipv6-icmp", dialedNetwork)
	require.False(t, r.OK, "known defect: correct IPv6 echo reply is rejected")
	require.Contains(t, r.Error, "network: ICMP echo reply code 0")
}
