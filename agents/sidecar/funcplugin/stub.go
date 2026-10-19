//go:build no_go_plugin

package funcplugin

import (
	"context"
	"errors"
	"log/slog"
	"time"

	coreplugin "github.com/cuihairu/croupier/core/pluginhost"
)

// Host is the trimmed replacement for the function plugin host
// (todo.md:35). Built with the no_go_plugin tag this package contains no
// go-plugin, gRPC or generated protocol code, so none of it links into
// the sidecar binary: no subprocess is ever started and no function is
// ever registered. The sidecar config gate rejects plugin declarations
// in this build before a host could exist, so the stub below is a
// compile-shape guard, not a live path.
type Host struct{}

// NewHost always fails: function plugin support is compiled out.
func NewHost(AgentScope, []PluginConfig, *slog.Logger) (*Host, error) {
	return nil, errors.New(coreplugin.CauseNotCompiled + ": binary was built with the no_go_plugin tag; function plugin runtime is compiled out")
}

func (*Host) Start(context.Context) error {
	return errors.New(coreplugin.CauseNotCompiled + ": function plugin runtime is compiled out")
}

func (*Host) IsPluginFunction(string) bool { return false }

func (*Host) Registered() []Registered { return nil }

func (*Host) Invoke(context.Context, string, string, time.Duration) (InvokeOutput, string) {
	return InvokeOutput{}, coreplugin.CauseNotCompiled + ": function plugin runtime is compiled out"
}

func (*Host) Stop() {}
