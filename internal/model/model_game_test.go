package model

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

// Split from model_test.go; test bodies moved verbatim.

func TestNewGameModel(t *testing.T) {
	db := setupTestDB(t)
	model := NewGameModel(db)
	assert.NotNil(t, model)
}

func TestGameModel_Create(t *testing.T) {
	db := setupTestDB(t)
	model := NewGameModel(db)
	ctx := context.Background()

	game := &Game{
		Name:      "Test Game",
		AliasName: "testgame",
		Status:    "dev",
	}

	err := model.Create(ctx, game)
	require.NoError(t, err)
	assert.NotZero(t, game.ID)
}

func TestGameModel_FindOne(t *testing.T) {
	db := setupTestDB(t)
	model := NewGameModel(db)
	ctx := context.Background()

	game := &Game{
		Name:      "Find One Game",
		AliasName: "findonegame",
		Status:    "dev",
	}
	err := model.Create(ctx, game)
	require.NoError(t, err)

	found, err := model.FindOne(ctx, game.ID)
	require.NoError(t, err)
	assert.Equal(t, game.Name, found.Name)
	assert.Equal(t, game.AliasName, found.AliasName)

	_, err = model.FindOne(ctx, 99999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "game not found")
}

func TestGameModel_FindByName(t *testing.T) {
	db := setupTestDB(t)
	model := NewGameModel(db)
	ctx := context.Background()

	game := &Game{
		Name:      "Find By Name Game",
		AliasName: "findbynamegame",
		Status:    "dev",
	}
	err := model.Create(ctx, game)
	require.NoError(t, err)

	found, err := model.FindByName(ctx, "Find By Name Game")
	require.NoError(t, err)
	assert.Equal(t, game.ID, found.ID)

	_, err = model.FindByName(ctx, "Nonexistent")
	assert.Error(t, err)
}

func TestGameModel_ExistsByNameIgnoreCase(t *testing.T) {
	db := setupTestDB(t)
	model := NewGameModel(db)
	ctx := context.Background()

	game := &Game{
		Name:      "Test Game Case",
		AliasName: "testcasegame",
		Status:    "dev",
	}
	err := model.Create(ctx, game)
	require.NoError(t, err)

	// Test case-insensitive check
	exists, err := model.ExistsByNameIgnoreCase(ctx, "TEST GAME CASE")
	require.NoError(t, err)
	assert.True(t, exists)

	// Test non-existent
	exists, err = model.ExistsByNameIgnoreCase(ctx, "Nonexistent Game")
	require.NoError(t, err)
	assert.False(t, exists)

	// Test with exclude ID
	exists, err = model.ExistsByNameIgnoreCase(ctx, "TEST GAME CASE", game.ID)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestGameModel_Update(t *testing.T) {
	db := setupTestDB(t)
	model := NewGameModel(db)
	ctx := context.Background()

	game := &Game{
		Name:      "Update Game",
		AliasName: "updategame",
		Status:    "dev",
	}
	err := model.Create(ctx, game)
	require.NoError(t, err)

	updates := map[string]interface{}{
		"name":   "Updated Game Name",
		"status": "test",
	}
	err = model.Update(ctx, game.ID, updates)
	require.NoError(t, err)

	found, err := model.FindOne(ctx, game.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Game Name", found.Name)
	assert.Equal(t, "test", found.Status)
}

func TestGameModel_Delete(t *testing.T) {
	db := setupTestDB(t)
	model := NewGameModel(db)
	ctx := context.Background()

	game := &Game{
		Name:      "Delete Game",
		AliasName: "deletegame",
		Status:    "dev",
	}
	err := model.Create(ctx, game)
	require.NoError(t, err)

	err = model.Delete(ctx, game.ID)
	require.NoError(t, err)

	_, err = model.FindOne(ctx, game.ID)
	assert.Error(t, err)
}

func TestGameModel_List(t *testing.T) {
	db := setupTestDB(t)
	model := NewGameModel(db)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		game := &Game{
			Name:      "List Game " + string(rune('a'+i)),
			AliasName: "listgame" + string(rune('a'+i)),
			Status:    "dev",
		}
		err := model.Create(ctx, game)
		require.NoError(t, err)
	}

	games, total, err := model.List(ctx, ListGamesOptions{
		Page:     1,
		PageSize: 10,
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(5))
	assert.GreaterOrEqual(t, len(games), 5)
}

func TestGameModel_List_StatusFilter(t *testing.T) {
	db := setupTestDB(t)
	model := NewGameModel(db)
	ctx := context.Background()

	game1 := &Game{Name: "Dev Game 123", AliasName: "devgame123", Status: "dev"}
	game2 := &Game{Name: "Test Game 123", AliasName: "testgame123", Status: "test"}
	err := model.Create(ctx, game1)
	require.NoError(t, err)
	err = model.Create(ctx, game2)
	require.NoError(t, err)

	games, total, err := model.List(ctx, ListGamesOptions{
		Page:     1,
		PageSize: 10,
		Status:   "dev",
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))
	for _, g := range games {
		assert.Equal(t, "dev", g.Status)
	}
}

func TestGameModel_List_Search(t *testing.T) {
	db := setupTestDB(t)
	model := NewGameModel(db)
	ctx := context.Background()

	game1 := &Game{Name: "Searchable Game", AliasName: "searchgame1", Status: "dev", Description: "A searchable game"}
	game2 := &Game{Name: "Other Game", AliasName: "othergame", Status: "dev", Description: "Other description"}
	err := model.Create(ctx, game1)
	require.NoError(t, err)
	err = model.Create(ctx, game2)
	require.NoError(t, err)

	games, total, err := model.List(ctx, ListGamesOptions{
		Page:     1,
		PageSize: 10,
		Search:   "searchable",
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))
	found := false
	for _, g := range games {
		if g.Name == "Searchable Game" {
			found = true
			break
		}
	}
	assert.True(t, found)
}

func TestGameModel_UpdateStatus(t *testing.T) {
	db := setupTestDB(t)
	model := NewGameModel(db)
	ctx := context.Background()

	game := &Game{
		Name:      "Status Game",
		AliasName: "statusgame",
		Status:    "dev",
	}
	err := model.Create(ctx, game)
	require.NoError(t, err)

	err = model.UpdateStatus(ctx, game.ID, "test")
	require.NoError(t, err)

	found, err := model.FindOne(ctx, game.ID)
	require.NoError(t, err)
	assert.Equal(t, "test", found.Status)
}

func TestGameModel_ToggleEnabled(t *testing.T) {
	db := setupTestDB(t)
	model := NewGameModel(db)
	ctx := context.Background()

	game := &Game{
		Name:      "Toggle Game",
		AliasName: "togglegame",
		Status:    "dev",
	}
	err := model.Create(ctx, game)
	require.NoError(t, err)

	// Toggle from false to true (default is true)
	err = model.ToggleEnabled(ctx, game.ID)
	require.NoError(t, err)

	found, err := model.FindOne(ctx, game.ID)
	require.NoError(t, err)
	// After toggle from default true, it should be false
	assert.False(t, found.Enabled)

	// Toggle from false to true
	err = model.ToggleEnabled(ctx, game.ID)
	require.NoError(t, err)

	found, err = model.FindOne(ctx, game.ID)
	require.NoError(t, err)
	assert.True(t, found.Enabled)
}

func TestGameModel_ListAll(t *testing.T) {
	db := setupTestDB(t)
	model := NewGameModel(db)
	ctx := context.Background()

	game1 := &Game{Name: "All Game 1", AliasName: "allgame1", Status: "dev"}
	game2 := &Game{Name: "All Game 2", AliasName: "allgame2", Status: "test"}
	err := model.Create(ctx, game1)
	require.NoError(t, err)
	err = model.Create(ctx, game2)
	require.NoError(t, err)

	games, err := model.ListAll(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(games), 2)
}

func TestGameModel_FindByGameID(t *testing.T) {
	db := setupTestDB(t)
	model := NewGameModel(db)
	ctx := context.Background()

	game := &Game{
		Name:      "GameID Game",
		AliasName: "gameidgame",
		Status:    "dev",
	}
	err := model.Create(ctx, game)
	require.NoError(t, err)

	found, err := model.FindByGameID(ctx, game.ID)
	require.NoError(t, err)
	assert.Equal(t, game.Name, found.Name)
}

// ===== Game Helper Methods Tests =====

func TestGame_GetEnvs(t *testing.T) {
	game := &Game{}

	// Test empty envs
	envs, err := game.GetEnvs()
	require.NoError(t, err)
	assert.Nil(t, envs)

	// Test with envs
	envList := []GameEnv{
		{Env: "dev", Description: "Development"},
		{Env: "prod", Description: "Production"},
	}
	err = game.SetEnvs(envList)
	require.NoError(t, err)

	retrieved, err := game.GetEnvs()
	require.NoError(t, err)
	assert.Len(t, retrieved, 2)
	assert.Equal(t, "dev", retrieved[0].Env)
}

func TestGame_SetEnvs(t *testing.T) {
	game := &Game{}

	envList := []GameEnv{
		{Env: "test", Description: "Test Env", Color: "blue"},
	}
	err := game.SetEnvs(envList)
	require.NoError(t, err)
	assert.NotNil(t, game.Envs)
}

func TestGame_GetConfig(t *testing.T) {
	game := &Game{}

	// Test empty config
	var dest map[string]interface{}
	err := game.GetConfig(&dest)
	require.NoError(t, err)

	// Test with config
	game.Config = `{"key":"value"}`
	err = game.GetConfig(&dest)
	require.NoError(t, err)
	assert.Equal(t, "value", dest["key"])
}

func TestGame_SetConfig(t *testing.T) {
	game := &Game{}

	config := map[string]string{
		"host": "localhost",
		"port": "8080",
	}
	err := game.SetConfig(config)
	require.NoError(t, err)
	assert.Contains(t, game.Config, "localhost")
}

// ===== RoleModel Tests =====

func TestGameModel_EnvBindings(t *testing.T) {
	db := setupTestDB(t)
	model := NewGameModel(db)
	ctx := context.Background()

	// Create a game first
	game := &Game{
		GameID: "binding-test",
		Name:   "Binding Test Game",
		Status: "dev",
	}
	err := model.Create(ctx, game)
	require.NoError(t, err)

	// Add env bindings
	err = model.AddEnvBinding(ctx, "binding-test", "prod", "game_binding_test_prod", "Production", "#FF0000")
	require.NoError(t, err)

	err = model.AddEnvBinding(ctx, "binding-test", "dev", "game_binding_test_dev", "Development", "#00FF00")
	require.NoError(t, err)

	// List env bindings
	bindings, err := model.ListEnvBindings(ctx, "binding-test")
	require.NoError(t, err)
	assert.Len(t, bindings, 2)

	// Find env binding
	binding, err := model.FindEnvBinding(ctx, "binding-test", "prod")
	require.NoError(t, err)
	assert.NotNil(t, binding)
	assert.Equal(t, "game_binding_test_prod", binding.DatabaseName)

	// Lookup database name
	dbName, err := model.LookupDatabaseName(ctx, "binding-test", "prod")
	require.NoError(t, err)
	assert.Equal(t, "game_binding_test_prod", dbName)

	// Has env binding
	has, err := model.HasEnvBinding(ctx, "binding-test", "prod")
	require.NoError(t, err)
	assert.True(t, has)

	has, err = model.HasEnvBinding(ctx, "binding-test", "staging")
	require.NoError(t, err)
	assert.False(t, has)

	// List all env bindings
	allBindings, err := model.ListAllEnvBindings(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(allBindings), 2)

	// Remove env binding
	err = model.RemoveEnvBinding(ctx, "binding-test", "dev")
	require.NoError(t, err)

	bindings, err = model.ListEnvBindings(ctx, "binding-test")
	require.NoError(t, err)
	assert.Len(t, bindings, 1)

	// Remove all env bindings
	err = model.RemoveAllEnvBindings(ctx, "binding-test")
	require.NoError(t, err)

	bindings, err = model.ListEnvBindings(ctx, "binding-test")
	require.NoError(t, err)
	assert.Len(t, bindings, 0)
}

func TestGameModel_FindByGameIDString(t *testing.T) {
	db := setupTestDB(t)
	model := NewGameModel(db)
	ctx := context.Background()

	// Create a game with unique alias
	game := &Game{
		GameID:    "find-string-test",
		Name:      "Find String Test",
		Status:    "dev",
		AliasName: "find-string-test-alias",
	}
	err := model.Create(ctx, game)
	require.NoError(t, err)

	// Find by game ID string
	found, err := model.FindByGameIDString(ctx, "find-string-test")
	require.NoError(t, err)
	assert.Equal(t, "find-string-test", found.GameID)

	// Not found
	_, err = model.FindByGameIDString(ctx, "nonexistent")
	assert.Error(t, err)
}
