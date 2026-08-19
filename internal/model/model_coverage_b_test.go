package model

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ===== RegistrationWarningModel =====

func TestRegistrationWarningModel_CRUD(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&RegistrationWarningDB{}))
	m := NewRegistrationWarningModel(db)
	ctx := context.Background()

	assert.Equal(t, "registration_warnings", RegistrationWarningDB{}.TableName())

	now := time.Now()
	warn := &registry.FunctionRegistrationWarning{
		Key:    "demo/prod/agent-1/fn.deploy/schema_missing",
		GameID: "demo", Env: "prod", AgentID: "agent-1",
		FunctionID: "fn.deploy", Version: "v2", Code: "schema_missing",
		Message: "input schema missing", Count: 1,
		FirstSeen: now, LastSeen: now,
	}
	require.NoError(t, m.Upsert(ctx, warn))

	// Upsert again bumps count/last_seen via the conflict path.
	warn2 := *warn
	warn2.Count = 5
	warn2.Version = "v3"
	require.NoError(t, m.Upsert(ctx, &warn2))

	got, err := m.List(ctx, WarningFilter{})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, 5, got[0].Count)
	assert.Equal(t, "v3", got[0].Version)
	assert.Equal(t, "fn.deploy", got[0].FunctionID)

	// Filters narrow results.
	got, err = m.List(ctx, WarningFilter{GameID: "demo", Env: "prod", AgentID: "agent-1", FunctionID: "fn.deploy", Code: "schema_missing", Status: "pending"})
	require.NoError(t, err)
	assert.Len(t, got, 1)

	got, err = m.List(ctx, WarningFilter{GameID: "other"})
	require.NoError(t, err)
	assert.Empty(t, got)

	// Limit=1 returns at most one row.
	require.NoError(t, m.Upsert(ctx, &registry.FunctionRegistrationWarning{
		Key: "demo/prod/agent-1/other.fn/deprecated", GameID: "demo", Env: "prod",
		AgentID: "agent-1", FunctionID: "other.fn", Code: "deprecated",
		Message: "deprecated", FirstSeen: now, LastSeen: now,
	}))
	got, err = m.List(ctx, WarningFilter{Limit: 1})
	require.NoError(t, err)
	assert.Len(t, got, 1)

	// Status update sets resolved_at only for resolved.
	require.NoError(t, m.UpdateStatus(ctx, warn.Key, "read"))
	row, err := m.List(ctx, WarningFilter{Code: "schema_missing"})
	require.NoError(t, err)
	require.Len(t, row, 1)

	require.NoError(t, m.UpdateStatus(ctx, warn.Key, "resolved"))

	counts, err := m.CountByStatus(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), counts["resolved"])
	assert.Equal(t, int64(1), counts["pending"])

	deleted, err := m.DeleteResolved(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)

	cleared, err := m.ClearByAgent(ctx, "agent-1")
	require.NoError(t, err)
	assert.Equal(t, int64(1), cleared)

	left, err := m.List(ctx, WarningFilter{})
	require.NoError(t, err)
	assert.Empty(t, left)
}

// ===== ConfigVersionModel scoped methods =====

func TestConfigVersionModel_ScopedMethods(t *testing.T) {
	db := setupTestDB(t)
	m := NewConfigVersionModel(db)
	ctx := context.Background()

	created, err := m.CreateWithMeta(ctx, ConfigVersionPayload{
		Key: "feature.flags", Content: "{}", GameID: "demo", Env: "prod", Message: "init",
	}, "alice")
	require.NoError(t, err)
	assert.Equal(t, 1, created.Version)
	_, err = m.CreateWithMeta(ctx, ConfigVersionPayload{
		Key: "feature.flags", Content: `{"x":1}`, GameID: "demo", Env: "prod",
	}, "bob")
	require.NoError(t, err)

	// ListByScope: guards and results.
	empty, err := m.ListByScope(ctx, "", "demo", "prod")
	require.NoError(t, err)
	assert.Empty(t, empty)

	records, err := m.ListByScope(ctx, " feature.flags ", "demo", "prod")
	require.NoError(t, err)
	require.Len(t, records, 2)
	assert.Equal(t, 2, records[0].Version, "newest first")

	// FindByScope: guards, hit, and miss.
	_, err = m.FindByScope(ctx, "", 1, "demo", "prod")
	assert.ErrorIs(t, err, ErrNotFound)
	_, err = m.FindByScope(ctx, "feature.flags", 0, "demo", "prod")
	assert.ErrorIs(t, err, ErrNotFound)

	rec, err := m.FindByScope(ctx, "feature.flags", 2, "demo", "prod")
	require.NoError(t, err)
	assert.Equal(t, `{"x":1}`, rec.Value)

	_, err = m.FindByScope(ctx, "feature.flags", 2, "demo", "dev")
	assert.ErrorIs(t, err, ErrNotFound)

	// FindLatestByScope: guards, hit, and miss.
	_, err = m.FindLatestByScope(ctx, "", "demo", "prod")
	assert.ErrorIs(t, err, ErrNotFound)

	latest, err := m.FindLatestByScope(ctx, "feature.flags", "demo", "prod")
	require.NoError(t, err)
	assert.Equal(t, 2, latest.Version)

	_, err = m.FindLatestByScope(ctx, "feature.flags", "demo", "dev")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestConfigVersionModel_CreateWithMeta_OptimisticLock(t *testing.T) {
	db := setupTestDB(t)
	m := NewConfigVersionModel(db)
	ctx := context.Background()

	_, err := m.CreateWithMeta(ctx, ConfigVersionPayload{Content: "x"}, "alice")
	require.Error(t, err, "empty key rejected")

	// Base version mismatch against empty history.
	_, err = m.CreateWithMeta(ctx, ConfigVersionPayload{Key: "k", Content: "x", BaseVersion: 3}, "alice")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "base version mismatch")

	_, err = m.CreateWithMeta(ctx, ConfigVersionPayload{Key: "k", Content: "x"}, "alice")
	require.NoError(t, err)

	// Stale base version conflicts with current latest.
	_, err = m.CreateWithMeta(ctx, ConfigVersionPayload{Key: "k", Content: "y", BaseVersion: 99}, "bob")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "updated by another user")

	// Correct base version succeeds.
	rec, err := m.CreateWithMeta(ctx, ConfigVersionPayload{Key: "k", Content: "y", BaseVersion: 1}, "bob")
	require.NoError(t, err)
	assert.Equal(t, 2, rec.Version)
}

// ===== GameModel env bindings =====

func TestGameModel_UpdateEnvsAndBindings(t *testing.T) {
	db := setupTestDB(t)
	m := NewGameModel(db)
	ctx := context.Background()

	game := &Game{GameID: "envgame", Name: "Env Game", AliasName: "envgame-alias"}
	require.NoError(t, m.Create(ctx, game))
	require.NoError(t, game.SetEnvs([]GameEnv{{Env: "prod", Description: "Production"}, {Env: "dev"}}))
	require.NoError(t, db.Save(game).Error)

	err := m.UpdateEnvsAndBindings(ctx, game.GameID, game.ID, game.Envs,
		[]string{"dev"},
		[]GameEnvBinding{
			{Env: "prod", DatabaseName: "game_envgame_prod", Description: "Production"},
			{Env: "staging", DatabaseName: "game_envgame_staging"},
		})
	require.NoError(t, err)

	bindings, err := m.ListEnvBindings(ctx, game.GameID)
	require.NoError(t, err)
	require.Len(t, bindings, 2)

	binding, err := m.FindEnvBinding(ctx, game.GameID, "prod")
	require.NoError(t, err)
	require.NotNil(t, binding)
	assert.Equal(t, "game_envgame_prod", binding.DatabaseName)

	// Not-found binding returns nil without error.
	missing, err := m.FindEnvBinding(ctx, game.GameID, "dev")
	require.NoError(t, err)
	assert.Nil(t, missing)

	// LookupDatabaseName resolves and misses.
	dbName, err := m.LookupDatabaseName(ctx, game.GameID, "staging")
	require.NoError(t, err)
	assert.Equal(t, "game_envgame_staging", dbName)
	dbName, err = m.LookupDatabaseName(ctx, game.GameID, "ghost")
	require.NoError(t, err)
	assert.Empty(t, dbName)

	// Binding without database name is rejected.
	err = m.UpdateEnvsAndBindings(ctx, game.GameID, game.ID, game.Envs, nil,
		[]GameEnvBinding{{Env: "bad", DatabaseName: " "}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires env and database name")
}

func TestGameModel_DeleteWithEnvBindings(t *testing.T) {
	db := setupTestDB(t)
	m := NewGameModel(db)
	ctx := context.Background()

	game := &Game{GameID: "delgame", Name: "Delete Me", AliasName: "delgame-alias"}
	require.NoError(t, m.Create(ctx, game))
	require.NoError(t, m.UpdateEnvsAndBindings(ctx, game.GameID, game.ID, nil, nil,
		[]GameEnvBinding{
			{Env: "prod", DatabaseName: "game_delgame_prod"},
			{Env: "dev", DatabaseName: "game_delgame_dev"},
		}))

	require.NoError(t, m.DeleteWithEnvBindings(ctx, game.ID, game.GameID))

	_, err := m.FindOne(ctx, game.ID)
	assert.Error(t, err)

	bindings, err := m.ListEnvBindings(ctx, game.GameID)
	require.NoError(t, err)
	assert.Empty(t, bindings, "routing bindings must be hard-deleted")
}

// setupIsolatedDB opens a per-test in-memory DB so tests that scan whole
// tables (e.g. BackfillEnvBindings) never see rows from other tests sharing
// the default cache=shared database.
func setupIsolatedDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:isolated_%s_%d.db?mode=memory&cache=shared", t.Name(), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Game{}, &GameEnvBinding{}))
	return db
}

func TestGameModel_BackfillEnvBindings(t *testing.T) {
	db := setupIsolatedDB(t)
	m := NewGameModel(db)
	ctx := context.Background()

	_, err := m.BackfillEnvBindings(ctx, nil)
	require.Error(t, err)

	game := &Game{GameID: "fillgame", Name: "Fill", AliasName: "fillgame-alias"}
	require.NoError(t, m.Create(ctx, game))
	require.NoError(t, game.SetEnvs([]GameEnv{{Env: "prod", Description: "prod"}, {Env: ""}}))
	require.NoError(t, db.Save(game).Error)

	created, err := m.BackfillEnvBindings(ctx, func(gameID, env string) string {
		return "game_" + gameID + "_" + env
	})
	require.NoError(t, err)
	assert.Equal(t, 1, created)

	binding, err := m.FindEnvBinding(ctx, game.GameID, "prod")
	require.NoError(t, err)
	require.NotNil(t, binding)
	assert.Equal(t, "game_fillgame_prod", binding.DatabaseName)

	// Idempotent: no duplicates on rerun.
	created, err = m.BackfillEnvBindings(ctx, func(gameID, env string) string {
		return "game_" + gameID + "_" + env
	})
	require.NoError(t, err)
	assert.Equal(t, 0, created)

	// Empty database name from resolver aborts.
	game2 := &Game{GameID: "emptydb", Name: "Empty", AliasName: "emptydb-alias"}
	require.NoError(t, m.Create(ctx, game2))
	require.NoError(t, game2.SetEnvs([]GameEnv{{Env: "prod"}}))
	require.NoError(t, db.Save(game2).Error)
	_, err = m.BackfillEnvBindings(ctx, func(string, string) string { return " " })
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty database name")
}

// ===== OpenAPISourceBindingModel =====

func TestOpenAPISourceBindingModel_Methods(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&OpenAPISourceBinding{}))
	m := NewOpenAPISourceBindingModel(db)
	ctx := context.Background()

	b := &OpenAPISourceBinding{
		GameID: "demo", Env: "prod", SourceID: "src-1",
		BindingID: "op.list", OperationID: "op.list", Kind: "provider", FunctionID: "fn.list",
	}
	require.NoError(t, m.Upsert(ctx, b))
	assert.NotZero(t, b.ID)

	// Upsert again updates the existing row.
	b2 := &OpenAPISourceBinding{
		GameID: "demo", Env: "prod", SourceID: "src-1",
		BindingID: "op.list", OperationID: "op.list", Kind: "provider",
		FunctionID: "fn.list-v2",
	}
	require.NoError(t, m.Upsert(ctx, b2))
	assert.Equal(t, b.ID, b2.ID)

	items, err := m.ListBySource(ctx, "demo", "prod", "src-1")
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "fn.list-v2", items[0].FunctionID)

	found, err := m.FindByScopeSourceAndBindingID(ctx, "demo", "prod", "src-1", "op.list")
	require.NoError(t, err)
	assert.Equal(t, "fn.list-v2", found.FunctionID)

	_, err = m.FindByScopeSourceAndBindingID(ctx, "demo", "prod", "src-1", "missing")
	assert.ErrorIs(t, err, ErrNotFound)

	byFn, err := m.ListByScopeAndFunctionID(ctx, "demo", "prod", "fn.list-v2")
	require.NoError(t, err)
	require.Len(t, byFn, 1)

	byFn, err = m.ListByScopeAndFunctionID(ctx, "demo", "prod", "fn.nope")
	require.NoError(t, err)
	assert.Empty(t, byFn)

	require.NoError(t, m.Delete(ctx, "demo", "prod", "src-1", "op.list"))
	items, err = m.ListBySource(ctx, "demo", "prod", "src-1")
	require.NoError(t, err)
	assert.Empty(t, items)
}

// ===== BlockedProposalIssueModel.Upsert =====

func TestBlockedProposalIssueModel_Upsert(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&BlockedProposalIssue{}))
	m := NewBlockedProposalIssueModel(db)
	ctx := context.Background()

	issue := &BlockedProposalIssue{
		GameID: "demo", Env: "prod", ResourceKey: "player", FunctionID: "",
		SourceDigests: datatypes.JSON(`["d1"]`),
		Diagnostics:   datatypes.JSON(`[]`),
		RepairHint:    map[string]interface{}{"zh-CN": "修复"},
		Status:        "open", UpdatedBy: "system",
	}
	require.NoError(t, m.Upsert(ctx, issue))

	issue2 := &BlockedProposalIssue{
		GameID: "demo", Env: "prod", ResourceKey: "player", FunctionID: "",
		SourceDigests: datatypes.JSON(`["d2"]`),
		Diagnostics:   datatypes.JSON(`[{"code":"x"}]`),
		RepairHint:    map[string]interface{}{"en-US": "fix"},
		Status:        "open", UpdatedBy: "system",
	}
	require.NoError(t, m.Upsert(ctx, issue2))

	got, err := m.FindByScopeAndResourceKey(ctx, "demo", "prod", "player")
	require.NoError(t, err)
	assert.Equal(t, `["d2"]`, string(got.SourceDigests))

	list, err := m.ListByScope(ctx, "demo", "prod")
	require.NoError(t, err)
	assert.Len(t, list, 1)

	byResource, err := m.ListByScopeAndResourceKey(ctx, "demo", "prod", "player")
	require.NoError(t, err)
	assert.Len(t, byResource, 1)

	require.NoError(t, m.Resolve(ctx, "demo", "prod", "player", "", "admin"))
	list, err = m.ListByScope(ctx, "demo", "prod")
	require.NoError(t, err)
	assert.Empty(t, list, "resolved issues are filtered from open lists")
}

// ===== PageVersionModel pagination =====

func TestPageVersionModel_ListByScopeAndPageKeyPaged(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&PageVersion{}))
	m := NewPageVersionModel(db)
	ctx := context.Background()

	for v := 1; v <= 3; v++ {
		require.NoError(t, m.UpsertByScopePageKeyVersion(ctx, &PageVersion{
			GameID: "demo", Env: "prod", PageKey: "player.list",
			Version: v, SpecJSON: `{}`, Status: "published", CreatedBy: "op",
		}))
	}

	items, total, err := m.ListByScopeAndPageKeyPaged(ctx, "demo", "prod", "player.list", 2, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	require.Len(t, items, 2)
	assert.Equal(t, 3, items[0].Version, "newest first")
	assert.Equal(t, 2, items[1].Version)

	items, total, err = m.ListByScopeAndPageKeyPaged(ctx, "demo", "prod", "player.list", 2, 2)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	require.Len(t, items, 1)
	assert.Equal(t, 1, items[0].Version)
}

// ===== CapabilitySemanticVersionModel pagination =====

func TestCapabilitySemanticVersionModel_ListBySemanticsIDPaged(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&CapabilitySemanticVersion{}))
	m := NewCapabilitySemanticVersionModel(db)
	ctx := context.Background()

	for v := 1; v <= 3; v++ {
		require.NoError(t, db.WithContext(ctx).Create(&CapabilitySemanticVersion{
			SemanticsID: 7, Version: v, Semantics: datatypes.JSON(`{}`),
			SourceDigest: "d", ChangeReason: "r", CreatedBy: "op",
		}).Error)
	}

	vers, total, err := m.ListBySemanticsIDPaged(ctx, 7, 2, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	require.Len(t, vers, 2)
	assert.Equal(t, 3, vers[0].Version)

	vers, total, err = m.ListBySemanticsIDPaged(ctx, 7, 2, 2)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	require.Len(t, vers, 1)

	vers, total, err = m.ListBySemanticsIDPaged(ctx, 999, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, vers)
}

// ===== Pagination helper =====

func TestNewPagination(t *testing.T) {
	assert.Equal(t, PaginationOptions{Page: 1, PageSize: 20}, NewPagination(0, 0))
	assert.Equal(t, PaginationOptions{Page: 1, PageSize: 20}, NewPagination(-3, -1))
	assert.Equal(t, PaginationOptions{Page: 4, PageSize: 50}, NewPagination(4, 50))
	assert.Equal(t, PaginationOptions{Page: 2, PageSize: 100}, NewPagination(2, 500))
	paged := NewPagination(2, 500)
	assert.Equal(t, 100, paged.Offset())
}

// ===== Migration entry points on sqlite =====

func TestAutoMigrateMetaAndGame(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, AutoMigrateMeta(db))
	require.NoError(t, AutoMigrateGame(db))
	require.NoError(t, MigrateAgentSessions(db))
	// Idempotent reruns must stay clean.
	require.NoError(t, AutoMigrateMeta(db))
}
