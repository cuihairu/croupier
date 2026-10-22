//go:build !no_go_plugin

package funcplugin

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	coreplugin "github.com/cuihairu/croupier/core/pluginhost"

	"github.com/cuihairu/croupier/agents/sidecar/funcplugin/declare"
)

// DefaultReconcileInterval is how often the registration reconcile loop
// re-enumerates the declared plugins. Declarations are runtime state, not
// boot-time config: a plugin that is restarted after a crash or that
// advertises a different function list must be visible to the
// registration chain without a host restart, and the loop is what keeps
// the reported snapshot honest. One ListFunctions call per enabled plugin
// per interval is the whole cost.
const DefaultReconcileInterval = 30 * time.Second

// Host manages the declared function adapter plugins for one sidecar
// (todo.md:35): explicit config declarations, lazily started resident
// go-plugin subprocesses on the shared core engine, namespace-checked
// function registration, per-call deadlines and closed-set error
// envelopes.
type Host struct {
	scope AgentScope
	cfgs  []PluginConfig
	ctrl  *coreplugin.Controller[FuncPlugin]
	log   *slog.Logger

	mu         sync.RWMutex
	registered map[string]Registered
	pluginVer  map[string]string

	// reconcileInterval is the registration reconcile loop period; set by
	// NewHost to the package default and fixed for the host's lifetime, so
	// the only writer runs before the loop goroutine reads it. A non
	// positive value keeps the Start-time set frozen.
	reconcileInterval time.Duration
	reconcileStop     chan struct{}
	reconcileWG       sync.WaitGroup
	loopStartOnce     sync.Once
	loopStopOnce      sync.Once
}

// NewHost validates the scope and plugin names and builds the host; no
// subprocess is launched until Start.
func NewHost(scope AgentScope, plugins []PluginConfig, log *slog.Logger) (*Host, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if log == nil {
		log = slog.Default()
	}
	for _, p := range plugins {
		if err := declare.ValidatePluginName(p.Name); err != nil {
			return nil, fmt.Errorf("function plugin: %w", err)
		}
	}
	cfgs := make([]coreplugin.Config, 0, len(plugins))
	for _, p := range plugins {
		cfgs = append(cfgs, coreplugin.Config{
			Name:    p.Name,
			Command: p.Command,
			Args:    p.Args,
			Timeout: p.Timeout,
			Enabled: p.IsEnabled(),
		})
	}
	return &Host{
		scope:             scope,
		cfgs:              plugins,
		ctrl:              coreplugin.NewController(cfgs, contract(), coreplugin.Knobs{Log: log}),
		log:               log,
		registered:        map[string]Registered{},
		pluginVer:         map[string]string{},
		reconcileInterval: DefaultReconcileInterval,
		reconcileStop:     make(chan struct{}),
	}, nil
}

// Start launches every enabled declared plugin, collects and validates
// its declarations, and registers the functions. Fail-closed: a declared
// and enabled plugin that cannot start or declare is a startup error —
// unlike probe checks (which retry every tick and degrade visibly), a
// function that silently failed to register would be a missing feature,
// not a degraded one. Start applies nothing until every enabled plugin
// has been validated, so a failure leaves the host with no functions, and
// it starts the registration reconcile loop only after that.
func (h *Host) Start(ctx context.Context) error {
	var sets []pluginDeclarations
	for _, pc := range h.cfgs {
		if !pc.IsEnabled() {
			continue
		}
		ds, err := h.describePlugin(ctx, pc)
		if err != nil {
			return err
		}
		if ds.unanswered != "" {
			return fmt.Errorf("function plugin %q registration failed: %s", pc.Name, ds.unanswered)
		}
		sets = append(sets, ds)
	}
	for _, ds := range sets {
		if err := h.applyPlugin(ds); err != nil {
			h.clear()
			return err
		}
	}
	h.startReconcileLoop()
	return nil
}

// pluginDeclarations is one plugin's answer to ListFunctions after
// validation: every entry is registrable, ids are unique within the
// plugin and confined to its own namespace.
type pluginDeclarations struct {
	plugin  string
	version string
	items   map[string]Registered
	// unanswered carries the degradation cause when the plugin process did
	// not answer at all (crash backoff, open circuit, wedged call). That is
	// not a declaration fault — it is "unreachable right now".
	unanswered string
}

// describePlugin asks one plugin for its current declarations and turns
// the answer into a validated set. It is the single declaration path for
// both the fail-closed Start and the tolerant Reconcile, so the two can
// never disagree about what a plugin is allowed to declare.
func (h *Host) describePlugin(ctx context.Context, pc PluginConfig) (pluginDeclarations, error) {
	var descs []declare.Descriptor
	cause := h.ctrl.Call(ctx, pc.Name, pc.Timeout, func(ctx context.Context, impl FuncPlugin) error {
		var err error
		descs, err = impl.ListFunctions(ctx)
		return err
	})
	if cause != "" {
		return pluginDeclarations{plugin: pc.Name, unanswered: cause}, nil
	}
	// The version comes from the live process: after a crash restart the
	// engine has re-run the Describe handshake, so re-reading it is what
	// "re-Describe" means here.
	ds := pluginDeclarations{plugin: pc.Name, version: h.ctrl.Version(pc.Name), items: map[string]Registered{}}
	for _, d := range descs {
		if err := declare.Validate(pc.Name, d); err != nil {
			return pluginDeclarations{}, fmt.Errorf("function plugin %q declaration rejected: %w", pc.Name, err)
		}
		if _, dup := ds.items[d.ID]; dup {
			return pluginDeclarations{}, fmt.Errorf("function plugin %q declared duplicate id %q", pc.Name, d.ID)
		}
		md, err := declare.Metadata(pc.Name, ds.version, d)
		if err != nil {
			return pluginDeclarations{}, fmt.Errorf("function plugin %q metadata rejected: %w", pc.Name, err)
		}
		ds.items[d.ID] = Registered{Plugin: pc.Name, Version: ds.version, Declared: d, Metadata: md}
	}
	return ds, nil
}

// applyPlugin makes one plugin's reconciled set live: its previous
// registrations are replaced by the fresh set in a single locked step, so
// a function the plugin stopped declaring stops being a plugin function
// in the same instant the rest of its set updates. The result is
// registered per id, so the registration source never sees a torn
// snapshot.
func (h *Host) applyPlugin(ds pluginDeclarations) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	for id := range ds.items {
		if existing, ok := h.registered[id]; ok && existing.Plugin != ds.plugin {
			// Namespace validation already confines every declared id to
			// plugin.<name>.; a foreign owner under this id means the same
			// plugin name was declared twice. Refuse instead of stealing the
			// other plugin's registration.
			return fmt.Errorf("function plugin %q declared duplicate id %q already owned by %q", ds.plugin, id, existing.Plugin)
		}
	}
	for id, existing := range h.registered {
		if existing.Plugin == ds.plugin {
			delete(h.registered, id)
		}
	}
	for id, r := range ds.items {
		h.registered[id] = r
	}
	h.pluginVer[ds.plugin] = ds.version
	return nil
}

// Reconcile re-enumerates every enabled declared plugin and updates this
// host's registration set with what the plugin processes declare right
// now (todo.md:38). This is the half the Start-time collection is
// missing: a plugin restarted after a crash, or one whose declared list
// changes at runtime, never tells the host, and the registration chain
// must not keep advertising a function the plugin stopped declaring — or
// miss one it started to offer.
//
// A plugin's set only ever changes on a fully valid answer. An
// unreachable process (crash backoff, open circuit, wedged call) keeps
// its last known declarations and degrades on the next dispatch, which
// owns the restart retry; an invalid declaration returns an error and
// also keeps the last set — withdrawing a working function over one bad
// line would be a worse outcome. Start is the strict variant: the same
// unanswered answer is a startup error there.
//
// Reconcile is safe to call from the loop, from a registration sync or by
// hand: it is idempotent and bounded by each plugin's own configured
// timeout, never by a caller deadline.
func (h *Host) Reconcile(ctx context.Context) error {
	var errs []error
	for _, pc := range h.cfgs {
		if !pc.IsEnabled() {
			continue
		}
		ds, err := h.describePlugin(ctx, pc)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if ds.unanswered != "" {
			h.log.Warn("function plugin declarations not reconciled; keeping the last known set",
				"plugin", pc.Name, "cause", ds.unanswered)
			continue
		}
		if err := h.applyPlugin(ds); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// startReconcileLoop starts the registration reconcile loop owned by the
// host: the upstream registration source re-reads Registered() on every
// sync, so a reconciled set reaches the server on the next registration
// cycle with no extra plumbing.
func (h *Host) startReconcileLoop() {
	if h.reconcileInterval <= 0 {
		return
	}
	h.loopStartOnce.Do(func() {
		h.reconcileWG.Add(1)
		go h.reconcileLoop()
	})
}

func (h *Host) reconcileLoop() {
	defer h.reconcileWG.Done()
	for {
		select {
		case <-h.reconcileStop:
			return
		case <-time.After(h.reconcileInterval):
		}
		// The tick uses no caller deadline of its own: each ListFunctions
		// call is bounded by the plugin's configured timeout, exactly like
		// Start and Invoke, so the loop can never poison the engine's
		// failure accounting with a deadline of its own making.
		if err := h.Reconcile(context.Background()); err != nil {
			h.log.Warn("function plugin declaration reconcile failed", "error", err)
		}
	}
}

func (h *Host) clear() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.registered = map[string]Registered{}
	h.pluginVer = map[string]string{}
}

// IsPluginFunction reports whether the id is a registered plugin function
// of this host. Unregistered or foreign-namespace ids are never plugin
// functions, so the dispatch seam ignores them.
func (h *Host) IsPluginFunction(functionID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.registered[functionID]
	return ok
}

// Registered snapshots the validated plugin functions for registration
// and reporting.
func (h *Host) Registered() []Registered {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]Registered, 0, len(h.registered))
	for _, r := range h.registered {
		out = append(out, r)
	}
	return out
}

// Invoke dispatches one function call to the owning plugin under the
// shared engine's bounded gate and deadline. Host-side failures come
// back as (zero, cause) with a plugin_* cause; plugin-side business
// failures come back as (out, "") with OK=false and the closed-set
// error_code. Params are passed through as the declared contract JSON —
// schema enforcement is the plugin's declared contract plus the
// translation table in the design doc, not a host-side JSON Schema
// engine.
func (h *Host) Invoke(ctx context.Context, functionID, paramsJSON string, timeout time.Duration) (InvokeOutput, string) {
	h.mu.RLock()
	reg, ok := h.registered[functionID]
	h.mu.RUnlock()
	if !ok {
		return InvokeOutput{}, fmt.Sprintf("%s: function %q is not a registered plugin function", coreplugin.CauseNotConfigured, functionID)
	}
	var out InvokeOutput
	cause := h.ctrl.Call(ctx, reg.Plugin, timeout, func(ctx context.Context, impl FuncPlugin) error {
		var err error
		out, err = impl.Invoke(ctx, InvokeInput{FunctionID: functionID, ParamsJSON: paramsJSON, Timeout: timeout})
		// Envelope sanity: a plugin answering ok=false must use the
		// closed-set target codes; anything else is a plugin contract
		// violation, classified plugin_error instead of leaking an
		// unknown code into the caller's error taxonomy.
		if err == nil && !out.OK && !validTargetCode[out.ErrorCode] {
			return fmt.Errorf("plugin returned error_code %q outside the closed set target_unavailable|target_rejected|bad_response", out.ErrorCode)
		}
		return err
	})
	if cause != "" {
		return InvokeOutput{}, cause
	}
	return out, ""
}

// validTargetCode is the plugin-side half of the error envelope's closed
// set; host-side plugin_* codes are derived from degradation causes.
var validTargetCode = map[string]bool{
	"target_unavailable": true,
	"target_rejected":    true,
	"bad_response":       true,
}

// Stop stops accepting new plugin calls, drains in-flight ones up to a
// bounded window (each ongoing call owns its own deadline), then shuts
// down every resident plugin process (single lifecycle owner: the host).
// It also stops the registration reconcile loop and waits for a tick in
// flight, which is bounded by the plugin's configured timeout.
func (h *Host) Stop() {
	h.loopStopOnce.Do(func() {
		close(h.reconcileStop)
		h.reconcileWG.Wait()
	})
	h.ctrl.Stop()
}
