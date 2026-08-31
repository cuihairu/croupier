package model

import (
	"context"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var gameRelSeq int

func setupGameRelDB(t *testing.T) *gorm.DB {
	t.Helper()
	gameRelSeq++
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:grl%d?mode=memory&cache=shared", gameRelSeq)),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&GameRelease{}))
	return db
}

func seedGameRelease(t *testing.T, db *gorm.DB, status, version string) *GameRelease {
	t.Helper()
	rel := &GameRelease{
		GameID: "demo", Env: "prod", Channel: "official", Platform: "android",
		Version: version, Type: ReleaseTypeFull, Status: status,
		GrayPercent: 0, GraySeed: "seed", ObjectKey: "key-" + version,
	}
	require.NoError(t, db.Create(rel).Error)
	return rel
}

func TestGameReleaseModel_ListFilters(t *testing.T) {
	db := setupGameRelDB(t)
	m := NewGameReleaseModel(db)
	ctx := context.Background()

	seedGameRelease(t, db, "full", "1.0.0")
	seedGameRelease(t, db, "gray", "1.1.0")
	seedGameRelease(t, db, "archived", "0.9.0")

	all, total, err := m.List(ctx, ReleaseQueryOptions{GameID: "demo", Env: "prod", Channel: "official", Platform: "android", PaginationOptions: PaginationOptions{Page: 1, PageSize: 10}})
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, all, 3)

	// 状态过滤
	grayOnly, total, err := m.List(ctx, ReleaseQueryOptions{GameID: "demo", Env: "prod", Channel: "official", Platform: "android", Status: "gray", PaginationOptions: PaginationOptions{Page: 1, PageSize: 10}})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, grayOnly, 1)
	assert.Equal(t, "gray", grayOnly[0].Status)
}

func TestGameReleaseModel_FindOneAndByVersion(t *testing.T) {
	db := setupGameRelDB(t)
	m := NewGameReleaseModel(db)
	ctx := context.Background()

	rel := seedGameRelease(t, db, "full", "2.0.0")

	got, err := m.FindOne(ctx, rel.ID)
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", got.Version)

	_, err = m.FindOne(ctx, 999)
	assert.Error(t, err)

	byV, err := m.FindByVersion(ctx, "demo", "prod", "official", "android", "2.0.0")
	require.NoError(t, err)
	assert.Equal(t, rel.ID, byV.ID)

	_, err = m.FindByVersion(ctx, "demo", "prod", "official", "android", "9.9.9")
	assert.Error(t, err)
}

func TestGameReleaseModel_TransitionSingleFull(t *testing.T) {
	db := setupGameRelDB(t)
	m := NewGameReleaseModel(db)
	ctx := context.Background()

	full1 := seedGameRelease(t, db, "testing", "1.0.0")
	require.NoError(t, db.Model(full1).Update("status", "gray").Error)
	upgraded, err := m.Transition(ctx, full1.ID, ReleaseStatusFull, nil)
	require.NoError(t, err)
	assert.Equal(t, ReleaseStatusFull, upgraded.Status)

	// 再推一个 full：旧的 full 应降为 archived
	r2 := seedGameRelease(t, db, "testing", "2.0.0")
	require.NoError(t, db.Model(r2).Update("status", "gray").Error)
	upgraded2, err := m.Transition(ctx, r2.ID, ReleaseStatusFull, nil)
	require.NoError(t, err)
	assert.Equal(t, ReleaseStatusFull, upgraded2.Status)

	// 旧的 1.0.0 已被降级
	old, err := m.FindOne(ctx, full1.ID)
	require.NoError(t, err)
	assert.Equal(t, ReleaseStatusArchived, old.Status)
}

func TestGameReleaseModel_FindCandidates(t *testing.T) {
	db := setupGameRelDB(t)
	m := NewGameReleaseModel(db)
	ctx := context.Background()

	seedGameRelease(t, db, "gray", "2.0.0")
	seedGameRelease(t, db, "full", "1.0.0")
	seedGameRelease(t, db, "archived", "0.5.0")

	q := CheckUpdateQuery{GameID: "demo", Env: "prod", Channel: "official", Platform: "android"}
	cands, err := m.FindCandidates(ctx, q)
	require.NoError(t, err)
	assert.NotEmpty(t, cands) // gray/full 都应入选
}
