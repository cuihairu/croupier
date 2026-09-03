package extension

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// --- v9 fixtures ---

func newV9DB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return db
}

type v9Fixture struct {
	db     *gorm.DB
	svcCtx *svc.ServiceContext
	ctx    context.Context
	svc    *Service
}

func newV9Fixture(t *testing.T) *v9Fixture {
	t.Helper()
	db := newV9DB(t)
	svcCtx := setupExtensionTestContext(t, db)
	ctx := setupAdminContext(t, svcCtx)
	return &v9Fixture{db: db, svcCtx: svcCtx, ctx: ctx, svc: NewService(svcCtx)}
}

func createV9Catalog(t *testing.T, db *gorm.DB, extID, version, manifest string) {
	t.Helper()
	require.NoError(t, db.Create(&model.ExtensionCatalog{
		ExtensionID:   extID,
		Name:          extID,
		DisplayName:   extID,
		Vendor:        "v9",
		Kind:          "official",
		Status:        "active",
		LatestVersion: version,
	}).Error)
	require.NoError(t, db.Create(&model.ExtensionRelease{
		ExtensionID:     extID,
		Version:         version,
		ReleaseChannel:  "stable",
		MinCoreVersion:  "1.0.0",
		PublishedAtUnix: time.Now().Unix(),
		ManifestJSON:    model.JSON([]byte(manifest)),
	}).Error)
}

func createV9Installation(t *testing.T, db *gorm.DB, extID, version, configJSON string) uint {
	t.Helper()
	item := &model.ExtensionInstallation{
		InstallationKey: extID + "@" + version + "#v9-" + time.Now().Format("150405.000000000"),
		ExtensionID:     extID,
		ReleaseVersion:  version,
		ScopeType:       "system",
		ScopeID:         "global",
		TargetType:      "agent",
		TargetID:        "agent-v9",
		Status:          "enabled",
		DesiredState:    "enabled",
		Enabled:         true,
		ConfigJSON:      model.JSON([]byte(configJSON)),
	}
	require.NoError(t, db.Create(item).Error)
	return item.ID
}

func doV9Request(t *testing.T, r *gin.Engine, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func extKeyV9(prefix string, i int) string {
	return fmt.Sprintf("%s-%d#%d", prefix, i, time.Now().UnixNano())
}

// --- parseUintParam error branches across handlers ---

func TestParseUintParamHandlerErrors_V9(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(&Service{})

	r := gin.New()
	r.GET("/installations/:id", h.InstallationDetail)
	r.PUT("/installations/:id/config", h.UpdateConfig)
	r.GET("/installations/:id/config-schema", h.ConfigSchema)
	r.GET("/installations/:id/raw-config", h.Config)
	r.POST("/installations/:id/test-connection", h.TestConnection)
	r.GET("/installations/:id/capabilities", h.Capabilities)
	r.GET("/installations/:id/pages", h.Pages)
	r.POST("/installations/:id/health-check", h.HealthCheck)
	r.POST("/installations/:id/upgrade", h.Upgrade)
	r.POST("/installations/:id/reconcile", h.Reconcile)
	r.GET("/installations/:id/events", h.Events)
	r.POST("/installations/:id/enable", h.Enable)
	r.POST("/installations/:id/disable", h.Disable)
	r.DELETE("/installations/:id", h.Uninstall)

	targets := []struct{ method, path string }{
		{http.MethodGet, "/installations/abc"},
		{http.MethodPut, "/installations/abc/config"},
		{http.MethodGet, "/installations/abc/config-schema"},
		{http.MethodGet, "/installations/abc/raw-config"},
		{http.MethodPost, "/installations/abc/test-connection"},
		{http.MethodGet, "/installations/abc/capabilities"},
		{http.MethodGet, "/installations/abc/pages"},
		{http.MethodPost, "/installations/abc/health-check"},
		{http.MethodPost, "/installations/abc/upgrade"},
		{http.MethodPost, "/installations/abc/reconcile"},
		{http.MethodGet, "/installations/abc/events"},
		{http.MethodPost, "/installations/abc/enable"},
		{http.MethodPost, "/installations/abc/disable"},
		{http.MethodDelete, "/installations/abc"},
	}
	for _, tc := range targets {
		rec := doV9Request(t, r, tc.method, tc.path, "")
		assert.Equal(t, http.StatusBadRequest, rec.Code, "%s %s", tc.method, tc.path)
	}

	// Events: 合法 id + 非法分页参数 → ShouldBindQuery 失败
	rec := doV9Request(t, r, http.MethodGet, "/installations/1/events?page=abc", "")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- compat handler error branches ---

func TestCompatHandlerErrorBranches_V9(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newV9Fixture(t)
	h := NewHandler(f.svc)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("username", "test_admin")
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), "username", "test_admin"))
	})
	r.PUT("/compat/:id/config", h.CompatUpdateConfig)
	r.POST("/compat/:id/upgrade", h.CompatUpgrade)
	r.GET("/compat/:id/events", h.CompatEvents)
	r.POST("/compat/:id/enable", h.CompatEnable)

	// 非法 JSON body → 绑定失败
	rec := doV9Request(t, r, http.MethodPut, "/compat/1/config", "{invalid")
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	rec = doV9Request(t, r, http.MethodPost, "/compat/1/upgrade", "{invalid")
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// 合法 body 但安装不存在 → service 错误
	rec = doV9Request(t, r, http.MethodPost, "/compat/999/upgrade", `{"releaseVersion":"2.0.0"}`)
	assert.Equal(t, http.StatusNotFound, rec.Code)

	rec = doV9Request(t, r, http.MethodPut, "/compat/999/config", `{"config":{}}`)
	assert.Equal(t, http.StatusNotFound, rec.Code)

	// CompatEvents: 非法分页参数
	rec = doV9Request(t, r, http.MethodGet, "/compat/1/events?page=abc", "")
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// CompatEnable: 扩展名解析 → 安装不存在
	rec = doV9Request(t, r, http.MethodPost, "/compat/ghost.ext/enable", "")
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// --- handler service-error branches ---

func TestHandlerServiceErrorBranches_V9(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newRouterV9 := func(h *Handler) *gin.Engine {
		r := gin.New()
		r.Use(func(c *gin.Context) {
			c.Set("username", "test_admin")
			c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), "username", "test_admin"))
		})
		return r
	}

	t.Run("Install service error", func(t *testing.T) {
		f := newV9Fixture(t)
		h := NewHandler(f.svc)
		r := newRouterV9(h)
		r.POST("/install", h.Install)
		rec := doV9Request(t, r, http.MethodPost, "/install",
			`{"extensionId":"missing.ext","releaseVersion":"1.0.0","scopeType":"system","scopeId":"global","targetType":"agent"}`)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("InstallationList service error", func(t *testing.T) {
		f := newV9Fixture(t)
		require.NoError(t, f.db.Migrator().DropTable("extension_installations"))
		h := NewHandler(f.svc)
		r := newRouterV9(h)
		r.GET("/installations", h.InstallationList)
		rec := doV9Request(t, r, http.MethodGet, "/installations", "")
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("UpdateConfig service error", func(t *testing.T) {
		f := newV9Fixture(t)
		h := NewHandler(f.svc)
		r := newRouterV9(h)
		r.PUT("/installations/:id/config", h.UpdateConfig)
		rec := doV9Request(t, r, http.MethodPut, "/installations/999/config", `{"config":{}}`)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

// --- service error paths ---

func TestCatalogList_Errors_V9(t *testing.T) {
	t.Run("catalog list DB error", func(t *testing.T) {
		f := newV9Fixture(t)
		require.NoError(t, f.db.Migrator().DropTable("extension_catalogs"))
		_, err := f.svc.CatalogList(f.ctx, ExtensionCatalogListRequest{})
		require.Error(t, err)
	})

	t.Run("active installed set DB error", func(t *testing.T) {
		f := newV9Fixture(t)
		createV9Catalog(t, f.db, "list.ext", "1.0.0", `{"id":"list.ext"}`)
		require.NoError(t, f.db.Migrator().DropTable("extension_installations"))
		_, err := f.svc.CatalogList(f.ctx, ExtensionCatalogListRequest{})
		require.Error(t, err)
	})
}

func TestCatalogDetail_FindActiveError_V9(t *testing.T) {
	f := newV9Fixture(t)
	createV9Catalog(t, f.db, "detail.ext", "1.0.0", `{"id":"detail.ext"}`)
	require.NoError(t, f.db.Migrator().DropTable("extension_installations"))
	_, err := f.svc.CatalogDetail(f.ctx, "detail.ext")
	require.Error(t, err)
}

func TestInstall_Errors_V9(t *testing.T) {
	t.Run("conflict lookup DB error", func(t *testing.T) {
		f := newV9Fixture(t)
		createV9Catalog(t, f.db, "install.ext", "1.0.0", `{"id":"install.ext"}`)
		require.NoError(t, f.db.Migrator().DropTable("extension_installations"))
		_, err := f.svc.Install(f.ctx, ExtensionInstallRequest{
			ExtensionID: "install.ext", ReleaseVersion: "1.0.0",
		}, "v9-op")
		require.Error(t, err)
	})

	t.Run("duplicate installation key", func(t *testing.T) {
		f := newV9Fixture(t)
		createV9Catalog(t, f.db, "dup.ext", "1.0.0", `{"id":"dup.ext"}`)
		req := ExtensionInstallRequest{
			ExtensionID: "dup.ext", ReleaseVersion: "1.0.0",
			ScopeType: "system", ScopeID: "global",
			TargetType: "agent", TargetID: "agent-v9",
		}
		_, err := f.svc.Install(f.ctx, req, "v9-op")
		require.NoError(t, err)

		// 标记为已卸载 → 冲突检查放行，但 InstallationKey 唯一索引冲突
		require.NoError(t, f.db.Exec(
			"UPDATE extension_installations SET status='uninstalled', desired_state='uninstalled'",
		).Error)
		_, err = f.svc.Install(f.ctx, req, "v9-op")
		require.Error(t, err)
	})
}

func TestInstallationList_Error_V9(t *testing.T) {
	f := newV9Fixture(t)
	require.NoError(t, f.db.Migrator().DropTable("extension_installations"))
	_, err := f.svc.InstallationList(f.ctx, ExtensionInstallationListRequest{})
	require.Error(t, err)
}

func TestInstallationDetail_Errors_V9(t *testing.T) {
	t.Run("events DB error", func(t *testing.T) {
		f := newV9Fixture(t)
		createV9Catalog(t, f.db, "detail9.ext", "1.0.0", `{"id":"detail9.ext"}`)
		id := createV9Installation(t, f.db, "detail9.ext", "1.0.0", `{}`)
		require.NoError(t, f.db.Migrator().DropTable("extension_events"))
		_, err := f.svc.InstallationDetail(f.ctx, id)
		require.Error(t, err)
	})

	t.Run("bindings DB error", func(t *testing.T) {
		f := newV9Fixture(t)
		createV9Catalog(t, f.db, "detail9.ext", "1.0.0", `{"id":"detail9.ext"}`)
		id := createV9Installation(t, f.db, "detail9.ext", "1.0.0", `{}`)
		require.NoError(t, f.db.Migrator().DropTable("extension_runtime_bindings"))
		_, err := f.svc.InstallationDetail(f.ctx, id)
		require.Error(t, err)
	})
}

func TestUpdateConfig_MarshalError_V9(t *testing.T) {
	f := newV9Fixture(t)
	createV9Catalog(t, f.db, "upd.ext", "1.0.0", `{"id":"upd.ext"}`)
	id := createV9Installation(t, f.db, "upd.ext", "1.0.0", `{}`)

	_, err := f.svc.UpdateConfig(f.ctx, id, ExtensionConfigUpdateRequest{
		Config: map[string]any{"bad": make(chan int)},
	}, "v9-op")
	require.Error(t, err)
}

func TestCapabilities_ErrorsAndFallback_V9(t *testing.T) {
	t.Run("bindings DB error", func(t *testing.T) {
		f := newV9Fixture(t)
		createV9Catalog(t, f.db, "cap.ext", "1.0.0", `{"id":"cap.ext"}`)
		id := createV9Installation(t, f.db, "cap.ext", "1.0.0", `{}`)
		require.NoError(t, f.db.Migrator().DropTable("extension_runtime_bindings"))
		_, err := f.svc.Capabilities(f.ctx, id)
		require.Error(t, err)
	})

	t.Run("manifest fallback", func(t *testing.T) {
		f := newV9Fixture(t)
		createV9Catalog(t, f.db, "mfb.ext", "1.0.0",
			`{"id":"mfb.ext","capabilities":["mfb.read","mfb.write"]}`)
		id := createV9Installation(t, f.db, "mfb.ext", "1.0.0", `{}`)

		resp, err := f.svc.Capabilities(f.ctx, id)
		require.NoError(t, err)
		assert.Contains(t, resp.Capabilities, "mfb.read")
		assert.Contains(t, resp.Capabilities, "mfb.write")
		require.NotEmpty(t, resp.Details)
		assert.Equal(t, "manifest", resp.Details[0].Source)
	})
}

func TestPages_ErrorsAndFallback_V9(t *testing.T) {
	t.Run("bindings DB error", func(t *testing.T) {
		f := newV9Fixture(t)
		createV9Catalog(t, f.db, "pg.ext", "1.0.0", `{"id":"pg.ext"}`)
		id := createV9Installation(t, f.db, "pg.ext", "1.0.0", `{}`)
		require.NoError(t, f.db.Migrator().DropTable("extension_runtime_bindings"))
		_, err := f.svc.Pages(f.ctx, id)
		require.Error(t, err)
	})

	t.Run("manifest fallback skips nil routes", func(t *testing.T) {
		f := newV9Fixture(t)
		createV9Catalog(t, f.db, "pgm.ext", "1.0.0",
			`{"id":"pgm.ext","ui":{"pages":["/one",null,"/two"]}}`)
		id := createV9Installation(t, f.db, "pgm.ext", "1.0.0", `{}`)

		resp, err := f.svc.Pages(f.ctx, id)
		require.NoError(t, err)
		require.Len(t, resp.Pages, 2)
		assert.Equal(t, "/one", resp.Pages[0].Route)
		assert.Equal(t, "manifest", resp.Pages[0].Source)
	})
}

func TestUpgrade_Errors_V9(t *testing.T) {
	t.Run("installation missing", func(t *testing.T) {
		f := newV9Fixture(t)
		_, err := f.svc.Upgrade(f.ctx, 999, "2.0.0", "v9-op")
		require.Error(t, err)
	})

	t.Run("target equals current", func(t *testing.T) {
		f := newV9Fixture(t)
		createV9Catalog(t, f.db, "same.ext", "1.0.0", `{"id":"same.ext"}`)
		id := createV9Installation(t, f.db, "same.ext", "1.0.0", `{}`)
		_, err := f.svc.Upgrade(f.ctx, id, "1.0.0", "v9-op")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already on target")
	})

	t.Run("catalog lookup error", func(t *testing.T) {
		f := newV9Fixture(t)
		createV9Catalog(t, f.db, "cat.err.ext", "1.0.0", `{"id":"cat.err.ext"}`)
		id := createV9Installation(t, f.db, "cat.err.ext", "1.0.0", `{}`)
		require.NoError(t, f.db.Migrator().DropTable("extension_catalogs"))
		_, err := f.svc.Upgrade(f.ctx, id, "2.0.0", "v9-op")
		require.Error(t, err)
	})

	t.Run("config schema violation", func(t *testing.T) {
		f := newV9Fixture(t)
		createV9Catalog(t, f.db, "cfg.ext", "1.0.0", `{"id":"cfg.ext","version":"1.0.0"}`)
		// 2.0.0 要求 enabled 字段
		require.NoError(t, f.db.Create(&model.ExtensionRelease{
			ExtensionID:    "cfg.ext",
			Version:        "2.0.0",
			ReleaseChannel: "stable",
			ManifestJSON:   model.JSON([]byte(`{"id":"cfg.ext","version":"2.0.0","config_schema":{"type":"object","properties":{"enabled":{"type":"boolean"}},"required":["enabled"]}}`)),
		}).Error)
		id := createV9Installation(t, f.db, "cfg.ext", "1.0.0", `{}`)

		_, err := f.svc.Upgrade(f.ctx, id, "2.0.0", "v9-op")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing required field")
	})
}

func TestUninstall_Error_V9(t *testing.T) {
	f := newV9Fixture(t)
	createV9Catalog(t, f.db, "del.ext", "1.0.0", `{"id":"del.ext"}`)
	id := createV9Installation(t, f.db, "del.ext", "1.0.0", `{}`)
	require.NoError(t, f.db.Migrator().DropTable("extension_runtime_bindings"))

	_, err := f.svc.Uninstall(f.ctx, id, "v9-op")
	require.Error(t, err)
}

func TestEvents_Error_V9(t *testing.T) {
	f := newV9Fixture(t)
	createV9Catalog(t, f.db, "evt.ext", "1.0.0", `{"id":"evt.ext"}`)
	id := createV9Installation(t, f.db, "evt.ext", "1.0.0", `{}`)
	require.NoError(t, f.db.Migrator().DropTable("extension_events"))

	_, err := f.svc.Events(f.ctx, id, ExtensionEventListRequest{})
	require.Error(t, err)
}

func TestAgentSyncPayload_Error_V9(t *testing.T) {
	f := newV9Fixture(t)
	require.NoError(t, f.db.Migrator().DropTable("extension_installations"))
	_, err := f.svc.AgentSyncPayload(f.ctx, "agent-v9")
	require.Error(t, err)
}

// --- schema helpers ---

func TestResolveConfigSchema_CamelCase_V9(t *testing.T) {
	f := newV9Fixture(t)
	createV9Catalog(t, f.db, "camel.ext", "1.0.0",
		`{"id":"camel.ext","configSchema":{"type":"object"}}`)

	schema := f.svc.resolveConfigSchema(f.ctx, "camel.ext", "1.0.0")
	assert.Equal(t, map[string]any{"type": "object"}, schema)
}

func TestValidateConfigAgainstSchema_EdgeRules_V9(t *testing.T) {
	// required 中含空键 → 跳过
	err := validateConfigAgainstSchema(map[string]any{}, map[string]any{
		"properties": map[string]any{},
		"required":   []any{"", "host"},
	})
	require.Error(t, err)

	// properties 值非 map → 规则被跳过
	err = validateConfigAgainstSchema(map[string]any{"x": 1}, map[string]any{
		"properties": map[string]any{"x": "string"},
	})
	require.NoError(t, err)
}

// --- installation id resolution ---

func TestResolveInstallationID_Errors_V9(t *testing.T) {
	t.Run("extension id not found", func(t *testing.T) {
		f := newV9Fixture(t)
		_, err := f.svc.ResolveInstallationID(f.ctx, "ghost.ext")
		require.Error(t, err)
	})

	t.Run("lookup DB error", func(t *testing.T) {
		f := newV9Fixture(t)
		require.NoError(t, f.db.Migrator().DropTable("extension_installations"))
		_, err := f.svc.ResolveInstallationID(f.ctx, "any.ext")
		require.Error(t, err)
	})
}

// --- dependency validation ---

func TestValidateDependencies_Error_V9(t *testing.T) {
	f := newV9Fixture(t)
	require.NoError(t, f.db.Migrator().DropTable("extension_catalogs"))
	err := f.svc.validateDependencies(f.ctx, "any.ext", "1.0.0")
	require.Error(t, err)
}

func TestValidateDependencyNode_Branches_V9(t *testing.T) {
	t.Run("blank id is a no-op", func(t *testing.T) {
		f := newV9Fixture(t)
		err := f.svc.validateDependencyNode(f.ctx, extensionDependency{ExtensionID: "   "},
			map[string]bool{}, map[string]bool{})
		require.NoError(t, err)
	})

	t.Run("installed lookup DB error", func(t *testing.T) {
		f := newV9Fixture(t)
		require.NoError(t, f.db.Migrator().DropTable("extension_installations"))
		err := f.svc.validateDependencyNode(f.ctx, extensionDependency{ExtensionID: "dep.ext"},
			map[string]bool{}, map[string]bool{})
		require.Error(t, err)
	})

	t.Run("visited short-circuit and child manifest error", func(t *testing.T) {
		f := newV9Fixture(t)
		createV9Catalog(t, f.db, "dep.ext", "1.0.0", `{"id":"dep.ext","dependencies":["child.ext"]}`)
		createV9Installation(t, f.db, "dep.ext", "1.0.0", `{}`)

		// 子依赖的 manifest 读取失败（目录被删）
		require.NoError(t, f.db.Migrator().DropTable("extension_catalogs"))
		err := f.svc.validateDependencyNode(f.ctx, extensionDependency{ExtensionID: "dep.ext"},
			map[string]bool{}, map[string]bool{})
		require.Error(t, err)
	})
}

func TestFindActiveInstallationByExtension_Error_V9(t *testing.T) {
	f := newV9Fixture(t)
	require.NoError(t, f.db.Migrator().DropTable("extension_installations"))
	_, err := f.svc.findActiveInstallationByExtension(f.ctx, "any.ext")
	require.Error(t, err)
}

func TestActiveInstalledExtensionSet_V9(t *testing.T) {
	t.Run("DB error", func(t *testing.T) {
		f := newV9Fixture(t)
		require.NoError(t, f.db.Migrator().DropTable("extension_installations"))
		_, err := f.svc.activeInstalledExtensionSet(f.ctx)
		require.Error(t, err)
	})

	t.Run("pagination and inactive skip", func(t *testing.T) {
		f := newV9Fixture(t)
		rows := make([]*model.ExtensionInstallation, 0, 201)
		for i := 0; i < 201; i++ {
			rows = append(rows, &model.ExtensionInstallation{
				InstallationKey: extKeyV9("paged", i),
				ExtensionID:     "paged.ext",
				ReleaseVersion:  "1.0.0",
				Status:          "enabled",
				DesiredState:    "enabled",
			})
		}
		// 一条已卸载 → 应被跳过
		rows = append(rows, &model.ExtensionInstallation{
			InstallationKey: extKeyV9("paged", 999),
			ExtensionID:     "gone.ext",
			ReleaseVersion:  "1.0.0",
			Status:          "uninstalled",
			DesiredState:    "uninstalled",
		})
		require.NoError(t, f.db.CreateInBatches(rows, 100).Error)

		set, err := f.svc.activeInstalledExtensionSet(f.ctx)
		require.NoError(t, err)
		assert.True(t, set["paged.ext"])
		assert.NotContains(t, set, "gone.ext")
	})
}

func TestResolveCatalogMetadata_Error_V9(t *testing.T) {
	f := newV9Fixture(t)
	require.NoError(t, f.db.Migrator().DropTable("extension_catalogs"))
	def, tags := f.svc.resolveCatalogMetadata(f.ctx, "any.ext", "1.0.0")
	assert.False(t, def)
	assert.Empty(t, tags)
}

// --- binding extraction edge cases ---

func TestExtractCapabilityDetails_ProviderFunctionEdge_V9(t *testing.T) {
	t.Run("provider binding without provider info is skipped", func(t *testing.T) {
		bindings := []model.ExtensionRuntimeBinding{
			{BindingType: "provider", BindingKey: "", SpecJSON: nil},
		}
		caps, details := extractCapabilityDetailsFromBindings(bindings)
		// 复合键 "provider:" 仍会记录，但 provider 细节不生成
		assert.Equal(t, []string{"provider:"}, caps)
		assert.Empty(t, details)
	})

	t.Run("provider binding with spec operations", func(t *testing.T) {
		bindings := []model.ExtensionRuntimeBinding{
			{BindingType: "provider", BindingKey: "weather",
				SpecJSON: model.JSON([]byte(`{"provider":"weather","operations":["get_forecast"]}`))},
		}
		caps, details := extractCapabilityDetailsFromBindings(bindings)
		assert.Contains(t, caps, "external.weather")
		require.Len(t, details, 1)
		assert.Equal(t, "weather", details[0].Provider)
		assert.Equal(t, []string{"get_forecast"}, details[0].Operations)
	})

	t.Run("function binding invalid id is skipped", func(t *testing.T) {
		bindings := []model.ExtensionRuntimeBinding{
			{BindingType: "function", BindingKey: "not-external"},
		}
		_, details := extractCapabilityDetailsFromBindings(bindings)
		assert.Empty(t, details)
	})

	t.Run("function binding sanitized to empty is skipped", func(t *testing.T) {
		bindings := []model.ExtensionRuntimeBinding{
			{BindingType: "function", BindingKey: "external.外部.method"},
		}
		_, details := extractCapabilityDetailsFromBindings(bindings)
		assert.Empty(t, details)
	})

	t.Run("function binding creates detail then capability merges", func(t *testing.T) {
		bindings := []model.ExtensionRuntimeBinding{
			{BindingType: "function", BindingKey: "external.demo.query"},
			{BindingType: "function", BindingKey: "external.demo.export"},
			{BindingType: "capability", BindingKey: "external.demo", SpecJSON: model.JSON([]byte(
				`{"operations":["query"],"permissions":{"":["x"],"export":" ","query":"read"},"config_keys":["host","host"]}`,
			))},
		}
		caps, details := extractCapabilityDetailsFromBindings(bindings)
		assert.Contains(t, caps, "external.demo")
		require.Len(t, details, 1)
		// 两个 function 绑定的操作合并去重
		assert.ElementsMatch(t, []string{"query", "export"}, details[0].Operations)
		// permissions 空键/空值被过滤，仅保留 query=read
		assert.Equal(t, map[string]string{"query": "read"}, details[0].Permissions)
		// config_keys 去重
		assert.Equal(t, []string{"host"}, details[0].ConfigKeys)
	})

	t.Run("capability binding duplicate key merges operations", func(t *testing.T) {
		bindings := []model.ExtensionRuntimeBinding{
			{BindingType: "capability", BindingKey: "data", SpecJSON: model.JSON([]byte(`{"operations":["a"]}`))},
			{BindingType: "capability", BindingKey: "data", SpecJSON: model.JSON([]byte(`{"operations":["a","b"],"config_keys":["k1"]}`))},
		}
		_, details := extractCapabilityDetailsFromBindings(bindings)
		require.Len(t, details, 1)
		assert.Equal(t, []string{"a", "b"}, details[0].Operations)
		assert.Equal(t, []string{"k1"}, details[0].ConfigKeys)
	})
}

func TestPages_SortTieBreaker_V9(t *testing.T) {
	f := newV9Fixture(t)
	createV9Catalog(t, f.db, "sort.ext", "1.0.0", `{"id":"sort.ext"}`)
	id := createV9Installation(t, f.db, "sort.ext", "1.0.0", `{}`)
	for _, b := range []model.ExtensionRuntimeBinding{
		{InstallationID: id, BindingType: "page", BindingKey: "b-page", Status: "active",
			SpecJSON: model.JSON([]byte(`{"title":"B","order":2}`))},
		{InstallationID: id, BindingType: "page", BindingKey: "a-page", Status: "active",
			SpecJSON: model.JSON([]byte(`{"title":"A","order":2}`))},
		{InstallationID: id, BindingType: "page", BindingKey: "first", Status: "active",
			SpecJSON: model.JSON([]byte(`{"title":"First","order":1}`))},
	} {
		require.NoError(t, f.db.Create(&b).Error)
	}

	resp, err := f.svc.Pages(f.ctx, id)
	require.NoError(t, err)
	require.Len(t, resp.Pages, 3)
	// order 升序；order 相同时按 key 字典序（tie-breaker）
	assert.Equal(t, "first", resp.Pages[0].Key)
	assert.Equal(t, "a-page", resp.Pages[1].Key)
	assert.Equal(t, "b-page", resp.Pages[2].Key)
}

// --- version parsing edge case ---

func TestParseSemVersion_EmptyPart_V9(t *testing.T) {
	_, ok := parseSemVersion("1..3")
	assert.False(t, ok)
	_, ok = parseSemVersion("1.")
	assert.False(t, ok)
}
