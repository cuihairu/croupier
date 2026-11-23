package svc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/settings"
	jwtutil "github.com/cuihairu/croupier/internal/security/jwtutil"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const aud045TestSecret = "test-secret-aud045-fail-closed"

// newAud045Context 构造带真实 AdminModel 的中间件上下文，server.mode 显式
// 指定；CROUPIER_MODE/CROUPIER_ENV 清零，令环境信号只由 mode 驱动。
func newAud045Context(t *testing.T, mode string) (*ServiceContext, *AuthMiddleware) {
	t.Helper()
	svcCtx := newSvcTestContext(t)
	svcCtx.Config = config.Config{Server: config.ServerConfig{Mode: mode}}
	t.Setenv("CROUPIER_MODE", "")
	t.Setenv("CROUPIER_ENV", "")
	return svcCtx, NewAuthMiddlewareImpl(svcCtx)
}

// aud045CloseDB 关闭底层 sql.DB：后续模型查询一律返回真实错误，用来模拟
// 「认证关键查询打到故障存储」这一形态。gorm.DB 句柄仍非 nil，模型不会走
// 「未装配 → 整段跳过」的兼容分支。
func aud045CloseDB(t *testing.T, svcCtx *ServiceContext) {
	t.Helper()
	sqlDB, err := svcCtx.DB.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
}

// TestAuthMiddleware_Aud045_TokenVersionStorageFailure 令牌版本查询故障：
// 生产信号在场必须 fail-closed（401），测试/精简模式保留可用性优先兼容。
func TestAuthMiddleware_Aud045_TokenVersionStorageFailure(t *testing.T) {
	defer jwtutil.ResetGlobalSecretForTesting(aud045TestSecret)()

	for _, tc := range []struct {
		mode  string
		code  int
		body  string
		label string
	}{
		{mode: "prod", code: http.StatusUnauthorized, body: "authentication_failed", label: "prod fails closed"},
		{mode: "dev", code: http.StatusOK, body: "alice", label: "dev keeps compat bypass"},
	} {
		t.Run(tc.label, func(t *testing.T) {
			svcCtx, mw := newAud045Context(t, tc.mode)
			require.NoError(t, svcCtx.AdminModel.Create(context.Background(),
				&model.Admin{Model: gorm.Model{ID: 42}, Username: "alice", Status: 1}, "Str0ngPass!x"))

			token, err := jwtutil.Sign(aud045TestSecret, "alice", nil, 42, 0, time.Now())
			require.NoError(t, err)

			aud045CloseDB(t, svcCtx)

			gin.SetMode(gin.TestMode)
			r := gin.New()
			r.Use(mw.Handle)
			r.GET("/api/v1/anything", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"user": c.GetString("username")})
			})

			req := httptest.NewRequest(http.MethodGet, "/api/v1/anything", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, tc.code, w.Code, "mode=%s body=%s", tc.mode, w.Body.String())
			if tc.body != "" {
				assert.Contains(t, w.Body.String(), tc.body)
			}
		})
	}
}

// TestAuthMiddleware_Aud045_OTPStorageFailure OTP 状态查询故障：生产信号在场
// 按「未启用」处理 → mfaGate 拦截（403 mfa_required）；测试模式兼容放行。
func TestAuthMiddleware_Aud045_OTPStorageFailure(t *testing.T) {
	for _, tc := range []struct {
		mode   string
		passes bool
		label  string
	}{
		{mode: "prod", passes: false, label: "prod fails closed"},
		{mode: "dev", passes: true, label: "dev keeps compat bypass"},
	} {
		t.Run(tc.label, func(t *testing.T) {
			svcCtx, mw := newAud045Context(t, tc.mode)
			require.NoError(t, svcCtx.AdminModel.Create(context.Background(),
				&model.Admin{Model: gorm.Model{ID: 7}, Username: "unbound", Status: 1}, "Str0ngPass!x"))

			store := model.NewPlatformSettingModel(svcCtx.DB)
			settings.InitLayered(context.Background(), &settings.ConfigInput{}, store)
			require.NoError(t, store.Set(context.Background(), settings.KeySecurityMFARequired,
				json.RawMessage(`true`), "tester"))
			settings.Current().Reload(context.Background(), store)
			require.True(t, settings.Current().SecurityPolicy().MFARequired,
				"mfaRequired 未生效则 gate 直接短路，本用例不成立")
			t.Cleanup(settings.ResetForTest)

			aud045CloseDB(t, svcCtx)

			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/games", nil)
			assert.Equal(t, tc.passes, mw.mfaGate(c, 7), "mode=%s", tc.mode)
			if !tc.passes {
				assert.Equal(t, http.StatusForbidden, w.Code)
				assert.Contains(t, w.Body.String(), "mfa_required")
			}
		})
	}
}

// TestAuthorizeScope_Aud045_ScopeModelErrorFailsClosed scope 模型故障回归：
// 已授权账号在存储故障下必须被拒绝，绝不因查询失败而放行。
func TestAuthorizeScope_Aud045_ScopeModelErrorFailsClosed(t *testing.T) {
	svcCtx := newSvcTestContext(t)
	svcCtx.Config = config.Config{Server: config.ServerConfig{Mode: "prod"}}
	t.Setenv("CROUPIER_MODE", "")
	t.Setenv("CROUPIER_ENV", "")
	ctx := context.Background()

	game, admin := seedGameWithEnv(t, svcCtx, "demo", "prod")
	sc := &model.AdminGameEnvScope{AdminID: admin.ID, GameID: game.ID, Env: "prod"}
	require.NoError(t, svcCtx.DB.Create(sc).Error)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("adminID", admin.ID)

	// 对照组：授权 scope 存在时放行
	assert.NoError(t, authorizeScope(ctx, svcCtx, c, "demo", "prod"), "健康库 + 已授权 scope 必须放行")

	// 存储故障：角色查询与 scope 查询全部失败 → 必须拒绝
	aud045CloseDB(t, svcCtx)
	require.Error(t, authorizeScope(ctx, svcCtx, c, "demo", "prod"),
		"存储故障时 scope 校验必须拒绝，不能回退为放行")
}
