package probe

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

type icmpDialer func(network, address string) (net.PacketConn, error)

var icmpSequence atomic.Uint32

type ICMPProbe struct {
	Host    string
	Timeout time.Duration
	Count   int

	dial icmpDialer
}

func NewICMPProbe(host string, timeout time.Duration, count int) *ICMPProbe {
	if count <= 0 {
		count = 1
	}
	if timeout <= 0 {
		timeout = time.Second
	}
	return &ICMPProbe{Host: host, Timeout: timeout, Count: count, dial: net.ListenPacket}
}

func (p *ICMPProbe) Run(ctx context.Context) Result {
	result := Result{Target: p.Host, At: time.Now()}
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, p.Timeout)
	defer cancel()

	ip, err := net.ResolveIPAddr("ip", strings.TrimSpace(p.Host))
	if err != nil {
		result.Latency = time.Since(start)
		result.Error = classifyICMPError(err)
		return result
	}

	var lastErr error
	for attempt := 0; attempt < p.Count && ctx.Err() == nil; attempt++ {
		lastErr = p.pingOnce(ctx, ip)
		if lastErr == nil {
			result.OK = true
			result.Status = 200
			result.Latency = time.Since(start)
			return result
		}
	}
	result.Latency = time.Since(start)
	if ctx.Err() != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		result.Error = "timeout: no ICMP reply within " + p.Timeout.String()
	} else if ctx.Err() != nil {
		result.Error = "cancelled: " + ctx.Err().Error()
	} else {
		result.Error = lastErr.Error()
	}
	return result
}

func (p *ICMPProbe) pingOnce(ctx context.Context, ip *net.IPAddr) error {
	network, protocol := "ip4:icmp", 1
	requestType, replyType := ipv4.ICMPTypeEcho, ipv4.ICMPTypeEchoReply
	if ip.IP.To4() == nil {
		network, protocol = "ip6:ipv6-icmp", 58
		requestType, replyType = ipv4.ICMPType(ipv6.ICMPTypeEchoRequest), ipv4.ICMPType(ipv6.ICMPTypeEchoReply)
	}
	address := "0.0.0.0"
	if ip.IP.To4() == nil {
		address = "::"
	}

	conn, err := p.dial(network, address)
	if err != nil {
		return errors.New(classifyICMPError(err))
	}
	defer conn.Close()

	id := os.Getpid() & 0xffff
	seq := int(icmpSequence.Add(1) & 0xffff)
	payload := make([]byte, 8)
	_, _ = rand.Read(payload)
	message := icmp.Message{Type: requestType, Body: &icmp.Echo{ID: id, Seq: seq, Data: payload}}
	body, err := message.Marshal(nil)
	if err != nil {
		return fmt.Errorf("configuration: %w", err)
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(p.Timeout)
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return fmt.Errorf("network: %w", err)
	}
	if _, err := conn.WriteTo(body, ip); err != nil {
		return fmt.Errorf("network: send %w", err)
	}

	buffer := make([]byte, 1500)
	for {
		n, _, err := conn.ReadFrom(buffer)
		if err != nil {
			if errors.Is(err, os.ErrDeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return fmt.Errorf("timeout: no ICMP reply within %s", p.Timeout)
			}
			return fmt.Errorf("network: %w", err)
		}
		received, err := icmp.ParseMessage(protocol, buffer[:n])
		if err != nil {
			continue
		}
		echo, ok := received.Body.(*icmp.Echo)
		if !ok || echo.ID != id || echo.Seq != seq || binary.NativeEndian.Uint64(echo.Data) != binary.NativeEndian.Uint64(payload) {
			continue
		}
		if received.Type == replyType {
			return nil
		}
		return fmt.Errorf("network: ICMP %v code %d", received.Type, received.Code)
	}
}

func classifyICMPError(err error) string {
	if errors.Is(err, os.ErrPermission) {
		return (&CapabilityError{Capability: "CAP_NET_RAW", Err: err}).Error()
	}
	if strings.Contains(strings.ToLower(err.Error()), "no such host") {
		return "dns: " + err.Error()
	}
	return "network: " + err.Error()
}
