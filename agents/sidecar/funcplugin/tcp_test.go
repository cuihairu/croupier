//go:build !no_go_plugin

package funcplugin

import (
	"bufio"
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// targetTimeout is the adapter's own reply wait for tests; it stays well
// below the 10s plugin Timeout unless a test overrides it, which is what
// lets a wedged target surface as a business target_unavailable instead of
// a host-level plugin_timeout.
const targetTimeout = 5 * time.Second

func testTCPHost(t *testing.T, target string, timeout time.Duration, mutate func(*[]PluginConfig)) *Host {
	t.Helper()
	plugins := []PluginConfig{{
		Name:    "tcp",
		Command: tcpBin,
		Args:    []string{"-target", target, "-target-timeout", timeout.String()},
		Timeout: 10 * time.Second,
	}}
	if mutate != nil {
		mutate(&plugins)
	}
	h, err := NewHost(AgentScope{GameID: "game-1", Env: "prod"}, plugins, nil)
	if err != nil {
		t.Fatalf("NewHost: %v", err)
	}
	t.Cleanup(h.Stop)
	return h
}

// serveTCP runs a loopback line-protocol target: every connection reads one
// request line (recorded on frames), then answers with reply. neverReply
// accepts and never answers, which is what the timeout tests need.
func serveTCP(t *testing.T, reply string, neverReply bool) (addr string, frames chan string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	frames = make(chan string, 16)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				line, _ := bufio.NewReader(conn).ReadString('\n')
				select {
				case frames <- line:
				default:
				}
				if neverReply {
					// Hold the connection open until the plugin gives up on
					// its own deadline, then close with the client.
					_ = conn.SetReadDeadline(time.Now().Add(time.Hour))
					_, _ = io.ReadAll(conn)
					return
				}
				if reply != "" {
					_, _ = conn.Write([]byte(reply))
				}
			}()
		}
	}()
	return ln.Addr().String(), frames
}

// TestHostInvokeMapsToTCP covers the TCP half of 调用映射成功: the invoke
// reaches the plugin, the plugin maps it onto one newline-terminated
// request line of the game interface and carries the reply back.
func TestHostInvokeMapsToTCP(t *testing.T) {
	addr, frames := serveTCP(t, `{"ok":true,"answer":"pong"}`+"\n", false)
	h := testTCPHost(t, addr, targetTimeout, nil)
	if err := h.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	requireRegistered(t, h, "plugin.tcp.ping")
	if reg := registeredIDs(h)["plugin.tcp.ping"]; reg.Metadata == nil || reg.Metadata.Security == nil || !reg.Metadata.Security.AuditLog {
		t.Fatalf("shared metadata with audit: %+v", reg.Metadata)
	}

	out, cause := h.Invoke(context.Background(), "plugin.tcp.ping", `{"path":"/status","body":{"id":7,"reason":"cheat"}}`, 5*time.Second)
	if cause != "" {
		t.Fatalf("unexpected degradation cause: %s", cause)
	}
	if !out.OK || out.ErrorCode != "" {
		t.Fatalf("success envelope: %+v", out)
	}
	if !strings.Contains(out.ResultJSON, `"path":"/status"`) || !strings.Contains(out.ResultJSON, "pong") {
		t.Fatalf("mapped result: %q", out.ResultJSON)
	}
	frame := <-frames
	// The request frame is the path plus the body object as compact JSON,
	// newline-terminated: the whole wire convention of the reference adapter.
	want := "/status " + `{"id":7,"reason":"cheat"}` + "\n"
	if frame != want {
		t.Fatalf("game interface received frame %q, want %q", frame, want)
	}
}

// TestHostInvokeTranslatesTCPFailure covers 映射目标失败时的错误翻译 over
// TCP: every failure stays a business failure on the closed-set envelope,
// never a fake success and never a plugin_* cause.
func TestHostInvokeTranslatesTCPFailure(t *testing.T) {
	t.Run("connection refused", func(t *testing.T) {
		h := testTCPHost(t, "127.0.0.1:1", targetTimeout, nil)
		if err := h.Start(context.Background()); err != nil {
			t.Fatalf("start: %v", err)
		}
		out, cause := h.Invoke(context.Background(), "plugin.tcp.ping", `{"path":"/status"}`, 5*time.Second)
		if cause != "" {
			t.Fatalf("target failure must stay in the envelope, got cause %s", cause)
		}
		if out.OK || out.ErrorCode != "target_unavailable" {
			t.Fatalf("translated envelope: %+v", out)
		}
	})
	t.Run("closed without replying", func(t *testing.T) {
		addr, _ := serveTCP(t, "", false)
		h := testTCPHost(t, addr, targetTimeout, nil)
		if err := h.Start(context.Background()); err != nil {
			t.Fatalf("start: %v", err)
		}
		out, cause := h.Invoke(context.Background(), "plugin.tcp.ping", `{"path":"/status"}`, 5*time.Second)
		if cause != "" || out.OK || out.ErrorCode != "target_unavailable" {
			t.Fatalf("an empty reply is an unreachable target: out=%+v cause=%s", out, cause)
		}
	})
	t.Run("game interface refuses", func(t *testing.T) {
		addr, _ := serveTCP(t, `{"ok":false,"error":"player not found"}`+"\n", false)
		h := testTCPHost(t, addr, targetTimeout, nil)
		if err := h.Start(context.Background()); err != nil {
			t.Fatalf("start: %v", err)
		}
		out, cause := h.Invoke(context.Background(), "plugin.tcp.ping", `{"path":"/status"}`, 5*time.Second)
		if cause != "" || out.OK || out.ErrorCode != "target_rejected" {
			t.Fatalf("a refusal must be target_rejected: out=%+v cause=%s", out, cause)
		}
		if !strings.Contains(out.ErrorMessage, "player not found") {
			t.Fatalf("refusal reason lost: %+v", out)
		}
	})
	t.Run("never-terminated reply over the cap", func(t *testing.T) {
		// No newline: a real cap violation is a line that never terminates,
		// not a long well-formed one.
		addr, _ := serveTCP(t, strings.Repeat("A", 1<<20+1024), false)
		h := testTCPHost(t, addr, targetTimeout, nil)
		if err := h.Start(context.Background()); err != nil {
			t.Fatalf("start: %v", err)
		}
		out, cause := h.Invoke(context.Background(), "plugin.tcp.ping", `{"path":"/status"}`, 5*time.Second)
		if cause != "" || out.OK || out.ErrorCode != "bad_response" {
			t.Fatalf("an uncarryable reply is bad_response: out=%+v cause=%s", out, cause)
		}
	})
	t.Run("malformed params", func(t *testing.T) {
		addr, _ := serveTCP(t, "PONG\n", false)
		h := testTCPHost(t, addr, targetTimeout, nil)
		if err := h.Start(context.Background()); err != nil {
			t.Fatalf("start: %v", err)
		}
		for _, params := range []string{`{}`, `{"body":[]}`, `{"path":"","body":{}}`, `{"path":"/x","body":[]}`, `{not-json`} {
			out, cause := h.Invoke(context.Background(), "plugin.tcp.ping", params, 5*time.Second)
			if cause != "" || out.OK || out.ErrorCode != "bad_response" {
				t.Fatalf("%s must be bad_response: out=%+v cause=%s", params, out, cause)
			}
		}
	})
}

// TestHostInvokeTCPTargetTimeout covers the adapter's own timeout: a game
// interface that accepts and never answers must degrade as a BUSINESS
// failure (target_unavailable) bounded by -target-timeout, not as a host
// degradation. Without an adapter-owned timeout the host's deadline would
// always win this race and the adapter could never report that the game
// interface did not answer in time.
func TestHostInvokeTCPTargetTimeout(t *testing.T) {
	addr, _ := serveTCP(t, "", true)
	h := testTCPHost(t, addr, 200*time.Millisecond, nil)
	if err := h.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	started := time.Now()
	out, cause := h.Invoke(context.Background(), "plugin.tcp.ping", `{"path":"/status"}`, 5*time.Second)
	elapsed := time.Since(started)
	if cause != "" {
		t.Fatalf("a target timeout is a business failure, not a host degradation: %s", cause)
	}
	if out.OK || out.ErrorCode != "target_unavailable" {
		t.Fatalf("an unanswered reply is an unreachable target: %+v", out)
	}
	if !strings.Contains(out.ErrorMessage, "game interface") {
		t.Fatalf("timeout must be attributed to the game interface: %+v", out)
	}
	if elapsed > 3*time.Second {
		t.Fatalf("invoke was not bounded by the target timeout: %v", elapsed)
	}
}

// TestHostInvokeTCPCallDeadlineBounded covers the host half of 超时取消:
// when the call deadline is shorter than the adapter's target timeout, the
// host owns the bound and reports plugin_timeout — the call is bounded
// either way, never wedged.
func TestHostInvokeTCPCallDeadlineBounded(t *testing.T) {
	addr, _ := serveTCP(t, "", true)
	h := testTCPHost(t, addr, 10*time.Second, nil)
	if err := h.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	started := time.Now()
	out, cause := h.Invoke(context.Background(), "plugin.tcp.ping", `{"path":"/status"}`, 300*time.Millisecond)
	elapsed := time.Since(started)
	if !strings.Contains(cause, "plugin_timeout") {
		t.Fatalf("the host deadline owns this call: out=%+v cause=%q", out, cause)
	}
	if elapsed > 3*time.Second {
		t.Fatalf("invoke was not bounded by the call deadline: %v", elapsed)
	}
}

// TestHostInvokeTCPStatuslessReply covers the protocol with no status code:
// a TCP admin interface that answers in plain text has no success field, so
// the adapter returns the reply verbatim as data instead of inventing
// success semantics. -functions-file also proves the declaration list is
// driven by the same source the reconcile loop tracks.
func TestHostInvokeTCPStatuslessReply(t *testing.T) {
	file := filepath.Join(t.TempDir(), "functions.txt")
	if err := os.WriteFile(file, []byte("status\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	addr, _ := serveTCP(t, "PONG\n", false)
	h := testTCPHost(t, addr, targetTimeout, func(ps *[]PluginConfig) {
		(*ps)[0].Args = append((*ps)[0].Args, "-functions-file", file)
	})
	if err := h.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	requireRegistered(t, h, "plugin.tcp.status")

	out, cause := h.Invoke(context.Background(), "plugin.tcp.status", `{"path":"/status"}`, 5*time.Second)
	if cause != "" || !out.OK || out.ErrorCode != "" {
		t.Fatalf("a plain reply is data: out=%+v cause=%s", out, cause)
	}
	if !strings.Contains(out.ResultJSON, `"body":"PONG"`) {
		t.Fatalf("reply must be carried verbatim: %q", out.ResultJSON)
	}
}
