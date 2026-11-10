package tasks

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/db/dbctx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// fakeScopes is an in-memory ScopeResolver standing in for the cmd/server
// routerTaskScopes adapter: each scope maps to its own game database and
// ScopeContext injects it via dbctx.WithDB, exactly like Router.Resolve.
type fakeScopes struct {
	scopes []TaskScope
	dbs    map[string]*gorm.DB
	// failOn, when non-empty, makes ScopeContext error for that scope key.
	failOn string
}

func (f *fakeScopes) OpenScopes() []TaskScope { return f.scopes }

func (f *fakeScopes) ScopeContext(ctx context.Context, gameID, env string) (context.Context, error) {
	if f.failOn == gameID+"/"+env {
		return nil, errors.New("scope down")
	}
	db, ok := f.dbs[gameID+"/"+env]
	if !ok {
		return nil, errors.New("no db for scope")
	}
	return dbctx.WithDB(ctx, db), nil
}

// setupMultiGame mirrors the production multi-game shape (CROP-REAP-001):
// the store's own DB is the meta database WITHOUT a task_runs table
// (model.GameModels only), while each scope resolver entry carries a game
// database that has one. File-backed DBs so cross-handle visibility holds
// beyond :memory:'s single-connection pool.
func setupMultiGame(t *testing.T) (*Store, *gorm.DB, map[string]*gorm.DB) {
	t.Helper()
	newDB := func(name string, migrateGame bool) *gorm.DB {
		t.Helper()
		db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), name+".db")), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		require.NoError(t, err)
		t.Cleanup(func() {
			if sqlDB, err := db.DB(); err == nil {
				_ = sqlDB.Close()
			}
		})
		if migrateGame {
			require.NoError(t, db.AutoMigrate(&model.TaskRun{}, &model.TaskEvent{}))
		}
		return db
	}
	// Meta: NO task_runs — sweeping it must fail with "no such table", which
	// is precisely the bug the fan-out fixes.
	meta := newDB("meta", false)
	gameA := newDB("game_a", true)
	gameB := newDB("game_b", true)
	dbs := map[string]*gorm.DB{"alpha/prod": gameA, "beta/prod": gameB}

	store := NewStore(model.NewTaskRunModel(meta), model.NewTaskEventModel(meta))
	store.SetScopeResolver(&fakeScopes{
		scopes: []TaskScope{{GameID: "alpha", Env: "prod"}, {GameID: "beta", Env: "prod"}},
		dbs:    dbs,
	})
	return store, meta, dbs
}

func scopeRun(taskID, agentID, status string) *model.TaskRun {
	return &model.TaskRun{
		TaskID:     taskID,
		FunctionID: "fn-1",
		GameID:     "alpha",
		Env:        "prod",
		AgentID:    agentID,
		Status:     status,
	}
}

// TestStore_ReapAgentNonTerminal_FanOutScopes reaps across both game
// databases and touches neither terminal rows nor other agents; the meta DB
// (no task_runs table) is never swept.
func TestStore_ReapAgentNonTerminal_FanOutScopes(t *testing.T) {
	store, _, dbs := setupMultiGame(t)
	ctx := context.Background()
	gameA, gameB := dbs["alpha/prod"], dbs["beta/prod"]

	require.NoError(t, gameA.Create(scopeRun("a-active", "agent-x", StatusRunning)).Error)
	require.NoError(t, gameA.Create(scopeRun("a-done", "agent-x", StatusSucceeded)).Error)
	require.NoError(t, gameA.Create(scopeRun("a-other", "agent-y", StatusRunning)).Error)
	require.NoError(t, gameB.Create(scopeRun("b-active", "agent-x", StatusQueued)).Error)

	n, err := store.ReapAgentNonTerminal(ctx, "agent-x", StatusUnknown, "agent session lost; execution state unknown")
	require.NoError(t, err)
	assert.Equal(t, int64(2), n, "one active row per game scope")

	for name, db := range map[string]*gorm.DB{"alpha": gameA, "beta": gameB} {
		var run model.TaskRun
		require.NoError(t, db.Where("task_id = ?", name[0:1]+"-active").First(&run).Error)
		assert.Equal(t, StatusUnknown, run.Status, "scope %s active row reaped", name)
		assert.Equal(t, "agent session lost; execution state unknown", run.Message)
		require.NotNil(t, run.FinishedAt)
	}

	var done model.TaskRun
	require.NoError(t, gameA.Where("task_id = ?", "a-done").First(&done).Error)
	assert.Equal(t, StatusSucceeded, done.Status, "terminal row untouched")

	var other model.TaskRun
	require.NoError(t, gameA.Where("task_id = ?", "a-other").First(&other).Error)
	assert.Equal(t, StatusRunning, other.Status, "other agent's row untouched")
}

// TestStore_ReapStale_FanOutScopes sweeps stale rows in every open scope;
// fresh rows stay untouched even when stale peers elsewhere are reaped.
func TestStore_ReapStale_FanOutScopes(t *testing.T) {
	store, _, dbs := setupMultiGame(t)
	ctx := context.Background()

	for name, db := range dbs {
		taskID := name[0:1] + "-stale"
		run := scopeRun(taskID, "agent-z", StatusRunning)
		require.NoError(t, db.Create(run).Error)
		// Backdate without touching updated_at automatically.
		require.NoError(t, db.Model(&model.TaskRun{}).Where("task_id = ?", taskID).
			UpdateColumn("updated_at", time.Now().Add(-time.Hour)).Error)
		fresh := scopeRun(name[0:1]+"-fresh", "agent-z", StatusRunning)
		require.NoError(t, db.Create(fresh).Error)
	}

	n, err := store.ReapStale(ctx, 30*time.Minute, StatusTimedOut, "invoke budget exceeded")
	require.NoError(t, err)
	assert.Equal(t, int64(2), n, "one stale row per scope")

	for name, db := range dbs {
		var reaped model.TaskRun
		require.NoError(t, db.Where("task_id = ?", name[0:1]+"-stale").First(&reaped).Error)
		assert.Equal(t, StatusTimedOut, reaped.Status, "scope %s stale row reaped", name)

		var fresh model.TaskRun
		require.NoError(t, db.Where("task_id = ?", name[0:1]+"-fresh").First(&fresh).Error)
		assert.Equal(t, StatusRunning, fresh.Status, "scope %s fresh row untouched", name)
	}
}

// TestStore_ReapFanOut_ScopeErrorAggregated keeps reaping healthy scopes when
// one scope fails, and reports the failure instead of dropping it silently.
func TestStore_ReapFanOut_ScopeErrorAggregated(t *testing.T) {
	store, _, dbs := setupMultiGame(t)
	resolver, ok := store.scopes.(*fakeScopes)
	require.True(t, ok)
	resolver.failOn = "beta/prod"
	ctx := context.Background()

	require.NoError(t, dbs["alpha/prod"].Create(scopeRun("a-active", "agent-x", StatusRunning)).Error)
	require.NoError(t, dbs["beta/prod"].Create(scopeRun("b-active", "agent-x", StatusRunning)).Error)

	n, err := store.ReapAgentNonTerminal(ctx, "agent-x", StatusUnknown, "agent session lost")
	require.Error(t, err, "failing scope must surface, not be silently dropped")
	assert.Contains(t, err.Error(), "reap scope beta/prod")
	assert.Equal(t, int64(1), n, "healthy scope still reaped")

	var run model.TaskRun
	require.NoError(t, dbs["alpha/prod"].Where("task_id = ?", "a-active").First(&run).Error)
	assert.Equal(t, StatusUnknown, run.Status)
	require.NoError(t, dbs["beta/prod"].Where("task_id = ?", "b-active").First(&run).Error)
	assert.Equal(t, StatusRunning, run.Status, "failed scope left as-is")
}

// TestStore_Reap_NilResolverSingleDB pins the single-database contract: no
// resolver means one sweep on the store's own DB, where the ScopeSingle
// migration baseline creates task_runs (dev/CI default, drill included).
func TestStore_Reap_NilResolverSingleDB(t *testing.T) {
	store := setupTaskStore(t)
	ctx := context.Background()

	require.NoError(t, store.CreateRun(ctx, newTaskRun("task-1")))
	require.NoError(t, store.UpdateRun(ctx, "task-1", map[string]interface{}{
		"status":   StatusRunning,
		"agent_id": "agent-x",
	}))

	n, err := store.ReapAgentNonTerminal(ctx, "agent-x", StatusUnknown, "agent session lost")
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	got, err := store.GetRun(ctx, "task-1")
	require.NoError(t, err)
	assert.Equal(t, StatusUnknown, got.Status)
}
