// Package pluginhost implements the synthetics-agent side of the external
// probe plugin runtime (todo.md:36). It hosts HashiCorp go-plugin
// processes speaking the local gRPC mode on top of the shared resident
// host engine (core/pluginhost, todo.md:35 — the same lifecycle engine
// the sidecar function host runs on), and maps probe outcomes onto the
// shared probe result so plugin results traverse the existing
// AlertEvent / probe_events chain unchanged.
//
// The gRPC dependency is confined to the plugin host boundary — this
// package, the generated protocol package, plugin examples, and the
// shared core engine; game SDKs and the Agent↔Server link never import
// them (docs/archive/legacy-en/architecture/transport-no-grpc.md).
// Building with the no_go_plugin tag compiles only the stub in this
// package (stub.go), keeping go-plugin/gRPC/protocol code out of the
// binary; the controller, plugins, handshake and generated protocol live
// behind the !no_go_plugin tag.
package pluginhost

import (
	coreplugin "github.com/cuihairu/croupier/core/pluginhost"

	"github.com/cuihairu/croupier/agents/synthetics/probe"
)

// Degradation causes returned when a plugin cannot answer. Single-sourced
// in the shared core engine so every plugin face reports the same
// vocabulary; aliased here for the probe-facing callers (runner tests
// classify causes by these prefixes). The caller falls back to the
// native HTTP probe and records the cause verbatim in probe details,
// keeping plugin faults distinguishable from target faults.
const (
	// CauseNotConfigured: the target names a plugin that is not declared.
	CauseNotConfigured = coreplugin.CauseNotConfigured
	// CauseUnavailable: the plugin process could not start or exited.
	CauseUnavailable = coreplugin.CauseUnavailable
	// CauseTimeout: no answer within the per-check deadline.
	CauseTimeout = coreplugin.CauseTimeout
	// CauseError: the plugin call failed with a non-exit, non-timeout error.
	CauseError = coreplugin.CauseError
	// CauseDisabled: the runtime master switch or the per-plugin switch
	// is off, or (trimmed builds) the runtime is compiled out.
	CauseDisabled    = coreplugin.CauseDisabled
	CauseNotCompiled = coreplugin.CauseNotCompiled
	// CauseBackpressure: the concurrency limit was saturated and no slot
	// was freed within the bounded queue window.
	CauseBackpressure = coreplugin.CauseBackpressure
	// CauseBackoff: a previous crash/failure still gates the next restart.
	CauseBackoff = coreplugin.CauseBackoff
	// CauseCircuitOpen: consecutive plugin failures tripped the circuit.
	CauseCircuitOpen = coreplugin.CauseCircuitOpen
	// CauseCredentialMissing: the target's secretEnv references an env
	// var that is unset (or empty) at check time; the cause names the
	// variable, never its value.
	CauseCredentialMissing = coreplugin.CauseCredentialMissing
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
