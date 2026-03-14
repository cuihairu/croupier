package middleware

import (
	"strings"

	"github.com/cuihairu/croupier/internal/pkg2/jwt"
	"github.com/cuihairu/croupier/internal/pkg2/response"
	"github.com/gin-gonic/gin"
)

// Auth 认证中间件
func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取 Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			// 兼容 SSE 等无法自定义 header 的场景，支持 token 查询参数
			if token := strings.TrimSpace(c.Query("token")); token != "" {
				authHeader = "Bearer " + token
			}
		}

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
		claims, err := jwt.ParseToken(token)
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

// OptionalAuth 可选认证中间件（不强制要求登录）
func OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && parts[0] == "Bearer" {
			token := parts[1]
			if claims, err := jwt.ParseToken(token); err == nil {
				c.Set("username", claims.Username)
				c.Set("roles", claims.Roles)
				c.Set("adminID", claims.AdminID)
			}
		}

		c.Next()
	}
}
