//go:build !no_go_plugin

// Regression tests for the tcp reference adapter's line-protocol framing:
// one invoke writes exactly one request line, so caller-controlled path
// and body must never carry CR/LF onto the socket (CR/LF would splice
// extra commands into a line-protocol admin console — the injection this
// file pins shut).
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/sidecar/funcplugin"
)

// fakeGameInterface is a one-shot stand-in for the game's TCP admin
// console: it accepts one connection, records the request bytes, and
// answers with a single ok line.
type fakeGameInterface struct {
	listener net.Listener
	requests chan string
}

func newFakeGameInterface(t *testing.T) *fakeGameInterface {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	f := &fakeGameInterface{listener: ln, requests: make(chan string, 4)}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				line, err := bufio.NewReader(c).ReadString('\n')
				if err == nil || (err != nil && len(line) > 0) {
					f.requests <- line
				}
				_, _ = c.Write([]byte(`{"ok":true}` + "\n"))
				_ = c.Close()
			}(conn)
		}
	}()
	return f
}

func (f *fakeGameInterface) addr() string { return f.listener.Addr().String() }

// nextRequest reads one recorded request line; ok=false means nothing
// arrived within the window (the no-injection assertion for rejected
// frames).
func (f *fakeGameInterface) nextRequest(t *testing.T, want bool) (string, bool) {
	t.Helper()
	select {
	case line := <-f.requests:
		return line, true
	case <-time.After(300 * time.Millisecond):
		if want {
			t.Fatal("game interface received no request line")
		}
		return "", false
	}
}

func newTestPlugin(t *testing.T, target string) *tcpPlugin {
	t.Helper()
	return &tcpPlugin{target: target, targetTimeout: 5 * time.Second}
}

// TestInvokeRejectsPathLineBreaking pins the CR/LF injection guard: a path
// carrying a newline (or carriage return) is a frame splice, so Invoke must
// fail as bad_response BEFORE any socket work — the game interface must
// observe zero bytes.
func TestInvokeRejectsPathLineBreaking(t *testing.T) {
	f := newFakeGameInterface(t)
	p := newTestPlugin(t, f.addr())

	for _, path := range []string{
		"ping\nADMIN grant hacker",
		"ping\r\nADMIN grant hacker",
		"ping\r x",
	} {
		out, err := p.Invoke(context.Background(), funcplugin.InvokeInput{
			FunctionID: pluginNamespace + defaultFunction,
			ParamsJSON: `{"path":` + mustQuote(t, path) + `}`,
			Timeout:    2 * time.Second,
		})
		if err != nil {
			t.Fatalf("Invoke(%q) returned Go error: %v", path, err)
		}
		if out.OK {
			t.Fatalf("Invoke(%q) reported success, want bad_response", path)
		}
		if out.ErrorCode != "bad_response" {
			t.Fatalf("Invoke(%q) error code = %q, want bad_response", path, out.ErrorCode)
		}
		if line, arrived := f.nextRequest(t, false); arrived {
			t.Fatalf("Invoke(%q) reached the wire: %q — injection leaked through", path, line)
		}
	}
}

// TestInvokeRejectsEmptyPath pins the pre-existing empty-path guard along
// the same no-socket-work contract.
func TestInvokeRejectsEmptyPath(t *testing.T) {
	f := newFakeGameInterface(t)
	p := newTestPlugin(t, f.addr())

	out, err := p.Invoke(context.Background(), funcplugin.InvokeInput{
		FunctionID: pluginNamespace + defaultFunction,
		ParamsJSON: `{"path":""}`,
		Timeout:    2 * time.Second,
	})
	if err != nil {
		t.Fatalf("Invoke returned Go error: %v", err)
	}
	if out.OK || out.ErrorCode != "bad_response" {
		t.Fatalf("empty path: ok=%v code=%q, want bad_response", out.OK, out.ErrorCode)
	}
	if _, arrived := f.nextRequest(t, false); arrived {
		t.Fatal("empty path still reached the wire")
	}
}

// TestInvokeCompactsPrettyBody pins the one-line body contract: a
// pretty-printed JSON body (newlines between tokens) must go out as one
// compact line, not as a multi-line frame.
func TestInvokeCompactsPrettyBody(t *testing.T) {
	f := newFakeGameInterface(t)
	p := newTestPlugin(t, f.addr())

	out, err := p.Invoke(context.Background(), funcplugin.InvokeInput{
		FunctionID: pluginNamespace + defaultFunction,
		ParamsJSON: `{"path":"kick","body":{ "player" : "p-1" , "reason" : "afk x" }}`,
		Timeout:    2 * time.Second,
	})
	if err != nil {
		t.Fatalf("Invoke returned Go error: %v", err)
	}
	if !out.OK {
		t.Fatalf("Invoke failed: %s %s", out.ErrorCode, out.ErrorMessage)
	}
	line, _ := f.nextRequest(t, true)
	want := "kick {\"player\":\"p-1\",\"reason\":\"afk x\"}\n"
	if line != want {
		t.Fatalf("wire frame = %q, want %q", line, want)
	}
	if !strings.HasPrefix(out.ResultJSON, `{"path":"kick"`) {
		t.Fatalf("result path echo missing: %q", out.ResultJSON)
	}
}

// TestInvokePlainPathFrame pins the happy-path frame shape: path only, one
// trailing newline, nothing else.
func TestInvokePlainPathFrame(t *testing.T) {
	f := newFakeGameInterface(t)
	p := newTestPlugin(t, f.addr())

	out, err := p.Invoke(context.Background(), funcplugin.InvokeInput{
		FunctionID: pluginNamespace + defaultFunction,
		ParamsJSON: `{"path":"ping"}`,
		Timeout:    2 * time.Second,
	})
	if err != nil {
		t.Fatalf("Invoke returned Go error: %v", err)
	}
	if !out.OK {
		t.Fatalf("Invoke failed: %s %s", out.ErrorCode, out.ErrorMessage)
	}
	line, _ := f.nextRequest(t, true)
	if line != "ping\n" {
		t.Fatalf("wire frame = %q, want %q", line, "ping\n")
	}
}

// mustQuote JSON-encodes s for embedding into a ParamsJSON literal.
func mustQuote(t *testing.T, s string) string {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("quote %q: %v", s, err)
	}
	return string(b)
}
