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

// AtlasOptions atlas 适配器专属选项（atlas 仓已在本机对齐，2026-10-11；
// 路径模板 + 响应字段映射可配，映射不到的字段按零值处理）。
type AtlasOptions struct {
	// StatusPath 状态查询路径模板。含 {id} 占位符=路径参数替换（atlas
	// 真实口径 /v1/discovery/servers/{id}，无需鉴权）；否则拼 query
	// 参数 ?{matchKey}={match}（通用可配口径）。
	StatusPath string
	// MatchKey 定位参数名（契约键，默认 "agentId"；按 matchBy 可换 host/ip）。
	MatchKey string
	// Fields 响应字段映射（配置驱动，不写死供应商字段名——alertInbound
	// LabelMapping 同款先例）。键=契约字段名（healthy / inMaintenance /
	// windowStart / windowEnd / windowNote），值=atlas 响应 JSON 键。
	Fields map[string]string
	// InMaintenanceStatus 状态枚举值口径：映射到的 inMaintenance 字段不是
	// bool 时（atlas status 字段是字符串枚举）按此值判等。默认 "maintenance"。
	InMaintenanceStatus string
}

// DefaultAtlasOptions 返回默认适配选项（对齐 atlas 真实 API：
// /v1/discovery/servers/{id} 返回单对象，status 字段字符串枚举，id 定位）。
func DefaultAtlasOptions() AtlasOptions {
	return AtlasOptions{
		StatusPath:          "/v1/discovery/servers/{id}",
		MatchKey:            "agentId",
		InMaintenanceStatus: "maintenance",
		Fields: map[string]string{
			"agentId":       "id",
			"inMaintenance": "status",
		},
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
// ErrProviderUnavailable，gate 按 fail-open 照报标注）。配置侧字段映射是
// 部分覆盖（只写要改的键），故在此与默认映射合并——否则只配 inMaintenance
// 就丢 agentId→id 默认，匹配必错。
func NewAtlasProvider(opts Options) *AtlasProvider {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	def := DefaultAtlasOptions()
	fields := make(map[string]string, len(def.Fields))
	for k, v := range def.Fields {
		fields[k] = v
	}
	for k, v := range opts.Atlas.Fields {
		if v != "" {
			fields[k] = v
		}
	}
	a := opts.Atlas
	a.StatusPath = atOr(a.StatusPath, def.StatusPath)
	a.MatchKey = atOr(a.MatchKey, def.MatchKey)
	a.InMaintenanceStatus = atOr(a.InMaintenanceStatus, def.InMaintenanceStatus)
	a.Fields = fields
	return &AtlasProvider{
		baseURL: strings.TrimRight(opts.BaseURL, "/"),
		token:   opts.Token,
		client:  &http.Client{Timeout: timeout},
		opts:    a,
	}
}

// atOr 空值回落默认（配置部分覆盖口径）。
func atOr(v, def string) string {
	if v != "" {
		return v
	}
	return def
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
	u := p.queryURL(statusPath, key, match)
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+listPath(statusPath), nil)
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

// queryURL 拼状态查询 URL：StatusPath 含 {id} 占位符=路径参数替换
// （atlas 真实口径 /v1/discovery/servers/{id}，无需 admin 鉴权；路径段用
// PathEscape——QueryEscape 的空格变 +，路径里是字面加号不是空格）；否则
// 拼 query 参数 ?{matchKey}={match}（通用可配口径）。
func (p *AtlasProvider) queryURL(statusPath, key, match string) string {
	if strings.Contains(statusPath, "{id}") {
		return p.baseURL + strings.ReplaceAll(statusPath, "{id}", url.PathEscape(match))
	}
	return p.baseURL + statusPath + "?" + key + "=" + url.QueryEscape(match)
}

// listPath 列表查询路径：StatusPath 的 {id} 占位符截断掉
// （/v1/discovery/servers/{id} → /v1/discovery/servers；尾部斜杠一并去掉，
// 否则对 atlas ServeMux 是 301 重定向绕路）。
func listPath(statusPath string) string {
	if i := strings.Index(statusPath, "{"); i >= 0 {
		if trimmed := strings.TrimRight(statusPath[:i], "/"); trimmed != "" {
			return trimmed
		}
	}
	return statusPath
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
			InMaintenance: p.inMaintenance(row),
		})
	}
	return out, nil
}

// toStatus 把 atlas 行映射为统一契约；window 三个字段全空则不给 Window。
// inMaintenance 吃两种形态：bool（通用口径）或字符串状态枚举按
// InMaintenanceStatus 判等（atlas status 字段口径："maintenance"=在维护；
// "draining"/"suspect"/"offline" 不抑制——维护是运维显式声明，告警噪音
// 只有维护窗一种）。
func (p *AtlasProvider) toStatus(row map[string]interface{}) ServerStatus {
	st := ServerStatus{
		Healthy:       boolOf(row[p.field("healthy")]),
		InMaintenance: p.inMaintenance(row),
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

// inMaintenance 判定：映射字段是 bool 直取；字符串按 InMaintenanceStatus
// 判等；其他形态按零值（不在维护）。
func (p *AtlasProvider) inMaintenance(row map[string]interface{}) bool {
	v, ok := row[p.field("inMaintenance")]
	if !ok {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	if s, ok := v.(string); ok {
		return s == p.opts.InMaintenanceStatus
	}
	return false
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
