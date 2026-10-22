package serverstatus

import (
	"context"
)

// HTTPProvider 通用 HTTP 状态源（批 S4 注册的第二实现）：GET 一个返回状态
// JSON 的 URL——baseUrl 即端点（statusPath 可选子路径），响应按 Fields 映射
// 为统一契约（缺省=契约键名直读），目标是任意能返回状态 JSON 的简单服务
// 都能接。映射引擎与 atlas 共用 restSource，传输/解析失败同口径：错误=状态
// 未知，gate fail-open 照报标注，绝不静默。
type HTTPProvider struct {
	rest *restSource
}

// httpDefaultInMaintenanceStatus http 源 inMaintenance 字符串枚举缺省口径：
// 简易源最常见的非 bool 形态是字面 "true"/"false" 串；bool 形态无视该配置
// 直取（引擎通用口径）。枚举型服务请显式配置 inMaintenanceStatus。
const httpDefaultInMaintenanceStatus = "true"

// NewHTTPProvider 创建通用 HTTP 状态源。与 atlas 缺省选项的差异：
//   - StatusPath 缺省空：baseUrl 即端点，query 参数定位（atlas 缺省
//     /v1/discovery/servers/{id} 路径参数）；
//   - Fields 缺省空：契约键名直读（healthy / inMaintenance / windowStart /
//     windowEnd / windowNote；atlas 缺省 agentId→id / inMaintenance→status）；
//   - InMaintenanceStatus 缺省 "true"（见上）；
//   - 单对象响应不含匹配键字段时整体接受（URL 即标识——单服务器状态端点
//     无需回显自身 id；携带但值不符才算无映射）。
//
// baseURL 为空=未配置（查询恒 ErrProviderUnavailable，gate 按 fail-open
// 照报标注；svc 装配层对非 noop 源缺 baseUrl 时直接旁路不挂 gate）。
func NewHTTPProvider(opts Options) *HTTPProvider {
	a := opts.Atlas
	a.MatchKey = atOr(a.MatchKey, DefaultAtlasOptions().MatchKey)
	a.InMaintenanceStatus = atOr(a.InMaintenanceStatus, httpDefaultInMaintenanceStatus)
	rs := newRestSource(opts, a, ProviderHTTP)
	rs.acceptUnmatchedSingle = true
	return &HTTPProvider{rest: rs}
}

// Name 注册表键。
func (p *HTTPProvider) Name() string { return ProviderHTTP }

// GetServerStatus 查询状态（引擎共享口径：错误=状态未知，gate fail-open）。
func (p *HTTPProvider) GetServerStatus(ctx context.Context, ref ServerRef) (ServerStatus, error) {
	return p.rest.getServerStatus(ctx, ref)
}

// ListServers 拉状态源已知服务器清单（数组或 {servers|items} 包裹均可；
// 单服务器状态对象无清单语义，解出空清单不报错）。
func (p *HTTPProvider) ListServers(ctx context.Context) ([]ServerSummary, error) {
	return p.rest.listServers(ctx)
}
