package agent

import (
	"strings"
	"testing"
	"time"
)

// TestWithFunctionPluginsScopeBoundary pins the assembly-side scope
// defense (todo.md:35, 8508540 口径): plugin hosting requires the
// sidecar's own validated game/env binding — a blank upstream scope
// refuses to start hosting, and a nil/empty config is a no-op.
func TestWithFunctionPluginsScopeBoundary(t *testing.T) {
	t.Run("blank upstream scope refuses hosting", func(t *testing.T) {
		app := NewWithConfigDir("127.0.0.1:1", "agent-1", t.TempDir())
		t.Cleanup(app.Stop)
		// 无 WithUpstreamMetadata：upstream 的 game/env 为空。
		err := app.WithFunctionPlugins(&FunctionPluginsConfig{
			Plugins: []FunctionPluginConfig{{Name: "httpget", Command: "./httpget", Timeout: time.Second}},
		})
		if err == nil || !strings.Contains(err.Error(), "gameId") {
			t.Fatalf("blank scope must refuse plugin hosting: %v", err)
		}
		if app.FunctionPlugins() != nil {
			t.Fatal("refused hosting must leave no host behind")
		}
	})
	t.Run("nil or empty config is a no-op", func(t *testing.T) {
		app := NewWithConfigDir("127.0.0.1:1", "agent-1", t.TempDir())
		t.Cleanup(app.Stop)
		if err := app.WithFunctionPlugins(nil); err != nil {
			t.Fatalf("nil config: %v", err)
		}
		if err := app.WithFunctionPlugins(&FunctionPluginsConfig{}); err != nil {
			t.Fatalf("disabled config: %v", err)
		}
		if err := app.WithFunctionPlugins(&FunctionPluginsConfig{}); err != nil {
			t.Fatalf("no declared plugins: %v", err)
		}
		if app.FunctionPlugins() != nil {
			t.Fatal("no-op config must not build a host")
		}
	})
}
