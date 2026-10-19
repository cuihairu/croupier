//go:build !no_go_plugin

package funcplugin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var httpgetBin string

// TestMain compiles the slice-1 reference adapter once so the tests
// exercise the real go-plugin subprocess protocol end to end.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "funcplugin-test")
	if err != nil {
		panic(err)
	}
	httpgetBin = filepath.Join(dir, "httpget")
	cmd := exec.Command("go", "build", "-o", httpgetBin, "./examples/httpget")
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	if out, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		panic("build example plugin: " + string(out))
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func testHost(t *testing.T, target string, mutate func(*[]PluginConfig)) *Host {
	t.Helper()
	plugins := []PluginConfig{{
		Name:    "httpget",
		Command: httpgetBin,
		Args:    []string{"-target", target},
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

// TestHostRegistersDeclaredFunctions covers 注册与 schema 校验 through
// the real subprocess: the plugin starts, Describe stamps the version,
// declarations validate against the namespace contract and register as
// shared metadata.
func TestHostRegistersDeclaredFunctions(t *testing.T) {
	h := testHost(t, "http://127.0.0.1:1", nil)
	if err := h.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	if !h.IsPluginFunction("plugin.httpget.health") {
		t.Fatal("declared function must be registered")
	}
	if h.IsPluginFunction("player.ban") || h.IsPluginFunction("plugin.other.health") {
		t.Fatal("foreign-namespace ids must never be plugin functions")
	}
	reg := h.Registered()
	if len(reg) != 1 {
		t.Fatalf("registered: %+v", reg)
	}
	if reg[0].Plugin != "httpget" || reg[0].Version != "1.0.0" {
		t.Fatalf("plugin identity: %+v", reg[0])
	}
	if reg[0].Metadata == nil || reg[0].Metadata.Security == nil || !reg[0].Metadata.Security.AuditLog {
		t.Fatalf("shared metadata with audit: %+v", reg[0].Metadata)
	}
}

// TestHostInvokeMapsToHTTPGet covers 调用映射成功: the invoke reaches the
// plugin, the plugin maps it onto GET <target><path> and the result
// envelope carries the game interface's answer.
func TestHostInvokeMapsToHTTPGet(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	h := testHost(t, srv.URL, nil)
	if err := h.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	out, cause := h.Invoke(context.Background(), "plugin.httpget.health", `{"path":"/health"}`, 5*time.Second)
	if cause != "" {
		t.Fatalf("unexpected degradation cause: %s", cause)
	}
	if !out.OK || out.ErrorCode != "" {
		t.Fatalf("success envelope: %+v", out)
	}
	if !strings.Contains(out.ResultJSON, `"status":200`) || !strings.Contains(out.ResultJSON, `"body":"{\"ok\":true}"`) {
		t.Fatalf("mapped result: %q", out.ResultJSON)
	}
	if gotPath != "/health" {
		t.Fatalf("game interface received path %q", gotPath)
	}
}

// TestHostInvokeTranslatesTargetFailure covers 映射目标失败时的错误翻译:
// the game interface failing is a BUSINESS failure riding the closed-set
// envelope (ok=false, target_unavailable/target_rejected), never a fake
// success and never a plugin_* cause.
func TestHostInvokeTranslatesTargetFailure(t *testing.T) {
	t.Run("connection refused", func(t *testing.T) {
		h := testHost(t, "http://127.0.0.1:1", nil)
		if err := h.Start(context.Background()); err != nil {
			t.Fatalf("start: %v", err)
		}
		out, cause := h.Invoke(context.Background(), "plugin.httpget.health", `{"path":"/health"}`, 5*time.Second)
		if cause != "" {
			t.Fatalf("target failure must stay in the envelope, got cause %s", cause)
		}
		if out.OK || out.ErrorCode != "target_unavailable" {
			t.Fatalf("translated envelope: %+v", out)
		}
	})
	t.Run("game interface 500", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()
		h := testHost(t, srv.URL, nil)
		if err := h.Start(context.Background()); err != nil {
			t.Fatalf("start: %v", err)
		}
		out, cause := h.Invoke(context.Background(), "plugin.httpget.health", `{}`, 5*time.Second)
		if cause != "" || out.OK || out.ErrorCode != "target_unavailable" {
			t.Fatalf("500 must translate to target_unavailable: out=%+v cause=%s", out, cause)
		}
	})
	t.Run("game interface 404", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()
		h := testHost(t, srv.URL, nil)
		if err := h.Start(context.Background()); err != nil {
			t.Fatalf("start: %v", err)
		}
		out, cause := h.Invoke(context.Background(), "plugin.httpget.health", `{}`, 5*time.Second)
		if cause != "" || out.OK || out.ErrorCode != "target_rejected" {
			t.Fatalf("404 must translate to target_rejected: out=%+v cause=%s", out, cause)
		}
	})
}

// TestHostScopeBoundary covers scope 边界: a blank agent scope refuses
// to host plugins at all (fail closed — plugin functions inherit the
// sidecar scope), disabled plugins never launch, and unregistered ids
// never dispatch.
func TestHostScopeBoundary(t *testing.T) {
	t.Run("blank scope refuses hosting", func(t *testing.T) {
		for _, scope := range []AgentScope{
			{},
			{GameID: "game-1"},
			{Env: "prod"},
			{GameID: " ", Env: " "},
		} {
			if _, err := NewHost(scope, []PluginConfig{{Name: "httpget", Command: httpgetBin, Timeout: time.Second}}, nil); err == nil {
				t.Fatalf("blank scope %+v must refuse plugin hosting", scope)
			}
		}
	})
	t.Run("disabled plugin never launches", func(t *testing.T) {
		disabled := false
		h := testHost(t, "http://127.0.0.1:1", func(p *[]PluginConfig) {
			(*p)[0].Enabled = &disabled
		})
		if err := h.Start(context.Background()); err != nil {
			t.Fatalf("start with disabled plugin must succeed without launching: %v", err)
		}
		if len(h.Registered()) != 0 {
			t.Fatalf("disabled plugin must not register functions: %+v", h.Registered())
		}
		out, cause := h.Invoke(context.Background(), "plugin.httpget.health", `{}`, time.Second)
		if out.OK || !strings.HasPrefix(cause, "plugin_not_configured") {
			t.Fatalf("unregistered dispatch must degrade with an explicit cause: out=%+v cause=%s", out, cause)
		}
	})
	t.Run("unregistered id rejected", func(t *testing.T) {
		h := testHost(t, "http://127.0.0.1:1", nil)
		if err := h.Start(context.Background()); err != nil {
			t.Fatalf("start: %v", err)
		}
		if _, cause := h.Invoke(context.Background(), "player.ban", `{}`, time.Second); !strings.HasPrefix(cause, "plugin_not_configured") {
			t.Fatalf("foreign id must be rejected, cause=%s", cause)
		}
	})
	t.Run("broken plugin fails registration closed", func(t *testing.T) {
		h, err := NewHost(AgentScope{GameID: "game-1", Env: "prod"}, []PluginConfig{{Name: "httpget", Command: "/nonexistent/httpget", Timeout: time.Second}}, nil)
		if err != nil {
			t.Fatalf("NewHost: %v", err)
		}
		defer h.Stop()
		if err := h.Start(context.Background()); err == nil || !strings.Contains(err.Error(), "plugin_unavailable") {
			t.Fatalf("broken plugin must fail registration closed: %v", err)
		}
	})
}

func TestHostRejectsDuplicateDeclaration(t *testing.T) {
	t.Setenv("HTTPGET_DUPLICATE_FUNCTION", "1")
	h := testHost(t, "http://127.0.0.1:1", nil)
	if err := h.Start(context.Background()); err == nil || !strings.Contains(err.Error(), "duplicate id") {
		t.Fatalf("duplicate declarations must fail closed: %v", err)
	}
	if len(h.Registered()) != 0 {
		t.Fatal("failed registration must leave no functions")
	}
}
