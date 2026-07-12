package utils

import (
	"net"
	"strconv"
	"strings"
	"time"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
)

// BuildOpsAgentSnapshot converts a registry agent session into a response map.
// Caller must ensure the session won't be mutated concurrently or hold the store lock while calling.
func BuildOpsAgentSnapshot(sess *reg.AgentSession) map[string]interface{} {
	if sess == nil {
		return nil
	}

	agentID := strings.TrimSpace(sess.AgentID)
	if agentID == "" {
		return nil
	}

	var (
		ttl     int
		healthy bool
	)
	if !sess.ExpireAt.IsZero() {
		ttl = int(time.Until(sess.ExpireAt).Seconds())
		if ttl < 0 {
			ttl = 0
		} else if ttl > 0 {
			healthy = true
		}
	}

	snapshot := map[string]interface{}{
		"id":        agentID,
		"agent_id":  agentID,
		"game_id":   sess.GameID,
		"env":       sess.Env,
		"type":      firstNonEmpty(sess.Labels["type"], "agent"),
		"addr":      sess.Addr,
		"rpc_addr":  sess.Addr, // compatibility alias; prefer "addr"
		"ip":        guessAgentIP(sess.Addr),
		"version":   sess.Version,
		"region":    sess.Region,
		"zone":      sess.Zone,
		"labels":    sess.Labels,
		"functions": CountEnabledFunctions(sess.Functions),
		"providers": buildProviders(sess.Providers),
		"providers_count": func() int {
			if sess.Providers == nil {
				return 0
			}
			return len(sess.Providers)
		}(),
		"healthy":        healthy,
		"expires_in_sec": ttl,
		"last_seen":      guessAgentLastSeen(sess.ExpireAt),
	}

	injectMetrics(snapshot, sess.Labels)
	ensureMetricDefaults(snapshot)
	return snapshot
}

func buildProviders(providers []reg.ProviderSession) []map[string]interface{} {
	if len(providers) == 0 {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(providers))
	for _, p := range providers {
		pid := strings.TrimSpace(p.ProviderID)
		if pid == "" {
			continue
		}
		fnCount := 0
		if p.FunctionIDs != nil {
			fnCount = len(p.FunctionIDs)
		}
		out = append(out, map[string]interface{}{
			"provider_id":    pid,
			"game_id":        strings.TrimSpace(p.GameID),
			"env":            strings.TrimSpace(p.Env),
			"addr":           strings.TrimSpace(p.Addr),
			"version":        strings.TrimSpace(p.Version),
			"last_seen_unix": p.LastSeenUnix,
			"function_ids":   p.FunctionIDs,
			"functions":      fnCount,
		})
	}
	return out
}

// CountEnabledFunctions returns the number of enabled functions registered on the agent.
func CountEnabledFunctions(functions map[string]reg.FunctionMeta) int {
	count := 0
	for _, meta := range functions {
		if meta.Enabled {
			count++
		}
	}
	return count
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func guessAgentIP(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	if strings.Contains(addr, ":///") {
		parts := strings.Split(addr, ":///")
		addr = parts[len(parts)-1]
	}
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	if idx := strings.LastIndex(addr, ":"); idx > 0 {
		return strings.TrimSpace(addr[:idx])
	}
	return addr
}

func guessAgentLastSeen(expireAt time.Time) string {
	if expireAt.IsZero() {
		return ""
	}
	lastSeen := expireAt.Add(-60 * time.Second)
	return FormatTimestamp(lastSeen)
}

func injectMetrics(snapshot map[string]interface{}, labels map[string]string) {
	if labels == nil {
		return
	}
	if v, ok := parseFloatLabel(labels, "stats.qps_1m", "qps_1m"); ok {
		snapshot["qps_1m"] = v
	}
	if v, ok := parseFloatLabel(labels, "stats.error_rate", "error_rate"); ok {
		snapshot["error_rate"] = v
	}
	if v, ok := parseFloatLabel(labels, "stats.avg_latency_ms", "avg_latency_ms"); ok {
		snapshot["avg_latency_ms"] = v
	}
	if v, ok := parseFloatLabel(labels, "stats.qps_limit", "qps_limit"); ok {
		snapshot["qps_limit"] = v
	}
	if v, ok := parseIntLabel(labels, "stats.active_conns", "active_conns"); ok {
		snapshot["active_conns"] = v
	}
	if v, ok := parseIntLabel(labels, "stats.total_requests", "total_requests"); ok {
		snapshot["total_requests"] = v
	}
	if v, ok := parseIntLabel(labels, "stats.failed_requests", "failed_requests"); ok {
		snapshot["failed_requests"] = v
	}
}

func parseFloatLabel(labels map[string]string, keys ...string) (float64, bool) {
	for _, key := range keys {
		if val, ok := labels[key]; ok {
			if f, err := strconv.ParseFloat(strings.TrimSpace(val), 64); err == nil {
				return f, true
			}
		}
	}
	return 0, false
}

func parseIntLabel(labels map[string]string, keys ...string) (int64, bool) {
	for _, key := range keys {
		if val, ok := labels[key]; ok {
			if i, err := strconv.ParseInt(strings.TrimSpace(val), 10, 64); err == nil {
				return i, true
			}
		}
	}
	return 0, false
}

func ensureMetricDefaults(snapshot map[string]interface{}) {
	if _, ok := snapshot["qps_1m"]; !ok {
		snapshot["qps_1m"] = 0.0
	}
	if _, ok := snapshot["error_rate"]; !ok {
		snapshot["error_rate"] = 0.0
	}
	if _, ok := snapshot["avg_latency_ms"]; !ok {
		snapshot["avg_latency_ms"] = 0.0
	}
	if _, ok := snapshot["qps_limit"]; !ok {
		snapshot["qps_limit"] = 0.0
	}
	if _, ok := snapshot["active_conns"]; !ok {
		snapshot["active_conns"] = int64(0)
	}
	if _, ok := snapshot["total_requests"]; !ok {
		snapshot["total_requests"] = int64(0)
	}
	if _, ok := snapshot["failed_requests"]; !ok {
		snapshot["failed_requests"] = int64(0)
	}
}
