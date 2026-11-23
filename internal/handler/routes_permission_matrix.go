package handler

import (
	"net/http"

	"github.com/cuihairu/croupier/internal/common/response"
	logicutils "github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
)

// domainPermissionRule 是 AUD-042 管理域→权限矩阵的一行：某路径前缀（含
// /api/v1 基座）下的管理面所需 least-privilege 权限，按 HTTP 动词分为读/写
// 两组（任一命中即放行）。read 为空表示该前缀只门禁写面（如 /messages/
// broadcast）；write 为空表示只门禁读面。
type domainPermissionRule struct {
	prefix string   // 路由前缀，含 /api/v1 基座；匹配取最长前缀
	read   []string // GET/HEAD/OPTIONS 任一命中放行
	write  []string // POST/PUT/PATCH/DELETE 任一命中放行
}

// managementPermissionMatrix 是管理域→权限矩阵（AUD-042）。条目只覆盖
// 管理面；同前缀下的自服务/用户侧端点（如 /announcements/active、
// /messages/stream）不在此表，保持既有 JWT 认证，不被矩阵二次门禁。
// 矩阵权限码必须在 configs/permissions.json 目录中存在（与 RBAC 页面对齐，
// 由 TestManagementPermissionMatrix_CatalogAligned 机械校验）。
//
// 既有服务层已门禁的域（backup、storage、admin、role、page 等）不入表，
// 避免同端点双门禁语义冲突。
var managementPermissionMatrix = []domainPermissionRule{
	{prefix: "/api/v1/providers",
		read: []string{"admin:all", "providers:read"}, write: []string{"admin:all", "providers:write"}},
	{prefix: "/api/v1/cicd",
		read: []string{"admin:all", "cicd:read"}, write: []string{"admin:all", "cicd:write"}},
	{prefix: "/api/v1/incident-categories",
		read: []string{"admin:all", "incidents:read"}, write: []string{"admin:all", "incidents:write"}},
	{prefix: "/api/v1/incident-reports",
		read: []string{"admin:all", "incidents:read"}, write: []string{"admin:all", "incidents:write"}},
	{prefix: "/api/v1/incidents",
		read: []string{"admin:all", "incidents:read"}, write: []string{"admin:all", "incidents:write"}},
	{prefix: "/api/v1/configs",
		read: []string{"admin:all", "configs:read"}, write: []string{"admin:all", "configs:write"}},
	{prefix: "/api/v1/config-explorer",
		read: []string{"admin:all", "configs:read"}, write: []string{"admin:all", "configs:write"}},
	{prefix: "/api/v1/tools",
		read: []string{"admin:all", "tools:read"}, write: []string{"admin:all", "tools:write"}},
	{prefix: "/api/v1/admin/announcements",
		read: []string{"admin:all", "announcements:read"}, write: []string{"admin:all", "announcements:write"}},
	{prefix: "/api/v1/certificates",
		read:  []string{"admin:all", "certificates:read", "certificates:manage"},
		write: []string{"admin:all", "certificates:write", "certificates:manage"}},
	{prefix: "/api/v1/rate-limits",
		read: []string{"admin:all", "rate_limits:read"}, write: []string{"admin:all", "rate_limits:write"}},
	{prefix: "/api/v1/schemas",
		read: []string{"admin:all", "schemas:read"}, write: []string{"admin:all", "schemas:write"}},
	// /messages 是管理员自服务消息面（本人收件箱/已读/SSE 流），仅群发
	// 是管理操作，故只门禁 /messages/broadcast 写面；读面保持 JWT。
	{prefix: "/api/v1/messages/broadcast",
		write: []string{"admin:all", "messages:write"}},
	{prefix: "/api/v1/faqs",
		read: []string{"admin:all", "faqs:read"}, write: []string{"admin:all", "faqs:write"}},
	{prefix: "/api/v1/tickets",
		read: []string{"admin:all", "tickets:read"}, write: []string{"admin:all", "tickets:write"}},
	{prefix: "/api/v1/players",
		read: []string{"admin:all", "players:read"}, write: []string{"admin:all", "players:write"}},
	{prefix: "/api/v1/tasks",
		read: []string{"admin:all", "tasks:read"}, write: []string{"admin:all", "tasks:write"}},
	{prefix: "/api/v1/schedules",
		read: []string{"admin:all", "schedules:read"}, write: []string{"admin:all", "schedules:write"}},
}

// matchPermissionMatrixRule 返回最长前缀匹配的管理域规则；无匹配返回 nil。
// 前缀边界匹配（/api/v1/providers 不匹配 /api/v1/providersX）。
func matchPermissionMatrixRule(routePath string) *domainPermissionRule {
	best := -1
	var matched *domainPermissionRule
	for i := range managementPermissionMatrix {
		rule := &managementPermissionMatrix[i]
		if !prefixMatch(routePath, rule.prefix) {
			continue
		}
		if len(rule.prefix) > best {
			best = len(rule.prefix)
			matched = rule
		}
	}
	return matched
}

func prefixMatch(path, prefix string) bool {
	if path == prefix {
		return true
	}
	if len(path) > len(prefix) && path[len(prefix)] == '/' {
		return path[:len(prefix)] == prefix
	}
	return false
}

func isReadVerb(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

// permissionMatrixGuard 是 AUD-042 统一挂载的管理域权限矩阵中间件：在
// Authority（JWT 认证）之后运行，命中矩阵前缀的管理路由按动词做
// least-privilege 权限校验，未命中（含自服务/公开/既已服务层门禁的域）
// 直接放行。校验失败按响应契约 403 permission_denied 终止。
func permissionMatrixGuard(serverCtx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 受限机器 token 已在 Authority 内按路径 403，此处防御性跳过，
		// 不把机器身份再送矩阵（无管理面权限）。
		if _, isMachine := c.Get("machineIdentity"); isMachine {
			c.Next()
			return
		}

		route := c.FullPath()
		if route == "" {
			route = c.Request.URL.Path
		}
		rule := matchPermissionMatrixRule(route)
		if rule == nil {
			c.Next()
			return
		}

		required := rule.write
		message := "无权操作该管理域"
		if isReadVerb(c.Request.Method) {
			required = rule.read
			message = "无权查看该管理域"
		}
		if len(required) == 0 {
			// 该前缀只门禁另一侧动词（如 /messages/broadcast 只门禁写），
			// 当前动词不在门禁范围，放行。
			c.Next()
			return
		}

		if _, _, err := logicutils.RequireAnyPermission(
			c.Request.Context(), serverCtx, message, required...,
		); err != nil {
			response.Error(c, err)
			c.Abort()
			return
		}
		c.Next()
	}
}
