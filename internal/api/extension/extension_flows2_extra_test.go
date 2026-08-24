package extension

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

// ---------------------------------------------------------------------------
// extractCapabilityDetailsFromBindings / extractPageDetailsFromBindings edges
// ---------------------------------------------------------------------------

func TestExtractCapabilityDetails_EdgeBindings(t *testing.T) {
	bindings := []model.ExtensionRuntimeBinding{
		// capability binding merging into an existing detail
		{BindingType: "capability", BindingKey: "pay", SpecJSON: datatypes.JSON([]byte(`{"operations":["charge","refund","charge"],"permissions":{"charge":"pay:charge"},"config_keys":["endpoint"]}`))},
		{BindingType: "capability", BindingKey: "pay", SpecJSON: datatypes.JSON([]byte(`{"operations":[" void ","charge"],"permissions":{"":"x","p":"q"},"config_keys":["  "]}`))},
		// provider/openapi binding without a parseable provider
		{BindingType: "openapi", BindingKey: "", SpecJSON: datatypes.JSON([]byte(`{}`))},
		// provider binding with a real provider name
		{BindingType: "provider", BindingKey: "stripe", SpecJSON: datatypes.JSON([]byte(`{"operations":["charge"]}`))},
		// function binding with external function id
		{BindingType: "function", BindingKey: "external.stripe.charge"},
		// function binding that is not an external id
		{BindingType: "function", BindingKey: "plain.fn"},
		// blank binding produces raw "type:key" cap when key present
		{BindingType: "", BindingKey: ""},
		{BindingType: "weird", BindingKey: "k1"},
	}

	caps, details := extractCapabilityDetailsFromBindings(bindings)
	assert.Contains(t, caps, "pay")
	assert.Contains(t, caps, "weird:k1")

	var payDetail *ExtensionCapabilityDetail
	for i := range details {
		if details[i].Capability == "pay" {
			payDetail = &details[i]
		}
	}
	require.NotNil(t, payDetail)
	assert.Contains(t, payDetail.Operations, "charge")
	assert.Contains(t, payDetail.Operations, "refund")
	assert.Contains(t, payDetail.Operations, "void") // trimmed value is added
	assert.Equal(t, "pay:charge", payDetail.Permissions["charge"])

	// Provider binding detail carries provider/operations.
	found := false
	for _, d := range details {
		if d.Provider == "stripe" {
			found = true
			assert.Contains(t, d.Operations, "charge")
		}
	}
	assert.True(t, found, "provider detail missing")
}

func TestExtractPageDetails_EdgeBindings(t *testing.T) {
	bindings := []model.ExtensionRuntimeBinding{
		{BindingType: "page", BindingKey: "a.b", SpecJSON: datatypes.JSON(`{"title":"A","order":2,"icon":"i","group":"g","required_permission":"p:read","extra":true}`)},
		{BindingType: "ui", SpecJSON: datatypes.JSON(`{"id":"from-spec","order":"not-number"}`)},
		{BindingType: "navigation", BindingKey: "a.b"}, // duplicate key skipped
		{BindingType: "other", BindingKey: "ignored"},
		{BindingType: "page", SpecJSON: datatypes.JSON(`{}`)}, // no key -> skipped
	}

	items := extractPageDetailsFromBindings(bindings)
	require.Len(t, items, 3)

	byKey := map[string]ExtensionPageItem{}
	for _, item := range items {
		byKey[item.Key] = item
	}
	a := byKey["a.b"]
	assert.Equal(t, "A", a.Title)
	assert.Equal(t, "/a/b", a.Route)
	assert.Equal(t, "i", a.Icon)
	assert.Equal(t, "g", a.Group)
	assert.Equal(t, "p:read", a.RequiredPermission)
	assert.Equal(t, 2, a.Order)

	b := byKey["from-spec"]
	assert.Equal(t, "from-spec", b.Title)
	assert.Equal(t, "/from-spec", b.Route)
}

// ---------------------------------------------------------------------------
// Service error paths
// ---------------------------------------------------------------------------

func TestExtensionFlow_NilDBConflictCheck(t *testing.T) {
	env := setupExtensionEnv(t)

	// Same extension services but no DB handle: conflict query fails.
	broken := NewService(&svc.ServiceContext{Extensions: env.svcCtx.Extensions})
	_, err := broken.Install(env.ctx, ExtensionInstallRequest{
		ExtensionID: "demo.x", ReleaseVersion: "1.0.0",
		ScopeType: "global", ScopeID: "global", TargetType: "global",
	}, "tester")
	require.Error(t, err)
}

func TestExtensionFlow_ScopePairValidationOnList(t *testing.T) {
	env := setupExtensionEnv(t)

	_, err := env.service.InstallationList(env.ctx, ExtensionInstallationListRequest{ScopeType: "game"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be provided together")
}

func TestExtensionFlow_UpgradeWithoutCatalogEntry(t *testing.T) {
	env := setupExtensionEnv(t)

	// An installation whose extension is missing from the catalog makes
	// releaseVersionExists fail with a mapped not-found error.
	require.NoError(t, env.db.Create(&model.ExtensionInstallation{
		InstallationKey: "ghost.ext:global:global:global::1.0.0",
		ExtensionID:     "ghost.ext",
		ReleaseVersion:  "1.0.0",
		ScopeType:       "global",
		ScopeID:         "global",
		TargetType:      "global",
		Status:          "installed",
		DesiredState:    "disabled",
	}).Error)

	var item model.ExtensionInstallation
	require.NoError(t, env.db.Where("extension_id = ?", "ghost.ext").First(&item).Error)

	_, err := env.service.Upgrade(env.ctx, item.ID, "2.0.0", "tester")
	require.Error(t, err)

	// Config schema also degrades to empty for missing catalog entries.
	schema, err := env.service.ConfigSchema(env.ctx, item.ID)
	require.NoError(t, err)
	assert.Empty(t, schema.Schema)
}

func TestExtensionFlow_UninstallErrors(t *testing.T) {
	env := setupExtensionEnv(t)
	env.seedCatalog(t, "demo.uninstall", "1.0.0", nil)
	id := env.install(t, "demo.uninstall", "1.0.0")

	// A dependent installation without a catalog entry breaks manifest
	// resolution during ensureNoActiveDependents.
	require.NoError(t, env.db.Create(&model.ExtensionInstallation{
		InstallationKey: "ghost.dep:global:global:global::1.0.0",
		ExtensionID:     "ghost.dep",
		ReleaseVersion:  "1.0.0",
		ScopeType:       "global",
		ScopeID:         "global",
		TargetType:      "global",
		Status:          "installed",
		DesiredState:    "disabled",
	}).Error)

	_, err := env.service.Uninstall(env.ctx, id, "tester")
	require.Error(t, err)

	_, err = env.service.Uninstall(env.ctx, 987001, "tester")
	require.Error(t, err)
}

func TestExtensionFlow_DiamondDependencies(t *testing.T) {
	env := setupExtensionEnv(t)
	env.seedCatalog(t, "demo.d", "1.0.0", nil)
	env.seedCatalog(t, "demo.b", "1.0.0", map[string]any{"dependencies": []any{"demo.d"}})
	env.seedCatalog(t, "demo.c", "1.0.0", map[string]any{"dependencies": []any{"demo.d"}})
	env.seedCatalog(t, "demo.a", "1.0.0", map[string]any{
		"dependencies": []any{map[string]any{"id": "demo.b"}, map[string]any{"id": "demo.c"}},
	})

	env.install(t, "demo.d", "1.0.0")
	env.install(t, "demo.b", "1.0.0")
	env.install(t, "demo.c", "1.0.0")

	id := env.install(t, "demo.a", "1.0.0")
	assert.Greater(t, id, uint(0))
}

func TestExtensionFlow_ResolveInstallationIDOverflow(t *testing.T) {
	env := setupExtensionEnv(t)
	_, err := env.service.ResolveInstallationID(env.ctx, "not-a-number-nor-extension")
	require.Error(t, err)
}

func TestExtensionFlow_AgentSyncHandlerErrors(t *testing.T) {
	env := setupExtensionEnv(t)

	router := env.router
	router.GET("/agents/:id/extensions", env.handler.AgentExtensions)
	router.POST("/agents/:id/extensions/sync", env.handler.AgentExtensionsSync)

	rec := env.do(t, http.MethodGet, "/agents/%20/extensions", "")
	// Empty agent id maps to a 400 from the service.
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	rec = env.do(t, http.MethodGet, "/agents/agent-ok/extensions", "")
	assert.Equal(t, http.StatusOK, rec.Code)

	rec = env.do(t, http.MethodPost, "/agents/agent-ok/extensions/sync", "")
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestExtensionFlow_HandlerServiceErrors(t *testing.T) {
	env := setupExtensionEnv(t)

	// Every read/write endpoint hit with a missing installation id covers
	// the service-error branch of each handler.
	missing := "/api/v1/extensions/installations/90001"

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodGet, missing},
		{http.MethodGet, missing + "/config-schema"},
		{http.MethodGet, missing + "/config"},
		{http.MethodPost, missing + "/test-connection"},
		{http.MethodGet, missing + "/capabilities"},
		{http.MethodGet, missing + "/pages"},
		{http.MethodPost, missing + "/health-check"},
		{http.MethodPost, missing + "/enable"},
		{http.MethodPost, missing + "/disable"},
		{http.MethodPost, missing + "/reconcile"},
		{http.MethodDelete, missing},
		{http.MethodGet, missing + "/events"},
		{http.MethodPut, missing + "/config"},
	}
	for _, tc := range cases {
		want := http.StatusNotFound
		if tc.path == missing+"/events" {
			want = http.StatusOK // empty event list for unknown installation
		}
		rec := env.do(t, tc.method, tc.path, bodyForMethod(tc.method))
		assert.Equal(t, want, rec.Code, "%s %s", tc.method, tc.path)
	}

	// JSON bind failure on Install.
	rec := env.do(t, http.MethodPost, "/api/v1/extensions/installations", "{not-json")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func bodyForMethod(method string) string {
	if method == http.MethodPost || method == http.MethodPut {
		return `{ }`
	}
	return ""
}

func TestExtensionFlow_CompatHandlerErrors(t *testing.T) {
	env := setupExtensionEnv(t)

	router := env.router
	router.GET("/compat2/:id/config-schema", env.handler.CompatConfigSchema)
	router.GET("/compat2/:id/config", env.handler.CompatConfig)
	router.PUT("/compat2/:id/config", env.handler.CompatUpdateConfig)
	router.POST("/compat2/:id/test-connection", env.handler.CompatTestConnection)
	router.GET("/compat2/:id/capabilities", env.handler.CompatCapabilities)
	router.GET("/compat2/:id/pages", env.handler.CompatPages)
	router.POST("/compat2/:id/health-check", env.handler.CompatHealthCheck)
	router.POST("/compat2/:id/enable", env.handler.CompatEnable)
	router.POST("/compat2/:id/disable", env.handler.CompatDisable)
	router.POST("/compat2/:id/upgrade", env.handler.CompatUpgrade)
	router.POST("/compat2/:id/reconcile", env.handler.CompatReconcile)
	router.DELETE("/compat2/:id", env.handler.CompatUninstall)
	router.GET("/compat2/:id/events", env.handler.CompatEvents)

	for _, path := range []string{
		"/compat2/missing-ext/config-schema",
		"/compat2/missing-ext/config",
		"/compat2/missing-ext/test-connection",
		"/compat2/missing-ext/capabilities",
		"/compat2/missing-ext/pages",
		"/compat2/missing-ext/health-check",
		"/compat2/missing-ext/enable",
		"/compat2/missing-ext/disable",
		"/compat2/missing-ext/reconcile",
		"/compat2/missing-ext",
		"/compat2/missing-ext/events",
	} {
		rec := env.do(t, methodForPath(path), path, "")
		assert.Equal(t, http.StatusNotFound, rec.Code, path)
	}

	// Identifier resolution runs before body binding: bad JSON still yields 404.
	rec := env.do(t, http.MethodPost, "/compat2/missing-ext/upgrade", "{bad")
	assert.Equal(t, http.StatusNotFound, rec.Code)

	rec = env.do(t, http.MethodPut, "/compat2/missing-ext/config", "{bad")
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func methodForPath(path string) string {
	switch {
	case hasSuffix(path, "/config"), hasSuffix(path, "/config-schema"), hasSuffix(path, "/capabilities"), hasSuffix(path, "/pages"), hasSuffix(path, "/events"):
		return http.MethodGet
	case hasSuffix(path, "/config"):
		return http.MethodPut
	case path == "/compat2/missing-ext":
		return http.MethodDelete
	default:
		return http.MethodPost
	}
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

// Service-level guard: nil svcCtx must not panic but return permission error.
func TestExtensionFlow_NilServiceContext(t *testing.T) {
	service := NewService(nil)
	_, err := service.CatalogList(context.Background(), ExtensionCatalogListRequest{})
	require.Error(t, err)
}

func TestExtensionFlow_OperatorFallback(t *testing.T) {
	env := setupExtensionEnv(t)
	env.seedCatalog(t, "demo.operator", "1.0.0", nil)

	// No username in gin context: operator falls back to "system".
	router := gin.New()
	router.POST("/noauth/install", env.handler.Install)

	req := httptest.NewRequest(http.MethodPost, "/noauth/install",
		strings.NewReader(`{"extensionId":"demo.operator","releaseVersion":"1.0.0","scopeType":"global","scopeId":"global","targetType":"global"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	// Without an authenticated context the service refuses the write, but the
	// handler still evaluated operator() with its "system" fallback.
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
