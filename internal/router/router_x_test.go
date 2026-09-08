// 覆盖目标：registerPublicRoutes 的 JWT secret 解析失败 fallback 分支
// （router.go:79-86）、身份源配置无效告警分支（router.go:93-95），以及
// registerAuthenticatedRoutes 的审计存储初始化失败降级分支（router.go:131-133）。
package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func routerXNewEngineWithAPIGroup(t *testing.T) (*gin.Engine, *gin.RouterGroup) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	return r, r.Group("/api/v1")
}

func routerXMemoryDB(t *testing.T) (*gorm.DB, error) {
	t.Helper()
	cfg := &config.Config{
		Database: config.DatabaseConfig{DataSource: "file::memory:?mode=memory"},
	}
	return initDatabase(cfg)
}

// registerPublicRoutes：JWT secret 未配置且非 dev 模式 → ResolveSecret 报错，
// 走 slog.Warn + DevSecret() fallback，路由仍正常注册。
func TestRouterX_RegisterPublicRoutes_JWTSecretFallback(t *testing.T) {
	t.Setenv("CROUPIER_MODE", "production")

	cfg := &config.Config{
		Server: config.ServerConfig{Mode: "test"},
		Auth:   config.AuthConfig{JWTSecret: ""},
		Database: config.DatabaseConfig{
			DataSource: "file::memory:?mode=memory",
		},
	}

	db, err := initDatabase(cfg)
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()

	r, api := routerXNewEngineWithAPIGroup(t)
	registerPublicRoutes(api, db, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/monitoring/health", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"status":"ok"}`, rec.Body.String())

	// auth 路由组仍应注册成功
	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/providers", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.NotEqual(t, http.StatusNotFound, rec.Code)
}

// registerPublicRoutes：settings L3 覆盖启用 LDAP 但 addr/baseDn/userDnTemplate
// 均未配置 → BuildIdentityProviders 报错，走 slog.Error 分支并继续注册。
func TestRouterX_RegisterPublicRoutes_IdentityProviderConfigError(t *testing.T) {
	db, err := routerXMemoryDB(t)
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()

	require.NoError(t, db.AutoMigrate(&model.PlatformSetting{}))
	require.NoError(t, db.Create(&model.PlatformSetting{
		Key:   settings.KeyAuthLdapEnabled,
		Value: "true",
	}).Error)

	// 进程内首次（也是唯一一次）初始化分层设置：L3 含 ldap.enabled=true，
	// 其余 LDAP 键缺失 → AuthProviderConfig 返回不完整配置。
	settings.InitLayered(context.Background(), nil, model.NewPlatformSettingModel(db))

	cfg := &config.Config{
		Server: config.ServerConfig{Mode: "test"},
		Auth:   config.AuthConfig{JWTSecret: "router-x-secret"},
		Database: config.DatabaseConfig{
			DataSource: "file::memory:?mode=memory",
		},
	}

	r, api := routerXNewEngineWithAPIGroup(t)
	registerPublicRoutes(api, db, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/providers", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.NotEqual(t, http.StatusNotFound, rec.Code)
}

// registerAuthenticatedRoutes：db 为 nil → audit.NewSQLAuditStore 报错，
// 走 slog.Warn 降级（禁用 function.contract_updated 审计），其余路由照常装配。
func TestRouterX_RegisterAuthenticatedRoutes_NilDBAuditFallback(t *testing.T) {
	r, api := routerXNewEngineWithAPIGroup(t)
	registerAuthenticatedRoutes(api, nil, &config.Config{
		Server: config.ServerConfig{Mode: "test"},
	})

	routes := r.Routes()
	assert.NotEmpty(t, routes)

	known := map[string]bool{}
	for _, rt := range routes {
		known[rt.Method+" "+rt.Path] = true
	}
	for _, want := range []string{
		"GET /api/v1/profile",
		"GET /api/v1/roles",
		"GET /api/v1/contracts",
		"GET /api/v1/ops/agents",
	} {
		assert.True(t, known[want], "missing route %s", want)
	}
}
