//go:build !no_go_plugin

package agent

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	coreplugin "github.com/cuihairu/croupier/core/pluginhost"
	"github.com/cuihairu/croupier/internal/agent"
	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
)

func TestWithFunctionPluginsAssemblesAndStopsHost(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "httpget")
	cmd := exec.Command("go", "build", "-o", bin, "github.com/cuihairu/croupier/agents/sidecar/funcplugin/examples/httpget")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build reference plugin: %v: %s", err, output)
	}
	app := New("127.0.0.1:1", "agent-1")
	app.WithUpstreamMetadata(UpstreamMetadata{GameID: "game-1", Env: "prod"})
	app.localHandler = agent.NewLocalHandler(agentlocal.NewLocalStore(), t.TempDir(), "agent-1", nil)
	t.Cleanup(app.Stop)
	if err := app.WithFunctionPlugins(&FunctionPluginsConfig{
		Plugins: []FunctionPluginConfig{{Name: "httpget", Command: bin, Timeout: 5 * time.Second}},
	}); err != nil {
		t.Fatalf("assemble function plugins: %v", err)
	}
	host := app.FunctionPlugins()
	if host == nil || !host.IsPluginFunction("plugin.httpget.health") {
		t.Fatalf("function host was not wired: %+v", host)
	}
	app.Stop()
	if host := app.FunctionPlugins(); host != nil {
		t.Fatal("Stop must clear the wired function host")
	}
	out, cause := host.Invoke(context.Background(), "plugin.httpget.health", "{}", time.Second)
	if out.OK || !strings.HasPrefix(cause, coreplugin.CauseDisabled) {
		t.Fatalf("post-stop invoke must degrade, got out=%+v cause=%s", out, cause)
	}
}

func TestWithFunctionPluginsSkipsAssemblyWhenSwitchDisabled(t *testing.T) {
	disabled := false
	app := New("127.0.0.1:1", "agent-1")
	app.WithUpstreamMetadata(UpstreamMetadata{GameID: "game-1", Env: "prod"})
	app.localHandler = agent.NewLocalHandler(agentlocal.NewLocalStore(), t.TempDir(), "agent-1", nil)
	t.Cleanup(app.Stop)

	cfg := &FunctionPluginsConfig{
		Enabled: &disabled,
		Plugins: []FunctionPluginConfig{{
			Name:    "httpget",
			Command: "./httpget",
			Timeout: time.Second,
		}},
	}
	if err := app.WithFunctionPlugins(cfg); err != nil {
		t.Fatalf("disabled master switch must not assemble: %v", err)
	}
	if app.FunctionPlugins() != nil {
		t.Fatal("disabled master switch must not launch plugins")
	}

	perPluginDisabled := false
	cfg.Enabled = nil
	cfg.Plugins[0].Enabled = &perPluginDisabled
	if err := app.WithFunctionPlugins(cfg); err != nil {
		t.Fatalf("disabled plugin must be skipped: %v", err)
	}
	if app.FunctionPlugins() != nil {
		t.Fatal("all-disabled plugin list must not launch plugins")
	}
}
