//go:build no_go_plugin

package pluginhost

import (
	"context"
	"log/slog"

	"github.com/cuihairu/croupier/agents/synthetics/config"
)

// Controller is the trimmed replacement for the plugin host (todo.md:36
// 第二片). Built with the no_go_plugin tag this package contains no
// go-plugin, gRPC or generated protocol code, so none of it links into
// the synthetics-agent binary: no subprocess is ever started and checks
// routed at a plugin degrade with an explicit cause. Its API matches the
// real controller so the runner compiles unchanged in both builds.
type Controller struct{}

func NewController(_ config.Synthetics, _ *slog.Logger) *Controller { return &Controller{} }

// Check always degrades: plugin support is compiled out of this binary.
func (*Controller) Check(context.Context, config.Target) CheckOutcome {
	return CheckOutcome{Cause: CauseNotCompiled + ": binary was built with the no_go_plugin tag; plugin runtime is compiled out"}
}

func (*Controller) Version(string) string { return "" }

func (*Controller) InFlight() int { return 0 }

func (*Controller) Stop() {}
