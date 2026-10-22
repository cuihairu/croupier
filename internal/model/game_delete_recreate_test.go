package model

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 游戏删除必须物理删除：game_id/alias_name 是含软删行的物理唯一索引，
// 软删墓碑会占住索引位导致同名游戏重建直接 duplicate-key 500（线上实证
// idx_games_game_id 冲突 SQLSTATE 23505；0024/0026 软删残留同族）。
func TestGameModel_DeleteHardAllowsSameKeyRecreate(t *testing.T) {
	db := setupIsolatedDB(t)
	m := NewGameModel(db)
	ctx := context.Background()

	game := &Game{GameID: "reborn", Name: "reborn", AliasName: "重生回归"}
	require.NoError(t, m.Create(ctx, game))
	require.NoError(t, m.Delete(ctx, game.ID))

	var tombstones int64
	require.NoError(t, db.Unscoped().Model(&Game{}).Where("game_id = ?", "reborn").Count(&tombstones).Error)
	assert.Zero(t, tombstones, "删除后不得残留软删墓碑行")

	// 同 game_id + 同 alias_name 重建必须成功（曾 500 duplicate-key）。
	reborn := &Game{GameID: "reborn", Name: "reborn", AliasName: "重生回归"}
	require.NoError(t, m.Create(ctx, reborn))

	// 事务版删除（带绑定清理）同样物理删除、同样可重建。
	require.NoError(t, m.UpdateEnvsAndBindings(ctx, reborn.GameID, reborn.ID, nil, nil,
		[]GameEnvBinding{{Env: "prod", DatabaseName: "game_reborn_prod"}}))
	require.NoError(t, m.DeleteWithEnvBindings(ctx, reborn.ID, reborn.GameID))
	require.NoError(t, db.Unscoped().Model(&Game{}).Where("game_id = ?", "reborn").Count(&tombstones).Error)
	assert.Zero(t, tombstones)

	onceMore := &Game{GameID: "reborn", Name: "reborn", AliasName: "重生回归"}
	require.NoError(t, m.Create(ctx, onceMore))
}
