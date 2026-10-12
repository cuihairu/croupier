package serverstatus

import (
	"context"
)

// AtlasOptions atlas 适配器专属选项（atlas 仓已在本机对齐，2026-10-11；
// 路径模板 + 响应字段映射可配，映射不到的字段按零值处理）。批 S4 起同时
// 作为 REST 状态源的通用映射选项载体——http 源共用同一组字段（svc 装配层
// 对两种源统一映射 config→Options.Atlas）。
type AtlasOptions struct {
	// StatusPath 状态查询路径模板。含 {id} 占位符=路径参数替换（atlas
	// 真实口径 /v1/discovery/servers/{id}，无需鉴权）；否则拼 query
	// 参数 ?{matchKey}={match}（通用可配口径）。
	StatusPath string
	// MatchKey 定位参数名（契约键，默认 "agentId"；按 matchBy 可换 host/ip）。
	MatchKey string
	// Fields 响应字段映射（配置驱动，不写死供应商字段名——alertInbound
	// LabelMapping 同款先例）。键=契约字段名（healthy / inMaintenance /
	// windowStart / windowEnd / windowNote），值=状态源响应 JSON 键。
	Fields map[string]string
	// InMaintenanceStatus 状态枚举值口径：映射到的 inMaintenance 字段不是
	// bool 时（atlas status 字段是字符串枚举）按此值判等。默认 "maintenance"。
	InMaintenanceStatus string
}

// DefaultAtlasOptions 返回 atlas 默认适配选项（对齐 atlas 真实 API：
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
// 映射引擎与通用 http 源共用 restSource（批 S4 抽取），差异只在缺省选项：
// 真实 atlas 口径 /v1/discovery/servers/{id} 路径参数 + status 字符串枚举
// 判等 + agentId→id 字段映射。
type AtlasProvider struct {
	rest *restSource
}

// NewAtlasProvider 创建 atlas 适配器。baseURL 为空=未配置（查询恒返回
// ErrProviderUnavailable，gate 按 fail-open 照报标注）。配置侧字段映射是
// 部分覆盖（只写要改的键），故在此与默认映射合并——否则只配 inMaintenance
// 就丢 agentId→id 默认，匹配必错。
func NewAtlasProvider(opts Options) *AtlasProvider {
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
	return &AtlasProvider{rest: newRestSource(opts, a, ProviderAtlas)}
}

// Name 注册表键。
func (p *AtlasProvider) Name() string { return ProviderAtlas }

// GetServerStatus 查询 atlas 维护状态（引擎共享口径：GET {baseURL}
// {statusPath}?{matchKey}={match}，错误=状态未知，gate fail-open 照报）。
func (p *AtlasProvider) GetServerStatus(ctx context.Context, ref ServerRef) (ServerStatus, error) {
	return p.rest.getServerStatus(ctx, ref)
}

// ListServers 拉状态源已知服务器清单（无匹配过滤；失败返回错误不吞）。
func (p *AtlasProvider) ListServers(ctx context.Context) ([]ServerSummary, error) {
	return p.rest.listServers(ctx)
}
