package svc

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestOpenReadOnlyGorm_MySQLBadDSN(t *testing.T) {
	db, err := openReadOnlyGorm("mysql", "totally invalid :: dsn")
	if db != nil {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			defer sqlDB.Close()
		}
	}
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid DSN")
}

func TestOpenReadOnlyGorm_SQLServerUnreachable(t *testing.T) {
	db, err := openReadOnlyGorm("sqlserver", "sqlserver://sa:pass@127.0.0.1:1433?database=none")
	if db != nil {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			defer sqlDB.Close()
		}
	}
	assert.Error(t, err)
}

func TestExecutionLogsMigrationClosedDB(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(filepath.Join(t.TempDir(), "migrate20.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	m := executionLogsTableMigration()
	require.NotNil(t, m.UpFnNoTxContext)
	err = m.UpFnNoTxContext(context.Background(), sqlDB)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "probe dialect")
}

func TestBuildGameFromSeed_UnderscoreOnlyGameID(t *testing.T) {
	game, err := buildGameFromSeed(bootstrapGameSeedEntry{GameID: "___"}, nil, 0)
	require.NoError(t, err)
	assert.Equal(t, "___", game.GameID)
	assert.Equal(t, "", game.AliasName)
}

func TestSeedBootstrapExtensionCatalog_UpdateFailsOnReadOnlyDB(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "ext.db")

	db, err := gorm.Open(gsqlite.Open(dbPath), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.Migrator().CreateTable(&model.ExtensionCatalog{}))
	require.NoError(t, db.Create(&model.ExtensionCatalog{ExtensionID: "demo-ext", Name: "demo", DisplayName: "demo", Status: "active"}).Error)
	sqlDB, _ := db.DB()
	require.NoError(t, sqlDB.Close())

	ro, err := gorm.Open(gsqlite.Open("file:"+dbPath+"?mode=ro"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)

	seedDir := filepath.Join(dir, "bootstrap", "extensions")
	require.NoError(t, os.MkdirAll(seedDir, 0o755))
	catalog := map[string]any{
		"items": []map[string]any{{
			"extensionId": "demo-ext",
			"name":        "demo",
			"releases":    []map[string]any{{"version": "1.0.0"}},
		}},
	}
	blob, err := json.Marshal(catalog)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(seedDir, "catalog.json"), blob, 0o644))

	ctx := &ServiceContext{DB: ro, Config: config.Config{BootstrapData: config.BootstrapDataConfig{BaseDir: filepath.Join(dir, "bootstrap")}}}
	err = seedBootstrapExtensionCatalog(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update extension catalog seed failed")
}

func TestSeedBootstrapExtensionCatalog_ReleaseQueryFailsWithoutTable(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "ext.db")

	db, err := gorm.Open(gsqlite.Open(dbPath), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.Migrator().CreateTable(&model.ExtensionCatalog{}))
	require.NoError(t, db.Create(&model.ExtensionCatalog{ExtensionID: "demo-ext", Name: "demo", DisplayName: "demo", Status: "active"}).Error)

	seedDir := filepath.Join(dir, "bootstrap", "extensions")
	require.NoError(t, os.MkdirAll(seedDir, 0o755))
	catalog := map[string]any{
		"items": []map[string]any{{
			"extensionId": "demo-ext",
			"name":        "demo",
			"releases":    []map[string]any{{"version": "1.0.0"}},
		}},
	}
	blob, err := json.Marshal(catalog)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(seedDir, "catalog.json"), blob, 0o644))

	ctx := &ServiceContext{DB: db, Config: config.Config{BootstrapData: config.BootstrapDataConfig{BaseDir: filepath.Join(dir, "bootstrap")}}}
	err = seedBootstrapExtensionCatalog(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "query extension release seed failed")
}

func TestNewServiceContext_TaskRoutingCleanupFailureLogged(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("requires non-root to enforce directory permissions")
	}
	routingDir := t.TempDir()
	stale := map[string]any{
		"t1": map[string]any{
			"taskId":    "t1",
			"agentId":   "a1",
			"createdAt": "2020-01-01T00:00:00Z",
			"updatedAt": "2020-01-01T00:00:00Z",
		},
	}
	blob, err := json.Marshal(stale)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(routingDir, "task_routing.json"), blob, 0o444))
	require.NoError(t, os.Chmod(routingDir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(routingDir, 0o755) })

	cfg := newSvcConfig(t, false)
	cfg.AgentDispatch.TaskRoutingDir = routingDir
	cfg.AgentDispatch.TaskRoutingTTL = "1h"
	ctx := NewServiceContext(cfg)
	require.NotNil(t, ctx)
	require.NotNil(t, ctx.Dispatcher)
	_ = time.Now()
}
