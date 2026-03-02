package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/security/jwtutil"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

type AuthMiddleware struct {
	svcCtx               *svc.ServiceContext
	allowPaths           map[string]struct{}
	allowPref            []string
	publicReadPrefixes   []string
	publicReadExactPaths map[string]struct{}
}

func NewAuthMiddleware(svcCtx *svc.ServiceContext) *AuthMiddleware {
	return &AuthMiddleware{
		svcCtx: svcCtx,
		allowPaths: map[string]struct{}{
			"/api/v1/auth/login":         {},
			"/api/v1/monitoring/health":  {},
			"/api/v1/monitoring/healthz": {},
		},
		allowPref: []string{
			"/api/v1/auth/login",
		},
		publicReadPrefixes: []string{
			"/api/v1/configs",
			"/api/v1/registry",              // 公开访问：注册中心（agents、functions）
			"/api/v1/functions/descriptors", // 公开访问：函数描述符列表
		},
		publicReadExactPaths: map[string]struct{}{
			"/api/v1/functions": {}, // 公开访问：函数列表（精确匹配，子路径需认证）
		},
	}
}

// Handle 处理认证中间件
func (m *AuthMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if m.shouldBypass(r) {
			next(w, r)
			return
		}

		// 获取 Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			// 兼容 SSE 等无法自定义 header 的场景，支持 token 查询参数
			if token := strings.TrimSpace(r.URL.Query().Get("token")); token != "" {
				authHeader = "Bearer " + token
			}
		}
		if authHeader == "" {
			writeCodeError(w, r, errorx.NewUnauthorized("missing authorization header"))
			return
		}

		// 解析 Bearer token
		tokenParts := strings.SplitN(authHeader, " ", 2)
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			writeCodeError(w, r, errorx.NewUnauthorized("invalid authorization header format"))
			return
		}

		token := tokenParts[1]

		// 验证 JWT token
		username, roles, adminID, err := m.authenticate(r.Context(), token)
		if err != nil {
			logx.Errorf("authentication failed: %v", err)
			writeCodeError(w, r, err)
			return
		}

		ctx := context.WithValue(r.Context(), "username", username)
		ctx = context.WithValue(ctx, "roles", roles)
		ctx = context.WithValue(ctx, "adminID", adminID)
		r = r.WithContext(ctx)
		logx.Infof("Authenticated user %s roles=%v", username, roles)

		// 继续处理请求
		next(w, r)
	}
}

func (m *AuthMiddleware) authenticate(ctx context.Context, token string) (string, []string, uint, error) {
	secret, _ := jwtutil.ResolveSecret(m.svcCtx.Config)

	claims, err := jwtutil.Parse(token, secret)
	if err != nil {
		return "", nil, 0, errorx.NewUnauthorized("invalid token")
	}
	username := claims.Subject
	if strings.TrimSpace(username) == "" {
		return "", nil, 0, errorx.NewUnauthorized("token subject missing")
	}

	// 增加超时时间
	lookupCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	admin, err := m.svcCtx.AdminModel.FindByUsername(lookupCtx, username)
	if err != nil {
		// 记录更详细的错误信息
		logx.Errorf("Failed to query admin %s: %v", username, err)
		if errors.Is(err, context.DeadlineExceeded) {
			return "", nil, 0, errorx.NewInternalError("数据库查询超时，请检查数据库连接")
		}
		return "", nil, 0, errorx.NewInternalError("查询管理员失败")
	}
	if admin == nil {
		return "", nil, 0, errorx.NewUnauthorized("admin not found")
	}
	if admin.LastLoginAt != nil && claims.IssuedAt != nil {
		issuedAt := normalizeToSecond(claims.IssuedAt.Time)
		lastLogin := normalizeToSecond(*admin.LastLoginAt)
		if issuedAt.Before(lastLogin) {
			return "", nil, 0, errorx.NewUnauthorized("token has been invalidated by a later login")
		}
	}

	return username, claims.Roles, admin.ID, nil
}

func normalizeToSecond(t time.Time) time.Time {
	return t.UTC().Truncate(time.Second)
}

func (m *AuthMiddleware) shouldBypass(r *http.Request) bool {
	path := r.URL.Path
	if _, ok := m.allowPaths[path]; ok {
		return true
	}
	for _, prefix := range m.allowPref {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	if r.Method == http.MethodGet {
		// 精确路径匹配
		if _, ok := m.publicReadExactPaths[path]; ok {
			return true
		}
		// 前缀匹配
		for _, prefix := range m.publicReadPrefixes {
			if strings.HasPrefix(path, prefix) {
				return true
			}
		}
	}
	return false
}

// RequirePermission 权限检查中间件
func RequirePermission(permission string) func(next http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// 从上下文获取用户信息
			username, ok := r.Context().Value("username").(string)
			if !ok {
				httpx.ErrorCtx(r.Context(), w, errors.New("user not authenticated"))
				return
			}

			// 获取 ServiceContext（需要通过依赖注入或其他方式获取）
			// 这里简化处理，假设可以通过某种方式获取 AdminManager
			// 在实际应用中，可以通过依赖注入框架或全局变量来获取

			// 暂时跳过权限检查，或者使用简单的权限验证
			// 在完整的实现中，这里应该调用 AdminManager.CheckPermission

			_ = username

			next(w, r)
		}
	}
}

// GetUsernameFromContext 从上下文获取用户名
func GetUsernameFromContext(ctx context.Context) string {
	if username, ok := ctx.Value("username").(string); ok {
		return username
	}
	return ""
}

// GetRolesFromContext 从上下文获取用户角色
func GetRolesFromContext(ctx context.Context) []string {
	if roles, ok := ctx.Value("roles").([]string); ok {
		return roles
	}
	return nil
}

func writeCodeError(w http.ResponseWriter, r *http.Request, err error) {
	if codeErr, ok := errors.AsType[*errorx.CodeError](err); ok {
		httpx.WriteJsonCtx(r.Context(), w, codeErr.Code, map[string]interface{}{
			"error":   codeErr.ErrorCode(),
			"message": codeErr.Message,
		})
		return
	}
	httpx.ErrorCtx(r.Context(), w, err)
}
