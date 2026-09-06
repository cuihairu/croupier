package handler

import (
	"context"
	"encoding/json"
	"testing"

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
