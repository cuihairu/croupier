//go:build no_go_plugin

package funcplugin

import (
	"context"
	"strings"
	"testing"
	"time"

	coreplugin "github.com/cuihairu/croupier/core/pluginhost"
)

// TestTrimmedFunctionHostAlwaysDegrades pins the trimmed stub (todo.md:35):
// without the go-plugin runtime the API surface the assembly consumes
// still works, hosting fails closed with an explicit cause, nothing
// launches and nothing registers.
func TestTrimmedFunctionHostAlwaysDegrades(t *testing.T) {
	if _, err := NewHost(AgentScope{GameID: "g", Env: "e"}, []PluginConfig{{Name: "httpget", Command: "./httpget"}}, nil); err == nil {
		t.Fatal("trimmed build must refuse to host function plugins")
	}
	h := &Host{}
	if err := h.Start(context.Background()); err == nil || !strings.HasPrefix(err.Error(), coreplugin.CauseNotCompiled) {
		t.Fatalf("trimmed start must fail closed: %v", err)
	}
	if h.IsPluginFunction("plugin.httpget.health") {
		t.Fatal("trimmed build registers no functions")
	}
	if out, cause := h.Invoke(context.Background(), "plugin.httpget.health", `{}`, time.Second); out.OK || !strings.HasPrefix(cause, coreplugin.CauseNotCompiled) {
		t.Fatalf("trimmed invoke must degrade: out=%+v cause=%s", out, cause)
	}
	if len(h.Registered()) != 0 {
		t.Fatal("trimmed build never registers")
	}
	h.Stop()
}
