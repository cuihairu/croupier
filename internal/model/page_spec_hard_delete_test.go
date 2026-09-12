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

// 软删除 + 物理唯一索引回归：删除路径改硬删（Unscoped）后，同 key 重建
// 必须成功且不留残留行。软删行占着唯一索引位会让重建 INSERT 直接
// duplicate-key 500（线上 DELETE /versioning/pages/:key 后重建实证过）。

var hardDeleteSeq int

func setupHardDeleteDB(t *testing.T) *gorm.DB {
	t.Helper()
	hardDeleteSeq++
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:harddel%d?mode=memory&cache=shared", hardDeleteSeq)),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&PageSpec{}, &PublishedPageSpec{}, &PageProposal{}))
	return db
}

// TestPageSpecDeleteThenRecreateSameKey 草稿页：Delete 后同 (game,env,pageKey)
// 重建 Upsert 必须成功，库内不留软删残留。
func TestPageSpecDeleteThenRecreateSameKey(t *testing.T) {
	db := setupHardDeleteDB(t)
	m := NewPageSpecModel(db)
	ctx := context.Background()

	first := &PageSpec{GameID: "demo", Env: "dev", PageKey: "hard-del-page", Type: "composite", SpecJSON: "{}"}
	require.NoError(t, m.Upsert(ctx, first))
	require.NoError(t, m.Delete(ctx, "demo", "dev", "hard-del-page"))

	reborn := &PageSpec{GameID: "demo", Env: "dev", PageKey: "hard-del-page", Type: "composite", SpecJSON: `{"v":2}`}
	require.NoError(t, m.Upsert(ctx, reborn), "recreating the same pageKey after delete must not hit the unique index")

	var rawCount int64
	require.NoError(t, db.Unscoped().Model(&PageSpec{}).Where("page_key = ?", "hard-del-page").Count(&rawCount).Error)
	assert.Equal(t, int64(1), rawCount, "soft-deleted residue must not linger")
}

// TestPublishedPageSpecDeleteThenRepublishSameVersion 发布页：
// DeleteByScopeAndPageKey 后同 (game,env,pageKey,version) 重新 Create 必须
// 成功——发布链的 unique 索引含 version。PublishedPageSpec 无 DeletedAt
// （删除天然是硬删），本用例锁死该边界，防止未来加软删字段引入占位回归。
func TestPublishedPageSpecDeleteThenRepublishSameVersion(t *testing.T) {
	db := setupHardDeleteDB(t)
	m := NewPublishedPageSpecModel(db)
	ctx := context.Background()

	v1 := &PublishedPageSpec{
		GameID: "demo", Env: "dev", PageKey: "hard-del-page", Version: 1,
		SpecJSON: "{}", RendererSchemaVersion: "page-spec:1",
	}
	require.NoError(t, m.Create(ctx, v1))
	require.NoError(t, m.DeleteByScopeAndPageKey(ctx, "demo", "dev", "hard-del-page"))

	reborn := &PublishedPageSpec{
		GameID: "demo", Env: "dev", PageKey: "hard-del-page", Version: 1,
		SpecJSON: `{"v":2}`, RendererSchemaVersion: "page-spec:1",
	}
	require.NoError(t, m.Create(ctx, reborn), "republishing the same pageKey+version after delete must not hit the unique index")

	var rawCount int64
	require.NoError(t, db.Unscoped().Model(&PublishedPageSpec{}).Where("page_key = ?", "hard-del-page").Count(&rawCount).Error)
	assert.Equal(t, int64(1), rawCount, "soft-deleted residue must not linger")
}

// TestPageProposalDeleteThenRecreate 提案：两种删除入口（按 proposalKey /
// 按 pageKey）之后同 key 重建 UpsertProposal 必须成功且不留残留。
func TestPageProposalDeleteThenRecreate(t *testing.T) {
	db := setupHardDeleteDB(t)
	m := NewPageProposalModel(db)
	ctx := context.Background()

	first := &PageProposal{
		GameID: "demo", Env: "dev", ProposalKey: "composite--p1",
		PageKey: "p1", PageType: "composite", Quality: "ready",
	}
	require.NoError(t, m.UpsertProposal(ctx, first))
	require.NoError(t, m.DeleteByScopeAndKey(ctx, "demo", "dev", "composite--p1"))

	reborn := &PageProposal{
		GameID: "demo", Env: "dev", ProposalKey: "composite--p1",
		PageKey: "p1", PageType: "composite", Quality: "ready",
	}
	require.NoError(t, m.UpsertProposal(ctx, reborn), "recreating the same proposalKey after delete must not hit the unique index")

	var rawCount int64
	require.NoError(t, db.Unscoped().Model(&PageProposal{}).Where("proposal_key = ?", "composite--p1").Count(&rawCount).Error)
	assert.Equal(t, int64(1), rawCount, "soft-deleted residue must not linger")

	// 按 pageKey 清理入口同样不留残留、可重建。
	require.NoError(t, m.DeleteByScopeAndPageKey(ctx, "demo", "dev", "p1"))
	again := &PageProposal{
		GameID: "demo", Env: "dev", ProposalKey: "composite--p1",
		PageKey: "p1", PageType: "composite", Quality: "ready",
	}
	require.NoError(t, m.UpsertProposal(ctx, again))
	require.NoError(t, db.Unscoped().Model(&PageProposal{}).Where("proposal_key = ?", "composite--p1").Count(&rawCount).Error)
	assert.Equal(t, int64(1), rawCount)
}
