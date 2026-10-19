//go:build !no_go_plugin

package pluginhost

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"sync"
	"time"

	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/cuihairu/croupier/agents/synthetics/config"
	"github.com/cuihairu/croupier/agents/synthetics/probe"
)

// Defaults for the bounded plugin runtime knobs (todo.md:36 第二片).
// Every value can be overridden by the synthetics config; zero values
// fall back here so a minimal config keeps the slice-1 behavior.
const (
	DefaultMaxConcurrent = 8
	DefaultQueueWindow   = 2 * time.Second
	DefaultBackoff       = time.Second
	DefaultBackoffCap    = 30 * time.Second
	DefaultBreakAfter    = 5
	DefaultOpenWindow    = 30 * time.Second
)

// stopDrainWindow bounds how long Stop() waits for in-flight plugin
// calls to finish (their own per-check deadlines own cancellation) before
// the single lifecycle owner kills the resident processes: stopping never
// unconditionally murders an in-flight call, but it is also bounded and
// never blocks shutdown forever.
const stopDrainWindow = 5 * time.Second

// Controller manages declared probe plugin processes for one synthetics
// agent run: it discovers plugins from the explicit config, lazily starts
// the go-plugin subprocess per plugin, keeps it resident across interval
// ticks, enforces per-check deadlines, bounds concurrent calls, applies
// crash restart backoff and a consecutive-failure circuit breaker, and
// reports a degradation cause instead of a fake success when the plugin
// side is broken.
type Controller struct {
	cfgs map[string]config.PluginConfig
	log  *slog.Logger

	// Backpressure knobs (todo.md:36 第二片): gate is a bounded slot
	// pool ACQUIRED BEFORE any plugin work, so concurrent callers queue
	// (wait for a free slot at most queueWindow) or are rejected
	// promptly with plugin_backpressure instead of piling up; the check
	// path spawns no goroutines at all — concurrency never exceeds the
	// slot capacity.
	maxConcurrent int
	queueWindow   time.Duration
	gate          chan struct{}

	// Crash backoff knobs: after a plugin crash or failed call the next
	// restart waits base * 2^(consecutive-1), capped at backoffCap.
	backoff    time.Duration
	backoffCap time.Duration
	// Circuit knobs: after breakAfter consecutive failures the circuit
	// stays open for openWindow, degrading calls without touching the
	// subprocess; after the window one probe attempt is allowed (the
	// same restart path) and closes on success.
	breakAfter int
	openWindow time.Duration

	mu sync.Mutex // guards all mutable controller state below
	// active keeps the resident subprocess per plugin name; health keeps
	// the crash/breaker state per plugin name; stopped marks a drained
	// controller.
	active  map[string]*pluginProcess
	health  map[string]*pluginHealth
	stopped bool
}

// pluginHealth is the crash-backoff and circuit-breaker state for one
// plugin name. Zero nextAttempt/openUntil read as "may attempt now".
type pluginHealth struct {
	consecFails int
	nextAttempt time.Time
	openUntil   time.Time
}

func NewController(syn config.Synthetics, log *slog.Logger) *Controller {
	if log == nil {
		log = slog.Default()
	}
	maxConcurrent := syn.PluginMaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = DefaultMaxConcurrent
	}
	backoff := syn.PluginBackoff
	if backoff <= 0 {
		backoff = DefaultBackoff
	}
	backoffCap := syn.PluginBackoffMax
	if backoffCap <= 0 {
		backoffCap = DefaultBackoffCap
	}
	breakAfter := syn.PluginBreakAfter
	if breakAfter <= 0 {
		breakAfter = DefaultBreakAfter
	}
	openWindow := syn.PluginOpenWindow
	if openWindow <= 0 {
		openWindow = DefaultOpenWindow
	}
	c := &Controller{
		cfgs:          map[string]config.PluginConfig{},
		log:           log,
		maxConcurrent: maxConcurrent,
		queueWindow:   syn.PluginQueueWindow,
		gate:          make(chan struct{}, maxConcurrent),
		backoff:       backoff,
		backoffCap:    backoffCap,
		breakAfter:    breakAfter,
		openWindow:    openWindow,
		active:        map[string]*pluginProcess{},
		health:        map[string]*pluginHealth{},
	}
	for _, pc := range syn.Plugins {
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

// InFlight reports how many plugin calls currently hold a concurrency
// slot; bounded by maxConcurrent by construction.
func (c *Controller) InFlight() int { return len(c.gate) }

// Check runs one probe through the named plugin under the bounded gate:
// 1) a free slot must be acquired within the queue window (or reject),
// 2) the plugin must pass switch/backoff/circuit gates, 3) the call runs
// under the per-check deadline. Failures go through the backoff/breaker
// record instead of silently restarting.
func (c *Controller) Check(ctx context.Context, t config.Target) CheckOutcome {
	if c.isStopped() {
		return CheckOutcome{Cause: fmt.Sprintf("%s: controller stopped", CauseDisabled)}
	}
	release, cause := c.acquire(ctx)
	if cause != "" {
		return CheckOutcome{Cause: cause}
	}
	defer release()
	p, cause := c.ensure(t.Plugin)
	if cause != "" {
		return CheckOutcome{Cause: cause}
	}
	ctx, cancel := context.WithTimeout(ctx, t.Timeout)
	defer cancel()
	out, err := p.impl.Check(ctx, CheckInput{Target: t.Name, Timeout: t.Timeout})
	if err != nil {
		return CheckOutcome{Cause: c.fail(t.Plugin, p, c.callFailureCause(p, err))}
	}
	c.succeed(t.Plugin)
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

// acquire takes one concurrency slot. The wait is bounded twice: by the
// caller context and by the queue window, so a saturated plugin either
// queues briefly for a freed slot or is rejected with plugin_backpressure
// — callers are plain function calls, never unbounded goroutines.
func (c *Controller) acquire(ctx context.Context) (func(), string) {
	waitCtx := ctx
	if c.queueWindow > 0 {
		var cancel context.CancelFunc
		waitCtx, cancel = context.WithTimeout(ctx, c.queueWindow)
		defer cancel()
	}
	start := time.Now()
	select {
	case c.gate <- struct{}{}:
		return func() { <-c.gate }, ""
	case <-waitCtx.Done():
		return nil, fmt.Sprintf("%s: concurrency limit %d reached; no slot within %.1fs (waited %.0fms)",
			CauseBackpressure, c.maxConcurrent, c.queueWindow.Seconds(), time.Since(start).Seconds()*1000)
	}
}

// callFailureCause classifies a failed plugin call into a prefixed
// degradation cause; terminal exits are distinguished from deadlines and
// plain errors so the caller records the plugin fault verbatim.
// client.Exited() detects crashes with a small delay, so a broken local
// connection (Unavailable / EOF) is classified unavailable too — both
// mean the plugin subprocess stopped answering mid-call.
func (c *Controller) callFailureCause(p *pluginProcess, err error) string {
	if isDeadline(err) {
		return fmt.Sprintf("%s: no answer within deadline: %v", CauseTimeout, err)
	}
	if (p.client != nil && p.client.Exited()) || status.Code(err) == codes.Unavailable {
		return fmt.Sprintf("%s: plugin process exited: %v", CauseUnavailable, err)
	}
	return fmt.Sprintf("%s: %v", CauseError, err)
}

// ensure returns the resident process for the plugin, honoring the
// switch/backoff/circuit gates and starting the process on first use.
func (c *Controller) ensure(name string) (*pluginProcess, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped {
		return nil, fmt.Sprintf("%s: controller stopped", CauseDisabled)
	}
	pc, ok := c.cfgs[name]
	if !ok {
		return nil, fmt.Sprintf("%s: no plugin named %q declared in config", CauseNotConfigured, name)
	}
	if !pc.IsEnabled() {
		return nil, fmt.Sprintf("%s: plugin %q declared disabled in config", CauseDisabled, name)
	}
	st := c.healthStateLocked(name)
	now := time.Now()
	if st.openUntil.After(now) {
		return nil, fmt.Sprintf("%s: circuit open; degrading for %.1fs", CauseCircuitOpen, time.Until(st.openUntil).Seconds())
	}
	if now.Before(st.nextAttempt) {
		return nil, fmt.Sprintf("%s: restart of failed plugin in %.1fs", CauseBackoff, time.Until(st.nextAttempt).Seconds())
	}
	if p, ok := c.active[name]; ok {
		if !p.client.Exited() {
			return p, ""
		}
		// Previous instance crashed between calls; the single lifecycle
		// owner (this controller) tears it down and records the failure
		// so the next attempts back off instead of tight-loop restarts.
		p.client.Kill()
		delete(c.active, name)
		return nil, c.failLocked(name, fmt.Sprintf("%s: plugin process exited", CauseUnavailable))
	}
	p, cause := c.startLocked(pc)
	if cause != "" {
		return nil, c.failLocked(name, cause)
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

// fail records one failed plugin attempt (call error, timeout, crash or
// start failure): it tears down the broken resident process, grows the
// exponential restart backoff and, on enough consecutive failures, opens
// the degradation circuit. Returns the final degradation cause.
func (c *Controller) fail(name string, p *pluginProcess, last string) string {
	c.log.Warn("probe plugin check failed; degrading", "plugin", name, "error", last)
	c.mu.Lock()
	defer c.mu.Unlock()
	if cur, ok := c.active[name]; ok && cur == p {
		p.client.Kill()
		delete(c.active, name)
	}
	return c.failLocked(name, last)
}

// failLocked is fail() without locking; callers already hold c.mu.
func (c *Controller) failLocked(name string, last string) string {
	st := c.healthStateLocked(name)
	st.consecFails++
	now := time.Now()
	// Exponential restart backoff: base * 2^(consecFails-1), capped.
	// Chosen over a fixed step because a one-off crash recovers after a
	// single short wait (first retry ≈ 1s default), while a broken or
	// crash-looping binary backs off geometrically — a fixed step would
	// let a permanently dead plugin restart at a constant cadence
	// forever. The cap keeps worst-case recovery latency bounded.
	d := c.backoff
	for i := 1; i < st.consecFails && d < c.backoffCap; i++ {
		d *= 2
	}
	if d > c.backoffCap {
		d = c.backoffCap
	}
	if st.consecFails >= c.breakAfter {
		// Circuit opens on consecutive failures: degrade without touching
		// the subprocess; after openWindow exactly one restart/probe path
		// is allowed again and a success closes the circuit.
		st.openUntil = now.Add(c.openWindow)
		st.nextAttempt = st.openUntil
		c.log.Warn("probe plugin circuit opened", "plugin", name, "consecutive_failures", st.consecFails, "open_window", c.openWindow.String())
		return fmt.Sprintf("%s: %d consecutive plugin failures; circuit open for %.1fs (last: %s)", CauseCircuitOpen, st.consecFails, c.openWindow.Seconds(), last)
	}
	st.nextAttempt = now.Add(d)
	return last
}

// succeed resets the backoff and breaker state after a healthy call.
func (c *Controller) succeed(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if st, ok := c.health[name]; ok {
		*st = pluginHealth{}
	}
}

func (c *Controller) isStopped() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stopped
}

func (c *Controller) healthStateLocked(name string) *pluginHealth {
	st, ok := c.health[name]
	if !ok {
		st = &pluginHealth{}
		c.health[name] = st
	}
	return st
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

// Stop stops accepting new plugin calls, drains in-flight ones up to a
// bounded window (each ongoing call owns its own deadline), then shuts
// down every resident plugin process; the runner calls it when the agent
// exits so no plugin subprocess outlives its lifecycle owner.
func (c *Controller) Stop() {
	c.mu.Lock()
	c.stopped = true
	c.mu.Unlock()
	deadline := time.Now().Add(stopDrainWindow)
	for c.InFlight() > 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for name, p := range c.active {
		p.client.Kill()
		delete(c.active, name)
	}
}
