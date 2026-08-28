// Package lbstats 提供 LB 监控的只读代理：转发受限的 PromQL 查询到
// 配置的 Prometheus（ops.prometheusUrl / CROUPIER_LB_PROMETHEUS_URL）。
// 白名单限定 LB 监控指标前缀（docs/operations/load-balancing.md「LB 监控」），
// 平台 RBAC 鉴权在上层 handler；未配置 Prometheus 时功能整体隐藏。
package lbstats

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// 允许代理查询的指标前缀（LB 监控域，防任意 PromQL/SSRF 滥用）。
var allowedMetricPrefixes = []string{
	"haproxy_",
}

// Service 代理受限的 PromQL 即时查询。
type LBStatsService struct {
	promURL string
	client  *http.Client
}

func NewLBStatsService(promURL string) *LBStatsService {
	return &LBStatsService{
		promURL: strings.TrimRight(strings.TrimSpace(promURL), "/"),
		client:  &http.Client{Timeout: 5 * time.Second},
	}
}

// Enabled 报告 LB 监控是否配置（未配置时前端隐藏 /ops/lb）。
func (s *LBStatsService) Enabled() bool { return s != nil && s.promURL != "" }

// QueryResult 是 Prometheus /api/v1/query 的最小化响应形状。
type QueryResult struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []interface{}     `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

// Query 执行白名单内的即时查询，返回裁剪后的结果。
func (s *LBStatsService) Query(ctx context.Context, query string) (*QueryResult, error) {
	if s == nil || s.promURL == "" {
		return nil, fmt.Errorf("prometheus url not configured")
	}
	query = strings.TrimSpace(query)
	if !allowedQuery(query) {
		return nil, fmt.Errorf("query not allowed: only %v metrics", allowedMetricPrefixes)
	}
	q := url.Values{"query": []string{query}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		s.promURL+"/api/v1/query?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("prometheus query: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prometheus query failed: status %s", resp.Status)
	}
	var out QueryResult
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("prometheus response: %w", err)
	}
	return &out, nil
}

// allowedQuery 白名单校验：仅允许以 LB 指标前缀开头的即时查询
// （{...} 选择器语法允许，因为前缀在 { 之前）。
func allowedQuery(q string) bool {
	head := q
	if i := strings.IndexAny(q, "{[ \t"); i >= 0 {
		head = q[:i]
	}
	for _, prefix := range allowedMetricPrefixes {
		if strings.HasPrefix(head, prefix) {
			return true
		}
	}
	return false
}
