//go:build no_go_plugin

package agent

import (
	"strings"
	"testing"
	"time"
)

// TestTrimBuildRejectsFunctionPluginDeclarations pins the fail-closed
// trim contract for the function plugin face (todo.md:35): a binary
// built with the no_go_plugin tag refuses config that declares function
// plugins at assembly, instead of silently dropping declared functions.
func TestTrimBuildRejectsFunctionPluginDeclarations(t *testing.T) {
	app := NewWithConfigDir("127.0.0.1:1", "agent-1", t.TempDir())
	t.Cleanup(app.Stop)
	app.WithUpstreamMetadata(UpstreamMetadata{GameID: "game-1", Env: "prod"})
	err := app.WithFunctionPlugins(&FunctionPluginsConfig{
		Plugins: []FunctionPluginConfig{{Name: "httpget", Command: "./httpget", Timeout: time.Second}},
	})
	if err == nil || !strings.Contains(err.Error(), "no_go_plugin") {
		t.Fatalf("trimmed build must reject declared function plugins: %v", err)
	}
}
