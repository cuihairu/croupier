package lbstats

import "testing"

func TestAllowedQuery(t *testing.T) {
	ok := []string{
		"haproxy_server_status",
		"sum by (backend) (haproxy_backend_current_sessions)",
		"sum by (backend) (rate(haproxy_backend_errors_total[5m]))",
		"topk(5, haproxy_backend_current_sessions)",
		"haproxy_server_status{proxy=\"croupier_agent_lb\"}",
	}
	for _, q := range ok {
		if !allowedQuery(q) {
			t.Errorf("allowedQuery(%q) = false, want true", q)
		}
	}

	bad := []string{
		"",              // 空查询
		"sum(1)",        // 无指标
		"go_goroutines", // 非白名单指标
		"sum(haproxy_backend_current_sessions) + go_goroutines", // 混入内部指标
		"up", // 通用指标（防探测后端拓扑）
	}
	for _, q := range bad {
		if allowedQuery(q) {
			t.Errorf("allowedQuery(%q) = true, want false", q)
		}
	}
}
