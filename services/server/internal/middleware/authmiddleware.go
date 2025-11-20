package middleware

import (
	"net/http"
	"strings"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// AuthMiddleware enforces authentication and permission checking.
type AuthMiddleware struct {
	ctx *svc.ServiceContext
}

func NewAuthMiddleware(ctx *svc.ServiceContext) *AuthMiddleware {
	return &AuthMiddleware{ctx: ctx}
}

// Handle wraps handlers with auth logic; perms is the list of allowed permissions.
func (m *AuthMiddleware) Handle(perms ...string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			user, roles, ok := m.ctx.Authenticate(r)
			if !ok {
				httpx.WriteJsonCtx(r.Context(), w, http.StatusUnauthorized, map[string]any{
					"code":    http.StatusUnauthorized,
					"message": "unauthorized",
				})
				return
			}
			if skipPermCheck(roles) {
				next(w, r.WithContext(svc.WithActor(r.Context(), user)))
				return
			}
			if len(perms) == 0 {
				next(w, r.WithContext(svc.WithActor(r.Context(), user)))
				return
			}
			for _, perm := range perms {
				if strings.TrimSpace(perm) == "" {
					continue
				}
				if m.ctx.EnforcePermission(user, roles, perm) {
					next(w, r.WithContext(svc.WithActor(r.Context(), user)))
					return
				}
			}
			logx.WithContext(r.Context()).Infof("permission denied: user=%s perms=%v", user, perms)
			httpx.WriteJsonCtx(r.Context(), w, http.StatusForbidden, map[string]any{
				"code":    http.StatusForbidden,
				"message": "forbidden",
			})
		}
	}
}

func skipPermCheck(roles []string) bool {
	for _, role := range roles {
		if role == "admin" || role == "super_admin" {
			return true
		}
	}
	return false
}
