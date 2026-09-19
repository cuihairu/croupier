package handler

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/api/component"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/settings"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestRegisterHandlersPrometheusExposition covers the branch that mounts the
// Prometheus exposition route when telemetry.prometheus.enabled is on.
func TestRegisterHandlersPrometheusExposition(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	serverCtx := &svc.ServiceContext{}
	serverCtx.Config.Telemetry.Prometheus.Enabled = true
	RegisterHandlers(r, serverCtx)

	path := serverCtx.Config.Telemetry.Prometheus.PrometheusPath()
	found := false
	for _, ri := range r.Routes() {
		if ri.Method == "GET" && ri.Path == path {
			found = true
			break
		}
	}
	assert.True(t, found, "Prometheus exposition route %s should be registered", path)
}

// TestRegisterAuthRoutesSecretFallback covers the ResolveSecret failure
// branch in registerAuthRoutes: outside development mode without a JWT
// secret, the dev fallback secret is applied.
func TestRegisterAuthRoutesSecretFallback(t *testing.T) {
	t.Setenv("CROUPIER_ENV", "production")
	t.Setenv("CROUPIER_MODE", "prod")

	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterHandlers(r, &svc.ServiceContext{})
}

// newPlatformSettingStore opens an in-memory DB with the platform settings
// table migrated.
func newPlatformSettingStore(t *testing.T) *model.PlatformSettingModel {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.PlatformSetting{}))
	return model.NewPlatformSettingModel(db)
}

// TestRegisterAuthRoutesInvalidIdentityProviders covers the
// BuildIdentityProviders error branch: an L3 LDAP override enabled without
// addr/baseDn makes provider construction fail (logged, not fatal).
func TestRegisterAuthRoutesInvalidIdentityProviders(t *testing.T) {
	settings.ResetForTest()
	defer settings.ResetForTest()

	store := newPlatformSettingStore(t)
	require.NoError(t, store.Set(context.Background(), settings.KeyAuthLdapEnabled, json.RawMessage(`true`), "tester"))

	settings.InitLayered(context.Background(), &settings.ConfigInput{}, store)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterHandlers(r, &svc.ServiceContext{})
}

// TestRegisterRoutesRoutes covers the static route catalogue registration.
func TestRegisterRoutesRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	registerRoutesRoutes(&r.RouterGroup, &svc.ServiceContext{})

	found := false
	for _, ri := range r.Routes() {
		if ri.Method == "GET" && ri.Path == "/" {
			found = true
			break
		}
	}
	assert.True(t, found, "route catalogue GET / should be registered")
}

// buildContractTemplateRegenerator：nil 依赖守卫返回 nil；合法依赖返回
// 闭包——契约加载失败包装错误、空契约集安全走 RegenerateFromContracts。
func TestBuildContractTemplateRegenerator(t *testing.T) {
	assert.Nil(t, buildContractTemplateRegenerator(nil, nil))
	assert.Nil(t, buildContractTemplateRegenerator(&svc.ServiceContext{}, nil))

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	serverCtx := &svc.ServiceContext{DB: db}
	componentHandler := component.NewHandler(
		model.NewComponentTemplateModel(db),
		model.NewFunctionContractModel(db))

	regen := buildContractTemplateRegenerator(serverCtx, componentHandler)
	require.NotNil(t, regen)

	// 空契约集：regenerate 为 no-op 成功
	require.NoError(t, regen(context.Background(), "demo-game", "development"))

	// 契约加载失败：闭包包装错误
	closedCtx := &svc.ServiceContext{DB: closedTemplateRegenDB(t)}
	closedRegen := buildContractTemplateRegenerator(closedCtx, componentHandler)
	require.NotNil(t, closedRegen)
	err = closedRegen(context.Background(), "demo-game", "development")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load contracts for template regen")
}

func closedTemplateRegenDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	return db
}
