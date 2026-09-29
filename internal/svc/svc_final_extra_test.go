package svc

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/db/router"
	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// stubDriver fails on connect without any network I/O. It makes
// database/sql's "postgres" registration exist so createPostgresDatabase
// reaches its pool configuration and Exec error paths in tests.
type stubDriver struct{}

func (stubDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("connect refused by stub")
}

func init() {
	for _, existing := range sql.Drivers() {
		if existing == "postgres" {
			return
		}
	}
	sql.Register("postgres", stubDriver{})
}

func TestCreatePostgresDatabase_UsesRegisteredDriver(t *testing.T) {
	err := createPostgresDatabase("host=127.0.0.1 port=1 user=postgres password=x sslmode=disable", "newdb")
	assert.Error(t, err)
}

func TestOpenGorm_SQLiteDirFailure(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o644))

	_, err := openGorm("sqlite", filepath.Join(blocker, "sub", "db.sqlite"))
	assert.Error(t, err)
}

func TestEnsureGameDatabase_SQLiteDirFailure(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	require.NoError(t, os.WriteFile(sub, []byte("x"), 0o644))

	_, err := EnsureGameDatabase("sqlite", filepath.Join(sub, "meta.db"), "game_x")
	assert.Error(t, err)
}

func TestReplaceMySQLDSNDB_SchemePrefix(t *testing.T) {
	// With a query string the scheme prefix is dropped by design (documented
	// behaviour of the regex path); without one it is preserved.
	got := replaceMySQLDSNDB("mysql://user:pass@tcp(localhost:3306)/meta?param=1", "game_x")
	assert.Equal(t, "user:pass@tcp(localhost:3306)/game_x?param=1", got)

	got = replaceMySQLDSNDB("mysql://user:pass@tcp(localhost:3306)/meta", "game_x")
	assert.Equal(t, "mysql://user:pass@tcp(localhost:3306)/game_x", got)
}

func TestCloneOpsState_MarshalFailureReturnsOriginal(t *testing.T) {
	state := defaultOpsState()
	state.Audit.Entries = append(state.Audit.Entries, OpsAuditEntry{
		ID:        "a1",
		Action:    "test",
		CreatedAt: time.Now(),
		Metadata:  map[string]interface{}{"chan": make(chan int)},
	})

	cloned := cloneOpsState(state)
	assert.Equal(t, "redis", cloned.MQ.Type)
	require.Len(t, cloned.Audit.Entries, 1)
}

func TestNewServiceContext_BrokenBootstrapFilesStillBoots(t *testing.T) {
	cfg := newSvcConfig(t, false)
	// Corrupt admins.json forces AdminManager.Initialize down its error path;
	// corrupt games.json pushes the game seeder onto the default config.
	badJSON := "{not json"
	adminsPath := filepath.Join(cfg.BootstrapData.BaseDir, "admins.json")
	require.NoError(t, os.MkdirAll(cfg.BootstrapData.BaseDir, 0o755))
	require.NoError(t, os.WriteFile(adminsPath, []byte(badJSON), 0o644))
	gamesPath := filepath.Join(cfg.BootstrapData.BaseDir, "games.json")
	require.NoError(t, os.WriteFile(gamesPath, []byte(badJSON), 0o644))

	ctx := NewServiceContext(cfg)
	require.NotNil(t, ctx)
	games, err := ctx.GameModel.ListAll(context.Background())
	require.NoError(t, err)
	assert.NotEmpty(t, games)
}

func TestNewServiceContext_PanicsOnBrokenStorage(t *testing.T) {
	cfg := newSvcConfig(t, false)
	cfg.Storage = config.StorageConfig{Driver: "s3"}
	assert.Panics(t, func() { NewServiceContext(cfg) })
}

func TestNewServiceContext_OptionsAndDispatchSettings(t *testing.T) {
	cfg := newSvcConfig(t, false)

	// Task routing dir beneath a regular file breaks the file store init but
	// must not prevent boot.
	blocker := filepath.Join(filepath.Dir(cfg.Database.DataSource), "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o644))
	cfg.AgentDispatch.TaskRoutingDir = filepath.Join(blocker, "routing")
	cfg.AgentDispatch.TaskRoutingTTL = "not-a-duration"

	store := reg.NewStore()
	dispatcher := reg.NewStore()
	_ = dispatcher
	ctx := NewServiceContext(cfg, WithRegistryStore(store), WithDispatcher(nil))
	require.NotNil(t, ctx)
	assert.Same(t, store, ctx.RegistryStore)
	require.NotNil(t, ctx.Dispatcher)
}

func TestSeedBootstrapRoleAndAdminErrorBranches(t *testing.T) {
	svcCtx := setupTestServiceContext(t)

	// Bootstrap role referencing unknown permission ids -> validation error.
	// Bootstrap admin referencing an unknown role -> lookup error.
	managerDir := t.TempDir()
	manager := NewAdminManager(managerDir)
	require.NoError(t, os.WriteFile(filepath.Join(managerDir, "roles.json"), []byte(
		`[{"code":"ops","name":"Ops","description":"Operators","level":2,"permissions":["missing.permission"]}]`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(managerDir, "admins.json"), []byte(
		`[{"username":"limited","password":"secret123","roles":["ghost-role"],"status":1}]`), 0o644))
	require.NoError(t, manager.Initialize())

	svcCtx.AdminManager = manager
	require.NoError(t, seedBootstrapRoles(svcCtx))
	require.NoError(t, seedBootstrapAdmins(svcCtx))
}

func TestSeedBootstrapExtensionCatalog_HappyAndUpdates(t *testing.T) {
	svcCtx := setupTestServiceContext(t)
	baseDir := t.TempDir()
	svcCtx.Config.BootstrapData.BaseDir = baseDir

	catalogDir := filepath.Join(baseDir, "extensions")
	require.NoError(t, os.MkdirAll(catalogDir, 0o755))
	catalogPath := filepath.Join(catalogDir, "catalog.json")

	payload := `{"items":[{"extensionId":"demo-ext","displayName":"Demo Ext","releases":[{"version":"1.0.0","releaseChannel":"","packageRef":"packs/demo.tgz","checksum":"abc","publishedAt":"2026-01-02T03:04:05Z","manifest":{"id":"demo-ext"}}]}]}`
	require.NoError(t, os.WriteFile(catalogPath, []byte(payload), 0o644))
	require.NoError(t, seedBootstrapExtensionCatalog(svcCtx))

	// Second run exercises the update-existing branches.
	require.NoError(t, os.WriteFile(catalogPath, []byte(payload), 0o644))
	require.NoError(t, seedBootstrapExtensionCatalog(svcCtx))

	var releases []model.ExtensionRelease
	require.NoError(t, svcCtx.DB.Find(&releases).Error)
	require.Len(t, releases, 1)
	assert.Equal(t, "stable", releases[0].ReleaseChannel)
	assert.NotZero(t, releases[0].PublishedAtUnix)
}

func TestGameDBMiddleware_RouterFailureReturns400(t *testing.T) {
	svcCtx := setupTestServiceContext(t)
	ctx := context.Background()

	require.NoError(t, svcCtx.GameModel.Create(ctx, &model.Game{Name: "Demo", GameID: "demo", AliasName: "demo"}))
	require.NoError(t, svcCtx.GameModel.AddEnvBinding(ctx, "demo", "prod", "db_demo_prod", "", ""))

	// A router whose naming function yields an empty database name always
	// fails, which drives the middleware's game_database_unavailable branch.
	svcCtx.Router = router.New(router.Config{
		Driver:      "sqlite",
		NameForGame: func(string, string) string { return "" },
		Open:        func(string, string) (*gorm.DB, error) { return nil, nil },
	}, svcCtx.DB)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/anything", nil)
	c.Request.Header.Set(GameDBHeader, "demo")
	c.Request.Header.Set(EnvHeader, "prod")

	GameDBMiddleware(svcCtx)(c)
	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestOfficialExtensionSeedFollowsUnifiedPattern（#46 批次 3）：仓库 seed 文件里
// official.* 四扩展必须符合 official-extension-unified-pattern.md 统一模式——
// ① 三层权限键 <domain>.read/operate/admin；② 每个版本 manifest 声明页面
// （required_permission 必填）；③ configSchema 每个属性必须带 type/description。
func TestOfficialExtensionSeedFollowsUnifiedPattern(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "configs", "extensions", "catalog.json"))
	if err != nil {
		t.Skipf("seed file not available: %v", err)
	}
	var payload struct {
		Items []struct {
			ExtensionID string `json:"extensionId"`
			Releases    []struct {
				Version  string         `json:"version"`
				Manifest map[string]any `json:"manifest"`
			} `json:"releases"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("seed json invalid: %v", err)
	}
	checked := 0
	for _, item := range payload.Items {
		domain, ok := strings.CutPrefix(item.ExtensionID, "official.")
		// 统一模式只约束四个业务扩展（见 official-extension-unified-pattern.md §1）；
		// official.external-platform 是先于模式的连接器条目，不回溯改造。
		if !ok || !map[string]bool{"notification": true, "alerting": true, "approval": true, "backup-advanced": true}[domain] {
			continue
		}
		checked++
		for _, release := range item.Releases {
			manifest := release.Manifest
			if manifest == nil {
				t.Fatalf("%s@%s: manifest missing", item.ExtensionID, release.Version)
			}
			perms, _ := manifest["permissions"].(map[string]any)
			for _, tier := range []string{"read", "operate", "admin"} {
				key, _ := perms[tier].(string)
				if key != domain+"."+tier {
					t.Fatalf("%s@%s: permission tier %q = %q, want %q", item.ExtensionID, release.Version, tier, key, domain+"."+tier)
				}
			}
			pages, _ := manifest["pages"].([]any)
			if len(pages) == 0 {
				t.Fatalf("%s@%s: pages declaration missing", item.ExtensionID, release.Version)
			}
			for _, rawPage := range pages {
				page, _ := rawPage.(map[string]any)
				if page["requiredPermission"] == "" {
					t.Fatalf("%s@%s: page %v missing requiredPermission", item.ExtensionID, release.Version, page["key"])
				}
			}
			schema, _ := manifest["configSchema"].(map[string]any)
			props, _ := schema["properties"].(map[string]any)
			for name, rawProp := range props {
				prop, _ := rawProp.(map[string]any)
				if prop["type"] == "" || prop["description"] == "" {
					t.Fatalf("%s@%s: config prop %q must declare type and description", item.ExtensionID, release.Version, name)
				}
			}
		}
	}
	if checked < 4 {
		t.Fatalf("expected at least 4 official.* seed items, got: %d", checked)
	}
}
