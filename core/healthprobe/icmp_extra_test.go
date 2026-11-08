package healthprobe

// icmp.go 补测：经 open seam 注入 fake PacketConn 驱动 pingOnce 全生命周期
// （回声应答 v4/v6 成功、非回声/错序号/坏载荷/非应答类型逐跳跳过、写失败、
// SetDeadline 失败、读错误、无回包超时、重试次数），listen 错误分类
// （CAP_NET_RAW CapabilityError / 普通网络错误）、Check 守卫（空地址/DNS
// 失败/取消/超时）与 CapabilityError 文案面。不依赖真实 raw socket。

import (
	"context"
	"errors"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

// fakeICMPConn net.PacketConn 替身：WriteTo 记录请求，ReadFrom 按 reply 闭包
// 动态生成回包（nil 回包 = 无应答，睡到 deadline 后报超时）。
type fakeICMPConn struct {
	reply       func(req []byte) ([]byte, error)
	writeErr    error
	deadlineErr error

	mu       sync.Mutex
	deadline time.Time
	written  [][]byte
	reads    int
}

func (c *fakeICMPConn) WriteTo(p []byte, _ net.Addr) (int, error) {
	if c.writeErr != nil {
		return 0, c.writeErr
	}
	c.mu.Lock()
	c.written = append(c.written, append([]byte(nil), p...))
	c.mu.Unlock()
	return len(p), nil
}

func (c *fakeICMPConn) ReadFrom(p []byte) (int, net.Addr, error) {
	c.mu.Lock()
	c.reads++
	c.mu.Unlock()
	if c.reply != nil {
		c.mu.Lock()
		var req []byte
		if n := len(c.written); n > 0 {
			req = c.written[n-1]
		}
		c.mu.Unlock()
		reply, err := c.reply(req)
		if err != nil {
			return 0, nil, err
		}
		if reply != nil {
			return copy(p, reply), &net.IPAddr{IP: net.IPv4(127, 0, 0, 1)}, nil
		}
	}
	c.mu.Lock()
	until := time.Until(c.deadline)
	c.mu.Unlock()
	if until > 0 {
		time.Sleep(until)
	}
	return 0, nil, os.ErrDeadlineExceeded
}

func (c *fakeICMPConn) SetDeadline(t time.Time) error {
	if c.deadlineErr != nil {
		return c.deadlineErr
	}
	c.mu.Lock()
	c.deadline = t
	c.mu.Unlock()
	return nil
}

func (c *fakeICMPConn) SetReadDeadline(t time.Time) error { return c.SetDeadline(t) }
func (c *fakeICMPConn) SetWriteDeadline(time.Time) error  { return nil }
func (c *fakeICMPConn) LocalAddr() net.Addr               { return &net.IPAddr{IP: net.IPv4zero} }
func (c *fakeICMPConn) Close() error                      { return nil }

func (c *fakeICMPConn) readCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.reads
}

func icmpProtoFor(ip string) int {
	if net.ParseIP(ip).To4() != nil {
		return icmpProtocolIPv4
	}
	return icmpProtocolIPv6
}

// echoReplyOf 从请求字节构造同 ID/Seq/载荷的回声应答；mutate 可改字段造
// 错配回包。replyType 换成请求类型可造「非应答类型」分支。
func echoReplyOf(t *testing.T, proto int, req []byte, mutate func(*icmp.Echo), replyType icmp.Type) []byte {
	t.Helper()
	msg, err := icmp.ParseMessage(proto, req)
	require.NoError(t, err)
	echo, ok := msg.Body.(*icmp.Echo)
	require.True(t, ok, "request body must be echo")
	if mutate != nil {
		mutate(echo)
	}
	reply := icmp.Message{Type: replyType, Body: echo}
	out, err := reply.Marshal(nil)
	require.NoError(t, err)
	return out
}

func newStubbedICMP(t *testing.T, address string, conn *fakeICMPConn, count int) *ICMPChecker {
	t.Helper()
	c := NewICMPChecker(address, 200*time.Millisecond, count)
	c.open = func(string, string) (net.PacketConn, error) { return conn, nil }
	return c
}

func TestICMPPingOnceEchoReplySuccess(t *testing.T) {
	ctx := context.Background()

	// IPv4 回声应答 → 通过
	c := newStubbedICMP(t, "127.0.0.1", &fakeICMPConn{
		reply: func(req []byte) ([]byte, error) {
			return echoReplyOf(t, icmpProtocolIPv4, req, nil, ipv4.ICMPTypeEchoReply), nil
		},
	}, 1)
	require.NoError(t, c.Check(ctx))

	// IPv6：open 参数走 ip6 形态。注意（生产缺陷，已录 todo 续批记录）：
	// pingOnce 的 replyType 为 ipv4.ICMPType(ipv6.ICMPTypeEchoReply)，而
	// ParseMessage(proto 58) 产出 ipv6.ICMPType 动态类型，接口比较恒不等 →
	// 正确的 IPv6 回声应答被误判为失败。此处断言现状；修复方向（replyType
	// 直接用 ipv6.ICMPType）待用户定夺，本批只加测试不改行为。
	var gotNetwork, gotAddr string
	c6 := NewICMPChecker("::1", 200*time.Millisecond, 1)
	c6.open = func(network, address string) (net.PacketConn, error) {
		gotNetwork, gotAddr = network, address
		return &fakeICMPConn{
			reply: func(req []byte) ([]byte, error) {
				return echoReplyOf(t, icmpProtocolIPv6, req, nil, ipv6.ICMPTypeEchoReply), nil
			},
		}, nil
	}
	err6 := c6.Check(ctx)
	require.ErrorContains(t, err6, "network: ICMP echo reply code 0")
	require.Equal(t, "ip6:ipv6-icmp", gotNetwork)
	require.Equal(t, "::", gotAddr)
}

func TestICMPPingOnceRejectsMismatchedReplies(t *testing.T) {
	ctx := context.Background()

	// 序号错配 → 跳过 → 超时
	c := newStubbedICMP(t, "127.0.0.1", &fakeICMPConn{
		reply: func(req []byte) ([]byte, error) {
			return echoReplyOf(t, icmpProtocolIPv4, req, func(e *icmp.Echo) { e.Seq++ }, ipv4.ICMPTypeEchoReply), nil
		},
	}, 1)
	require.ErrorContains(t, c.Check(ctx), "timeout: no ICMP reply")

	// 载荷错配 → 跳过 → 超时
	c = newStubbedICMP(t, "127.0.0.1", &fakeICMPConn{
		reply: func(req []byte) ([]byte, error) {
			return echoReplyOf(t, icmpProtocolIPv4, req, func(e *icmp.Echo) {
				e.Data = []byte("tail-padding-bytes")
			}, ipv4.ICMPTypeEchoReply), nil
		},
	}, 1)
	require.ErrorContains(t, c.Check(ctx), "timeout: no ICMP reply")

	// 非回声体 → 跳过 → 超时
	c = newStubbedICMP(t, "127.0.0.1", &fakeICMPConn{
		reply: func([]byte) ([]byte, error) {
			msg := icmp.Message{
				Type: ipv4.ICMPTypeDestinationUnreachable,
				Body: &icmp.DstUnreach{Data: []byte("x")},
			}
			out, err := msg.Marshal(nil)
			require.NoError(t, err)
			return out, nil
		},
	}, 1)
	require.ErrorContains(t, c.Check(ctx), "timeout: no ICMP reply")

	// 坏报文（解析失败）→ 跳过 → 超时
	c = newStubbedICMP(t, "127.0.0.1", &fakeICMPConn{
		reply: func([]byte) ([]byte, error) { return []byte{0x21, 0x03, 0x00}, nil },
	}, 1)
	require.ErrorContains(t, c.Check(ctx), "timeout: no ICMP reply")

	// 回的是请求类型（非应答类型）→ 显式报 ICMP 类型/码
	c = newStubbedICMP(t, "127.0.0.1", &fakeICMPConn{
		reply: func(req []byte) ([]byte, error) {
			return echoReplyOf(t, icmpProtocolIPv4, req, nil, ipv4.ICMPTypeEcho), nil
		},
	}, 1)
	err := c.Check(ctx)
	require.ErrorContains(t, err, "network: ICMP")
	require.ErrorContains(t, err, "code 0")
}

func TestICMPPingOnceNetworkFaults(t *testing.T) {
	ctx := context.Background()

	// 写失败
	c := newStubbedICMP(t, "127.0.0.1", &fakeICMPConn{writeErr: errors.New("broken pipe")}, 1)
	require.ErrorContains(t, c.Check(ctx), "network: send")

	// SetDeadline 失败
	c = newStubbedICMP(t, "127.0.0.1", &fakeICMPConn{deadlineErr: errors.New("bad fd")}, 1)
	require.ErrorContains(t, c.Check(ctx), "network:")

	// 读错误（非超时类）
	c = newStubbedICMP(t, "127.0.0.1", &fakeICMPConn{
		reply: func([]byte) ([]byte, error) { return nil, errors.New("conn reset") },
	}, 1)
	require.ErrorContains(t, c.Check(ctx), "network:")

	// 无回包（睡到 deadline）→ 超时
	c = newStubbedICMP(t, "127.0.0.1", &fakeICMPConn{}, 1)
	require.ErrorContains(t, c.Check(ctx), "timeout: no ICMP reply within")
}

func TestICMPCheckRetriesEachAttempt(t *testing.T) {
	// 每次尝试快速失败（读错误）→ Count=3 跑满三轮；末次错误透传。
	// 注意共享 ctx 超时语义：单次尝试烧满超时的场景只会跑一轮。
	conn := &fakeICMPConn{
		reply: func([]byte) ([]byte, error) { return nil, errors.New("conn reset") },
	}
	c := newStubbedICMP(t, "127.0.0.1", conn, 3)
	err := c.Check(context.Background())
	require.ErrorContains(t, err, "conn reset")
	require.Equal(t, 3, conn.readCount())
}

func TestICMPListenErrorClassification(t *testing.T) {
	ctx := context.Background()

	// 权限不足 → CapabilityError(CAP_NET_RAW)
	c := NewICMPChecker("127.0.0.1", 200*time.Millisecond, 1)
	c.open = func(string, string) (net.PacketConn, error) {
		return nil, os.ErrPermission
	}
	err := c.Check(ctx)
	var capErr *CapabilityError
	require.True(t, errors.As(err, &capErr))
	require.Equal(t, "CAP_NET_RAW", capErr.Capability)
	require.ErrorIs(t, capErr, os.ErrPermission)
	require.Contains(t, capErr.Error(), "CAP_NET_RAW is required for ICMP")

	// 普通网络错误 → 原样包装
	c.open = func(string, string) (net.PacketConn, error) { return nil, errors.New("no such device") }
	require.ErrorContains(t, c.Check(ctx), "network: no such device")

	// CapabilityError 文案：无内因 / 有内因 / Unwrap
	bare := &CapabilityError{Capability: "CAP_NET_RAW"}
	require.Equal(t, "capability: CAP_NET_RAW is required for ICMP", bare.Error())
	require.Nil(t, bare.Unwrap())
	wrapped := &CapabilityError{Capability: "CAP_NET_RAW", Err: os.ErrPermission}
	require.Contains(t, wrapped.Error(), "permission denied")
	require.ErrorIs(t, wrapped.Unwrap(), os.ErrPermission)
}

func TestICMPCheckGuards(t *testing.T) {
	ctx := context.Background()

	// 空地址
	require.ErrorContains(t, NewICMPChecker("", time.Second, 1).Check(ctx), "address is required")

	// DNS 解析失败分支（.invalid 应 NXDOMAIN；若本环境 wildcard 解析成功则跳过）
	if _, err := net.ResolveIPAddr("ip", "no-such-host.invalid"); err != nil {
		require.ErrorContains(t, NewICMPChecker("no-such-host.invalid", time.Second, 1).Check(ctx), "dns:")
	}

	// 父 ctx 已取消 → cancelled 分支
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	require.ErrorContains(t, NewICMPChecker("127.0.0.1", time.Second, 1).Check(cancelled), "cancelled:")

	// 默认值归一
	c := NewICMPChecker("127.0.0.1", 0, 0)
	require.Equal(t, 2*time.Second, c.Timeout)
	require.Equal(t, 1, c.Count)

	// 本地监听地址分派
	require.Equal(t, "0.0.0.0", localICMPAddress(true))
	require.Equal(t, "::", localICMPAddress(false))
}
