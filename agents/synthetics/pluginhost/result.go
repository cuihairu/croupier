// Package pluginhost implements the synthetics-agent side of the external
// probe plugin runtime (todo.md:36). It hosts HashiCorp go-plugin
// processes speaking the local gRPC mode, runs call concurrency under a
// bounded queue, restarts crashed plugin processes with exponential
// backoff, and circuit-breaks consecutive plugin failures into explicit
// degradation causes — the native HTTP probe remains the fallback of
// last resort.
//
// The gRPC dependency is confined to this package, the generated
// protocol package, and plugin examples; game SDKs and the Agent↔Server
// link never import it (docs/archive/legacy-en/architecture/transport-no-grpc.md).
// Building with the no_go_plugin tag compiles only the stub in this
// package (stub.go), keeping go-plugin/gRPC/protocol code out of the
// binary; the controller, plugins, handshake and generated protocol live
// behind the !no_go_plugin tag.
package pluginhost

import (
	"github.com/cuihairu/croupier/agents/synthetics/probe"
)

// Degradation causes returned when a plugin cannot answer. Each cause is
// prefixed so runner.errorClass() can classify it; the caller falls back
// to the native HTTP probe and records the cause verbatim in probe
// details, keeping plugin faults distinguishable from target faults.
const (
	// CauseNotConfigured: the target names a plugin that is not declared.
	CauseNotConfigured = "plugin_not_configured"
	// CauseUnavailable: the plugin process could not start or exited.
	CauseUnavailable = "plugin_unavailable"
	// CauseTimeout: no answer within the per-check deadline.
	CauseTimeout = "plugin_timeout"
	// CauseError: the plugin call failed with a non-exit, non-timeout error.
	CauseError = "plugin_error"
	// CauseDisabled: the runtime master switch or the per-plugin switch
	// is off, or (trimmed builds) the runtime is compiled out.
	CauseDisabled    = "plugin_disabled"
	CauseNotCompiled = "plugin_not_compiled"
	// CauseBackpressure: the concurrency limit was saturated and no slot
	// was freed within the bounded queue window.
	CauseBackpressure = "plugin_backpressure"
	// CauseBackoff: a previous crash/failure still gates the next restart.
	CauseBackoff = "plugin_backoff"
	// CauseCircuitOpen: consecutive plugin failures tripped the circuit.
	CauseCircuitOpen = "plugin_circuit_open"
)

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
