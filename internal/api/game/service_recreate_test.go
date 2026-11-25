package game

import (
	"strconv"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Service-layer regression: recreating a game with the same name after deleting it must
// succeed, not hit a duplicate-key 500
// (production evidence: soft-delete tombstones held the game_id/alias_name physical unique
// indexes and recreation failed with SQLSTATE 23505; the delete path was switched to hard
// delete and the 0039 migration cleared pre-existing tombstones).
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

	// The tombstone rows must be gone physically (not even Unscoped finds them).
	var tombstones int64
	require.NoError(t, db.Unscoped().Model(&model.Game{}).Where("name = ?", "recreate_me").Count(&tombstones).Error)
	assert.Zero(t, tombstones, "删除后不应残留软删墓碑行")

	// Recreate with the same name and same display name (both unique indexes had been held
	// by tombstones).
	resp2, err := svc.Create(ctx, &GameCreateRequest{Name: "recreate_me", AliasName: "重建回归"})
	require.NoError(t, err)
	assert.NotEqual(t, resp.Game.ID, resp2.Game.ID)

	// The display-name default backfill path (alias_name = name) must likewise not step on
	// the unique index.
	require.NoError(t, svc.Delete(ctx, &GameDeleteRequest{
		ID: strconv.FormatUint(uint64(resp2.Game.ID), 10)}))
	resp3, err := svc.Create(ctx, &GameCreateRequest{Name: "recreate_me"})
	require.NoError(t, err)
	assert.NotZero(t, resp3.Game.ID)
}
