package svc

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	jwtutil "github.com/cuihairu/croupier/internal/security/jwtutil"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuthMiddleware_MachineToken_PathRestriction 验证 ARCH-021：受限机器
// token（X-Croupier-Agent-Token 专用头）只放行扩展同步端点——机器路径上
// 合法 token 通过并注入机器身份；机器 token 出现在其它路径一律 403；
// 机器路径上无凭据/坏 token 走 401。
func TestAuthMiddleware_MachineToken_PathRestriction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// 全量跑包内其它测试会先初始化全局 secret（sync.Once），直接
	// InitGlobalSecret 是 no-op；必须走测试专用 re-key + restore。
	restore := jwtutil.ResetGlobalSecretForTesting("unit-test-machine-secret")
	defer restore()

	m := NewAuthMiddlewareImpl(&ServiceContext{})

	token, err := jwtutil.SignMachine("unit-test-machine-secret", "agent-1", "demo", "prod", time.Hour, time.Now())
	require.NoError(t, err)

	newRouter := func() *gin.Engine {
		r := gin.New()
		r.Use(m.Handle)
		r.GET("/api/v1/agents/:id/extensions", func(c *gin.Context) { c.Status(http.StatusOK) })
		r.POST("/api/v1/agents/:id/extensions/sync", func(c *gin.Context) { c.Status(http.StatusOK) })
		r.GET("/api/v1/players", func(c *gin.Context) { c.Status(http.StatusOK) })
		return r
	}
	router := newRouter()

	// 机器路径 + 合法 token：通过，且请求上下文里能取回机器身份。
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/agent-1/extensions", nil)
	req.Header.Set(jwtutil.MachineTokenHeader, token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 机器路径（POST sync 变体）+ 合法 token：同样放行。
	req = httptest.NewRequest(http.MethodPost, "/api/v1/agents/agent-1/extensions/sync", nil)
	req.Header.Set(jwtutil.MachineTokenHeader, token)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 非机器路径 + 机器 token：403（受限身份不可用于管理面）。
	req = httptest.NewRequest(http.MethodGet, "/api/v1/players", nil)
	req.Header.Set(jwtutil.MachineTokenHeader, token)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "machine_token_forbidden")

	// 近似但不匹配的路径（更深一层）：同样 403。
	req = httptest.NewRequest(http.MethodGet, "/api/v1/agents/agent-1/extensions/extra", nil)
	req.Header.Set(jwtutil.MachineTokenHeader, token)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)

	// 机器路径 + 坏 token：401。
	req = httptest.NewRequest(http.MethodGet, "/api/v1/agents/agent-1/extensions", nil)
	req.Header.Set(jwtutil.MachineTokenHeader, "not-a-jwt")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "machine_token_invalid")

	// 机器路径 + 管理员 JWT 冒充机器 token：tokenType 判型拒绝 → 401。
	adminToken, err := jwtutil.Sign("unit-test-machine-secret", "alice", []string{"admin"}, 1, 0, time.Now())
	require.NoError(t, err)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/agents/agent-1/extensions", nil)
	req.Header.Set(jwtutil.MachineTokenHeader, adminToken)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// 机器路径 + 无任何凭据：走管理员分支 → 401 missing authorization。
	req = httptest.NewRequest(http.MethodGet, "/api/v1/agents/agent-1/extensions", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "missing authorization")
}
