// GAME-013: coverage for explicit gameId support in the games REST API (hyphen preservation
// in Create/Update, uniqueness, and the rename guard).
package game

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newGameServiceTestCtx(t *testing.T) (context.Context, *Service) {
	t.Helper()
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)
	ctx, _ := createTestAdminWithContext(t, db, "gameid-admin", "password123", "admin")
	return ctx, NewService(svcCtx)
}

// An explicit gameId keeps its hyphen verbatim, consistent with the SDK-side registration
// convention (demo-game); business string addressing resolves to that game.
func Test_Create_ExplicitGameIDWithHyphen(t *testing.T) {
	ctx, service := newGameServiceTestCtx(t)

	resp, err := service.Create(ctx, &GameCreateRequest{
		Name:        "DemoGame",
		AliasName:   "Demo Game",
		GameID:      "demo-game",
		Description: "explicit hyphenated game id",
	})

	require.NoError(t, err)
	assert.Equal(t, "demo-game", resp.Game.GameID)
	assert.Equal(t, "DemoGame", resp.Game.Name)

	// Business string addressing (same convention as X-Game-ID) resolves.
	found, err := service.svcCtx.GameModel.FindByGameIDString(ctx, "demo-game")
	require.NoError(t, err)
	assert.Equal(t, resp.Game.ID, found.ID)
}

// Without an explicit gameId, the existing semantics hold: derived from Name, with hyphens
// normalized to underscores.
func Test_Create_GameIDDerivedFromNameNormalizesHyphen(t *testing.T) {
	ctx, service := newGameServiceTestCtx(t)

	resp, err := service.Create(ctx, &GameCreateRequest{Name: "demo-game", AliasName: "Derived"})

	require.NoError(t, err)
	assert.Equal(t, "demo_game", resp.Game.GameID)
}

// An explicit gameId with an invalid character set or out-of-bounds length is rejected.
func Test_Create_ExplicitGameID_InvalidCharacters(t *testing.T) {
	ctx, service := newGameServiceTestCtx(t)

	for i, bad := range []string{"bad id!", "demo/game", "demo@game", "demo game!!"} {
		resp, err := service.Create(ctx, &GameCreateRequest{
			Name:      "game_for_bad_" + strconv.Itoa(i),
			AliasName: "bad",
			GameID:    bad,
		})
		assert.Error(t, err, "input %q", bad)
		assert.Nil(t, resp)
	}
}

// A duplicate explicit gameId (conflicting after case-insensitive normalization) is rejected
// with 409.
func Test_Create_ExplicitGameID_Duplicate(t *testing.T) {
	ctx, service := newGameServiceTestCtx(t)

	_, err := service.Create(ctx, &GameCreateRequest{Name: "first", AliasName: "First", GameID: "demo-game"})
	require.NoError(t, err)

	// Same name in a different spelling (uppercase normalization + hyphen) conflicts.
	resp, err := service.Create(ctx, &GameCreateRequest{Name: "second", AliasName: "Second", GameID: "Demo-Game"})
	assert.Error(t, err)
	assert.Nil(t, resp)
}

// A game with no environment bindings may change its game_id; after the rename the old
// identifier is no longer addressable and the new one is.
func Test_Update_RenameGameID_Success(t *testing.T) {
	ctx, service := newGameServiceTestCtx(t)

	created, err := service.Create(ctx, &GameCreateRequest{Name: "renamable", AliasName: "Renamable"})
	require.NoError(t, err)

	resp, err := service.Update(ctx, &GameUpdateRequest{
		ID:        strconv.Itoa(int(created.Game.ID)),
		GameID:    "renamable-game",
		AliasName: "Renamable",
	})
	require.NoError(t, err)
	assert.Equal(t, "renamable-game", resp.Game.GameID)

	_, err = service.svcCtx.GameModel.FindByGameIDString(ctx, "renamable")
	assert.ErrorIs(t, err, model.ErrGameNotFound)
	found, err := service.svcCtx.GameModel.FindByGameIDString(ctx, "renamable-game")
	require.NoError(t, err)
	assert.Equal(t, created.Game.ID, found.ID)
}

// Renaming game_id is rejected when the game already has environment bindings (prevents
// dangling routing data).
func Test_Update_RenameGameID_BlockedByEnvBinding(t *testing.T) {
	ctx, service := newGameServiceTestCtx(t)

	created, err := service.Create(ctx, &GameCreateRequest{Name: "bound-game", AliasName: "Bound"})
	require.NoError(t, err)

	require.NoError(t, service.svcCtx.GameModel.AddEnvBinding(
		ctx, created.Game.GameID, "demo", "game_bound_game_demo", "Demo env", "#13c2c2",
	))

	resp, err := service.Update(ctx, &GameUpdateRequest{
		ID:        strconv.Itoa(int(created.Game.ID)),
		GameID:    "bound-game-new",
		AliasName: "Bound",
	})
	assert.Error(t, err)
	assert.Nil(t, resp)
}

// Non-empty game envs metadata (the transitional state before bindings are backfilled) also
// blocks renaming game_id.
func Test_Update_RenameGameID_BlockedByEnvsMetadata(t *testing.T) {
	ctx, service := newGameServiceTestCtx(t)

	created, err := service.Create(ctx, &GameCreateRequest{Name: "envs-meta-game", AliasName: "EnvsMeta"})
	require.NoError(t, err)
	_ = created

	game, err := service.svcCtx.GameModel.FindByGameIDString(ctx, "envs_meta_game")
	require.NoError(t, err)
	envsJSON, err := json.Marshal([]model.GameEnv{{Env: "demo", Description: "Demo env"}})
	require.NoError(t, err)
	require.NoError(t, service.svcCtx.GameModel.Update(ctx, game.ID, map[string]interface{}{"envs": envsJSON}))

	resp, err := service.Update(ctx, &GameUpdateRequest{
		ID:        strconv.Itoa(int(game.ID)),
		GameID:    "envs-meta-game-new",
		AliasName: "EnvsMeta",
	})
	assert.Error(t, err)
	assert.Nil(t, resp)
}

// Renaming game_id onto an existing identifier is rejected with 409.
func Test_Update_RenameGameID_Duplicate(t *testing.T) {
	ctx, service := newGameServiceTestCtx(t)

	_, err := service.Create(ctx, &GameCreateRequest{Name: "taken-id", AliasName: "Taken", GameID: "taken-game"})
	require.NoError(t, err)
	target, err := service.Create(ctx, &GameCreateRequest{Name: "rename-target", AliasName: "RenameTarget"})
	require.NoError(t, err)

	resp, err := service.Update(ctx, &GameUpdateRequest{
		ID:        strconv.Itoa(int(target.Game.ID)),
		GameID:    "taken-game",
		AliasName: "RenameTarget",
	})
	assert.Error(t, err)
	assert.Nil(t, resp)
}
