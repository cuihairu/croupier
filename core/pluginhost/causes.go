// Package pluginhost is the agent-agnostic resident go-plugin host engine
// shared by the role-side plugin hosts (todo.md:35): the synthetics probe
// host and the sidecar function host both run on this single lifecycle
// implementation instead of keeping two copies of the concurrency gate,
// crash backoff and circuit breaker.
//
// The package holds no plugin business semantics: a role supplies a
// Contract (dispense key, handshake, gRPC server/client adapters) plus
// per-plugin process configs, and drives calls through Call with its own
// typed closure. Degradation causes are shared verbatim so every plugin
// face reports the same vocabulary.
//
// The gRPC dependency is confined to the plugin host boundary (see
// scripts/check-no-grpc.sh): game SDKs and the Agent↔Server link never
// import this package. Building with the no_go_plugin tag drops the
// engine file entirely — role hosts compile their own tagged stubs, so
// nothing here links into trimmed binaries.
package pluginhost

// Degradation causes returned when a plugin cannot answer. Each cause is
// prefixed so callers can classify it and record it verbatim in details,
// keeping plugin faults distinguishable from target faults. Shared by the
// probe and function plugin faces (and visible to trimmed builds, which
// degrade with CauseNotCompiled before any process exists).
const (
	// CauseNotConfigured: the caller names a plugin that is not declared.
	CauseNotConfigured = "plugin_not_configured"
	// CauseUnavailable: the plugin process could not start or exited.
	CauseUnavailable = "plugin_unavailable"
	// CauseTimeout: no answer within the per-call deadline.
	CauseTimeout = "plugin_timeout"
	// CauseError: the plugin call failed with a non-exit, non-timeout error.
	CauseError = "plugin_error"
	// CauseDisabled: a runtime master switch or per-plugin switch is off,
	// or (trimmed builds) the runtime is compiled out.
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
