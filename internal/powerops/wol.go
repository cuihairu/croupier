package powerops

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

// wolPort is the conventional WOL UDP port (design §3).
const wolPort = 9

// MagicPacket builds the WOL magic packet: six 0xFF bytes followed by the
// target MAC repeated sixteen times.
func MagicPacket(mac string) ([]byte, error) {
	hw, err := parseMAC(mac)
	if err != nil {
		return nil, err
	}
	pkt := make([]byte, 6+16*len(hw))
	for i := 0; i < 6; i++ {
		pkt[i] = 0xFF
	}
	for i := 0; i < 16; i++ {
		copy(pkt[6+i*len(hw):], hw)
	}
	return pkt, nil
}

func parseMAC(mac string) ([]byte, error) {
	mac = strings.TrimSpace(mac)
	if mac == "" {
		return nil, fmt.Errorf("powerops: wol machine missing mac")
	}
	for _, sep := range []string{":", "-"} {
		if strings.Contains(mac, sep) {
			parts := strings.Split(mac, sep)
			if len(parts) != 6 {
				return nil, fmt.Errorf("powerops: malformed mac %q", mac)
			}
			hw := make([]byte, 6)
			for i, p := range parts {
				v, err := parseHexByte(p)
				if err != nil {
					return nil, fmt.Errorf("powerops: malformed mac %q", mac)
				}
				hw[i] = v
			}
			return hw, nil
		}
	}
	return nil, fmt.Errorf("powerops: malformed mac %q", mac)
}

func parseHexByte(s string) (byte, error) {
	if len(s) != 2 {
		return 0, fmt.Errorf("bad hex byte %q", s)
	}
	hi, ok1 := hexNibble(s[0])
	lo, ok2 := hexNibble(s[1])
	if !ok1 || !ok2 {
		return 0, fmt.Errorf("bad hex byte %q", s)
	}
	return hi<<4 | lo, nil
}

func hexNibble(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	default:
		return 0, false
	}
}

// WOLProvider sends Wake-on-LAN magic packets. WOL is wake-only: PowerOff /
// Reboot / Status return ErrUnsupportedAction (design §3 — the protocol
// cannot express them), and wake confirmation is agent reconnection handled
// by the verify loop, not by this provider.
type WOLProvider struct {
	// Send is injectable for tests; the default UDP-sends one packet.
	Send func(ctx context.Context, addr string, packet []byte) error
	// DefaultAddr is used when the machine carries no BmcAddr (broadcast
	// target, "ip:port"; default "255.255.255.255:9").
	DefaultAddr string
}

// NewWOLProvider wires the default UDP sender.
func NewWOLProvider() *WOLProvider {
	return &WOLProvider{Send: udpSend, DefaultAddr: fmt.Sprintf("255.255.255.255:%d", wolPort)}
}

func udpSend(ctx context.Context, addr string, packet []byte) error {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "udp", addr)
	if err != nil {
		return fmt.Errorf("powerops: wol dial %s: %w", addr, err)
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	}
	if _, err := conn.Write(packet); err != nil {
		return fmt.Errorf("powerops: wol write %s: %w", addr, err)
	}
	return nil
}

func (p *WOLProvider) Kind() string { return ProviderWOL }

func (p *WOLProvider) target(m Machine) string {
	if m.BmcAddr != "" {
		if m.BmcPort > 0 {
			return fmt.Sprintf("%s:%d", m.BmcAddr, m.BmcPort)
		}
		return m.BmcAddr
	}
	return p.DefaultAddr
}

// PowerOn sends the magic packet; success means the packet left, not that
// the machine woke (verification is the verify loop's job).
func (p *WOLProvider) PowerOn(ctx context.Context, m Machine) (string, error) {
	pkt, err := MagicPacket(m.MAC)
	if err != nil {
		return "", err
	}
	send := p.Send
	if send == nil {
		send = udpSend
	}
	addr := p.target(m)
	if err := send(ctx, addr, pkt); err != nil {
		return "", err
	}
	return "magic packet sent to " + addr, nil
}

// PowerOff is not expressible over WOL.
func (p *WOLProvider) PowerOff(ctx context.Context, m Machine, force bool) (string, error) {
	return "", ErrUnsupportedAction
}

// Reboot is not expressible over WOL.
func (p *WOLProvider) Reboot(ctx context.Context, m Machine) (string, error) {
	return "", ErrUnsupportedAction
}

// Status cannot be observed over WOL.
func (p *WOLProvider) Status(ctx context.Context, m Machine) (string, error) {
	return "unknown", ErrUnknownMachineState
}
