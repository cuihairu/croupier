package runner

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cuihairu/croupier/agents/controlconn"
	"github.com/cuihairu/croupier/agents/synthetics/config"
	"github.com/cuihairu/croupier/agents/synthetics/pluginhost"
	"github.com/cuihairu/croupier/agents/synthetics/probe"
	"github.com/cuihairu/croupier/core/configsync"
	"github.com/cuihairu/croupier/core/healthprobe"
	"github.com/cuihairu/croupier/core/register"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

var Version = "dev"
var GitCommit, BuildTime = "unknown", "unknown"

type Runner struct {
	cfg config.Config
	log *slog.Logger
	reg *register.Client
	// plugins hosts external probe plugins (go-plugin local gRPC).
	// nil when no plugin is declared: no plugin process is ever started
	// and every target keeps the native HTTP probe.
	plugins *pluginhost.Controller
	policy  atomic.Pointer[healthprobe.ProbePolicy]

	// targets is the active target set (local config plus server-pulled
	// synthetics.targets). It is goroutine-safe: the target puller mutates
	// it from its own goroutine while runTargets reads it in the select loop.
	targets *targetSet
	// wake nudges the runTargets select loop after the target set changes.
	wake chan struct{}
}

// targetLoop is one active probe timer for a single target.
type targetLoop struct {
	target         config.Target
	timer          *time.Ticker
	targetInterval time.Duration
}

// targetSet is the goroutine-safe active target collection.
type targetSet struct {
	mu    sync.Mutex
	loops map[string]*targetLoop
}

func newTargetSet() *targetSet {
	return &targetSet{loops: map[string]*targetLoop{}}
}

// snapshot returns the current loops in deterministic (name-sorted) order.
func (ts *targetSet) snapshot() []*targetLoop {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	names := make([]string, 0, len(ts.loops))
	for name := range ts.loops {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]*targetLoop, 0, len(names))
	for _, name := range names {
		out = append(out, ts.loops[name])
	}
	return out
}

// has reports whether the loop pointer is still the active one for its target.
func (ts *targetSet) has(loop *targetLoop) bool {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.loops[loop.target.Name] == loop
}

// get returns the loop for a target name, if present.
func (ts *targetSet) get(name string) *targetLoop {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.loops[name]
}

// probeSender is the slice of the upstream control connection the probe
// loops consume. The register client owns the connection lifecycle
// (background registration/reconnect swap it under its mutex); probes
// must take a snapshot via upstream() instead of caching a Runner-owned
// pointer that reconnect would overwrite unsynchronized.
type probeSender interface {
	Connected() bool
	SendProbeEvent(ctx context.Context, ev *opsv1.AlertEvent) error
	PullProbePolicy(ctx context.Context, req *opsv1.ProbePolicyConfigRequest) (*opsv1.ProbePolicyConfigResponse, error)
	PullSyntheticsTargets(ctx context.Context, req *opsv1.SyntheticsTargetsConfigRequest) (*opsv1.SyntheticsTargetsConfigResponse, error)
}

// upstream returns the current synchronized connection snapshot from the
// register client. nil when the runner has no session (never started,
// registration not yet completed, failed, or stopped).
func (r *Runner) upstream() probeSender {
	if r.reg == nil {
		return nil
	}
	conn, _ := r.reg.Conn().(probeSender)
	return conn
}

func New(cfg config.Config, log *slog.Logger) *Runner {
	if log == nil {
		log = slog.Default()
	}
	r := &Runner{cfg: cfg, log: log}
	// Runtime master switch (todo.md:36 第二片): pluginsEnabled: false
	// runs pure native HTTP probing; no plugin controller is built and
	// no plugin process is ever started. In the default build the
	// controller exists whenever plugin support is compiled in and
	// declared; under the no_go_plugin build tag config validation
	// rejects declared plugins at startup instead.
	if cfg.Synthetics.PluginsOn() && len(cfg.Synthetics.Plugins) > 0 {
		r.plugins = pluginhost.NewController(cfg.Synthetics, log)
	}
	return r
}
func (r *Runner) Run(ctx context.Context) error {
	reg := register.New(register.Config{AgentID: r.cfg.Agent.ID, Dial: func(context.Context) (register.Conn, error) {
		// No Runner-owned alias: the client stores the dialed connection
		// under its own lock and reconnect swaps it there; probes read it
		// back through upstream().
		return controlconn.Dial(r.cfg.Agent.ServerAddr, tlsFromCfg(r.cfg))
	}, Payload: r.buildPayload, Logger: r.log})
	r.reg = reg
	reg.Start(ctx)
	defer reg.Stop()

	r.targets = newTargetSet()
	r.wake = make(chan struct{}, 1)
	for _, target := range r.cfg.Synthetics.Targets {
		r.addTarget(target, true)
	}
	r.newPolicyPuller().Start(ctx)
	r.newTargetsPuller().Start(ctx)

	if r.plugins != nil {
		defer r.plugins.Stop()
	}
	return r.runTargets(ctx)
}

// addTarget installs a target loop. When immediate is true the probe is
// executed once before the first tick. Called from Run before any puller
// goroutine starts (single-threaded setup), so locking is defensive.
func (r *Runner) addTarget(target config.Target, immediate bool) {
	interval := r.intervalFor(target.Name, true)
	loop := &targetLoop{target: target, timer: time.NewTicker(interval), targetInterval: interval}
	r.targets.mu.Lock()
	r.targets.loops[target.Name] = loop
	r.targets.mu.Unlock()
	if immediate {
		r.execute(context.Background(), target)
	}
	r.nudge()
}

// applyTargets replaces the active target set with a validated pulled list.
// Targets no longer present stop their timers; new targets start probing.
// Called only after the puller validated and parsed the payload, so a
// bad version never mutates the running set.
func (r *Runner) applyTargets(targets []config.Target) error {
	// Validate against local plugin declarations before touching anything.
	if err := r.cfg.Synthetics.ValidateTargets(targets); err != nil {
		return err
	}
	want := make(map[string]config.Target, len(targets))
	for _, t := range targets {
		if t.Name == "" {
			return fmt.Errorf("synthetics targets: empty target name")
		}
		want[t.Name] = t
	}
	r.targets.mu.Lock()
	defer r.targets.mu.Unlock()
	// Stop and remove loops that are no longer wanted.
	for name, loop := range r.targets.loops {
		if _, keep := want[name]; !keep {
			loop.timer.Stop()
			delete(r.targets.loops, name)
			r.log.Info("synthetics target removed", "target", name)
		}
	}
	// Add new loops and replace changed configurations. A changed target
	// replaces the whole loop object (stop the old timer, drop the stale
	// pointer) instead of mutating fields in place: runTargets may hold the
	// old pointer in its select cases and read target/timer without the set
	// lock, so loop objects are immutable after creation.
	for name, t := range want {
		if loop, ok := r.targets.loops[name]; ok {
			if reflect.DeepEqual(loop.target, t) {
				continue // unchanged: keep window state
			}
			loop.timer.Stop()
			interval := r.intervalFor(name, true)
			r.targets.loops[name] = &targetLoop{target: t, timer: time.NewTicker(interval), targetInterval: interval}
			r.log.Info("synthetics target updated", "target", name)
			continue
		}
		interval := r.intervalFor(name, true)
		r.targets.loops[name] = &targetLoop{target: t, timer: time.NewTicker(interval), targetInterval: interval}
		r.execute(context.Background(), t)
		r.log.Info("synthetics target added", "target", name)
	}
	r.nudge()
	return nil
}

func (r *Runner) nudge() {
	if r.wake == nil {
		return
	}
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

// startPolicyPuller wires the central probe.policy version puller. Fetch
// failures and invalid payloads keep the last-good snapshot; the configsync
// cursor also does not advance on apply errors.
func (r *Runner) newPolicyPuller() *configsync.Puller[string] {
	puller := configsync.New[string](r.cfg.Synthetics.Interval,
		func(ctx context.Context) (*configsync.Versioned[string], error) {
			conn := r.upstream()
			if conn == nil || !conn.Connected() {
				return nil, fmt.Errorf("probe policy pull: upstream session unavailable")
			}
			resp, err := conn.PullProbePolicy(ctx, &opsv1.ProbePolicyConfigRequest{AgentId: r.cfg.Agent.ID})
			if err != nil {
				return nil, err
			}
			if resp == nil || resp.Version == "" {
				return nil, nil
			}
			return &configsync.Versioned[string]{Version: resp.Version, Payload: resp.PolicyJson}, nil
		},
		func(_ context.Context, payload string) error {
			policy, err := healthprobe.ParseProbePolicy(payload)
			if err != nil {
				return err
			}
			r.policy.Store(policy)
			return nil
		},
		configsync.WithOnError[string](func(err error) {
			r.log.Warn("probe policy pull failed; keep last-good", "error", err)
		}),
		configsync.WithOnApply[string](func(version string) {
			r.log.Info("probe sampling policy applied", "version", version)
		}),
	)
	return puller
}

// runTargets drives the active probe loops until ctx is canceled. The set
// mutates under r.targets (pulled targets) so the select loop rebuilds its
// case list whenever the target set or a timer interval changes.
func (r *Runner) runTargets(ctx context.Context) error {
	for {
		loops := r.targets.snapshot()
		cases := make([]reflect.SelectCase, 0, len(loops)+2)
		cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ctx.Done())})
		cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(r.wake)})
		for _, loop := range loops {
			cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(loop.timer.C)})
		}
		chosen, _, _ := reflect.Select(cases)
		if chosen == 0 {
			return nil
		}
		if chosen == 1 {
			continue // target set changed: rebuild cases
		}
		loop := loops[chosen-2]
		// The loop may have been removed/restarted while we waited. The
		// timer.C is drained by restart; a stale case still selects, so
		// verify the loop is still the active one before executing.
		if !r.targets.has(loop) {
			continue
		}
		ok := r.execute(ctx, loop.target)
		interval := r.intervalFor(loop.target.Name, ok)
		if interval != loop.targetInterval {
			loop.timer.Reset(interval)
			loop.targetInterval = interval
		}
	}
}

// newTargetsPuller wires the central synthetics.targets version puller.
// Fetch failures, invalid payloads, and validation failures all keep the
// last-good target set; the configsync cursor does not advance on apply
// errors. Empty version = nothing published (keep local config targets).
func (r *Runner) newTargetsPuller() *configsync.Puller[string] {
	return configsync.New[string](r.cfg.Synthetics.Interval,
		func(ctx context.Context) (*configsync.Versioned[string], error) {
			conn := r.upstream()
			if conn == nil || !conn.Connected() {
				return nil, fmt.Errorf("synthetics targets pull: upstream session unavailable")
			}
			resp, err := conn.PullSyntheticsTargets(ctx, &opsv1.SyntheticsTargetsConfigRequest{AgentId: r.cfg.Agent.ID})
			if err != nil {
				return nil, err
			}
			if resp == nil || resp.Version == "" {
				return nil, nil
			}
			return &configsync.Versioned[string]{Version: resp.Version, Payload: resp.TargetsJson}, nil
		},
		func(_ context.Context, payload string) error {
			targets, err := config.ParseTargetsJSON(payload)
			if err != nil {
				return err
			}
			return r.applyTargets(targets)
		},
		configsync.WithOnError[string](func(err error) {
			r.log.Warn("synthetics targets pull failed; keep last-good", "error", err)
		}),
		configsync.WithOnApply[string](func(version string) {
			r.log.Info("synthetics targets applied", "version", version)
		}),
	)
}
func (r *Runner) execute(ctx context.Context, t config.Target) bool {
	res, details := r.runProbe(ctx, t)
	ev := buildProbeEvent(r.cfg, t, res, details)
	// Synchronized snapshot: reconnect may swap the connection while this
	// probe loop runs; a stale snapshot at worst reports a closed session.
	conn := r.upstream()
	if conn == nil || !conn.Connected() {
		r.log.Warn("probe result not reported: session unavailable", "target", t.Name)
		return res.OK
	}
	sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := conn.SendProbeEvent(sendCtx, ev); err != nil {
		r.log.Warn("probe result report failed", "target", t.Name, "error", err)
	}
	r.log.Info("probe completed", "target", t.Name, "ok", res.OK, "status", res.Status, "latencyMs", res.Latency.Milliseconds(), "error", res.Error)
	return res.OK
}

// intervalFor resolves the effective sampling interval for a target from the
// last applied policy snapshot. unavailable=true (AutoBoost C#5) halves the
// interval with a 5s floor. No policy snapshot keeps the configured interval.
func (r *Runner) intervalFor(target string, unavailable bool) time.Duration {
	if p := r.policy.Load(); p != nil {
		return p.EffectiveIntervalFor(target, time.Now(), unavailable)
	}
	return r.cfg.Synthetics.Interval
}

// runProbe routes one target through its declared external plugin, falling
// back to the native HTTP probe with an explicit plugin cause when the
// plugin side degrades or is disabled. Targets without plugin routing
// keep the native HTTP probe untouched. A nil controller with a
// plugin-routed target only happens with the runtime switch off (or a
// command line shim) and is reported as plugin_disabled instead of
// silently dropping the routing decision.
func (r *Runner) runProbe(ctx context.Context, t config.Target) (probe.Result, map[string]string) {
	if t.Plugin != "" {
		if r.plugins == nil {
			return r.fallbackHTTP(ctx, t, fmt.Sprintf("%s: plugin runtime switch is off", pluginhost.CauseDisabled))
		}
		out := r.plugins.Check(ctx, t)
		if out.Cause == "" {
			probeType := "plugin"
			if strings.EqualFold(strings.TrimSpace(t.Type), config.TypeICMP) {
				probeType = "plugin:icmp"
			}
			details := map[string]string{"probeType": probeType, "status": statusString(out.Result.OK), "errorClass": errorClass(out.Result.Error), "pluginName": t.Plugin}
			if v := r.plugins.Version(t.Plugin); v != "" {
				details["pluginVersion"] = v
			}
			if out.Detail != "" {
				details["pluginDetail"] = out.Detail
			}
			return out.Result, details
		}
		return r.fallbackHTTP(ctx, t, out.Cause)
	}
	if strings.EqualFold(strings.TrimSpace(t.Type), config.TypeICMP) {
		res := probe.NewICMPProbe(t.Address, t.Timeout, t.Count).Run(ctx)
		details := map[string]string{"probeType": "icmp", "status": statusString(res.OK), "errorClass": errorClass(res.Error), "address": t.Address}
		if res.Status > 0 {
			details["replyStatus"] = itoa(res.Status)
		}
		return res, details
	}
	res := probe.Run(ctx, t)
	return res, map[string]string{"probeType": "http", "status": statusString(res.OK), "httpStatus": itoa(res.Status), "errorClass": errorClass(res.Error)}
}

// fallbackHTTP is the explicit degradation path (todo.md:36): the plugin
// subprocess could not answer (unavailable / timed out / call error /
// backpressure / backoff / circuit / disabled); the native HTTP probe
// keeps the target alive and the cause is recorded for the operator
// instead of masking the plugin fault.
func (r *Runner) fallbackHTTP(ctx context.Context, t config.Target, cause string) (probe.Result, map[string]string) {
	r.log.Warn("probe plugin degraded; falling back to native HTTP probe", "target", t.Name, "plugin", t.Plugin, "cause", cause)
	res := probe.Run(ctx, t)
	details := map[string]string{
		"probeType": "http", "status": statusString(res.OK), "httpStatus": itoa(res.Status),
		"errorClass": errorClass(res.Error), "pluginName": t.Plugin,
		"pluginFallback": "true", "pluginCause": cause,
		"probeError": "plugin degraded: " + cause,
	}
	return res, details
}

func statusString(ok bool) string {
	if ok {
		return "success"
	}
	return "failure"
}

// buildProbeEvent maps one probe outcome onto the existing AlertEvent /
// ProbeDetail contract; plugin results land in the same event shape as
// HTTP results, with routing metadata carried in Details.
func buildProbeEvent(c config.Config, t config.Target, res probe.Result, details map[string]string) *opsv1.AlertEvent {
	return &opsv1.AlertEvent{Kind: "probe.detail", AgentId: c.Agent.ID, GameId: c.Agent.GameID, Env: c.Agent.Env, TargetCategory: t.TargetCategory, Detail: &opsv1.ProbeDetail{Target: t.Name, Semantics: "liveness", Ok: res.OK, Err: res.Error, AtTsUnixMs: res.At.UnixMilli(), LatencyMs: res.Latency.Milliseconds(), TargetCategory: t.TargetCategory, Details: details}}
}
func (r *Runner) buildPayload(context.Context) (*agentv1.RegisterRequest, error) {
	host, _ := os.Hostname()
	return &agentv1.RegisterRequest{AgentId: r.cfg.Agent.ID, Version: Version, GameId: r.cfg.Agent.GameID, Env: r.cfg.Agent.Env, Labels: map[string]string{"role": "synthetics", "hostname": host, "os": runtime.GOOS, "arch": runtime.GOARCH, "gitCommit": GitCommit, "buildTime": BuildTime}}, nil
}
func tlsFromCfg(c config.Config) *controlconn.TLS {
	if !c.OutboundTLS.Enabled {
		return nil
	}
	return &controlconn.TLS{Enabled: true, CertFile: c.OutboundTLS.CertFile, KeyFile: c.OutboundTLS.KeyFile, CAFile: c.OutboundTLS.CAFile, ServerName: c.OutboundTLS.ServerName, InsecureSkipVerify: c.OutboundTLS.InsecureSkipVerify}
}
func itoa(i int) string { return strconv.Itoa(i) }
func errorClass(s string) string {
	if s == "" {
		return ""
	}
	if i := strings.IndexByte(s, ':'); i > 0 {
		return s[:i]
	}
	return "probe_error"
}
