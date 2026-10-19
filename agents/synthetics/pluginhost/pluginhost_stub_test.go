//go:build no_go_plugin

package pluginhost

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/synthetics/config"
)

// TestTrimmedRuntimeAlwaysDegrades exercises the trimmed controller stub
// (todo.md:36 第二片): without the go-plugin runtime the API surface the
// runner consumes still works, every plugin-routed check degrades with an
// explicit cause, and nothing launches or exposes plugin metadata.
func TestTrimmedRuntimeAlwaysDegrades(t *testing.T) {
	c := NewController(config.Synthetics{}, nil)
	if c == nil {
		t.Fatal("trimmed build must still build a controller for the runner API")
	}
	out := c.Check(context.Background(), config.Target{Name: "web", Plugin: "echo", Timeout: time.Second})
	if !strings.HasPrefix(out.Cause, "plugin_not_compiled") {
		t.Fatalf("trimmed binary must degrade with an explicit cause: %+v", out)
	}
	if c.Version("echo") != "" {
		t.Fatal("trimmed binary must not expose plugin handshake metadata")
	}
	if c.InFlight() != 0 {
		t.Fatal("trimmed binary never runs plugin calls")
	}
	c.Stop()
}
