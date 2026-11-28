//go:build !no_go_plugin

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/synthetics/pluginhost"
)

const mockStatusJSON = `{"version":{"name":"mcslp-mock-1.0","protocol":769},` +
	`"players":{"online":3,"max":20},"description":{"text":"unit mock"}}`

// startMockSLP a minimal speaking SLP server: reads two varint-framed
// packets (handshake, status request) then answers with the canned
// status response frame.
func startMockSLP(t *testing.T, statusJSON string) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("mock SLP listen: %v", err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer func() { _ = c.Close() }()
				if _, _, err := slpReadFrame(c); err != nil {
					return
				}
				if _, _, err := slpReadFrame(c); err != nil {
					return
				}
				data := []byte{0x00}
				data = appendVarInt(data, uint32(len(statusJSON)))
				data = append(data, statusJSON...)
				frame := appendVarInt(nil, uint32(len(data)))
				frame = append(frame, data...)
				_, _ = c.Write(frame)
			}(conn)
		}
	}()
	return ln.Addr().String(), func() { _ = ln.Close() }
}

// startSilentTCP accepts and then never speaks: the plugin must report a
// business timeout failure, not a host degradation.
func startSilentTCP(t *testing.T) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("silent listen: %v", err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) { time.Sleep(time.Minute); _ = c.Close() }(conn)
		}
	}()
	return ln.Addr().String(), func() { _ = ln.Close() }
}

func TestMCSLPPluginDescribe(t *testing.T) {
	p := &mcslpProbe{}
	name, version, err := p.Describe(context.Background())
	if err != nil {
		t.Fatalf("describe: %v", err)
	}
	if name != "probe-mc-slp" || version == "" {
		t.Fatalf("describe: name=%q version=%q", name, version)
	}
}

func TestMCSLPPluginCheckAgainstMockServer(t *testing.T) {
	addr, stop := startMockSLP(t, mockStatusJSON)
	defer stop()
	host, portStr, _ := net.SplitHostPort(addr)
	var port int
	_, _ = fmt.Sscanf(portStr, "%d", &port)

	p := &mcslpProbe{}
	out, err := p.Check(context.Background(), pluginhost.CheckInput{
		Target:      "mc-mock",
		Timeout:     5 * time.Second,
		PayloadJSON: fmt.Sprintf(`{"host":%q,"port":%d}`, host, port),
	})
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !out.OK || out.Status != 200 {
		t.Fatalf("check: ok=%v status=%d error=%q", out.OK, out.Status, out.Error)
	}
	for _, want := range []string{"version=mcslp-mock-1.0", "protocol=769", "players=3/20"} {
		if !strings.Contains(out.Detail, want) {
			t.Fatalf("detail %q missing %q", out.Detail, want)
		}
	}
}

func TestMCSLPPluginConfigurationErrors(t *testing.T) {
	p := &mcslpProbe{}
	cases := []struct {
		name    string
		payload string
	}{
		{"missing host", `{"port":25565}`},
		{"bad json", `{`},
		{"port out of range", `{"host":"127.0.0.1","port":70000}`},
	}
	for _, tc := range cases {
		out, err := p.Check(context.Background(), pluginhost.CheckInput{
			Target: "cfg", Timeout: time.Second, PayloadJSON: tc.payload,
		})
		if err != nil {
			t.Fatalf("%s: check returned error %v", tc.name, err)
		}
		if out.OK {
			t.Fatalf("%s: configuration error must not pass", tc.name)
		}
		if !strings.HasPrefix(out.Error, "configuration:") {
			t.Fatalf("%s: error %q is not a configuration error", tc.name, out.Error)
		}
	}
}

func TestMCSLPPluginSilentTargetIsBusinessFailure(t *testing.T) {
	addr, stop := startSilentTCP(t)
	defer stop()
	host, portStr, _ := net.SplitHostPort(addr)
	var port int
	_, _ = fmt.Sscanf(portStr, "%d", &port)

	p := &mcslpProbe{}
	out, err := p.Check(context.Background(), pluginhost.CheckInput{
		Target:      "mc-silent",
		Timeout:     300 * time.Millisecond,
		PayloadJSON: fmt.Sprintf(`{"host":%q,"port":%d}`, host, port),
	})
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if out.OK {
		t.Fatal("silent non-SLP target must not pass")
	}
	if !strings.Contains(out.Error, "minecraft slp:") {
		t.Fatalf("error %q lacks the slp prefix", out.Error)
	}
}

// ---- minimal SLP framing (mirrors core/healthprobe/games/minecraft) ----

func appendVarInt(b []byte, v uint32) []byte {
	for {
		if v&^0x7F == 0 {
			return append(b, byte(v))
		}
		b = append(b, byte(v&0x7F)|0x80)
		v >>= 7
	}
}

func slpReadVarInt(r *bytes.Reader) (uint32, error) {
	var result uint32
	for i := 0; i < 5; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		result |= uint32(b&0x7F) << (7 * i)
		if b&0x80 == 0 {
			return result, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

func slpReadFrame(c net.Conn) (byte, []byte, error) {
	var lenBuf []byte
	one := make([]byte, 1)
	for i := 0; i < 5; i++ {
		if _, err := io.ReadFull(c, one); err != nil {
			return 0, nil, err
		}
		lenBuf = append(lenBuf, one[0])
		if one[0]&0x80 == 0 {
			break
		}
	}
	length, err := slpReadVarInt(bytes.NewReader(lenBuf))
	if err != nil {
		return 0, nil, err
	}
	if length == 0 || length > 64*1024 {
		return 0, nil, fmt.Errorf("bad frame length %d", length)
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(c, payload); err != nil {
		return 0, nil, err
	}
	return payload[0], payload[1:], nil
}
