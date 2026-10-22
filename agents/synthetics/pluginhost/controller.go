//go:build !no_go_plugin

package pluginhost

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/hashicorp/go-plugin"

	coreplugin "github.com/cuihairu/croupier/core/pluginhost"

	"github.com/cuihairu/croupier/agents/synthetics/config"
	"github.com/cuihairu/croupier/agents/synthetics/probe"
)

// Defaults for the bounded plugin runtime knobs (todo.md:36 第二片) live
// in the shared core engine; re-exported here for config-facing callers.
const (
	DefaultMaxConcurrent = coreplugin.DefaultMaxConcurrent
	DefaultQueueWindow   = coreplugin.DefaultQueueWindow
	DefaultBackoff       = coreplugin.DefaultBackoff
	DefaultBackoffCap    = coreplugin.DefaultBackoffCap
	DefaultBreakAfter    = coreplugin.DefaultBreakAfter
	DefaultOpenWindow    = coreplugin.DefaultOpenWindow
)

// Controller manages declared probe plugin processes for one synthetics
// agent run. It is a thin probe-contract adapter over the shared resident
// host engine (core/pluginhost, todo.md:35): lifecycle, bounded
// concurrency, crash backoff and circuit breaking are single-sourced
// there; this wrapper only maps the probe contract onto it.
type Controller struct {
	host *coreplugin.Controller[Probe]
}

// probeContract pins the probe IPC contract onto the shared engine.
func probeContract() coreplugin.Contract[Probe] {
	return coreplugin.Contract[Probe]{
		Key:       PluginName,
		Handshake: Handshake,
		GRPCServer: func(impl Probe) plugin.Plugin {
			return &probeGRPCPlugin{impl: impl}
		},
		Client: func(raw any) (Probe, error) {
			p, ok := raw.(Probe)
			if !ok {
				return nil, fmt.Errorf("dispensed client does not implement Probe")
			}
			return p, nil
		},
		Describe: func(ctx context.Context, impl Probe) (string, string, error) {
			return impl.Describe(ctx)
		},
	}
}

// NewController builds the probe controller from the synthetics config.
func NewController(syn config.Synthetics, log *slog.Logger) *Controller {
	cfgs := make([]coreplugin.Config, 0, len(syn.Plugins))
	for _, pc := range syn.Plugins {
		cfgs = append(cfgs, coreplugin.Config{
			Name:    pc.Name,
			Command: pc.Command,
			Args:    pc.Args,
			Timeout: pc.Timeout,
			Enabled: pc.IsEnabled(),
		})
	}
	return &Controller{host: coreplugin.NewController(cfgs, probeContract(), coreplugin.Knobs{
		MaxConcurrent: syn.PluginMaxConcurrent,
		QueueWindow:   syn.PluginQueueWindow,
		Backoff:       syn.PluginBackoff,
		BackoffCap:    syn.PluginBackoffMax,
		BreakAfter:    syn.PluginBreakAfter,
		OpenWindow:    syn.PluginOpenWindow,
		Log:           log,
	})}
}

// Check runs one probe through the named plugin under the shared
// engine's bounded gate, then maps the plugin output onto the shared
// probe result so plugin outcomes traverse the existing AlertEvent /
// probe_events chain unchanged.
func (c *Controller) Check(ctx context.Context, t config.Target) CheckOutcome {
	var out CheckOutput
	cause := c.host.Call(ctx, t.Plugin, t.Timeout, func(ctx context.Context, impl Probe) error {
		var err error
		out, err = impl.Check(ctx, CheckInput{Target: t.Name, Timeout: t.Timeout, PayloadJSON: t.PayloadJSON})
		return err
	})
	if cause != "" {
		return CheckOutcome{Cause: cause}
	}
	return CheckOutcome{
		Result: probe.Result{
			Target:  t.Name,
			OK:      out.OK,
			Status:  out.Status,
			Latency: out.Latency,
			Error:   out.Error,
			At:      out.At,
		},
		Detail: out.Detail,
	}
}

// Version returns the Describe handshake metadata of a started plugin,
// used by the runner to stamp plugin metadata onto probe details.
func (c *Controller) Version(name string) string { return c.host.Version(name) }

// InFlight reports how many plugin calls currently hold a concurrency
// slot; bounded by the engine's slot capacity by construction.
func (c *Controller) InFlight() int { return c.host.InFlight() }

// Stop stops accepting new plugin calls, drains in-flight ones up to a
// bounded window, then shuts down every resident plugin process; the
// runner calls it when the agent exits so no plugin subprocess outlives
// its lifecycle owner.
func (c *Controller) Stop() { c.host.Stop() }
