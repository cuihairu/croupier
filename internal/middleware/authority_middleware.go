package middleware

import (
	"net/http"

	permissionservice "github.com/cuihairu/croupier/internal/service/permission"
)

type AuthorityMiddleware struct {
	permissionService *permissionservice.PermissionService
	jwtSecret         string
	config            permissionservice.PermissionConfig
}

func NewAuthorityMiddleware(permissionService *permissionservice.PermissionService, jwtSecret string, config ...permissionservice.PermissionConfig) *AuthorityMiddleware {
	m := &AuthorityMiddleware{
		permissionService: permissionService,
		jwtSecret:         jwtSecret,
	}
	if len(config) > 0 {
		m.config = config[0]
	}
	return m
}

func (m *AuthorityMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	if m == nil || m.permissionService == nil {
		return next
	}

	return func(w http.ResponseWriter, r *http.Request) {
		handler := permissionservice.PermissionMiddleware(m.permissionService, m.jwtSecret, m.config)(http.HandlerFunc(next))
		handler.ServeHTTP(w, r)
	}
}
