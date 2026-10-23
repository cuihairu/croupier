//go:build !no_go_plugin

package pluginhost

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"

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
	// lookupEnv resolves the target's secretEnv references at check time;
	// injectable for tests, defaulting to the host process environment.
	lookupEnv func(string) (string, bool)
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
	}), lookupEnv: os.LookupEnv}
}

// Check runs one probe through the named plugin under the shared
// engine's bounded gate, then maps the plugin output onto the shared
// probe result so plugin outcomes traverse the existing AlertEvent /
// probe_events chain unchanged.
//
// Credential boundary (todo.md:36 凭证边界): the target's secretEnv
// references are resolved from the host environment per check and ride
// only this one call into the plugin — never the handshake or Describe.
// A missing reference degrades with plugin_credential_missing before any
// process is touched, and every string the host lets through (plugin
// error/detail, call errors in degradation causes and engine logs) is
// scrubbed of the resolved values, so a chatty plugin cannot leak a
// secret into probe_events or the log sink.
func (c *Controller) Check(ctx context.Context, t config.Target) CheckOutcome {
	creds, cause := c.resolveCredentials(t)
	if cause != "" {
		return CheckOutcome{Cause: cause}
	}
	redact := newRedactor(creds)
	var out CheckOutput
	cause = c.host.Call(ctx, t.Plugin, t.Timeout, func(ctx context.Context, impl Probe) error {
		var err error
		out, err = impl.Check(ctx, CheckInput{Target: t.Name, Timeout: t.Timeout, PayloadJSON: t.PayloadJSON, Credentials: creds})
		if err != nil {
			// Redact before the shared engine sees the error: its
			// classification (timeout/unavailable/…) is kept via
			// Unwrap, but the message it logs and forwards is clean.
			if msg := redact(err.Error()); msg != err.Error() {
				return &redactedError{msg: msg, err: err}
			}
		}
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
			Error:   redact(out.Error),
			At:      out.At,
		},
		Detail: redact(out.Detail),
	}
}

// resolveCredentials maps the target's secretEnv references onto their
// host-environment values. Names were validated at config load; an unset
// or empty value at check time degrades the check with an explicit cause
// that names the variable but never carries its value.
func (c *Controller) resolveCredentials(t config.Target) (map[string]string, string) {
	if len(t.SecretEnv) == 0 {
		return nil, ""
	}
	creds := make(map[string]string, len(t.SecretEnv))
	for _, name := range t.SecretEnv {
		v, ok := c.lookupEnv(name)
		if !ok || v == "" {
			return nil, fmt.Sprintf("%s: target %q references env %q which is unset or empty", coreplugin.CauseCredentialMissing, t.Name, name)
		}
		creds[name] = v
	}
	return creds, ""
}

// redactedError carries an error message with credential values scrubbed
// while preserving the original error chain, so the shared engine's
// deadline/unavailable classification (errors.Is / status.Code) still
// sees the real cause underneath.
type redactedError struct {
	msg string
	err error
}

func (e *redactedError) Error() string { return e.msg }
func (e *redactedError) Unwrap() error { return e.err }

// newRedactor builds a scrubber over the resolved credential values.
// Longer values replace first so a value that is a prefix of another
// cannot leave a partial secret behind. With no credentials it is the
// identity, keeping plain error passthrough (and its classification)
// untouched.
func newRedactor(creds map[string]string) func(string) string {
	vals := make([]string, 0, len(creds))
	for _, v := range creds {
		if v != "" {
			vals = append(vals, v)
		}
	}
	if len(vals) == 0 {
		return func(s string) string { return s }
	}
	sort.Slice(vals, func(i, j int) bool { return len(vals[i]) > len(vals[j]) })
	return func(s string) string {
		for _, v := range vals {
			s = strings.ReplaceAll(s, v, "[redacted]")
		}
		return s
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
