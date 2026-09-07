package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	jwtutil "github.com/cuihairu/croupier/internal/security/jwtutil"
)

// TestAuth_EmptyGlobalSecret 覆盖 Auth() 中全局 secret 未初始化的分支
// （auth.go:24）：应返回 500 并中止，不进入后续 handler。
func TestAuth_EmptyGlobalSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)
	restore := jwtutil.ResetGlobalSecretForTesting("")
	defer restore()

	reached := false
	router := gin.New()
	router.Use(Auth())
	router.GET("/secure", func(c *gin.Context) {
		reached = true
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/secure", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.False(t, reached, "handler must not run when secret is uninitialized")
	assert.Contains(t, w.Body.String(), "JWT secret not initialized")
}

// TestOptionalAuth_EmptyGlobalSecret 覆盖 OptionalAuth() 中
// secret 为空直接放行的分支（auth.go:68）：请求应正常到达 handler，
// 且 context 中无用户信息。
func TestOptionalAuth_EmptyGlobalSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)
	restore := jwtutil.ResetGlobalSecretForTesting("")
	defer restore()

	router := gin.New()
	router.Use(OptionalAuth())
	router.GET("/public", func(c *gin.Context) {
		_, exists := c.Get("username")
		assert.False(t, exists, "no username should be set without secret")
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/public", nil)
	req.Header.Set("Authorization", "Bearer whatever-token")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	// 恢复后 OptionalAuth 不再走空 secret 分支（防守性确认，非覆盖目标）。
	_ = jwtutil.GetGlobalSecret()
}
