//go:build !no_go_plugin

package pluginhost

import "github.com/hashicorp/go-plugin"

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
