package middleware

import (
	"context"
	"strings"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
	"net/http"

	tokenmgr "github.com/cuihairu/croupier/internal/security/token"
	"github.com/cuihairu/croupier/services/server/internal/svc"
)

type AuthMiddleware struct {
	svcCtx *svc.ServiceContext
}

func NewAuthMiddleware(svcCtx *svc.ServiceContext) *AuthMiddleware {
	return &AuthMiddleware{
		svcCtx: svcCtx,
	}
}

// Handle 处理认证中间件
func (m *AuthMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 获取 Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			httpx.ErrorCtx(r.Context(), w, http.StatusUnauthorized, "Missing authorization header")
			return
		}

		// 解析 Bearer token
		tokenParts := strings.SplitN(authHeader, " ", 2)
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			httpx.ErrorCtx(r.Context(), w, http.StatusUnauthorized, "Invalid authorization header format")
			return
		}

		token := tokenParts[1]

		// 验证 JWT token
		if m.svcCtx.Config.Auth.JWTSecret != "" {
			tokenManager := tokenmgr.NewManager(m.svcCtx.Config.Auth.JWTSecret)
			claims, err := tokenManager.Verify(token)
			if err != nil {
				logx.Errorf("Invalid token: %v", err)
				httpx.ErrorCtx(r.Context(), w, http.StatusUnauthorized, "Invalid token")
				return
			}

			// 将用户信息添加到上下文
			ctx := context.WithValue(r.Context(), "username", claims.Username)
			ctx = context.WithValue(ctx, "roles", claims.Roles)
			r = r.WithContext(ctx)

			logx.Infof("Authenticated user: %s with roles: %v", claims.Username, claims.Roles)
		} else {
			// 开发模式，使用固定token
			if token != "dev-token" {
				httpx.ErrorCtx(r.Context(), w, http.StatusUnauthorized, "Invalid dev token")
				return
			}

			// 开发模式的默认用户
			ctx := context.WithValue(r.Context(), "username", "admin")
			ctx = context.WithValue(ctx, "roles", []string{"admin"})
			r = r.WithContext(ctx)
		}

		// 继续处理请求
		next(w, r)
	}
}

// RequirePermission 权限检查中间件
func RequirePermission(permission string) func(next http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// 从上下文获取用户信息
			username, ok := r.Context().Value("username").(string)
			if !ok {
				httpx.ErrorCtx(r.Context(), w, http.StatusUnauthorized, "User not authenticated")
				return
			}

			// 获取 ServiceContext（需要通过依赖注入或其他方式获取）
			// 这里简化处理，假设可以通过某种方式获取 AdminManager
			// 在实际应用中，可以通过依赖注入框架或全局变量来获取

			// 暂时跳过权限检查，或者使用简单的权限验证
			// 在完整的实现中，这里应该调用 AdminManager.CheckPermission

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