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
		// 默认对齐 Server 派发层 invokeTimeout（15s），消除旧 10s<15s 倒挂。
		providerCallTimeout: 15 * time.Second,
	}
	// TaskRunner executes tasks via the handler's invoke path.
	h.tasks = NewTaskRunner(h.executeTask, nil, logger)
	return h
}

// SetProviderCallTimeout 配置 Agent → Provider 同步调用默认预算。
// 非正值回落默认 15s；上限 60s（同步通道边界，更长操作应走异步任务）。
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

// providerCallDeadline 计算本次 Provider 调用的超时：请求 metadata 声明的
// timeout_ms 是权威预算（clamp [1s, 60s] 硬顶），无声明时用配置默认。
// 垃圾值 → 配置默认。
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
