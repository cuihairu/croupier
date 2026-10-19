//go:build !no_go_plugin

package funcplugin

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	coreplugin "github.com/cuihairu/croupier/core/pluginhost"

	"github.com/cuihairu/croupier/agents/sidecar/funcplugin/declare"
	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
)

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
		scope:      scope,
		cfgs:       plugins,
		ctrl:       coreplugin.NewController(cfgs, contract(), coreplugin.Knobs{Log: log}),
		log:        log,
		registered: map[string]Registered{},
		pluginVer:  map[string]string{},
	}, nil
}

// Start launches every enabled declared plugin, collects and validates
// its declarations, and registers the functions. Fail-closed: a declared
// and enabled plugin that cannot start or declare is a startup error —
// unlike probe checks (which retry every tick and degrade visibly), a
// function that silently failed to register would be a missing feature,
// not a degraded one. The registration reconcile loop is future work.
func (h *Host) Start(ctx context.Context) error {
	for _, pc := range h.cfgs {
		if !pc.IsEnabled() {
			continue
		}
		var descs []declare.Descriptor
		cause := h.ctrl.Call(ctx, pc.Name, pc.Timeout, func(ctx context.Context, impl FuncPlugin) error {
			var err error
			descs, err = impl.ListFunctions(ctx)
			return err
		})
		if cause != "" {
			return fmt.Errorf("function plugin %q registration failed: %s", pc.Name, cause)
		}
		version := h.ctrl.Version(pc.Name)
		for _, d := range descs {
			if err := declare.Validate(pc.Name, d); err != nil {
				return fmt.Errorf("function plugin %q declaration rejected: %w", pc.Name, err)
			}
			md, err := declare.Metadata(pc.Name, version, d)
			if err != nil {
				return fmt.Errorf("function plugin %q metadata rejected: %w", pc.Name, err)
			}
			if err := h.register(pc.Name, version, d, md); err != nil {
				h.clear()
				return err
			}
		}
	}
	return nil
}

func (h *Host) clear() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.registered = map[string]Registered{}
	h.pluginVer = map[string]string{}
}

func (h *Host) register(pluginName, version string, d declare.Descriptor, md *functionv1.FunctionMetadata) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if existing, ok := h.registered[d.ID]; ok {
		delete(h.registered, d.ID)
		if existing.Plugin != pluginName {
			delete(h.pluginVer, existing.Plugin)
		}
		owner := existing.Plugin
		if owner == pluginName {
			owner = ""
		} else {
			owner = " already owned by " + owner
		}
		return fmt.Errorf("function plugin %q declared duplicate id %q%s", pluginName, d.ID, owner)
	}
	h.registered[d.ID] = Registered{Plugin: pluginName, Version: version, Declared: d, Metadata: md}
	h.pluginVer[pluginName] = version
	return nil
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

// Stop drains in-flight calls within the bounded window and shuts down
// every resident plugin process (single lifecycle owner: the host).
func (h *Host) Stop() { h.ctrl.Stop() }
