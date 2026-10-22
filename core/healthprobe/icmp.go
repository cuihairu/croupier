package healthprobe

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const (
	icmpProtocolIPv4 = 1
	icmpProtocolIPv6 = 58
	icmpPayloadSize  = 8
)

type CapabilityError struct {
	Capability string
	Err        error
}

func (e *CapabilityError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("capability: %s is required for ICMP", e.Capability)
	}
	return fmt.Sprintf("capability: %s is required for ICMP: %v", e.Capability, e.Err)
}

func (e *CapabilityError) Unwrap() error { return e.Err }

type icmpConnOpener func(network, address string) (net.PacketConn, error)

var icmpSequence atomic.Uint32

type ICMPChecker struct {
	Address string
	Timeout time.Duration
	Count   int

	open icmpConnOpener
}

func NewICMPChecker(address string, timeout time.Duration, count int) *ICMPChecker {
	if count <= 0 {
		count = 1
	}
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return &ICMPChecker{Address: address, Timeout: timeout, Count: count, open: listenICMP}
}

func (c *ICMPChecker) Check(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()

	host, _, err := net.SplitHostPort(c.Address)
	if err != nil {
		host = c.Address
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return fmt.Errorf("configuration: address is required")
	}
	ip, err := net.ResolveIPAddr("ip", host)
	if err != nil {
		return fmt.Errorf("dns: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt < c.Count && ctx.Err() == nil; attempt++ {
		lastErr = c.pingOnce(ctx, ip)
		if lastErr == nil {
			return nil
		}
	}
	if ctx.Err() != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("timeout: no ICMP reply within %s", c.Timeout)
		}
		return fmt.Errorf("cancelled: %w", ctx.Err())
	}
	return lastErr
}

func (c *ICMPChecker) pingOnce(ctx context.Context, ip *net.IPAddr) error {
	network, protocol := "ip4:icmp", icmpProtocolIPv4
	requestType, replyType := ipv4.ICMPTypeEcho, ipv4.ICMPTypeEchoReply
	if ip.IP.To4() == nil {
		network, protocol = "ip6:ipv6-icmp", icmpProtocolIPv6
		requestType, replyType = ipv4.ICMPType(ipv6.ICMPTypeEchoRequest), ipv4.ICMPType(ipv6.ICMPTypeEchoReply)
	}

	conn, err := c.open(network, localICMPAddress(ip.IP.To4() != nil))
	if err != nil {
		return classifyICMPListenError(err)
	}
	defer conn.Close()

	id := os.Getpid() & 0xffff
	seq := int(icmpSequence.Add(1) & 0xffff)
	payload := make([]byte, icmpPayloadSize)
	_, _ = rand.Read(payload)
	message := icmp.Message{Type: requestType, Body: &icmp.Echo{ID: id, Seq: seq, Data: payload}}
	body, err := message.Marshal(nil)
	if err != nil {
		return fmt.Errorf("configuration: %w", err)
	}

	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(c.Timeout)
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return fmt.Errorf("network: %w", err)
	}
	if _, err := conn.WriteTo(body, ip); err != nil {
		return fmt.Errorf("network: send %w", err)
	}

	buffer := make([]byte, 1500)
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("timeout: no ICMP reply within %s", c.Timeout)
		}
		n, _, err := conn.ReadFrom(buffer)
		if err != nil {
			if errors.Is(err, os.ErrDeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return fmt.Errorf("timeout: no ICMP reply within %s", c.Timeout)
			}
			return fmt.Errorf("network: %w", err)
		}
		received, parseErr := icmp.ParseMessage(protocol, buffer[:n])
		if parseErr != nil {
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

func localICMPAddress(isIPv4 bool) string {
	if isIPv4 {
		return "0.0.0.0"
	}
	return "::"
}

func listenICMP(network, address string) (net.PacketConn, error) {
	return net.ListenPacket(network, address)
}

func classifyICMPListenError(err error) error {
	if errors.Is(err, os.ErrPermission) {
		return &CapabilityError{Capability: "CAP_NET_RAW", Err: err}
	}
	return fmt.Errorf("network: %w", err)
}
