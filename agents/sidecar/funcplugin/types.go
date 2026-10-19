package funcplugin

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cuihairu/croupier/agents/sidecar/funcplugin/declare"
	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
)

// This file holds the build-independent contract types (todo.md:35):
// both the default and the no_go_plugin build compile them, so the
// assembly layer and tests see one API shape. The go-plugin/gRPC
// machinery lives in the tagged contract.go/host.go (default build) and
// stub.go (trimmed build).

// AgentScope is the sidecar's own game/env binding. It is the ONLY scope
// a plugin function can ever carry: plugins do not declare scope, and a
// blank agent scope refuses to start plugin hosting at all (same
// invariant as internal/app/agent.ValidateAgentScope, enforced here as
// defense in depth because agents/ cannot import internal/).
type AgentScope struct {
	GameID string
	Env    string
}

// Validate fails closed on a blank scope (blank after trim, same 口径 as
// internal/app/agent.ValidateAgentScope): plugin functions inherit the
// sidecar scope, so "no scope" must never silently become "no scope
// check".
func (s AgentScope) Validate() error {
	if strings.TrimSpace(s.GameID) == "" || strings.TrimSpace(s.Env) == "" {
		return fmt.Errorf("function plugins require a non-blank agent gameId/env scope (plugin functions inherit the sidecar scope)")
	}
	return nil
}

// PluginConfig declares one external function adapter plugin process.
// Commands are explicit config entries: nothing is launched unless the
// sidecar config declares it.
type PluginConfig struct {
	Name    string
	Command string
	Args    []string
	Timeout time.Duration
	// Enabled is the per-plugin runtime switch; nil defaults to enabled.
	Enabled *bool
}

// IsEnabled reports the per-plugin runtime switch; nil defaults to
// enabled (默认集成).
func (p PluginConfig) IsEnabled() bool { return p.Enabled == nil || *p.Enabled }

// InvokeInput is the host→plugin invoke call payload.
type InvokeInput struct {
	// FunctionID is the registered plugin-namespaced function id.
	FunctionID string
	// ParamsJSON is the JSON-encoded params object (the sidecar invoke
	// chain passes function payloads as UTF-8 JSON in protocol v1).
	ParamsJSON string
	// Timeout is the remaining host-side deadline.
	Timeout time.Duration
}

// InvokeOutput is the plugin→host invoke result. Business failures ride
// the closed-set error envelope (OK=false), never a Go error: a Go
// error means the plugin process itself is broken, which the host
// classifies into a plugin_* degradation cause.
type InvokeOutput struct {
	OK           bool
	ResultJSON   string
	ErrorCode    string
	ErrorMessage string
	At           time.Time
}

// FuncPlugin is the language-neutral contract a function adapter plugin
// implements. Go plugins embed it via Serve; non-Go plugins implement
// the generated FunctionPlugin gRPC service over the same contract.
type FuncPlugin interface {
	Describe(ctx context.Context) (name, version string, err error)
	ListFunctions(ctx context.Context) ([]declare.Descriptor, error)
	Invoke(ctx context.Context, in InvokeInput) (InvokeOutput, error)
}

// Registered is one validated plugin function: the declared descriptor,
// the plugin identity/version, and the shared function metadata the
// registration/approval/audit chain consumes.
type Registered struct {
	Plugin   string
	Version  string
	Declared declare.Descriptor
	Metadata *functionv1.FunctionMetadata
}
