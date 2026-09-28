package svc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/settings"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newMFAGateFixture 构造带 AdminModel 的中间件 + 真实 settings 单例
// （mfaRequired 可控），并建一个未绑定 TOTP 的本地账号（OTPEnabled=false），
// 返回中间件与该账号——gate 拦截断言须落在真实存在的账号上（查无此人走
// 存储故障 fail-open，不是强制绑定的语义载体）。
func newMFAGateFixture(t *testing.T, mfaRequired bool) (*AuthMiddleware, *model.Admin) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dsn := "file:" + t.TempDir() + "/mfagate.db?_pragma=synchronous(OFF)"
	db, err := gorm.Open(gsqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))

	adminModel := model.NewAdminModel(db)
	unbound := &model.Admin{Username: "unbound", Nickname: "unbound", Status: 1, OTPEnabled: false}
	require.NoError(t, adminModel.Create(context.Background(), unbound, "Str0ngPass!x"))

	store := model.NewPlatformSettingModel(db)
	settings.InitLayered(context.Background(), &settings.ConfigInput{}, store)
	raw := `false`
	if mfaRequired {
		raw = `true`
	}
	require.NoError(t, store.Set(context.Background(), settings.KeySecurityMFARequired,
		json.RawMessage(raw), "tester"))
	settings.Current().Reload(context.Background(), store)
	t.Cleanup(settings.ResetForTest)

	return NewAuthMiddlewareImpl(&ServiceContext{AdminModel: adminModel}), unbound
}

func runMFAGate(c *gin.Context, m *AuthMiddleware, adminID uint) bool {
	ctx := c.Request.Context()
	if !m.mfaGate(c, adminID) {
		return false
	}
	_ = ctx
	return true
}

// TestMFAGate_PolicyOffAllowsAll 策略关闭（默认）：一切放行，连库都不查。
func TestMFAGate_PolicyOffAllowsAll(t *testing.T) {
	m, _ := newMFAGateFixture(t, false)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/games", nil)
	assert.True(t, runMFAGate(c, m, 0))
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestMFAGate_BlocksUnboundLocalExceptBypass 策略开启：未绑定 TOTP 的账号
// 普通端点 403 mfa_required，绑定/资料白名单放行；adminID=0（模型不可用
// 形态）fail-open。
func TestMFAGate_BlocksUnboundLocalExceptBypass(t *testing.T) {
	m, unbound := newMFAGateFixture(t, true)

	// 普通端点：未绑定 TOTP 的本地账号 → 403 mfa_required
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/games", nil)
	assert.False(t, runMFAGate(c, m, unbound.ID))
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "mfa_required")

	// 绑定流程白名单放行（同一真实未绑定账号，gate 本应拦截但白名单优先）
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/mfa/setup", nil)
	assert.True(t, runMFAGate(c, m, unbound.ID))

	// 个人资料（含改密恢复通道）放行
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/profile/password", nil)
	assert.True(t, runMFAGate(c, m, unbound.ID))

	// adminID=0（单测/精简部署）fail-open
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/games", nil)
	assert.True(t, runMFAGate(c, m, 0))
}

// TestMFAGate_EnabledAdminPasses 已绑定 TOTP 的账号全端点放行。
func TestMFAGate_EnabledAdminPasses(t *testing.T) {
	m, _ := newMFAGateFixture(t, true)

	// 同 fixture 库补一个已绑定账号（缓存按 adminID 分键，互不影响）
	adminModel := m.svcCtx.AdminModel
	bound := &model.Admin{Username: "bound", Nickname: "bound", Status: 1, OTPEnabled: true}
	require.NoError(t, adminModel.Create(context.Background(), bound, "Str0ngPass!x"))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/games", nil)
	assert.True(t, runMFAGate(c, m, bound.ID))
	assert.Equal(t, http.StatusOK, w.Code)
}
