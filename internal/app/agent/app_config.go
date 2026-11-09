package agent

import (
	"crypto/tls"
	"time"

	"github.com/cuihairu/croupier/internal/platform/tlsutil"
)

// SetProviderCallTimeout 配置 Agent → Provider 同步调用默认预算（透传
// LocalHandler；非正值/超上限在 LocalHandler 内归一）。
func (a *App) SetProviderCallTimeout(d time.Duration) {
	if a == nil || a.localHandler == nil {
		return
	}
	a.localHandler.SetProviderCallTimeout(d)
}

// WithUpstreamMetadata updates metadata fields propagated to the control server.
func (a *App) WithUpstreamMetadata(meta UpstreamMetadata) {
	if a == nil || a.upstream == nil {
		return
	}
	a.upstream.WithMetadata(meta)
	// 同步 agent 期望的 game_id/env 给 LocalHandler，用于校验 SDK 注册
	if a.localHandler != nil {
		a.localHandler.SetExpectedGameEnv(meta.GameID, meta.Env)
	}
}

func (a *App) WithUpstreamTLSConfig(cfg *tlsutil.ClientTLSConfig) {
	if a == nil || a.upstream == nil {
		return
	}
	a.upstream.SetTLSConfig(cfg)
}

// SetUpstreamTransportKind sets the control-plane transport kind for server connections.
func (a *App) SetUpstreamTransportKind(kind string) {
	if a == nil || a.upstream == nil {
		return
	}
	a.upstream.SetTransportKind(kind)
}

// OnConnected sets a callback to be invoked when the agent successfully connects to the server.
// The callback is called after successful registration. The recorder (if
// armed) fires Connected first; the user callback still runs.
func (a *App) OnConnected(callback func()) {
	if a == nil {
		return
	}
	a.userOnConnected = callback
	a.armRecorderChaining()
}

// OnDisconnected sets a callback to be invoked when the agent disconnects from the server.
// The callback is called with the error that caused the disconnection. The
// recorder (if armed) fires Disconnected first.
func (a *App) OnDisconnected(callback func(error)) {
	if a == nil {
		return
	}
	a.userOnDisconnected = callback
	a.armRecorderChaining()
}

func (a *App) WithOutboundTLSConfig(cfg *tlsutil.ClientTLSConfig) {
	if a == nil {
		return
	}
	a.outTLS = cfg
}

// WithLocalTLSConfig enables TLS on the Agent's local gateway — the listener
// SDK Providers dial into (configs/agent.yaml `tls:` section). Nil (default)
// keeps plain TCP.
func (a *App) WithLocalTLSConfig(cfg *tls.Config) {
	if a == nil {
		return
	}
	a.localTLS = cfg
}

// WithOpsConfig sets the ops module configuration.
// Call this before RegisterGRPC to configure ops capabilities.
func (a *App) WithOpsConfig(cfg *OpsConfig) {
	if a == nil {
		return
	}
	a.opsConfig = cfg

	// Enable metrics reporting if ops is configured
	if cfg != nil && cfg.MetricsEnabled {
		a.upstream.WithMetricsReporting(cfg.MetricsInterval)
	}
}

// WithVersion sets the agent version for reporting.
func (a *App) WithVersion(version string) {
	if a == nil {
		return
	}
	a.version = version
}
