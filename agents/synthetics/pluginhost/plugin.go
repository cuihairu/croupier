// Package pluginhost implements the synthetics-agent side of the external
// probe plugin runtime (todo.md:36, slice 1). It hosts HashiCorp go-plugin
// processes speaking the local gRPC mode, and degrades to the plugin's
// cause when a plugin process cannot be launched, fails a call, or times
// out — the native HTTP probe remains the fallback of last resort.
//
// The gRPC dependency is confined to this package, the generated
// protocol package, and plugin examples; game SDKs and the Agent↔Server
// link never import it (docs/archive/legacy-en/architecture/transport-no-grpc.md).
package pluginhost

import (
	"github.com/hashicorp/go-plugin"
)

const (
	// PluginName is the go-plugin dispense key both sides agree on.
	PluginName = "probe"
	// ProtocolVersion of the handshake. The wire contract is versioned
	// separately from the Agent↔Server TCP protocol.
	ProtocolVersion = 1
	// CookieKey / CookieValue authenticate that the subprocess really is a
	// Croupier probe plugin and not an arbitrary binary launched by mistake.
	CookieKey   = "CROUPIER_SYNTHETICS_PLUGIN"
	CookieValue = "croupier-synthetics-probe-plugin"
)

// Handshake is shared by host and plugin processes to authenticate and
// pin the protocol before any gRPC traffic.
var Handshake = plugin.HandshakeConfig{
	ProtocolVersion:  ProtocolVersion,
	MagicCookieKey:   CookieKey,
	MagicCookieValue: CookieValue,
}
