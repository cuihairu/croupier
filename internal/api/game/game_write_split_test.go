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

// createPermUser 建一个仅持有指定功能权限点的非 admin 用户并返回其 ctx。
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

// 游戏本体增删改与环境管理权限分离：games:manage 不得创建/编辑/删除游戏。
func TestGameWrite_PermissionSplit(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)
	svc := NewService(svcCtx)

	manageCtx := createPermUser(t, db, "perm_env_manager", "games:manage")
	writeCtx := createPermUser(t, db, "perm_game_writer", "games:write")

	// 仅 games:manage：创建/编辑/删除游戏全部拒绝
	_, err := svc.Create(manageCtx, &GameCreateRequest{Name: "split_manage_game"})
	require.Error(t, err)

	// games:write：创建通过，icon 落库
	resp, err := svc.Create(writeCtx, &GameCreateRequest{
		Name:      "split_write_game",
		AliasName: "分拆写手",
		Icon:      "https://cdn.example.com/game.png",
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Game)
	assert.Equal(t, "https://cdn.example.com/game.png", resp.Game.Icon)
	gameID := resp.Game.GameID

	// 仅 games:manage：编辑与删除均拒绝
	_, err = svc.Update(manageCtx, &GameUpdateRequest{ID: gameID, AliasName: "越权改名", Icon: strPtr("x")})
	require.Error(t, err)
	require.Error(t, svc.Delete(manageCtx, &GameDeleteRequest{ID: gameID}))

	// games:write：编辑（显示名 + 图标）通过；图标指针置空 = 清除
	upd, err := svc.Update(writeCtx, &GameUpdateRequest{
		ID:        gameID,
		AliasName: "改后显示名",
		Icon:      strPtr(""),
	})
	require.NoError(t, err)
	assert.Equal(t, "改后显示名", upd.Game.AliasName)
	assert.Empty(t, upd.Game.Icon)

	// 无环境的游戏，games:write 可删
	require.NoError(t, svc.Delete(writeCtx, &GameDeleteRequest{ID: gameID}))
}

// 显示名缺省回填游戏标识（alias_name 唯一索引不接受多个空串）。
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

// 删除游戏前必须清空环境：Envs 元数据与 game_envs 绑定表两处都拦（409 语义）。
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

	// ① Envs 元数据非空 → 拒绝
	require.NoError(t, game.SetEnvs([]model.GameEnv{{Env: "prod", Description: "Production"}}))
	require.NoError(t, db.Model(&model.Game{}).Where("id = ?", game.ID).
		Update("envs", game.Envs).Error)
	err := svc.Delete(writeCtx, &GameDeleteRequest{ID: gameID})
	require.Error(t, err)
	require.Contains(t, err.Error(), "环境")

	// ② 清掉 Envs 元数据但绑定表仍有记录 → 仍拒绝
	require.NoError(t, db.Model(&model.Game{}).Where("id = ?", game.ID).
		Update("envs", "[]").Error)
	require.NoError(t, db.Create(&model.GameEnvBinding{
		GameID: game.GameID, Env: "prod", DatabaseName: "game_demo_prod",
	}).Error)
	err = svc.Delete(writeCtx, &GameDeleteRequest{ID: gameID})
	require.Error(t, err)
	require.Contains(t, err.Error(), "环境")

	// ③ 两处清空 → 放行
	require.NoError(t, db.Where("game_id = ? AND env = ?", game.GameID, "prod").
		Delete(&model.GameEnvBinding{}).Error)
	require.NoError(t, svc.Delete(writeCtx, &GameDeleteRequest{ID: gameID}))
}
