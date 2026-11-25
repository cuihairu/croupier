package game

import (
	"context"
	"strconv"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// createPermUser creates a non-admin user holding only the given permission points and returns
// its ctx.
func createPermUser(t *testing.T, db *gorm.DB, username string, permIDs ...string) context.Context {
	t.Helper()

	adminModel := model.NewAdminModel(db)
	admin := &model.Admin{Username: username, Nickname: username, Status: 1}
	require.NoError(t, adminModel.Create(context.Background(), admin, "password123"))

	role := &model.Role{Name: "role-" + username, Description: "scoped role"}
	require.NoError(t, db.Where("name = ?", role.Name).FirstOrCreate(role).Error)
	for _, pid := range permIDs {
		perm := &model.Permission{ID: pid, Name: pid, Resource: "games", Action: "manage", Category: "games"}
		require.NoError(t, db.Where("id = ?", pid).FirstOrCreate(perm).Error)
		rolePerm := &model.RolePermission{RoleID: role.ID, PermissionID: pid}
		require.NoError(t, db.Where("role_id = ? AND permission_id = ?", role.ID, pid).
			FirstOrCreate(rolePerm).Error)
	}
	require.NoError(t, adminModel.AssignRole(context.Background(), admin.ID, role.ID))

	ctx := context.WithValue(context.Background(), "username", username)
	return context.WithValue(ctx, "adminID", admin.ID)
}

func strPtr(s string) *string { return &s }

// Game create/update/delete is separated from environment management permissions: games:manage
// must not create, edit, or delete games.
func TestGameWrite_PermissionSplit(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)
	svc := NewService(svcCtx)

	manageCtx := createPermUser(t, db, "perm_env_manager", "games:manage")
	writeCtx := createPermUser(t, db, "perm_game_writer", "games:write")

	// games:manage only: creating, editing, and deleting games are all rejected
	_, err := svc.Create(manageCtx, &GameCreateRequest{Name: "split_manage_game"})
	require.Error(t, err)

	// games:write: creation passes, the icon lands in the database
	resp, err := svc.Create(writeCtx, &GameCreateRequest{
		Name:      "split_write_game",
		AliasName: "分拆写手",
		Icon:      "https://cdn.example.com/game.png",
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Game)
	assert.Equal(t, "https://cdn.example.com/game.png", resp.Game.Icon)
	gameID := resp.Game.GameID

	// games:manage only: both editing and deleting are rejected
	_, err = svc.Update(manageCtx, &GameUpdateRequest{ID: gameID, AliasName: "越权改名", Icon: strPtr("x")})
	require.Error(t, err)
	require.Error(t, svc.Delete(manageCtx, &GameDeleteRequest{ID: gameID}))

	// games:write: editing (display name + icon) passes; a nil icon pointer = clear
	upd, err := svc.Update(writeCtx, &GameUpdateRequest{
		ID:        gameID,
		AliasName: "改后显示名",
		Icon:      strPtr(""),
	})
	require.NoError(t, err)
	assert.Equal(t, "改后显示名", upd.Game.AliasName)
	assert.Empty(t, upd.Game.Icon)

	// A game with no environments can be deleted by games:write
	require.NoError(t, svc.Delete(writeCtx, &GameDeleteRequest{ID: gameID}))
}

// The display name defaults back to the game identifier (the alias_name unique index does not
// accept multiple empty strings).
func TestGameCreate_AliasDefaultsToName(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)
	svc := NewService(svcCtx)

	writeCtx := createPermUser(t, db, "perm_alias_default", "games:write")

	resp, err := svc.Create(writeCtx, &GameCreateRequest{Name: "alias_default_game"})
	require.NoError(t, err)
	require.NotNil(t, resp.Game)
	assert.Equal(t, "alias_default_game", resp.Game.AliasName)
}

// Environments must be emptied before a game can be deleted: both the Envs metadata and the
// game_envs binding table block it (409 semantics).
func TestGameWrite_DeleteBlockedByEnvs(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)
	svc := NewService(svcCtx)

	writeCtx := createPermUser(t, db, "perm_env_gate", "games:write")

	gameModel := model.NewGameModel(db)
	game := &model.Game{Name: "del_with_env", AliasName: "带环境待删", Status: "dev", Enabled: true}
	require.NoError(t, gameModel.Create(context.Background(), game))
	gameID := strconv.FormatUint(uint64(game.ID), 10)

	// ① Envs metadata non-empty -> rejected
	require.NoError(t, game.SetEnvs([]model.GameEnv{{Env: "prod", Description: "Production"}}))
	require.NoError(t, db.Model(&model.Game{}).Where("id = ?", game.ID).
		Update("envs", game.Envs).Error)
	err := svc.Delete(writeCtx, &GameDeleteRequest{ID: gameID})
	require.Error(t, err)
	require.Contains(t, err.Error(), "环境")

	// ② Envs metadata cleared but the binding table still has rows -> still rejected
	require.NoError(t, db.Model(&model.Game{}).Where("id = ?", game.ID).
		Update("envs", "[]").Error)
	require.NoError(t, db.Create(&model.GameEnvBinding{
		GameID: game.GameID, Env: "prod", DatabaseName: "game_demo_prod",
	}).Error)
	err = svc.Delete(writeCtx, &GameDeleteRequest{ID: gameID})
	require.Error(t, err)
	require.Contains(t, err.Error(), "环境")

	// ③ Both cleared -> deletion is let through
	require.NoError(t, db.Where("game_id = ? AND env = ?", game.GameID, "prod").
		Delete(&model.GameEnvBinding{}).Error)
	require.NoError(t, svc.Delete(writeCtx, &GameDeleteRequest{ID: gameID}))
}
