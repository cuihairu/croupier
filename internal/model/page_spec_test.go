package model

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// PageSpec getter/setter unit tests (no database needed)
// ---------------------------------------------------------------------------

func TestPageSpec_GetTitle(t *testing.T) {
	tests := []struct {
		name     string
		titleJS  string
		expected map[string]string
	}{
		{"empty", "", nil},
		{"valid JSON", `{"zh-CN":"玩家"}`, map[string]string{"zh-CN": "玩家"}},
		{"invalid JSON", `{invalid`, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps := &PageSpec{TitleJSON: tt.titleJS}
			assert.Equal(t, tt.expected, ps.GetTitle())
		})
	}
}

func TestPageSpec_SetTitle(t *testing.T) {
	ps := &PageSpec{}
	title := map[string]string{"zh-CN": "玩家管理", "en-US": "Player Management"}
	require.NoError(t, ps.SetTitle(title))
	assert.Contains(t, ps.TitleJSON, "zh-CN")

	// Verify roundtrip
	got := ps.GetTitle()
	assert.Equal(t, title, got)
}

func TestPageSpec_GetCategoryLabels(t *testing.T) {
	tests := []struct {
		name     string
		labelsJS string
		expected map[string]string
	}{
		{"empty", "", nil},
		{"valid JSON", `{"zh-CN":"玩家"}`, map[string]string{"zh-CN": "玩家"}},
		{"invalid JSON", `{bad`, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps := &PageSpec{CategoryLabelsJSON: tt.labelsJS}
			assert.Equal(t, tt.expected, ps.GetCategoryLabels())
		})
	}
}

func TestPageSpec_SetCategoryLabels(t *testing.T) {
	ps := &PageSpec{}
	labels := map[string]string{"zh-CN": "分类"}
	require.NoError(t, ps.SetCategoryLabels(labels))
	got := ps.GetCategoryLabels()
	assert.Equal(t, labels, got)
}

func TestPageSpec_GetSpec(t *testing.T) {
	raw := json.RawMessage(`{"pageKey":"test","type":"operation"}`)
	ps := &PageSpec{SpecJSON: string(raw)}
	got := ps.GetSpec()
	assert.Equal(t, raw, got)
}

func TestPageSpec_SetSpec(t *testing.T) {
	ps := &PageSpec{}
	raw := json.RawMessage(`{"pageKey":"test"}`)
	ps.SetSpec(raw)
	assert.Equal(t, string(raw), ps.SpecJSON)
}

func TestPageSpec_TableName(t *testing.T) {
	assert.Equal(t, "page_specs", (PageSpec{}).TableName())
}

func TestPublishedPageSpec_TableName(t *testing.T) {
	assert.Equal(t, "published_page_specs", (PublishedPageSpec{}).TableName())
}

func TestPageVersion_TableName(t *testing.T) {
	assert.Equal(t, "page_versions", (PageVersion{}).TableName())
}

func TestPageSpec_SetTitle_NilAndEmpty(t *testing.T) {
	ps := &PageSpec{}
	require.NoError(t, ps.SetTitle(nil))
	assert.Equal(t, "null", ps.TitleJSON)

	ps2 := &PageSpec{}
	require.NoError(t, ps2.SetTitle(map[string]string{}))
	assert.Equal(t, "{}", ps2.TitleJSON)
}

func TestPageSpec_SetCategoryLabels_NilAndEmpty(t *testing.T) {
	ps := &PageSpec{}
	require.NoError(t, ps.SetCategoryLabels(nil))
	assert.Equal(t, "null", ps.CategoryLabelsJSON)

	ps2 := &PageSpec{}
	require.NoError(t, ps2.SetCategoryLabels(map[string]string{}))
	assert.Equal(t, "{}", ps2.CategoryLabelsJSON)
}

func TestPageSpecModel_NewModels(t *testing.T) {
	assert.NotNil(t, NewPageSpecModel(nil))
	assert.NotNil(t, NewPublishedPageSpecModel(nil))
	assert.NotNil(t, NewPageVersionModel(nil))
}

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
	assert.Equal(t, "resource:player", latestDev.BaseProposalKey)
	assert.Equal(t, 7, latestDev.BaseProposalVersion)
	assert.Equal(t, "function-digest-1", latestDev.FunctionDigest)
	assert.Equal(t, "semantics-digest-1", latestDev.SemanticsDigest)
	assert.Equal(t, "dashboard-test", latestDev.GeneratorVersion)
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
		SpecJSON:           `{"pageKey":"` + pageKey + `","type":"operation","title":{"zh-CN":"玩家管理"},"category":{"key":"player","labels":{"zh-CN":"玩家"}},"operation":{"form":{"jsonSchema":{"type":"object","properties":{}}}},"bindings":[]}`,
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
		SpecJSON:              `{"pageKey":"` + pageKey + `","type":"operation","title":{"zh-CN":"玩家管理"},"category":{"key":"player","labels":{"zh-CN":"玩家"}},"operation":{"form":{"jsonSchema":{"type":"object","properties":{}}}},"bindings":[]}`,
		BindingContractsJSON:  `[]`,
		RendererSchemaVersion: "page-spec:1",
		BaseProposalKey:       "resource:player",
		BaseProposalVersion:   7,
		FunctionDigest:        "function-digest-1",
		SemanticsDigest:       "semantics-digest-1",
		GeneratorVersion:      "dashboard-test",
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
