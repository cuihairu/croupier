//go:build no_go_plugin

package config

import (
	"strings"
	"testing"
	"time"
)

// TestTrimBuildRejectsPluginDeclarations pins the fail-closed trim
// contract (todo.md:36): a binary built with the no_go_plugin tag loads
// no configuration that declares plugins — startup errors instead of
// silently mapping plugin targets onto plain HTTP while the config lies.
func TestTrimBuildRejectsPluginDeclarations(t *testing.T) {
	c := valid()
	c.Synthetics.Plugins = []PluginConfig{{Name: "echo", Command: "./echo", Timeout: time.Second}}
	err := c.Validate()
	if err == nil || !strings.Contains(err.Error(), "no_go_plugin") {
		t.Fatalf("trimmed build must reject declared plugins at startup: %v", err)
	}
}
