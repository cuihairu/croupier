//go:build !no_go_plugin

package agent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cuihairu/croupier/agents/sidecar/funcplugin"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
)

// startFunctionPlugins builds and starts the real function adapter
// plugin host (todo.md:35): declared plugins launch, declarations are
// validated and registered fail-closed, and the dispatch seam feeds the
// local handler. Business failures surface as errors carrying the
// closed-set code so the invoke chain reports them verbatim.
func (a *App) startFunctionPlugins(plugins []funcplugin.PluginConfig) error {
	if a.localHandler == nil {
		return fmt.Errorf("local handler not initialized")
	}
	host, err := funcplugin.NewHost(funcplugin.AgentScope{
		GameID: a.upstream.GameID(),
		Env:    a.upstream.Env(),
	}, plugins, slog.Default())
	if err != nil {
		return err
	}
	if err := host.Start(context.Background()); err != nil {
		host.Stop()
		return err
	}
	a.funcHost = host
	a.localHandler.SetPluginFunctionManager(functionPluginDispatch{host: host})
	if a.upstream != nil {
		a.upstream.SetPluginFunctionsSource(pluginFunctionDescriptors(host))
	}
	slog.Info("function plugins wired", "plugins", len(plugins), "functions", len(host.Registered()))
	return nil
}

// pluginFunctionDescriptors returns the registration payload source for
// the host: each sync re-reads the validated declarations, so periodic
// resyncs and reconnects carry the current snapshot without extra
// plumbing (todo.md:35 remaining wiring).
func pluginFunctionDescriptors(host *funcplugin.Host) func() []*agentv1.FunctionDescriptor {
	return func() []*agentv1.FunctionDescriptor {
		regs := host.Registered()
		out := make([]*agentv1.FunctionDescriptor, 0, len(regs))
		for _, r := range regs {
			out = append(out, r.Descriptor())
		}
		return out
	}
}

// functionPluginDispatch adapts the funcplugin Host to the
// agent.PluginFunctionManager seam.
type functionPluginDispatch struct {
	host *funcplugin.Host
}

func (d functionPluginDispatch) IsPluginFunction(functionID string) bool {
	return d.host.IsPluginFunction(functionID)
}

func (d functionPluginDispatch) CallPluginFunction(ctx context.Context, functionID string, paramsJSON []byte) ([]byte, error) {
	out, cause := d.host.Invoke(ctx, functionID, string(paramsJSON), 0)
	if cause != "" {
		return nil, fmt.Errorf("function plugin call failed: %s", cause)
	}
	if !out.OK {
		return nil, fmt.Errorf("function plugin call rejected: %s: %s", out.ErrorCode, out.ErrorMessage)
	}
	return []byte(out.ResultJSON), nil
}
