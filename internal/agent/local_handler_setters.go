package agent

import (
	"log/slog"
	"strconv"
	"strings"
	"time"

	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	"github.com/cuihairu/croupier/internal/platform/tlsutil"
)

// SetProviderSessionStore enables callback over the Provider's established
// TCP session instead of dialing an address supplied by the Provider.
func (h *LocalHandler) SetProviderSessionStore(store *ProviderSessionStore) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.providerSessions = store
}

// NewLocalHandler creates a new LocalHandler instance
func NewLocalHandler(store *agentlocal.LocalStore, configDir, agentID string, logger *slog.Logger) *LocalHandler {
	if logger == nil {
		logger = slog.Default()
	}
	h := &LocalHandler{
		store:     store,
		logger:    logger,
		configDir: configDir,
		agentID:   agentID,
		// Default aligned with the Server dispatch layer's invokeTimeout (15s),
		// eliminating the old 10s<15s inversion.
		providerCallTimeout: 15 * time.Second,
	}
	// TaskRunner executes tasks via the handler's invoke path.
	h.tasks = NewTaskRunner(h.executeTask, nil, logger)
	return h
}

// SetProviderCallTimeout configures the default budget for synchronous
// Agent → Provider calls. Non-positive values fall back to the default 15s;
// the cap is 60s (the synchronous channel boundary — longer operations should
// go through async tasks).
func (h *LocalHandler) SetProviderCallTimeout(d time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if d <= 0 {
		d = 15 * time.Second
	}
	if d > 60*time.Second {
		d = 60 * time.Second
	}
	h.providerCallTimeout = d
}

// providerCallDeadline computes the timeout for this Provider call: the
// timeout_ms declared in request metadata is the authoritative budget (clamped
// to a hard [1s, 60s] ceiling); when nothing is declared, the configured
// default applies. Garbage values → configured default.
func (h *LocalHandler) providerCallDeadline(meta map[string]string) time.Duration {
	h.mu.RLock()
	def := h.providerCallTimeout
	h.mu.RUnlock()
	if def <= 0 {
		def = 15 * time.Second
	}
	raw := strings.TrimSpace(meta["timeoutMs"])
	if raw == "" {
		return def
	}
	ms, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || ms <= 0 {
		return def
	}
	budget := time.Duration(ms) * time.Millisecond
	if budget < time.Second {
		budget = time.Second
	}
	if budget > 60*time.Second {
		budget = 60 * time.Second
	}
	return budget
}

// SetProviderManager sets the provider manager
func (h *LocalHandler) SetProviderManager(pm ProviderManager) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.pm = pm
}

// SetPluginFunctionManager wires the function adapter plugin dispatch
// (todo.md:35); nil disables plugin dispatch (default, no plugins).
func (h *LocalHandler) SetPluginFunctionManager(m PluginFunctionManager) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.pluginFuncs = m
}

// SetOpsServer sets the ops server wrapper
func (h *LocalHandler) SetOpsServer(ops OpsServerWrapper) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.opsServer = ops
}

func (h *LocalHandler) SetTaskEventReporter(reporter TaskEventReporter) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.reporter = reporter
	if h.tasks != nil {
		h.tasks.SetReporter(reporter)
	}
}

// SetTLSConfig sets the TLS config for outbound connections
func (h *LocalHandler) SetTLSConfig(cfg *tlsutil.ClientTLSConfig) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.tlsCfg = cfg
}

// SetExpectedGameEnv sets the agent's expected gameId/env, used to validate
// SDK provider registrations. When set (non-empty), a provider registering with
// a mismatched game_id/env triggers a warning.
func (h *LocalHandler) SetExpectedGameEnv(gameID, env string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.expectedGameID = gameID
	h.expectedEnv = env
}
