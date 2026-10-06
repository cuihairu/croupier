package game

import (
	"strconv"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 服务层回归：删除游戏后同名重建必须成功，而不是 duplicate-key 500
// （线上实证：软删墓碑占住 game_id/alias_name 物理唯一索引，重建报
// SQLSTATE 23505；删除路径已改硬删，0039 迁移清存量墓碑）。
func TestGameDeleteThenRecreateSameName(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)
	svc := NewService(svcCtx)

	ctx := createPermUser(t, db, "perm_recreate", "games:write")

	resp, err := svc.Create(ctx, &GameCreateRequest{Name: "recreate_me", AliasName: "重建回归"})
	require.NoError(t, err)
	firstID := strconv.FormatUint(uint64(resp.Game.ID), 10)
	require.NoError(t, svc.Delete(ctx, &GameDeleteRequest{ID: firstID}))

	// 墓碑行必须物理消失（Unscoped 也查不到）。
	var tombstones int64
	require.NoError(t, db.Unscoped().Model(&model.Game{}).Where("name = ?", "recreate_me").Count(&tombstones).Error)
	assert.Zero(t, tombstones, "删除后不应残留软删墓碑行")

	// 同名 + 同显示名重建（两条唯一索引都被墓碑占过）。
	resp2, err := svc.Create(ctx, &GameCreateRequest{Name: "recreate_me", AliasName: "重建回归"})
	require.NoError(t, err)
	assert.NotEqual(t, resp.Game.ID, resp2.Game.ID)

	// 显示名缺省回填（alias_name = name）路径同样不能踩唯一索引。
	require.NoError(t, svc.Delete(ctx, &GameDeleteRequest{
		ID: strconv.FormatUint(uint64(resp2.Game.ID), 10)}))
	resp3, err := svc.Create(ctx, &GameCreateRequest{Name: "recreate_me"})
	require.NoError(t, err)
	assert.NotZero(t, resp3.Game.ID)
}
