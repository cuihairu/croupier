package model

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestPageSpecModelsUseScopedPageIdentity(t *testing.T) {
	db := setupPageSpecTestDB(t)
	ctx := context.Background()
	drafts := NewPageSpecModel(db)
	published := NewPublishedPageSpecModel(db)
	versions := NewPageVersionModel(db)

	require.NoError(t, drafts.Upsert(ctx, testPageSpec("game-a", "dev", "player.manage", "draft-a-dev")))
	require.NoError(t, drafts.Upsert(ctx, testPageSpec("game-a", "prod", "player.manage", "draft-a-prod")))
	require.NoError(t, drafts.Upsert(ctx, testPageSpec("game-b", "dev", "player.manage", "draft-b-dev")))
	require.NoError(t, drafts.Upsert(ctx, testPageSpec("game-a", "dev", "player.manage", "draft-a-dev-updated")))

	devDraft, err := drafts.FindByScopeAndPageKey(ctx, "game-a", "dev", "player.manage")
	require.NoError(t, err)
	assert.Equal(t, "draft-a-dev-updated", devDraft.TitleJSON)
	prodDraft, err := drafts.FindByScopeAndPageKey(ctx, "game-a", "prod", "player.manage")
	require.NoError(t, err)
	assert.Equal(t, "draft-a-prod", prodDraft.TitleJSON)

	devList, err := drafts.ListByScope(ctx, "game-a", "dev")
	require.NoError(t, err)
	require.Len(t, devList, 1)
	assert.Equal(t, "draft-a-dev-updated", devList[0].TitleJSON)

	require.NoError(t, published.Create(ctx, testPublishedPageSpec("game-a", "dev", "player.manage", 1, true)))
	require.NoError(t, published.Create(ctx, testPublishedPageSpec("game-a", "dev", "player.manage", 2, true)))
	require.NoError(t, published.Create(ctx, testPublishedPageSpec("game-a", "prod", "player.manage", 1, true)))
	require.NoError(t, published.Create(ctx, testPublishedPageSpec("game-b", "dev", "player.manage", 1, true)))

	latestDev, err := published.FindLatestByScopeAndPageKey(ctx, "game-a", "dev", "player.manage")
	require.NoError(t, err)
	assert.Equal(t, 2, latestDev.Version)
	latestProd, err := published.FindLatestByScopeAndPageKey(ctx, "game-a", "prod", "player.manage")
	require.NoError(t, err)
	assert.Equal(t, 1, latestProd.Version)

	require.NoError(t, published.DeactivatePage(ctx, "game-a", "dev", "player.manage", time.Now()))
	_, err = published.FindLatestByScopeAndPageKey(ctx, "game-a", "dev", "player.manage")
	require.Error(t, err)
	stillActive, err := published.FindLatestByScopeAndPageKey(ctx, "game-a", "prod", "player.manage")
	require.NoError(t, err)
	assert.Equal(t, "game-a", stillActive.GameID)
	assert.Equal(t, "prod", stillActive.Env)

	require.NoError(t, versions.UpsertByScopePageKeyVersion(ctx, testPageVersion("game-a", "dev", "player.manage", 1, "dev-v1")))
	require.NoError(t, versions.UpsertByScopePageKeyVersion(ctx, testPageVersion("game-a", "prod", "player.manage", 1, "prod-v1")))
	require.NoError(t, versions.UpsertByScopePageKeyVersion(ctx, testPageVersion("game-a", "dev", "player.manage", 1, "dev-v1-updated")))

	nextDev, err := versions.GetNextVersion(ctx, "game-a", "dev", "player.manage")
	require.NoError(t, err)
	assert.Equal(t, 2, nextDev)
	nextProd, err := versions.GetNextVersion(ctx, "game-a", "prod", "player.manage")
	require.NoError(t, err)
	assert.Equal(t, 2, nextProd)
	devVersions, err := versions.ListByScopeAndPageKey(ctx, "game-a", "dev", "player.manage")
	require.NoError(t, err)
	require.Len(t, devVersions, 1)
	assert.Equal(t, "dev-v1-updated", devVersions[0].SpecJSON)
}

func setupPageSpecTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&PageSpec{}, &PublishedPageSpec{}, &PageVersion{}))
	return db
}

func testPageSpec(gameID, env, pageKey, titleJSON string) *PageSpec {
	return &PageSpec{
		GameID:             gameID,
		Env:                env,
		PageKey:            pageKey,
		Type:               "operation",
		TitleJSON:          titleJSON,
		CategoryKey:        "player",
		CategoryLabelsJSON: `{"zh-CN":"玩家"}`,
		SchemaJSON:         `{"type":"object","x-component":"ConsolePage","x-component-props":{"schemaVersion":"formily-page:1"}}`,
		BindingsJSON:       `[]`,
		Status:             "draft",
		DraftRevision:      1,
	}
}

func testPublishedPageSpec(gameID, env, pageKey string, version int, active bool) *PublishedPageSpec {
	return &PublishedPageSpec{
		GameID:                gameID,
		Env:                   env,
		PageKey:               pageKey,
		Version:               version,
		SpecJSON:              `{"pageKey":"` + pageKey + `","type":"operation","title":{"zh-CN":"玩家管理"},"category":{"key":"player","labels":{"zh-CN":"玩家"}},"schema":{"type":"object","x-component":"ConsolePage","x-component-props":{"schemaVersion":"formily-page:1"}},"bindings":[]}`,
		BindingContractsJSON:  `[]`,
		RendererSchemaVersion: "formily-page:1",
		Active:                active,
		PublishedAt:           time.Now(),
		PublishedBy:           "tester",
	}
}

func testPageVersion(gameID, env, pageKey string, version int, specJSON string) *PageVersion {
	return &PageVersion{
		GameID:    gameID,
		Env:       env,
		PageKey:   pageKey,
		Version:   version,
		SpecJSON:  specJSON,
		Status:    "published",
		Message:   "test",
		CreatedBy: "tester",
	}
}
