package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/db/router"
	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/require"
	gsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// 单库模式（Router nil）：清扫直达 svcCtx.DB，宽限过期的 pending 契约真删。
func TestDashboardRegistrationPipelineSweepsExpiredRemovalsSingleDB(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	require.NoError(t, reg.MigrateAgentSessions(db))

	store := reg.NewStoreWithDB(db)
	svcCtx := &svc.ServiceContext{DB: db, RegistryStore: store}
	wireDashboardRegistrationPipeline(svcCtx)
	pipeline := &registrationContractPipeline{svcCtx: svcCtx}
	ctx := context.Background()

	session := &reg.AgentSession{
		AgentID:  "agent-sweep",
		GameID:   "demo-game",
		Env:      "development",
		ExpireAt: time.Now().Add(time.Minute),
		LastSeen: time.Now(),
		Functions: map[string]reg.FunctionMeta{
			"mail.send": {Enabled: true, Version: "1.0.0"},
		},
	}
	require.NoError(t, store.UpsertAgent(session))
	empty := *session
	empty.Functions = map[string]reg.FunctionMeta{}
	require.NoError(t, store.UpsertAgent(&empty))

	// 宽限未到期：不删。
	finalized, err := pipeline.FinalizeExpiredContractRemovals(ctx, 10*time.Minute)
	require.NoError(t, err)
	require.Zero(t, finalized)

	// 宽限过期：真删。
	finalized, err = pipeline.FinalizeExpiredContractRemovals(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, 1, finalized)
	_, err = model.NewFunctionContractModel(db).
		FindByScopeAndFunctionID(ctx, "demo-game", "development", "mail.send")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// 分库模式（Router 非 nil）：pending 行散布在各 game 库，清扫按
// game_envs 绑定逐 scope 收口，计数跨 scope 聚合。
func TestDashboardRegistrationPipelineSweepsExpiredRemovalsPerGameDB(t *testing.T) {
	dir := t.TempDir()
	metaDB, err := gorm.Open(gsqlite.Open(filepath.Join(dir, "meta.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, metaDB.AutoMigrate(&model.Game{}, &model.GameEnvBinding{}))

	nameFor := func(gameID, env string) string { return "game_" + gameID + "_" + env }
	dsnFor := func(dbName string) string { return filepath.Join(dir, dbName+".db") }
	seen := map[string]*gorm.DB{}
	open := func(driver, dsn string) (*gorm.DB, error) {
		db, err := gorm.Open(gsqlite.Open(dsn), &gorm.Config{})
		if err == nil {
			// 生产路径 game 库首用即全量迁移；清扫链会触达契约版本历史、
			// 独立提案与阻塞问题等 game 库表，播种必须给全（单库用例同）。
			require.NoError(t, model.AutoMigrate(db))
			seen[dsn] = db
		}
		return db, err
	}
	scopes := []struct{ gameID, env string }{
		{"demo-game", "development"},
		{"other-game", "production"},
	}
	for _, scope := range scopes {
		_, err := open("sqlite", dsnFor(nameFor(scope.gameID, scope.env)))
		require.NoError(t, err)
		require.NoError(t, metaDB.Create(&model.GameEnvBinding{
			GameID: scope.gameID, Env: scope.env, DatabaseName: nameFor(scope.gameID, scope.env),
		}).Error)
	}

	rt := router.New(router.Config{
		Driver:         "sqlite",
		MetaDSN:        filepath.Join(dir, "meta.db"),
		NameForGame:    nameFor,
		DSNForDatabase: func(_, dbName string) string { return dsnFor(dbName) },
		EnsureDatabase: func(_, _ string, dbName string) (string, error) {
			return dsnFor(dbName), nil
		},
		Open: open,
	}, metaDB)

	svcCtx := &svc.ServiceContext{
		DB:        metaDB,
		Router:    rt,
		GameModel: model.NewGameModel(metaDB),
	}
	pipeline := &registrationContractPipeline{svcCtx: svcCtx}
	ctx := context.Background()

	// 每个 game 库各埋一条宽限已到期的 pending 契约。
	pending := time.Now().Add(-time.Hour)
	for _, scope := range scopes {
		db, err := rt.GameDB(ctx, scope.gameID, scope.env)
		require.NoError(t, err)
		require.NoError(t, db.Create(&model.FunctionContract{
			GameID: scope.gameID, Env: scope.env,
			FunctionID: "mail.send", Version: "1.0.0", Enabled: true,
			RemovalPendingAt: &pending,
		}).Error)
	}

	finalized, err := pipeline.FinalizeExpiredContractRemovals(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, 2, finalized)

	for _, scope := range scopes {
		db, err := rt.GameDB(ctx, scope.gameID, scope.env)
		require.NoError(t, err)
		_, err = model.NewFunctionContractModel(db).
			FindByScopeAndFunctionID(ctx, scope.gameID, scope.env, "mail.send")
		require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	}
}
