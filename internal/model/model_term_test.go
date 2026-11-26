package model

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

// Split from model_test.go; test bodies moved verbatim.

func TestNewTermDictionaryModel(t *testing.T) {
	db := setupTermDictionaryTestDB(t)
	model := NewTermDictionaryModel(db)
	assert.NotNil(t, model)
}

func TestTermDictionaryModel_List(t *testing.T) {
	db := setupTermDictionaryTestDB(t)
	model := NewTermDictionaryModel(db)
	ctx := context.Background()

	terms := []*TermDictionary{
		{Domain: "resource", TermKey: "player", Alias: "玩家", Display: map[string]string{"zh-CN": "玩家", "en-US": "Player"}, SortOrder: 1},
		{Domain: "resource", TermKey: "game", Alias: "游戏", Display: map[string]string{"zh-CN": "游戏", "en-US": "Game"}, SortOrder: 2},
		{Domain: "operation", TermKey: "create", Alias: "创建", Display: map[string]string{"zh-CN": "创建", "en-US": "Create"}, SortOrder: 1},
	}
	for _, term := range terms {
		err := db.Create(term).Error
		require.NoError(t, err)
	}

	// List all
	all, err := model.List(ctx, "")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(all), 3)

	// Filter by domain
	resourceTerms, err := model.List(ctx, "resource")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(resourceTerms), 2)
}

func TestTermDictionaryModel_Upsert(t *testing.T) {
	db := setupTermDictionaryTestDB(t)
	model := NewTermDictionaryModel(db)
	ctx := context.Background()

	// Create new
	term := &TermDictionary{
		Domain:    "resource",
		TermKey:   "item",
		Alias:     "道具",
		Display:   map[string]string{"zh-CN": "道具", "en-US": "Item"},
		SortOrder: 3,
	}
	err := model.Upsert(ctx, term)
	require.NoError(t, err)
	assert.NotZero(t, term.ID)

	// Update existing
	term.Display = map[string]string{"zh-CN": "游戏道具"}
	term.SortOrder = 10
	err = model.Upsert(ctx, term)
	require.NoError(t, err)

	// Verify
	list, err := model.List(ctx, "resource")
	require.NoError(t, err)
	for _, term := range list {
		if term.Alias == "道具" {
			assert.Equal(t, "游戏道具", term.Display["zh-CN"])
			assert.Equal(t, 10, term.SortOrder)
		}
	}

	// Test nil term (should not error)
	err = model.Upsert(ctx, nil)
	require.NoError(t, err)

	// Test empty required fields (should not error)
	emptyTerm := &TermDictionary{}
	err = model.Upsert(ctx, emptyTerm)
	require.NoError(t, err)

	legacyTerm := &TermDictionary{
		Domain:  "entity",
		TermKey: "player",
		Alias:   "玩家",
	}
	err = model.Upsert(ctx, legacyTerm)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported term dictionary domain")
}

func TestTermDictionaryModel_AliasMap(t *testing.T) {
	db := setupTermDictionaryTestDB(t)
	model := NewTermDictionaryModel(db)
	ctx := context.Background()

	terms := []*TermDictionary{
		{Domain: "resource", TermKey: "player", Alias: "玩家2", Display: map[string]string{"zh-CN": "玩家", "en-US": "Player"}},
		{Domain: "resource", TermKey: "game", Alias: "游戏2", Display: map[string]string{"zh-CN": "游戏", "en-US": "Game"}},
		{Domain: "operation", TermKey: "create", Alias: "创建2", Display: map[string]string{"zh-CN": "创建", "en-US": "Create"}},
	}
	for _, term := range terms {
		err := db.Create(term).Error
		require.NoError(t, err)
	}

	// Get alias map
	aliasMap, err := model.AliasMap(ctx)
	require.NoError(t, err)
	assert.Contains(t, aliasMap, "resource")
	assert.Contains(t, aliasMap, "operation")
	assert.Equal(t, "player", aliasMap["resource"]["玩家2"])
	assert.Equal(t, "game", aliasMap["resource"]["游戏2"])
	assert.Equal(t, "create", aliasMap["operation"]["创建2"])
}

func TestTermDictionaryModel_DeleteByAlias(t *testing.T) {
	db := setupTermDictionaryTestDB(t)
	model := NewTermDictionaryModel(db)
	ctx := context.Background()

	term := &TermDictionary{
		Domain:  "resource",
		TermKey: "test_del",
		Alias:   "测试_del",
		Display: map[string]string{"zh-CN": "测试", "en-US": "Test"},
	}
	err := db.Create(term).Error
	require.NoError(t, err)

	// Delete by alias
	err = model.DeleteByAlias(ctx, "resource", "测试_del")
	require.NoError(t, err)

	// Verify
	list, err := model.List(ctx, "resource")
	require.NoError(t, err)
	for _, termItem := range list {
		assert.NotEqual(t, "测试_del", termItem.Alias)
	}

	// Empty params (should not error)
	err = model.DeleteByAlias(ctx, "", "")
	require.NoError(t, err)
}

// ===== AgentSessionModel Tests =====
