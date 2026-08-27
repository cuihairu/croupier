package extension

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	extensioncatalog "github.com/cuihairu/croupier/internal/core/extension/catalog"
	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	extensionmanifest "github.com/cuihairu/croupier/internal/core/extension/manifest"
	extensionruntime "github.com/cuihairu/croupier/internal/core/extension/runtime"
	extensionsync "github.com/cuihairu/croupier/internal/core/extension/sync"
	"github.com/cuihairu/croupier/internal/model"
	extensiongorm "github.com/cuihairu/croupier/internal/repo/gorm/extension"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type extensionTestEnv struct {
	db      *gorm.DB
	svcCtx  *svc.ServiceContext
	service *Service
	handler *Handler
	router  *gin.Engine
	ctx     context.Context
}

func extensionSharedDB(t *testing.T) *gorm.DB {
	t.Helper()
	// A fresh private in-memory database per test keeps installations fully
	// isolated (shared cache=shared databases intermittently lose tables).
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := model.AutoMigrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func setupExtensionEnv(t *testing.T) *extensionTestEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := extensionSharedDB(t)

	repos := extensiongorm.NewBundle(db)
	installationSvc := extensioninstallation.NewService(repos.Installation, repos.Event, repos.Binding)
	runtimeSvc := extensionruntime.NewService(repos.Installation, repos.Binding, repos.Event)
	syncSvc := extensionsync.NewService(repos.Installation, repos.Binding)
	catalogSvc := extensioncatalog.NewService(repos.Catalog, repos.Release)
	manifestSvc := extensionmanifest.NewService()

	// Admin + role holding both read and write permissions.
	admin := model.Admin{Username: "ext_tester", Status: 1}
	require.NoError(t, db.Create(&admin).Error)
	role := model.Role{Name: "ext_tester_role"}
	require.NoError(t, db.Create(&role).Error)
	require.NoError(t, db.Create(&model.AdminRole{AdminID: admin.ID, RoleID: role.ID}).Error)
	for _, perm := range []string{"extension:read", "extension:write"} {
		require.NoError(t, db.Create(&model.Permission{ID: perm, Name: perm, Resource: "extension", Action: "read"}).Error)
		require.NoError(t, db.Create(&model.RolePermission{RoleID: role.ID, PermissionID: perm}).Error)
	}

	svcCtx := &svc.ServiceContext{
		DB:         db,
		AdminModel: model.NewAdminModel(db),
		RoleModel:  model.NewRoleModel(db),
		Extensions: &svc.ExtensionServices{
			Catalog:      catalogSvc,
			Manifest:     manifestSvc,
			Installation: installationSvc,
			Runtime:      runtimeSvc,
			Sync:         syncSvc,
		},
	}

	service := NewService(svcCtx)
	handler := NewHandler(service)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), "username", "ext_tester"))
		c.Set("username", "ext_tester")
		c.Next()
	})
	router.GET("/api/v1/extensions/catalog", handler.CatalogList)
	router.GET("/api/v1/extensions/catalog/:id", handler.CatalogDetail)
	router.GET("/api/v1/extensions/catalog/:id/releases", handler.CatalogReleases)
	router.POST("/api/v1/extensions/installations", handler.Install)
	router.GET("/api/v1/extensions/installations", handler.InstallationList)
	router.GET("/api/v1/extensions/installations/:id", handler.InstallationDetail)
	router.PUT("/api/v1/extensions/installations/:id/config", handler.UpdateConfig)
	router.GET("/api/v1/extensions/installations/:id/config-schema", handler.ConfigSchema)
	router.GET("/api/v1/extensions/installations/:id/config", handler.Config)
	router.POST("/api/v1/extensions/installations/:id/test-connection", handler.TestConnection)
	router.GET("/api/v1/extensions/installations/:id/capabilities", handler.Capabilities)
	router.GET("/api/v1/extensions/installations/:id/pages", handler.Pages)
	router.POST("/api/v1/extensions/installations/:id/health-check", handler.HealthCheck)
	router.POST("/api/v1/extensions/installations/:id/enable", handler.Enable)
	router.POST("/api/v1/extensions/installations/:id/disable", handler.Disable)
	router.POST("/api/v1/extensions/installations/:id/upgrade", handler.Upgrade)
	router.POST("/api/v1/extensions/installations/:id/reconcile", handler.Reconcile)
	router.DELETE("/api/v1/extensions/installations/:id", handler.Uninstall)
	router.GET("/api/v1/extensions/installations/:id/events", handler.Events)
	router.GET("/api/v1/agents/:agentId/extensions/sync", handler.AgentSyncPayload)

	ctx := context.WithValue(context.Background(), "username", "ext_tester")
	return &extensionTestEnv{
		db:      db,
		svcCtx:  svcCtx,
		service: service,
		handler: handler,
		router:  router,
		ctx:     ctx,
	}
}

func (e *extensionTestEnv) do(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

func (e *extensionTestEnv) seedCatalog(t *testing.T, extID, version string, manifest map[string]any) {
	t.Helper()
	raw, err := json.Marshal(manifest)
	require.NoError(t, err)
	catalog := model.ExtensionCatalog{
		ExtensionID:   extID,
		Name:          extID,
		DisplayName:   extID + " display",
		Vendor:        "official",
		Kind:          "official",
		Status:        "active",
		LatestVersion: version,
	}
	require.NoError(t, e.db.Create(&catalog).Error)
	release := model.ExtensionRelease{
		ExtensionID:     extID,
		Version:         version,
		ManifestJSON:    model.JSON(raw),
		PublishedAtUnix: 1700000000,
	}
	require.NoError(t, e.db.Create(&release).Error)
}

func (e *extensionTestEnv) install(t *testing.T, extID, version string) uint {
	t.Helper()
	return e.installWithConfig(t, extID, version, nil)
}

func (e *extensionTestEnv) installWithConfig(t *testing.T, extID, version string, config map[string]any) uint {
	t.Helper()
	resp, err := e.service.Install(e.ctx, ExtensionInstallRequest{
		ExtensionID:    extID,
		ReleaseVersion: version,
		ScopeType:      "global",
		ScopeID:        "global",
		TargetType:     "global",
		Config:         config,
	}, "tester")
	require.NoError(t, err)
	return resp.InstallationID
}

// ---------------------------------------------------------------------------
// Catalog
// ---------------------------------------------------------------------------

func TestExtensionFlow_Catalog(t *testing.T) {
	env := setupExtensionEnv(t)

	env.seedCatalog(t, "demo.pay", "1.0.0", map[string]any{
		"tags":            []any{"payment", "official"},
		"default_install": true,
		"capabilities":    []any{"pay.submit", "pay.refund"},
		"ui":              map[string]any{"pages": []any{"/pay/console", ""}},
	})

	rec := env.do(t, http.MethodGet, "/api/v1/extensions/catalog", "")
	assert.Equal(t, http.StatusOK, rec.Code)
	var listResp struct {
		Total int                    `json:"total"`
		Items []ExtensionCatalogItem `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &listResp))
	require.Len(t, listResp.Items, 1)
	assert.True(t, listResp.Items[0].DefaultInstall)
	assert.Contains(t, listResp.Items[0].Tags, "payment")

	rec = env.do(t, http.MethodGet, "/api/v1/extensions/catalog/demo.pay", "")
	assert.Equal(t, http.StatusOK, rec.Code)
	var detail ExtensionCatalogDetailResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &detail))
	require.NotNil(t, detail.Item)
	assert.Len(t, detail.Releases, 1)
	assert.Contains(t, detail.Capabilities, "pay.submit")

	// Missing / empty ids.
	rec = env.do(t, http.MethodGet, "/api/v1/extensions/catalog/%20", "")
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	_, err := env.service.CatalogDetail(env.ctx, "no-such-ext")
	require.Error(t, err)

	_, err = env.service.CatalogReleases(env.ctx, " ")
	require.Error(t, err)
	_, err = env.service.CatalogReleases(env.ctx, "no-such-ext")
	require.Error(t, err)

	rec = env.do(t, http.MethodGet, "/api/v1/extensions/catalog/demo.pay/releases", "")
	assert.Equal(t, http.StatusOK, rec.Code)
	var releases struct {
		Releases []ExtensionReleaseItem `json:"releases"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &releases))
	assert.Len(t, releases.Releases, 1)
}

func TestExtensionFlow_CatalogList_InstalledFlag(t *testing.T) {
	env := setupExtensionEnv(t)

	env.seedCatalog(t, "demo.installed", "1.0.0", nil)
	env.seedCatalog(t, "demo.other", "1.0.0", nil)
	env.install(t, "demo.installed", "1.0.0")

	resp, err := env.service.CatalogList(env.ctx, ExtensionCatalogListRequest{})
	require.NoError(t, err)
	installed := map[string]bool{}
	for _, item := range resp.Items {
		installed[item.ID] = item.Installed
	}
	assert.True(t, installed["demo.installed"])
	assert.False(t, installed["demo.other"])
}

// ---------------------------------------------------------------------------
// Install / list / detail
// ---------------------------------------------------------------------------

func TestExtensionFlow_InstallValidation(t *testing.T) {
	env := setupExtensionEnv(t)
	env.seedCatalog(t, "demo.base", "1.0.0", nil)

	_, err := env.service.Install(env.ctx, ExtensionInstallRequest{
		ExtensionID: "demo.base", ReleaseVersion: "1.0.0",
		ScopeType: "game", ScopeID: "", TargetType: "global",
	}, "tester")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be provided together")

	_, err = env.service.Install(env.ctx, ExtensionInstallRequest{
		ExtensionID: "demo.base", ReleaseVersion: "1.0.0",
		ScopeType: "bogus-scope", ScopeID: "x", TargetType: "global",
	}, "tester")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported extension scope_type")

	// Missing dependency.
	env.seedCatalog(t, "demo.child", "1.0.0", map[string]any{
		"dependencies": []any{"demo.missing"},
	})
	_, err = env.service.Install(env.ctx, ExtensionInstallRequest{
		ExtensionID: "demo.child", ReleaseVersion: "1.0.0",
		ScopeType: "global", ScopeID: "global", TargetType: "global",
	}, "tester")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing dependency extension")

	// Config schema violation.
	env.seedCatalog(t, "demo.schema", "2.0.0", map[string]any{
		"config_schema": map[string]any{
			"required": []any{"endpoint"},
			"properties": map[string]any{
				"endpoint": map[string]any{"type": "string"},
			},
		},
	})
	_, err = env.service.Install(env.ctx, ExtensionInstallRequest{
		ExtensionID: "demo.schema", ReleaseVersion: "2.0.0",
		ScopeType: "global", ScopeID: "global", TargetType: "global",
		Config: map[string]any{"other": 1},
	}, "tester")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required field endpoint")
}

func TestExtensionFlow_InstallConflict(t *testing.T) {
	env := setupExtensionEnv(t)
	env.seedCatalog(t, "demo.conflict", "1.0.0", nil)

	id := env.install(t, "demo.conflict", "1.0.0")
	assert.Greater(t, id, uint(0))

	_, err := env.service.Install(env.ctx, ExtensionInstallRequest{
		ExtensionID: "demo.conflict", ReleaseVersion: "1.0.0",
		ScopeType: "global", ScopeID: "global", TargetType: "global",
	}, "tester")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already installed")

	// Different scope installs fine.
	_, err = env.service.Install(env.ctx, ExtensionInstallRequest{
		ExtensionID: "demo.conflict", ReleaseVersion: "1.0.0",
		ScopeType: "game", ScopeID: "game-1", TargetType: "global",
	}, "tester")
	require.NoError(t, err)
}

func TestExtensionFlow_InstallationListAndDetail(t *testing.T) {
	env := setupExtensionEnv(t)
	env.seedCatalog(t, "demo.list", "1.0.0", map[string]any{
		"config_schema": map[string]any{
			"properties": map[string]any{
				"retries": map[string]any{"type": "number"},
			},
		},
	})

	id := env.install(t, "demo.list", "1.0.0")

	resp, err := env.service.InstallationList(env.ctx, ExtensionInstallationListRequest{ScopeType: "game", ScopeID: "game-1"})
	require.NoError(t, err)
	assert.Zero(t, resp.Total)

	resp, err = env.service.InstallationList(env.ctx, ExtensionInstallationListRequest{ExtensionID: "demo.list"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.Total)

	_, err = env.service.InstallationDetail(env.ctx, id+999)
	require.Error(t, err)

	detail, err := env.service.InstallationDetail(env.ctx, id)
	require.NoError(t, err)
	require.NotNil(t, detail.Installation)
	assert.Equal(t, "demo.list", detail.Installation.ExtensionID)
	assert.NotNil(t, detail.ConfigSchema)

	rec := env.do(t, http.MethodGet, fmt.Sprintf("/api/v1/extensions/installations/%d", id), "")
	assert.Equal(t, http.StatusOK, rec.Code)

	rec = env.do(t, http.MethodGet, "/api/v1/extensions/installations/not-a-number", "")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// ---------------------------------------------------------------------------
// Config / schema / connection
// ---------------------------------------------------------------------------

func TestExtensionFlow_ConfigEndpoints(t *testing.T) {
	env := setupExtensionEnv(t)
	env.seedCatalog(t, "demo.cfg", "1.0.0", map[string]any{
		"config_schema": map[string]any{
			"required": []any{"endpoint"},
			"properties": map[string]any{
				"endpoint": map[string]any{"type": "string"},
				"mode":     map[string]any{"type": "string", "enum": []any{"fast", "slow"}},
			},
		},
	})

	id := env.installWithConfig(t, "demo.cfg", "1.0.0", map[string]any{"endpoint": "http://seed"})

	schema, err := env.service.ConfigSchema(env.ctx, id)
	require.NoError(t, err)
	assert.NotEmpty(t, schema.Schema)

	cfg, err := env.service.Config(env.ctx, id)
	require.NoError(t, err)
	assert.NotNil(t, cfg.Config)

	// Update violates enum.
	_, err = env.service.UpdateConfig(env.ctx, id, ExtensionConfigUpdateRequest{
		Config: map[string]any{"endpoint": "http://x", "mode": "turbo"},
	}, "tester")
	require.Error(t, err)

	// Update violates required field.
	_, err = env.service.UpdateConfig(env.ctx, id, ExtensionConfigUpdateRequest{
		Config: map[string]any{"mode": "fast"},
	}, "tester")
	require.Error(t, err)

	_, err = env.service.UpdateConfig(env.ctx, id, ExtensionConfigUpdateRequest{
		Config:     map[string]any{"endpoint": "http://x", "mode": "slow"},
		SecretRefs: map[string]string{"token": "vault://t"},
	}, "tester")
	require.NoError(t, err)

	cfg, err = env.service.Config(env.ctx, id)
	require.NoError(t, err)
	assert.Equal(t, "http://x", cfg.Config["endpoint"])
	assert.Equal(t, "vault://t", cfg.SecretRefs["token"])

	_, err = env.service.Config(env.ctx, 424242)
	require.Error(t, err)
	_, err = env.service.ConfigSchema(env.ctx, 424242)
	require.Error(t, err)
	_, err = env.service.UpdateConfig(env.ctx, 424242, ExtensionConfigUpdateRequest{}, "t")
	require.Error(t, err)

	// Handler-level bind failures.
	rec := env.do(t, http.MethodPut, fmt.Sprintf("/api/v1/extensions/installations/%d/config", id), "{invalid")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	rec = env.do(t, http.MethodPut, "/api/v1/extensions/installations/bad/config", `{}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestExtensionFlow_TestConnectionAndHealthCheck(t *testing.T) {
	env := setupExtensionEnv(t)
	env.seedCatalog(t, "demo.health", "1.0.0", nil)

	id := env.install(t, "demo.health", "1.0.0")

	resp, err := env.service.TestConnection(env.ctx, id, "tester")
	require.NoError(t, err)
	assert.Equal(t, "disabled", resp.Status) // fresh install is disabled

	require.NoError(t, env.svcCtx.Extensions.Installation.Enable(env.ctx, id, "tester"))
	resp, err = env.service.TestConnection(env.ctx, id, "tester")
	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Status)

	health, err := env.service.HealthCheck(env.ctx, id, "tester")
	require.NoError(t, err)
	assert.Equal(t, "healthy", health.Status)

	require.NoError(t, env.svcCtx.Extensions.Installation.Uninstall(env.ctx, id, "tester"))
	health, err = env.service.HealthCheck(env.ctx, id, "tester")
	require.NoError(t, err)
	assert.Equal(t, "uninstalled", health.Status)

	_, err = env.service.TestConnection(env.ctx, 987654, "tester")
	require.Error(t, err)
	_, err = env.service.HealthCheck(env.ctx, 987654, "tester")
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// Capabilities / pages
// ---------------------------------------------------------------------------

func TestExtensionFlow_CapabilitiesAndPages(t *testing.T) {
	env := setupExtensionEnv(t)
	env.seedCatalog(t, "demo.ui", "1.0.0", map[string]any{
		"capabilities": []any{"chart.render"},
		"ui":           map[string]any{"pages": []any{"/ui/one", "/ui/two"}},
	})

	id := env.install(t, "demo.ui", "1.0.0")

	// No bindings yet: manifest fallback kicks in.
	caps, err := env.service.Capabilities(env.ctx, id)
	require.NoError(t, err)
	assert.Contains(t, caps.Capabilities, "chart.render")
	require.NotEmpty(t, caps.Details)

	pages, err := env.service.Pages(env.ctx, id)
	require.NoError(t, err)
	require.Len(t, pages.Pages, 2)
	assert.Equal(t, "/ui/one", pages.Pages[0].Route)

	// Runtime bindings override manifest data.
	require.NoError(t, env.db.Create(&model.ExtensionRuntimeBinding{
		InstallationID: id,
		BindingType:    "capability",
		BindingKey:     "ticket.manage",
		SpecJSON:       model.JSON([]byte(`{"operations":["create","close"]}`)),
	}).Error)
	require.NoError(t, env.db.Create(&model.ExtensionRuntimeBinding{
		InstallationID: id,
		BindingType:    "page",
		BindingKey:     "binding/page",
		SpecJSON:       model.JSON([]byte(`{"title":"Binding Page","order":5}`)),
	}).Error)

	caps, err = env.service.Capabilities(env.ctx, id)
	require.NoError(t, err)
	assert.Contains(t, caps.Capabilities, "ticket.manage")

	pages, err = env.service.Pages(env.ctx, id)
	require.NoError(t, err)
	assert.Equal(t, "/binding/page", pages.Pages[0].Route)

	_, err = env.service.Capabilities(env.ctx, 55555)
	require.Error(t, err)
	_, err = env.service.Pages(env.ctx, 55555)
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// Lifecycle: enable / disable / upgrade / reconcile / uninstall
// ---------------------------------------------------------------------------

func TestExtensionFlow_LifecycleActions(t *testing.T) {
	env := setupExtensionEnv(t)
	env.seedCatalog(t, "demo.life", "1.0.0", map[string]any{})
	env.seedCatalogReleaseRow(t, "demo.life", "1.1.0", nil)

	id := env.install(t, "demo.life", "1.0.0")

	enableResp, err := env.service.Enable(env.ctx, id, "tester")
	require.NoError(t, err)
	assert.Equal(t, "enabled", enableResp.Status)

	disableResp, err := env.service.Disable(env.ctx, id, "tester")
	require.NoError(t, err)
	assert.Equal(t, "disabled", disableResp.Status)

	_, err = env.service.Enable(env.ctx, 654321, "tester")
	require.Error(t, err)
	_, err = env.service.Disable(env.ctx, 654321, "tester")
	require.Error(t, err)

	// Upgrade validation branches.
	_, err = env.service.Upgrade(env.ctx, id, "  ", "tester")
	require.Error(t, err)
	_, err = env.service.Upgrade(env.ctx, id, "1.0.0", "tester")
	require.Error(t, err) // same version conflict
	_, err = env.service.Upgrade(env.ctx, id, "9.9.9", "tester")
	require.Error(t, err) // not in catalog

	upgraded, err := env.service.Upgrade(env.ctx, id, "1.1.0", "tester")
	require.NoError(t, err)
	assert.Equal(t, "upgraded", upgraded.Status)

	reconciled, err := env.service.Reconcile(env.ctx, id)
	require.NoError(t, err)
	assert.Equal(t, "enabled", reconciled.Status)

	_, err = env.service.Reconcile(env.ctx, 654321)
	require.Error(t, err)

	uninstalled, err := env.service.Uninstall(env.ctx, id, "tester")
	require.NoError(t, err)
	assert.Equal(t, "uninstalled", uninstalled.Status)
}

func (e *extensionTestEnv) seedCatalogReleaseRow(t *testing.T, extID, version string, manifest map[string]any) {
	t.Helper()
	if manifest == nil {
		manifest = map[string]any{}
	}
	raw, err := json.Marshal(manifest)
	require.NoError(t, err)
	require.NoError(t, e.db.Create(&model.ExtensionRelease{
		ExtensionID:     extID,
		Version:         version,
		ManifestJSON:    model.JSON(raw),
		PublishedAtUnix: 1700000001,
	}).Error)
}

func TestExtensionFlow_UninstallBlockedByDependents(t *testing.T) {
	env := setupExtensionEnv(t)
	env.seedCatalog(t, "demo.parent", "1.0.0", nil)
	env.seedCatalog(t, "demo.child2", "1.0.0", map[string]any{
		"dependencies": []any{map[string]any{"id": "demo.parent", "version": "^1.0.0"}},
	})

	parentID := env.install(t, "demo.parent", "1.0.0")
	env.install(t, "demo.child2", "1.0.0")

	_, err := env.service.Uninstall(env.ctx, parentID, "tester")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required by installed extensions")
}

func TestExtensionFlow_DependencyVersionAndCycle(t *testing.T) {
	env := setupExtensionEnv(t)
	env.seedCatalog(t, "demo.depbase", "1.2.0", nil)
	env.seedCatalog(t, "demo.depchild", "1.0.0", map[string]any{
		"dependencies": []any{map[string]any{"id": "demo.depbase", "version": "^2.0.0"}},
	})
	env.seedCatalog(t, "demo.cycle-a", "1.0.0", map[string]any{
		"dependencies": []any{"demo.cycle-b"},
	})
	env.seedCatalog(t, "demo.cycle-b", "1.0.0", map[string]any{
		"dependencies": []any{"demo.cycle-a"},
	})

	env.install(t, "demo.depbase", "1.2.0")

	// Version mismatch.
	_, err := env.service.Install(env.ctx, ExtensionInstallRequest{
		ExtensionID: "demo.depchild", ReleaseVersion: "1.0.0",
		ScopeType: "global", ScopeID: "global", TargetType: "global",
	}, "tester")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dependency version mismatch")

	// Dependency cycle: seed an active installation for cycle-a directly so
	// installing cycle-b recurses back onto itself through a's manifest.
	require.NoError(t, env.db.Create(&model.ExtensionInstallation{
		InstallationKey: "demo.cycle-a:global:global:global::1.0.0",
		ExtensionID:     "demo.cycle-a",
		ReleaseVersion:  "1.0.0",
		ScopeType:       "global",
		ScopeID:         "global",
		TargetType:      "global",
		Status:          "installed",
		DesiredState:    "disabled",
	}).Error)

	_, err = env.service.Install(env.ctx, ExtensionInstallRequest{
		ExtensionID: "demo.cycle-b", ReleaseVersion: "1.0.0",
		ScopeType: "global", ScopeID: "global", TargetType: "global",
	}, "tester")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dependency cycle")
}

// ---------------------------------------------------------------------------
// Events / agent sync / resolve id
// ---------------------------------------------------------------------------

func TestExtensionFlow_EventsAndAgentSync(t *testing.T) {
	env := setupExtensionEnv(t)
	env.seedCatalog(t, "demo.evt", "1.0.0", nil)

	id := env.install(t, "demo.evt", "1.0.0")
	_, err := env.service.TestConnection(env.ctx, id, "tester")
	require.NoError(t, err)
	_, err = env.service.HealthCheck(env.ctx, id, "tester")
	require.NoError(t, err)

	events, err := env.service.Events(env.ctx, id, ExtensionEventListRequest{})
	require.NoError(t, err)
	assert.Greater(t, events.Total, int64(0))

	events, err = env.service.Events(env.ctx, id, ExtensionEventListRequest{Level: "warn", Page: 0, PageSize: 500})
	require.NoError(t, err)
	assert.Empty(t, events.Items)

	_, err = env.service.AgentSyncPayload(env.ctx, " ")
	require.Error(t, err)

	syncResp, err := env.service.AgentSyncPayload(env.ctx, "agent-1")
	require.NoError(t, err)
	assert.NotNil(t, syncResp.Payload)

	rec := env.do(t, http.MethodGet, "/api/v1/agents/agent-1/extensions/sync", "")
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestExtensionFlow_ResolveInstallationID(t *testing.T) {
	env := setupExtensionEnv(t)
	env.seedCatalog(t, "demo.resolve", "1.0.0", nil)

	id := env.install(t, "demo.resolve", "1.0.0")

	got, err := env.service.ResolveInstallationID(env.ctx, fmt.Sprintf("%d", id))
	require.NoError(t, err)
	assert.Equal(t, id, got)

	got, err = env.service.ResolveInstallationID(env.ctx, "demo.resolve")
	require.NoError(t, err)
	assert.Equal(t, id, got)

	_, err = env.service.ResolveInstallationID(env.ctx, "demo.unknown-ext")
	require.Error(t, err)

	_, err = env.service.ResolveInstallationID(env.ctx, "18446744073709551616")
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// Permission guards
// ---------------------------------------------------------------------------

func TestExtensionFlow_PermissionDenied(t *testing.T) {
	env := setupExtensionEnv(t)
	env.seedCatalog(t, "demo.perm", "1.0.0", nil)

	anonCtx := context.Background()
	_, err := env.service.CatalogList(anonCtx, ExtensionCatalogListRequest{})
	require.Error(t, err)

	_, err = env.service.Install(anonCtx, ExtensionInstallRequest{}, "x")
	require.Error(t, err)
	_, err = env.service.InstallationList(anonCtx, ExtensionInstallationListRequest{})
	require.Error(t, err)
	_, err = env.service.InstallationDetail(anonCtx, 1)
	require.Error(t, err)
	_, err = env.service.UpdateConfig(anonCtx, 1, ExtensionConfigUpdateRequest{}, "x")
	require.Error(t, err)
	_, err = env.service.ConfigSchema(anonCtx, 1)
	require.Error(t, err)
	_, err = env.service.Config(anonCtx, 1)
	require.Error(t, err)
	_, err = env.service.TestConnection(anonCtx, 1, "x")
	require.Error(t, err)
	_, err = env.service.Capabilities(anonCtx, 1)
	require.Error(t, err)
	_, err = env.service.Pages(anonCtx, 1)
	require.Error(t, err)
	_, err = env.service.HealthCheck(anonCtx, 1, "x")
	require.Error(t, err)
	_, err = env.service.Enable(anonCtx, 1, "x")
	require.Error(t, err)
	_, err = env.service.Disable(anonCtx, 1, "x")
	require.Error(t, err)
	_, err = env.service.Upgrade(anonCtx, 1, "1.0.0", "x")
	require.Error(t, err)
	_, err = env.service.Reconcile(anonCtx, 1)
	require.Error(t, err)
	_, err = env.service.Uninstall(anonCtx, 1, "x")
	require.Error(t, err)
	_, err = env.service.Events(anonCtx, 1, ExtensionEventListRequest{})
	require.Error(t, err)
	_, err = env.service.AgentSyncPayload(anonCtx, "agent")
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// Handler branches
// ---------------------------------------------------------------------------

func TestExtensionFlow_HandlerBranches(t *testing.T) {
	env := setupExtensionEnv(t)
	env.seedCatalog(t, "demo.handler", "1.0.0", nil)
	id := env.install(t, "demo.handler", "1.0.0")

	// Bind failures.
	rec := env.do(t, http.MethodPost, "/api/v1/extensions/installations", `{"extensionId":"x"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code) // missing required fields

	rec = env.do(t, http.MethodGet, "/api/v1/extensions/installations?page=abc", "")
	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	rec = env.do(t, http.MethodGet, "/api/v1/extensions/installations/"+fmt.Sprint(id)+"/events?page=abc", "")
	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	rec = env.do(t, http.MethodPost, fmt.Sprintf("/api/v1/extensions/installations/%d/upgrade", id), "{bad")
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// Successful lifecycle through HTTP.
	rec = env.do(t, http.MethodPost, fmt.Sprintf("/api/v1/extensions/installations/%d/enable", id), "")
	assert.Equal(t, http.StatusOK, rec.Code)

	rec = env.do(t, http.MethodPost, fmt.Sprintf("/api/v1/extensions/installations/%d/disable", id), "")
	assert.Equal(t, http.StatusOK, rec.Code)

	rec = env.do(t, http.MethodPost, fmt.Sprintf("/api/v1/extensions/installations/%d/reconcile", id), "")
	assert.Equal(t, http.StatusOK, rec.Code)

	rec = env.do(t, http.MethodGet, fmt.Sprintf("/api/v1/extensions/installations/%d/capabilities", id), "")
	assert.Equal(t, http.StatusOK, rec.Code)

	rec = env.do(t, http.MethodGet, fmt.Sprintf("/api/v1/extensions/installations/%d/pages", id), "")
	assert.Equal(t, http.StatusOK, rec.Code)

	rec = env.do(t, http.MethodPost, fmt.Sprintf("/api/v1/extensions/installations/%d/test-connection", id), "")
	assert.Equal(t, http.StatusOK, rec.Code)

	rec = env.do(t, http.MethodPost, fmt.Sprintf("/api/v1/extensions/installations/%d/health-check", id), "")
	assert.Equal(t, http.StatusOK, rec.Code)

	rec = env.do(t, http.MethodGet, fmt.Sprintf("/api/v1/extensions/installations/%d/config", id), "")
	assert.Equal(t, http.StatusOK, rec.Code)

	rec = env.do(t, http.MethodGet, fmt.Sprintf("/api/v1/extensions/installations/%d/config-schema", id), "")
	assert.Equal(t, http.StatusOK, rec.Code)

	// action helper with bad id.
	rec = env.do(t, http.MethodPost, "/api/v1/extensions/installations/oops/enable", "")
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	rec = env.do(t, http.MethodDelete, fmt.Sprintf("/api/v1/extensions/installations/%d", id), "")
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestExtensionFlow_CompatIdentifierRoutes(t *testing.T) {
	env := setupExtensionEnv(t)
	env.seedCatalog(t, "demo.compat", "1.0.0", nil)
	id := env.install(t, "demo.compat", "1.0.0")

	// Compat routes accept either numeric id or extension id.
	router := env.router
	router.GET("/compat/:id/config-schema", env.handler.CompatConfigSchema)
	router.GET("/compat/:id/config", env.handler.CompatConfig)
	router.PUT("/compat/:id/config", env.handler.CompatUpdateConfig)
	router.POST("/compat/:id/test-connection", env.handler.CompatTestConnection)
	router.GET("/compat/:id/capabilities", env.handler.CompatCapabilities)
	router.GET("/compat/:id/pages", env.handler.CompatPages)
	router.POST("/compat/:id/health-check", env.handler.CompatHealthCheck)
	router.GET("/compat/:id/events", env.handler.CompatEvents)
	router.POST("/compat/:id/enable", env.handler.CompatEnable)
	router.POST("/compat/:id/disable", env.handler.CompatDisable)
	router.POST("/compat/:id/upgrade", env.handler.CompatUpgrade)
	router.POST("/compat/:id/reconcile", env.handler.CompatReconcile)
	router.DELETE("/compat/:id", env.handler.CompatUninstall)

	for _, target := range []string{fmt.Sprintf("%d", id), "demo.compat"} {
		rec := env.do(t, http.MethodGet, "/compat/"+target+"/config-schema", "")
		assert.Equal(t, http.StatusOK, rec.Code)

		rec = env.do(t, http.MethodGet, "/compat/"+target+"/config", "")
		assert.Equal(t, http.StatusOK, rec.Code)

		rec = env.do(t, http.MethodGet, "/compat/"+target+"/capabilities", "")
		assert.Equal(t, http.StatusOK, rec.Code)

		rec = env.do(t, http.MethodGet, "/compat/"+target+"/pages", "")
		assert.Equal(t, http.StatusOK, rec.Code)

		rec = env.do(t, http.MethodPost, "/compat/"+target+"/test-connection", "")
		assert.Equal(t, http.StatusOK, rec.Code)

		rec = env.do(t, http.MethodPost, "/compat/"+target+"/health-check", "")
		assert.Equal(t, http.StatusOK, rec.Code)
	}

	rec := env.do(t, http.MethodPut, "/compat/demo.compat/config", `{"config":{"a":1}}`)
	assert.Equal(t, http.StatusOK, rec.Code)

	rec = env.do(t, http.MethodPut, "/compat/no-such-extension/config", `{"config":{}}`)
	assert.Equal(t, http.StatusNotFound, rec.Code)

	rec = env.do(t, http.MethodGet, "/compat/"+fmt.Sprint(id)+"/events", "")
	assert.Equal(t, http.StatusOK, rec.Code)

	rec = env.do(t, http.MethodPost, "/compat/"+fmt.Sprint(id)+"/enable", "")
	assert.Equal(t, http.StatusOK, rec.Code)
	rec = env.do(t, http.MethodPost, "/compat/"+fmt.Sprint(id)+"/disable", "")
	assert.Equal(t, http.StatusOK, rec.Code)
	rec = env.do(t, http.MethodPost, "/compat/"+fmt.Sprint(id)+"/upgrade", `{"releaseVersion":"1.0.0"}`)
	assert.Equal(t, http.StatusConflict, rec.Code)
	rec = env.do(t, http.MethodPost, "/compat/"+fmt.Sprint(id)+"/reconcile", "")
	assert.Equal(t, http.StatusOK, rec.Code)
	rec = env.do(t, http.MethodDelete, "/compat/"+fmt.Sprint(id), "")
	assert.Equal(t, http.StatusOK, rec.Code)
}
