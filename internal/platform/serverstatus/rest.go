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

// restSource REST 形态状态源的共享引擎（atlas / http 两个适配器的公共实现，
// 批 S4 抽取）：GET {baseURL}{statusPath}（{id} 占位符=路径参数替换，否则
// 拼 query 参数定位）→ JSON 数组/单对象 → Fields 映射为统一契约。传输/解析
// 失败一律按 ErrProviderUnavailable/ErrNoMapping 错误闭集返回，gate 按
// fail-open 处理（照报标注，绝不静默）。两个适配器的差异只在缺省选项与
// 单对象接受口径，见各自构造函数。
type restSource struct {
	baseURL string
	token   string
	client  *http.Client
	opts    AtlasOptions
	// source ServerStatus.Source 标签 + 错误文案主语（注册表键）。
	source string
	// acceptUnmatchedSingle 单对象响应不含匹配键字段时整体接受（http 源
	// 「URL 即标识」口径——单服务器状态端点无需回显自身 id）；atlas 关闭
	// （discovery 响应必须携带 id，匹配不上=无映射）。
	acceptUnmatchedSingle bool
}

// newRestSource 构造共享引擎（opts.Atlas 由调用方先解析好缺省值）。
func newRestSource(opts Options, a AtlasOptions, source string) *restSource {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &restSource{
		baseURL: strings.TrimRight(opts.BaseURL, "/"),
		token:   opts.Token,
		client:  &http.Client{Timeout: timeout},
		opts:    a,
		source:  source,
	}
}

// getServerStatus 查询目标服务器状态：GET 拼好的 URL，取匹配行映射为
// ServerStatus。任何传输/解析/映射失败都按错误返回（未知 → gate fail-open）。
func (s *restSource) getServerStatus(ctx context.Context, ref ServerRef) (ServerStatus, error) {
	if s.baseURL == "" {
		return ServerStatus{}, fmt.Errorf("%w: %s baseURL not configured", ErrProviderUnavailable, s.source)
	}
	match, key, ok := s.matchValue(ref)
	if !ok {
		return ServerStatus{}, fmt.Errorf("%w: no %s in ref", ErrNoMapping, key)
	}
	body, err := s.fetch(ctx, s.queryURL(s.opts.StatusPath, key, match))
	if err != nil {
		return ServerStatus{}, err
	}
	return s.mapResponse(body, key, match)
}

// listServers 拉状态源已知服务器清单（无匹配过滤；失败返回错误不吞）。
func (s *restSource) listServers(ctx context.Context) ([]ServerSummary, error) {
	if s.baseURL == "" {
		return nil, fmt.Errorf("%w: %s baseURL not configured", ErrProviderUnavailable, s.source)
	}
	body, err := s.fetch(ctx, s.baseURL+listPath(s.opts.StatusPath))
	if err != nil {
		return nil, err
	}
	return s.mapList(body)
}

// fetch 发 GET（Bearer token 头可选），非 200 与读失败都归一为
// ErrProviderUnavailable；响应体限 1MB（状态 JSON 不该大）。
func (s *restSource) fetch(ctx context.Context, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrProviderUnavailable, err)
	}
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrProviderUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %s http %d", ErrProviderUnavailable, s.source, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrProviderUnavailable, err)
	}
	return body, nil
}

// queryURL 拼状态查询 URL：StatusPath 含 {id} 占位符=路径参数替换（atlas
// 真实口径 /v1/discovery/servers/{id}，无需 admin 鉴权；路径段用
// PathEscape——QueryEscape 的空格变 +，路径里是字面加号不是空格）；否则
// 拼 query 参数 ?{matchKey}={match}（baseUrl 已带固定 query 时用 & 续接）。
func (s *restSource) queryURL(statusPath, key, match string) string {
	if strings.Contains(statusPath, "{id}") {
		return s.baseURL + strings.ReplaceAll(statusPath, "{id}", url.PathEscape(match))
	}
	sep := "?"
	if strings.Contains(s.baseURL+statusPath, "?") {
		sep = "&"
	}
	return s.baseURL + statusPath + sep + key + "=" + url.QueryEscape(match)
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

// matchValue 按 MatchKey 从 ref 取定位值（agentId/host/ip 三选一；
// MatchKey 空=agentId）。
func (s *restSource) matchValue(ref ServerRef) (string, string, bool) {
	switch s.opts.MatchKey {
	case "host":
		return ref.Host, "host", ref.Host != ""
	case "ip":
		return ref.IP, "ip", ref.IP != ""
	default:
		return ref.AgentID, "agentId", ref.AgentID != ""
	}
}

// mapResponse 解析单服务器响应：接受数组（取定位字段匹配的首条）或单对象。
// 匹配字段随 MatchKey 走（agentId/host/ip 各自的 JSON 键名可映射）。
// acceptUnmatchedSingle 时（http 源），单对象不含匹配键字段=URL 即标识，
// 整体接受；携带但值不符仍是无映射。
func (s *restSource) mapResponse(body []byte, matchKey, match string) (ServerStatus, error) {
	trimmed := strings.TrimSpace(string(body))
	var rows []map[string]interface{}
	single := false
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
		single = true
	}
	matchField := s.field(matchKey)
	for _, row := range rows {
		if v, _ := row[matchField].(string); v == match {
			return s.toStatus(row), nil
		}
	}
	if single && s.acceptUnmatchedSingle {
		if _, present := rows[0][matchField]; !present {
			return s.toStatus(rows[0]), nil
		}
	}
	return ServerStatus{}, fmt.Errorf("%w: no %s=%s row", ErrNoMapping, matchField, match)
}

// mapList 解析服务器清单：数组或 {servers|items} 包裹（单服务器状态对象
// 无清单语义，解出空清单不报错）。
func (s *restSource) mapList(body []byte) ([]ServerSummary, error) {
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
			AgentID:       strOf(row[s.field("agentId")]),
			Host:          strOf(row[s.field("host")]),
			Healthy:       boolOf(row[s.field("healthy")]),
			InMaintenance: s.inMaintenance(row),
		})
	}
	return out, nil
}

// toStatus 把状态行映射为统一契约；window 三个字段全空则不给 Window。
// inMaintenance 吃两种形态：bool（通用口径）或字符串状态枚举按
// InMaintenanceStatus 判等（atlas status 字段口径："maintenance"=在维护；
// "draining"/"suspect"/"offline" 不抑制——维护是运维显式声明，告警噪音
// 只有维护窗一种；http 简易源缺省 "true" 字面布尔串）。
func (s *restSource) toStatus(row map[string]interface{}) ServerStatus {
	st := ServerStatus{
		Healthy:       boolOf(row[s.field("healthy")]),
		InMaintenance: s.inMaintenance(row),
		Source:        s.source,
	}
	start, sok := row[s.field("windowStart")].(string)
	end, eok := row[s.field("windowEnd")].(string)
	note, _ := row[s.field("windowNote")].(string)
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
func (s *restSource) inMaintenance(row map[string]interface{}) bool {
	v, ok := row[s.field("inMaintenance")]
	if !ok {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	if str, ok := v.(string); ok {
		return str == s.opts.InMaintenanceStatus
	}
	return false
}

// field 取契约字段名→状态源 JSON 键的映射（未配置用契约默认名）。
func (s *restSource) field(contract string) string {
	if v, ok := s.opts.Fields[contract]; ok && v != "" {
		return v
	}
	return contract
}

// atOr 空值回落默认（配置部分覆盖口径）。
func atOr(v, def string) string {
	if v != "" {
		return v
	}
	return def
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
