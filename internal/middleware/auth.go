package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/cuihairu/croupier/internal/common/response"
	jwtutil "github.com/cuihairu/croupier/internal/security/jwtutil"
	"github.com/gin-gonic/gin"
)

// InitJWTSecret initializes the JWT secret for authentication middleware.
// This is a convenience wrapper around jwtutil.InitGlobalSecret.
// Deprecated: Use jwtutil.InitGlobalSecret directly.
func InitJWTSecret(secret string) {
	jwtutil.InitGlobalSecret(secret)
}

// Auth creates an authentication middleware that validates JWT tokens.
func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		secret := jwtutil.GetGlobalSecret()
		if secret == "" {
			response.InternalServerError(c, "JWT secret not initialized")
			c.Abort()
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Unauthorized(c, "未授权：缺少 Authorization header")
			c.Abort()
			return
		}

		// 解析 Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			response.Unauthorized(c, "未授权：Authorization header 格式错误")
			c.Abort()
			return
		}

		token := parts[1]

		// 验证 JWT token
		claims, err := jwtutil.Parse(token, secret)
		if err != nil {
			response.Unauthorized(c, "未授权：token 无效或已过期")
			c.Abort()
			return
		}

		// 将用户信息存入 context
		c.Set("username", claims.Username)
		c.Set("roles", claims.Roles)
		c.Set("adminID", claims.AdminID)

		c.Next()
	}
}

// OptionalAuth creates an optional authentication middleware that doesn't require login.
func OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		secret := jwtutil.GetGlobalSecret()
		if secret == "" {
			c.Next()
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && parts[0] == "Bearer" {
			token := parts[1]
			if claims, err := jwtutil.Parse(token, secret); err == nil {
				c.Set("username", claims.Username)
				c.Set("roles", claims.Roles)
				c.Set("adminID", claims.AdminID)
			}
		}

		c.Next()
	}
}

// AuthMiddleware is a configurable authentication middleware.
type AuthMiddleware struct {
	secret string
}

// NewAuthMiddleware creates a new AuthMiddleware with the given secret.
func NewAuthMiddleware(secret string) *AuthMiddleware {
	return &AuthMiddleware{secret: secret}
}

// Handle implements the gin.HandlerFunc interface.
func (m *AuthMiddleware) Handle(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header", "message": "未授权"})
		return
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format", "message": "授权头格式错误"})
		return
	}

	token := parts[1]

	claims, err := jwtutil.Parse(token, m.secret)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication_failed", "message": "认证失败"})
		return
	}

	c.Set("username", claims.Username)
	c.Set("roles", claims.Roles)
	c.Set("adminID", claims.AdminID)
	c.Next()
}

// Authenticate validates a JWT token and returns the claims.
func (m *AuthMiddleware) Authenticate(token string) (string, []string, uint, error) {
	claims, err := jwtutil.Parse(token, m.secret)
	if err != nil {
		return "", nil, 0, errors.New("invalid token")
	}
	return claims.Username, claims.Roles, claims.AdminID, nil
}
