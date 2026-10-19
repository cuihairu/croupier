package pluginhost

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"sync"

	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/cuihairu/croupier/agents/synthetics/config"
	"github.com/cuihairu/croupier/agents/synthetics/probe"
)

// Degradation causes returned when a plugin cannot answer. Each cause is
// prefixed so runner.errorClass() can classify it; the caller falls back
// to the native HTTP probe and records the cause verbatim in probe
// details, keeping plugin faults distinguishable from target faults.
const (
	CauseNotConfigured = "plugin_not_configured"
	CauseUnavailable   = "plugin_unavailable"
	CauseTimeout       = "plugin_timeout"
	CauseError         = "plugin_error"
)

// Controller manages declared probe plugin processes for one synthetics
// agent run: it discovers plugins from the explicit config, lazily starts
// the go-plugin subprocess per plugin, keeps it resident across interval
// ticks, enforces per-check deadlines, and reports a degradation cause
// instead of a fake success when the plugin side is broken.
type Controller struct {
	cfgs map[string]config.PluginConfig
	log  *slog.Logger

	mu sync.Mutex // guards active plugin processes
	// active keeps the resident subprocess per plugin name.
	active map[string]*pluginProcess
}

func NewController(cfgs []config.PluginConfig, log *slog.Logger) *Controller {
	if log == nil {
		log = slog.Default()
	}
	c := &Controller{cfgs: map[string]config.PluginConfig{}, active: map[string]*pluginProcess{}, log: log}
	for _, pc := range cfgs {
		c.cfgs[pc.Name] = pc
	}
	return c
}

// pluginProcess is one resident plugin subprocess plus its dispensed client.
type pluginProcess struct {
	pc      config.PluginConfig
	client  *plugin.Client
	impl    Probe
	name    string
	version string
}

// Version returns the Describe handshake metadata of a started plugin,
// used by the caller to stamp plugin metadata onto probe details.
func (c *Controller) Version(name string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if p, ok := c.active[name]; ok {
		return p.version
	}
	return ""
}

// CheckOutcome is one plugin-mediated check: the mapped shared probe
// result, the plugin's bounded detail string, and a degradation cause.
// A non-empty Cause means "degrade": the caller runs the native HTTP
// probe for the same target and records the cause verbatim in probe
// details, keeping plugin faults distinguishable from target faults.
type CheckOutcome struct {
	Result probe.Result
	Detail string
	Cause  string
}

// Check runs one probe through the named plugin.
func (c *Controller) Check(ctx context.Context, t config.Target) CheckOutcome {
	p, cause := c.ensure(t.Plugin)
	if cause != "" {
		return CheckOutcome{Cause: cause}
	}
	ctx, cancel := context.WithTimeout(ctx, t.Timeout)
	defer cancel()
	out, err := p.impl.Check(ctx, CheckInput{Target: t.Name, Timeout: t.Timeout})
	if err != nil {
		c.refresh(t.Plugin, p, err)
		if isDeadline(err) {
			return CheckOutcome{Cause: fmt.Sprintf("%s: no answer within %s", CauseTimeout, t.Timeout)}
		}
		if p.client != nil && p.client.Exited() {
			return CheckOutcome{Cause: fmt.Sprintf("%s: plugin process exited: %v", CauseUnavailable, err)}
		}
		return CheckOutcome{Cause: fmt.Sprintf("%s: %v", CauseError, err)}
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

// ensure returns the resident process for the plugin, starting it on
// first use and after a crashed previous instance.
func (c *Controller) ensure(name string) (*pluginProcess, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	pc, ok := c.cfgs[name]
	if !ok {
		return nil, fmt.Sprintf("%s: no plugin named %q declared in config", CauseNotConfigured, name)
	}
	if p, ok := c.active[name]; ok {
		if !p.client.Exited() {
			return p, ""
		}
		// Previous instance crashed; the single lifecycle owner (this
		// controller) tears it down before starting a replacement.
		p.client.Kill()
		delete(c.active, name)
	}
	p, cause := c.startLocked(pc)
	if cause != "" {
		return nil, cause
	}
	c.active[name] = p
	return p, ""
}

// startLocked launches the plugin subprocess, dispenses the probe client
// and runs the Describe handshake; any failure releases the process and
// reports CauseUnavailable.
func (c *Controller) startLocked(pc config.PluginConfig) (*pluginProcess, string) {
	cmd := exec.Command(pc.Command, pc.Args...)
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: Handshake,
		Plugins:         map[string]plugin.Plugin{PluginName: &probeGRPCPlugin{}},
		Cmd:             cmd,
		StartTimeout:    pc.Timeout,
		// The probe plugin runtime speaks go-plugin's local gRPC mode only;
		// the net/rpc legacy protocol is not part of the contract.
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
	})
	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, fmt.Sprintf("%s: connect %q: %v", CauseUnavailable, pc.Command, err)
	}
	raw, err := rpcClient.Dispense(PluginName)
	if err != nil {
		client.Kill()
		return nil, fmt.Sprintf("%s: dispense %q: %v", CauseUnavailable, pc.Command, err)
	}
	p := &pluginProcess{pc: pc, client: client, impl: raw.(Probe)}
	ctx, cancel := context.WithTimeout(context.Background(), pc.Timeout)
	defer cancel()
	name, version, err := p.impl.Describe(ctx)
	if err != nil {
		client.Kill()
		return nil, fmt.Sprintf("%s: describe handshake %q: %v", CauseUnavailable, pc.Command, err)
	}
	p.name, p.version = name, version
	c.log.Info("probe plugin started", "plugin", name, "version", version, "command", pc.Command)
	return p, ""
}

// refresh kills a broken resident process so the next Check starts a
// replacement, and logs the failure that broke the current call.
func (c *Controller) refresh(name string, p *pluginProcess, err error) {
	c.log.Warn("probe plugin check failed; degrading", "plugin", name, "error", err)
	c.mu.Lock()
	defer c.mu.Unlock()
	if cur, ok := c.active[name]; ok && cur == p {
		p.client.Kill()
		delete(c.active, name)
	}
}

// isDeadline reports whether the plugin call failed because the host-side
// deadline expired (context cancellation propagates through the local
// gRPC call, either as the context error or as the DeadlineExceeded code).
func isDeadline(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	return status.Code(err) == codes.DeadlineExceeded
}

// Stop shuts down every resident plugin process; the runner calls it when
// the agent exits so no plugin subprocess outlives its lifecycle owner.
func (c *Controller) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for name, p := range c.active {
		p.client.Kill()
		delete(c.active, name)
	}
}
