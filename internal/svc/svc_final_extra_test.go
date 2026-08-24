package svc

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
