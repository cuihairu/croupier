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
	"regexp"
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

// metricNameRe 提取 PromQL 中的裸指标名（跳过聚合/函数关键字、标签、数值）。
// stringLiteralRe 剥离字符串字面量（标签值），rangeRe 剥离范围时长（如 [5m]）——
// 它们的内部 token 不是标识符。
var stringLiteralRe = regexp.MustCompile(`"[^"]*"`)
var rangeRe = regexp.MustCompile(`\[[0-9]+[smhdwy]`)
var metricNameRe = regexp.MustCompile(`[a-zA-Z_:][a-zA-Z0-9_:]*`)

// promQLFuncs 是允许出现的聚合/函数关键字（白名单语义不构成指标）。
var promQLFuncs = map[string]struct{}{
	"sum": {}, "min": {}, "max": {}, "avg": {}, "count": {}, "count_values": {},
	"bottomk": {}, "topk": {}, "quantile": {}, "stddev": {}, "stdvar": {},
	"rate": {}, "irate": {}, "increase": {}, "delta": {}, "idelta": {},
	"by": {}, "without": {}, "on": {}, "ignoring": {}, "group_left": {},
	"group_right": {}, "offset": {}, "bool": {}, "and": {}, "or": {}, "unless": {},
}

// allowedLabelKeys 是白名单指标的已知标签键（by/on 分组与选择器中
// 合法出现）——封闭集：haproxy exporter 标签 + Prometheus 注入标签。
var allowedLabelKeys = map[string]struct{}{
	"proxy": {}, "server": {}, "backend": {}, "state": {}, "type": {},
	"instance": {}, "job": {}, "name": {}, "addr": {}, "iid": {},
}

// allowedQuery 白名单校验：查询体内出现的每个标识符必须属于
// 白名单指标前缀 / 聚合函数关键字 / 已知标签键 三者之一——支持
// `sum by (backend) (haproxy_backend_current_sessions)` 等表达式；
// 含任一其它标识符（go_goroutines、up 等内部指标）即拒绝。
func allowedQuery(q string) bool {
	// 剥离字符串字面量（标签值）与范围时长（[5m] 的 5m），它们不是标识符
	q = stringLiteralRe.ReplaceAllString(q, "")
	q = rangeRe.ReplaceAllString(q, "[")
	found := false
	for _, tok := range metricNameRe.FindAllString(q, -1) {
		if _, isFunc := promQLFuncs[tok]; isFunc {
			continue
		}
		if _, isLabel := allowedLabelKeys[tok]; isLabel {
			continue
		}
		matched := false
		for _, prefix := range allowedMetricPrefixes {
			if strings.HasPrefix(tok, prefix) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
		found = true
	}
	return found
}
