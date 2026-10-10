package serverstatus

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

// AtlasOptions atlas 适配器专属选项（设计 §4：atlas 具体端点/字段待对齐，
// 未对齐前以可配置路径模板 + 响应字段映射落地，映射不到的字段按零值处理）。
type AtlasOptions struct {
	// StatusPath 状态查询路径模板；占位符 {match} 会被替换为定位值
	//（需 url.QueryEscape 后的值）。默认 "/api/servers"。
	StatusPath string
	// MatchKey 定位参数名（默认 "agentId"；按 matchBy 配置可换 host/ip）。
	MatchKey string
	// Fields 响应字段映射（配置驱动，不写死供应商字段名——alertInbound
	// LabelMapping 同款先例）。键=契约字段名（healthy / inMaintenance /
	// windowStart / windowEnd / windowNote），值=atlas 响应里的 JSON 键；
	// 未配置的键用默认小驼峰名。
	Fields map[string]string
}

// DefaultAtlasOptions 返回默认适配选项。
func DefaultAtlasOptions() AtlasOptions {
	return AtlasOptions{
		StatusPath: "/api/servers",
		MatchKey:   "agentId",
	}
}

// AtlasProvider atlas REST 适配器（用户自有服务器状态/维护窗口管理）。
type AtlasProvider struct {
	baseURL string
	token   string
	client  *http.Client
	opts    AtlasOptions
}

// NewAtlasProvider 创建 atlas 适配器。baseURL 为空=未配置（查询恒返回
// ErrProviderUnavailable，gate 按 fail-open 照报标注）。
func NewAtlasProvider(opts Options) *AtlasProvider {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &AtlasProvider{
		baseURL: strings.TrimRight(opts.BaseURL, "/"),
		token:   opts.Token,
		client:  &http.Client{Timeout: timeout},
		opts:    opts.Atlas,
	}
}

// Name 注册表键。
func (p *AtlasProvider) Name() string { return ProviderAtlas }

// GetServerStatus 查询 atlas 维护状态：GET {baseURL}{statusPath}?{matchKey}=
// {match}，取匹配数组首条映射为 ServerStatus。任何传输/解析/映射失败都按
// 错误返回（未知 → gate fail-open 照报标注，绝不静默）。
func (p *AtlasProvider) GetServerStatus(ctx context.Context, ref ServerRef) (ServerStatus, error) {
	if p.baseURL == "" {
		return ServerStatus{}, fmt.Errorf("%w: atlas baseURL not configured", ErrProviderUnavailable)
	}
	match, key, ok := p.matchValue(ref)
	if !ok {
		return ServerStatus{}, fmt.Errorf("%w: no %s in ref", ErrNoMapping, key)
	}
	statusPath := p.opts.StatusPath
	if statusPath == "" {
		statusPath = DefaultAtlasOptions().StatusPath
	}
	u := p.baseURL + statusPath + "?" + key + "=" + url.QueryEscape(match)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return ServerStatus{}, fmt.Errorf("%w: %v", ErrProviderUnavailable, err)
	}
	if p.token != "" {
		req.Header.Set("Authorization", "Bearer "+p.token)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return ServerStatus{}, fmt.Errorf("%w: %v", ErrProviderUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return ServerStatus{}, fmt.Errorf("%w: atlas http %d", ErrProviderUnavailable, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return ServerStatus{}, fmt.Errorf("%w: %v", ErrProviderUnavailable, err)
	}
	return p.mapResponse(body, key, match)
}

// ListServers 拉状态源已知服务器清单（无匹配过滤；失败返回错误不吞）。
func (p *AtlasProvider) ListServers(ctx context.Context) ([]ServerSummary, error) {
	if p.baseURL == "" {
		return nil, fmt.Errorf("%w: atlas baseURL not configured", ErrProviderUnavailable)
	}
	statusPath := p.opts.StatusPath
	if statusPath == "" {
		statusPath = DefaultAtlasOptions().StatusPath
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+statusPath, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrProviderUnavailable, err)
	}
	if p.token != "" {
		req.Header.Set("Authorization", "Bearer "+p.token)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrProviderUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: atlas http %d", ErrProviderUnavailable, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrProviderUnavailable, err)
	}
	return p.mapList(body)
}

// matchKey 返回生效的定位键（MatchKey 配置优先，缺省 agentId）。
func (p *AtlasProvider) matchKey() string {
	if key := p.opts.MatchKey; key != "" {
		return key
	}
	return DefaultAtlasOptions().MatchKey
}

// matchValue 按 MatchKey 从 ref 取定位值（agentId/host/ip 三选一）。
func (p *AtlasProvider) matchValue(ref ServerRef) (string, string, bool) {
	key := p.matchKey()
	switch key {
	case "host":
		return ref.Host, key, ref.Host != ""
	case "ip":
		return ref.IP, key, ref.IP != ""
	default:
		return ref.AgentID, "agentId", ref.AgentID != ""
	}
}

// mapResponse 解析单服务器响应：接受数组（取定位字段匹配的首条）或单对象。
// 匹配字段随 MatchKey 走（agentId/host/ip 各自的 atlas 字段名可映射）。
func (p *AtlasProvider) mapResponse(body []byte, matchKey, match string) (ServerStatus, error) {
	trimmed := strings.TrimSpace(string(body))
	var rows []map[string]interface{}
	if strings.HasPrefix(trimmed, "[") {
		if err := json.Unmarshal(body, &rows); err != nil {
			return ServerStatus{}, fmt.Errorf("%w: decode: %v", ErrProviderUnavailable, err)
		}
	} else {
		var one map[string]interface{}
		if err := json.Unmarshal(body, &one); err != nil {
			return ServerStatus{}, fmt.Errorf("%w: decode: %v", ErrProviderUnavailable, err)
		}
		rows = []map[string]interface{}{one}
	}
	matchField := p.field(matchKey)
	for _, row := range rows {
		if v, _ := row[matchField].(string); v == match {
			return p.toStatus(row), nil
		}
	}
	return ServerStatus{}, fmt.Errorf("%w: no %s=%s row", ErrNoMapping, matchField, match)
}

// mapList 解析服务器清单。
func (p *AtlasProvider) mapList(body []byte) ([]ServerSummary, error) {
	trimmed := strings.TrimSpace(string(body))
	var rows []map[string]interface{}
	if strings.HasPrefix(trimmed, "[") {
		if err := json.Unmarshal(body, &rows); err != nil {
			return nil, fmt.Errorf("%w: decode: %v", ErrProviderUnavailable, err)
		}
	} else {
		var wrapper struct {
			Servers []map[string]interface{} `json:"servers"`
			Items   []map[string]interface{} `json:"items"`
		}
		if err := json.Unmarshal(body, &wrapper); err != nil {
			return nil, fmt.Errorf("%w: decode: %v", ErrProviderUnavailable, err)
		}
		rows = wrapper.Servers
		if len(rows) == 0 {
			rows = wrapper.Items
		}
	}
	out := make([]ServerSummary, 0, len(rows))
	for _, row := range rows {
		out = append(out, ServerSummary{
			AgentID:       strOf(row[p.field("agentId")]),
			Host:          strOf(row[p.field("host")]),
			Healthy:       boolOf(row[p.field("healthy")]),
			InMaintenance: boolOf(row[p.field("inMaintenance")]),
		})
	}
	return out, nil
}

// toStatus 把 atlas 行映射为统一契约；window 三个字段全空则不给 Window。
func (p *AtlasProvider) toStatus(row map[string]interface{}) ServerStatus {
	st := ServerStatus{
		Healthy:       boolOf(row[p.field("healthy")]),
		InMaintenance: boolOf(row[p.field("inMaintenance")]),
		Source:        ProviderAtlas,
	}
	start, sok := row[p.field("windowStart")].(string)
	end, eok := row[p.field("windowEnd")].(string)
	note, _ := row[p.field("windowNote")].(string)
	if sok || eok {
		w := &MaintenanceWindow{Note: note}
		if sok {
			w.Start = parseTimeOrZero(start)
		}
		if eok {
			w.End = parseTimeOrZero(end)
		}
		st.Window = w
	}
	return st
}

// field 取契约字段名→atlas JSON 键的映射（未配置用契约默认名）。
func (p *AtlasProvider) field(contract string) string {
	if v, ok := p.opts.Fields[contract]; ok && v != "" {
		return v
	}
	return contract
}

func strOf(v interface{}) string {
	s, _ := v.(string)
	return s
}

func boolOf(v interface{}) bool {
	b, _ := v.(bool)
	return b
}

func parseTimeOrZero(s string) time.Time {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
