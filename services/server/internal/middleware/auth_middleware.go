package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/security/jwtutil"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

type AuthMiddleware struct {
	svcCtx     *svc.ServiceContext
	allowPaths map[string]struct{}
	allowPref  []string
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
			httpx.ErrorCtx(r.Context(), w, errors.New("missing authorization header"))
			return
		}

		// 解析 Bearer token
		tokenParts := strings.SplitN(authHeader, " ", 2)
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			httpx.ErrorCtx(r.Context(), w, errors.New("invalid authorization header format"))
			return
		}

		token := tokenParts[1]

		// 验证 JWT token
		username, roles, adminID, err := m.authenticate(r.Context(), token)
		if err != nil {
			logx.Errorf("authentication failed: %v", err)
			httpx.ErrorCtx(r.Context(), w, err)
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
		return "", nil, 0, fmt.Errorf("invalid token: %w", err)
	}
	username := claims.Subject
	if strings.TrimSpace(username) == "" {
		return "", nil, 0, errors.New("token subject missing")
	}

	lookupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	admin, err := m.svcCtx.AdminModel.FindByUsername(lookupCtx, username)
	if err != nil {
		return "", nil, 0, fmt.Errorf("查询管理员失败: %w", err)
	}
	if admin == nil {
		return "", nil, 0, errors.New("admin not found")
	}
	if admin.LastLoginAt != nil && claims.IssuedAt != nil {
		issuedAt := truncateMonotonic(claims.IssuedAt.Time)
		lastLogin := truncateMonotonic(*admin.LastLoginAt)
		if issuedAt.Before(lastLogin) {
			return "", nil, 0, errors.New("token has been invalidated by a later login")
		}
	}

	return username, claims.Roles, admin.ID, nil
}

func truncateMonotonic(t time.Time) time.Time {
	return time.Unix(0, t.UnixNano()).UTC()
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
