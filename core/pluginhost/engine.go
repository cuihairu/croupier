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
)

// Defaults for the bounded runtime knobs. Every value can be overridden
// by the role config; zero values fall back here so a minimal config
// keeps the slice-1 behavior.
const (
	DefaultMaxConcurrent = 8
	DefaultQueueWindow   = 2 * time.Second
	DefaultBackoff       = time.Second
	DefaultBackoffCap    = 30 * time.Second
	DefaultBreakAfter    = 5
	DefaultOpenWindow    = 30 * time.Second
)

// stopDrainWindow bounds how long Stop() waits for in-flight plugin
// calls to finish (their own per-call deadlines own cancellation) before
// the single lifecycle owner kills the resident processes: stopping never
// unconditionally murders an in-flight call, but it is also bounded and
// never blocks shutdown forever.
const stopDrainWindow = 5 * time.Second

// Config declares one external plugin process. Commands are explicit
// config entries: nothing is launched unless the role config declares it
// and a caller references it by name.
type Config struct {
	Name    string
	Command string
	Args    []string
	Timeout time.Duration
	// Enabled is the per-plugin runtime switch. A false value keeps the
	// plugin process never launched and degrades calls with an explicit
	// plugin_disabled cause.
	Enabled bool
}

// Knobs are the bounded-runtime tuning knobs, applied by NewController.
type Knobs struct {
	MaxConcurrent int
	QueueWindow   time.Duration
	Backoff       time.Duration
	BackoffCap    time.Duration
	BreakAfter    int
	OpenWindow    time.Duration
	Log           *slog.Logger
}

// Contract pins one plugin IPC contract onto the generic engine: the
// dispense key and handshake both sides agree on, the server/client
// adapters for the role's Impl type, and the post-handshake Describe
// stamp used for plugin metadata.
type Contract[Impl any] struct {
	// Key is the go-plugin dispense key both sides agree on.
	Key string
	// Handshake authenticates that the subprocess really is a plugin of
	// this contract and not an arbitrary binary launched by mistake.
	Handshake plugin.HandshakeConfig
	// GRPCServer adapts the role Impl to the go-plugin server side.
	GRPCServer func(impl Impl) plugin.Plugin
	// Client casts the dispensed raw client to the role Impl.
	Client func(raw any) (Impl, error)
	// Describe stamps name/version metadata after the process started;
	// a failure fails the start.
	Describe func(ctx context.Context, impl Impl) (name, version string, err error)
}

// Controller manages declared plugin processes for one role: it discovers
// plugins from the explicit config, lazily starts the go-plugin subprocess
// per plugin, keeps it resident, enforces per-call deadlines, bounds
// concurrent calls, applies crash restart backoff and a consecutive-failure
// circuit breaker, and reports a degradation cause instead of a fake
// success when the plugin side is broken.
type Controller[Impl any] struct {
	cfgs map[string]Config
	log  *slog.Logger
	ct   Contract[Impl]

	// Backpressure knobs: gate is a bounded slot pool ACQUIRED BEFORE any
	// plugin work, so concurrent callers queue (wait for a free slot at
	// most queueWindow) or are rejected promptly with plugin_backpressure
	// instead of piling up; the call path spawns no goroutines at all —
	// concurrency never exceeds the slot capacity.
	maxConcurrent int
	queueWindow   time.Duration
	gate          chan struct{}

	// Crash backoff knobs: after a plugin crash or failed call the next
	// restart waits base * 2^(consecutive-1), capped at backoffCap.
	backoff    time.Duration
	backoffCap time.Duration
	// Circuit knobs: after breakAfter consecutive failures the circuit
	// stays open for openWindow, degrading calls without touching the
	// subprocess; after the window one attempt is allowed (the same
	// restart path) and closes on success.
	breakAfter int
	openWindow time.Duration

	mu sync.Mutex // guards all mutable controller state below
	// active keeps the resident subprocess per plugin name; health keeps
	// the crash/breaker state per plugin name; stopped marks a drained
	// controller.
	active  map[string]*process[Impl]
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

// NewController builds the shared engine for one contract. Configs and
// knobs come from the role config; zero knobs fall back to the defaults.
func NewController[Impl any](cfgs []Config, ct Contract[Impl], k Knobs) *Controller[Impl] {
	if k.Log == nil {
		k.Log = slog.Default()
	}
	if k.MaxConcurrent <= 0 {
		k.MaxConcurrent = DefaultMaxConcurrent
	}
	if k.Backoff <= 0 {
		k.Backoff = DefaultBackoff
	}
	if k.BackoffCap <= 0 {
		k.BackoffCap = DefaultBackoffCap
	}
	if k.BreakAfter <= 0 {
		k.BreakAfter = DefaultBreakAfter
	}
	if k.OpenWindow <= 0 {
		k.OpenWindow = DefaultOpenWindow
	}
	c := &Controller[Impl]{
		cfgs:          map[string]Config{},
		log:           k.Log,
		ct:            ct,
		maxConcurrent: k.MaxConcurrent,
		queueWindow:   k.QueueWindow,
		gate:          make(chan struct{}, k.MaxConcurrent),
		backoff:       k.Backoff,
		backoffCap:    k.BackoffCap,
		breakAfter:    k.BreakAfter,
		openWindow:    k.OpenWindow,
		active:        map[string]*process[Impl]{},
		health:        map[string]*pluginHealth{},
	}
	for _, pc := range cfgs {
		c.cfgs[pc.Name] = pc
	}
	return c
}

// process is one resident plugin subprocess plus its dispensed client.
type process[Impl any] struct {
	pc      Config
	client  *plugin.Client
	impl    Impl
	name    string
	version string
}

// Version returns the Describe handshake metadata of a started plugin,
// used by the caller to stamp plugin metadata onto its results.
func (c *Controller[Impl]) Version(name string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if p, ok := c.active[name]; ok {
		return p.version
	}
	return ""
}

// InFlight reports how many plugin calls currently hold a concurrency
// slot; bounded by maxConcurrent by construction.
func (c *Controller[Impl]) InFlight() int { return len(c.gate) }

// Call runs one role call through the named plugin under the bounded
// gate: 1) a free slot must be acquired within the queue window (or
// reject), 2) the plugin must pass switch/backoff/circuit gates, 3) the
// call runs under the per-call deadline. Failures go through the
// backoff/breaker record instead of silently restarting. The returned
// cause is empty on success; fn captures the role's own result payload.
func (c *Controller[Impl]) Call(ctx context.Context, name string, timeout time.Duration, fn func(ctx context.Context, impl Impl) error) string {
	if c.isStopped() {
		return fmt.Sprintf("%s: controller stopped", CauseDisabled)
	}
	release, cause := c.acquire(ctx)
	if cause != "" {
		return cause
	}
	defer release()
	p, cause := c.ensure(name)
	if cause != "" {
		return cause
	}
	// timeout<=0 means "no engine-side deadline": the caller context (for
	// function calls, the dispatch budget) governs on its own.
	if timeout > 0 {
		callCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		ctx = callCtx
	}
	if err := fn(ctx, p.impl); err != nil {
		return c.fail(name, p, c.callFailureCause(p, err))
	}
	c.succeed(name)
	return ""
}

// acquire takes one concurrency slot. The wait is bounded twice: by the
// caller context and by the queue window, so a saturated plugin either
// queues briefly for a freed slot or is rejected with plugin_backpressure
// — callers are plain function calls, never unbounded goroutines.
func (c *Controller[Impl]) acquire(ctx context.Context) (func(), string) {
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
func (c *Controller[Impl]) callFailureCause(p *process[Impl], err error) string {
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
func (c *Controller[Impl]) ensure(name string) (*process[Impl], string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped {
		return nil, fmt.Sprintf("%s: controller stopped", CauseDisabled)
	}
	pc, ok := c.cfgs[name]
	if !ok {
		return nil, fmt.Sprintf("%s: no plugin named %q declared in config", CauseNotConfigured, name)
	}
	if !pc.Enabled {
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

// startLocked launches the plugin subprocess, dispenses the client and
// runs the Describe handshake; any failure releases the process and
// reports CauseUnavailable.
func (c *Controller[Impl]) startLocked(pc Config) (*process[Impl], string) {
	cmd := exec.Command(pc.Command, pc.Args...)
	var zero Impl // the host never serves the contract; only the dispense key matters here
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: c.ct.Handshake,
		Plugins:         map[string]plugin.Plugin{c.ct.Key: c.ct.GRPCServer(zero)},
		Cmd:             cmd,
		StartTimeout:    pc.Timeout,
		// The plugin runtime speaks go-plugin's local gRPC mode only; the
		// net/rpc legacy protocol is not part of the contract.
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
	})
	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, fmt.Sprintf("%s: connect %q: %v", CauseUnavailable, pc.Command, err)
	}
	raw, err := rpcClient.Dispense(c.ct.Key)
	if err != nil {
		client.Kill()
		return nil, fmt.Sprintf("%s: dispense %q: %v", CauseUnavailable, pc.Command, err)
	}
	impl, err := c.ct.Client(raw)
	if err != nil {
		client.Kill()
		return nil, fmt.Sprintf("%s: client %q: %v", CauseUnavailable, pc.Command, err)
	}
	p := &process[Impl]{pc: pc, client: client, impl: impl}
	ctx, cancel := context.WithTimeout(context.Background(), pc.Timeout)
	defer cancel()
	name, version, err := c.ct.Describe(ctx, impl)
	if err != nil {
		client.Kill()
		return nil, fmt.Sprintf("%s: describe handshake %q: %v", CauseUnavailable, pc.Command, err)
	}
	p.name, p.version = name, version
	c.log.Info("plugin started", "plugin", name, "version", version, "command", pc.Command)
	return p, ""
}

// fail records one failed plugin attempt (call error, timeout, crash or
// start failure): it tears down the broken resident process, grows the
// exponential restart backoff and, on enough consecutive failures, opens
// the degradation circuit. Returns the final degradation cause.
func (c *Controller[Impl]) fail(name string, p *process[Impl], last string) string {
	c.log.Warn("plugin call failed; degrading", "plugin", name, "error", last)
	c.mu.Lock()
	defer c.mu.Unlock()
	if cur, ok := c.active[name]; ok && cur == p {
		p.client.Kill()
		delete(c.active, name)
	}
	return c.failLocked(name, last)
}

// failLocked is fail() without locking; callers already hold c.mu.
func (c *Controller[Impl]) failLocked(name string, last string) string {
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
		// the subprocess; after openWindow exactly one restart/call path
		// is allowed again and a success closes the circuit.
		st.openUntil = now.Add(c.openWindow)
		st.nextAttempt = st.openUntil
		c.log.Warn("plugin circuit opened", "plugin", name, "consecutive_failures", st.consecFails, "open_window", c.openWindow.String())
		return fmt.Sprintf("%s: %d consecutive plugin failures; circuit open for %.1fs (last: %s)", CauseCircuitOpen, st.consecFails, c.openWindow.Seconds(), last)
	}
	st.nextAttempt = now.Add(d)
	return last
}

// succeed resets the backoff and breaker state after a healthy call.
func (c *Controller[Impl]) succeed(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if st, ok := c.health[name]; ok {
		*st = pluginHealth{}
	}
}

func (c *Controller[Impl]) isStopped() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stopped
}

func (c *Controller[Impl]) healthStateLocked(name string) *pluginHealth {
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
// down every resident plugin process; the role caller invokes it when the
// agent exits so no plugin subprocess outlives its lifecycle owner.
func (c *Controller[Impl]) Stop() {
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
