package svc

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/db/migrate"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fanoutConfig(t *testing.T, multiGame bool) config.Config {
	t.Helper()
	dir := t.TempDir()
	// Pin the env overrides so CI's DATABASE_URL=":memory:" cannot hijack
	// resolveDriverAndDSN and make every test share one memory database.
	t.Setenv("DB_DRIVER", "sqlite")
	t.Setenv("DATABASE_URL", filepath.Join(dir, "meta.db"))
	return config.Config{
		Server: config.ServerConfig{Mode: "dev"},
		Database: config.DatabaseConfig{
			Driver:     "sqlite",
			DataSource: filepath.Join(dir, "meta.db"),
			MultiGame:  multiGame,
		},
	}
}

func seedEnvBinding(t *testing.T, cfg config.Config, gameID, env string) {
	t.Helper()
	driver, metaDSN := resolveDriverAndDSN(cfg)
	metaDB, err := OpenGormForRouter(driver, metaDSN)
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrateMeta(metaDB))
	require.NoError(t, model.NewGameModel(metaDB).AddEnvBinding(context.Background(),
		gameID, env, gameDBNameFor(cfg, gameID, env), "", ""))
}

func TestRunMigrationFanout_SingleGame(t *testing.T) {
	cfg := fanoutConfig(t, false)

	reports, err := RunMigrationFanout(context.Background(), cfg, false)
	require.NoError(t, err)
	require.Len(t, reports, 1)
	assert.Equal(t, FanoutStatusMigrated, reports[0].Status)
	assert.Equal(t, migrate.MinimumRequiredVersion, reports[0].After)

	// Second pass is a no-op.
	reports, err = RunMigrationFanout(context.Background(), cfg, false)
	require.NoError(t, err)
	require.Len(t, reports, 1)
	assert.Equal(t, FanoutStatusCurrent, reports[0].Status)
}

func TestRunMigrationFanout_MultiGameRollsEveryDatabase(t *testing.T) {
	cfg := fanoutConfig(t, true)
	seedEnvBinding(t, cfg, "demo", "prod")
	seedEnvBinding(t, cfg, "demo", "development")

	reports, err := RunMigrationFanout(context.Background(), cfg, false)
	require.NoError(t, err)
	// meta + two game databases
	require.Len(t, reports, 3)
	assert.Equal(t, FanoutStatusMigrated, reports[0].Status)
	dbs := map[string]bool{}
	for _, r := range reports[1:] {
		assert.Equal(t, FanoutStatusMigrated, r.Status, "game %s/%s", r.GameID, r.Env)
		assert.Equal(t, migrate.MinimumRequiredVersion, r.After)
		dbs[r.Database] = true
	}
	assert.Equal(t, map[string]bool{"game_demo_prod": true, "game_demo_development": true}, dbs)

	// Dry-run afterwards reports current versions without DDL.
	dry, err := RunMigrationFanout(context.Background(), cfg, true)
	require.NoError(t, err)
	require.Len(t, dry, 3)
	for _, r := range dry {
		assert.Equal(t, FanoutStatusCurrent, r.Status)
		assert.Equal(t, migrate.MinimumRequiredVersion, r.After)
	}

	out := FormatFanoutReports(dry)
	assert.Contains(t, out, "game_demo_prod")
	assert.Contains(t, out, "total=3")
}
