package announcement

// Coverage sweep follow-up tests (8 remaining blocks in service.go): all of
// them are "DB operation fails -> error passed through unchanged" returns,
// injected the same way as the svc batch -- **missing table** (DropTable
// makes the target table disappear; gorm errors out immediately with no side
// effects). Block-by-block mapping:
//
//	List     -> announcements missing (query fails)
//	Create   -> announcements missing (write fails)
//	Update   -> binding replacement fails when gameIds is non-nil; binding
//	            batch query fails when omitted
//	Delete   -> read table missing after the announcement itself deletes
//	            successfully (cascade cleanup fails)
//	User side -> announcement_games missing (binding batch query fails)
//
// The two private helpers (replaceGameBindings / gameIDsByAnnouncement) get
// direct-call assertions so they are not covered only indirectly via the
// upper layers.

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dropTable wipes the target table to produce a deterministic DB failure
// (same-package test, so the handle is reachable via s.db()).
func dropTable(t *testing.T, s *Service, model any) {
	t.Helper()
	require.NoError(t, s.db().Migrator().DropTable(model))
}

func TestAnnouncementService_ListAndCreateTableMissing(t *testing.T) {
	s := newFixture(t)
	ctx := context.Background()
	dropTable(t, s, &model.Announcement{})

	list, err := s.List(ctx, "")
	assert.Error(t, err, "announcements 缺表时列表查询失败须透传")
	assert.Nil(t, list)

	created, err := s.Create(ctx, &CreateRequest{Title: "缺表公告", ContentMd: "body", Audience: "all"})
	assert.Error(t, err, "announcements 缺表时创建失败须透传")
	assert.Nil(t, created)
}

// The announcement table exists but the binding table is missing: the second
// half of each path fails (binding backfill on list / binding write after
// create). Errors must pass through instead of being silently treated as
// "no bindings".
func TestAnnouncementService_BindingTableMissingAfterWrite(t *testing.T) {
	s := newFixture(t)
	ctx := context.Background()
	dropTable(t, s, &model.AnnouncementGame{})

	created, err := s.Create(ctx, &CreateRequest{
		Title: "绑定表缺", ContentMd: "body", Audience: "all", GameIds: []string{"game-a"},
	})
	assert.Error(t, err, "公告写入成功但绑定替换失败须透传")
	assert.Nil(t, created)
}

// The announcement itself has landed in the database but the binding table is
// missing: the list binding backfill must pass the error through (never
// silently return "no bindings", disguising data corruption as a normal
// list).
func TestAnnouncementService_ListBindingReadFailure(t *testing.T) {
	s := newFixture(t)
	ctx := context.Background()
	created, err := s.Create(ctx, &CreateRequest{
		Title: "回填失败", ContentMd: "body", Audience: "all", GameIds: []string{"game-a"},
	})
	require.NoError(t, err)
	dropTable(t, s, &model.AnnouncementGame{})

	list, err := s.List(ctx, "")
	assert.Error(t, err, "列表回填绑定失败须透传（绑定表缺）")
	assert.Nil(t, list)

	// Also pin down the "short-circuit when there are no announcements"
	// behavior: empty ids -> binding query never runs -> normal return.
	// (A database with no announcements and a database with a dropped table
	// cannot coexist, so only the short-circuit semantics itself is verified
	// here.)
	empty := newFixture(t)
	emptyList, err := empty.List(ctx, "")
	require.NoError(t, err)
	require.NotNil(t, emptyList)
	assert.Empty(t, emptyList.Items)
	assert.Zero(t, emptyList.Total)
	assert.NotZero(t, created.ID)
}

func TestAnnouncementService_UpdateBindingAndReadPaths(t *testing.T) {
	s := newFixture(t)
	ctx := context.Background()
	gameIDs := []string{"game-a"}

	created, err := s.Create(ctx, &CreateRequest{
		Title: "绑定公告", ContentMd: "body", Audience: "all", GameIds: gameIDs,
	})
	require.NoError(t, err)
	dropTable(t, s, &model.AnnouncementGame{})

	// gameIds non-nil -> full binding replacement runs first and fails since
	// the binding table is missing
	item, err := s.Update(ctx, uint(created.ID), &UpdateRequest{GameIds: &gameIDs})
	assert.Error(t, err, "绑定替换失败须透传（表缺）")
	assert.Nil(t, item)

	// gameIds omitted -> replacement skipped, the read-back binding batch
	// query hits the missing table
	item, err = s.Update(ctx, uint(created.ID), &UpdateRequest{})
	assert.Error(t, err, "回读绑定失败须透传（表缺）")
	assert.Nil(t, item)

	// User side: the binding batch query hits the missing table too (a fetch
	// failure must not silently drop bindings)
	resp, err := s.ActiveForUser(ctx, "alice", nil, "")
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestAnnouncementService_DeleteCascadeReadTableMissing(t *testing.T) {
	s := newFixture(t)
	ctx := context.Background()
	created, err := s.Create(ctx, &CreateRequest{Title: "待删公告", ContentMd: "body", Audience: "all"})
	require.NoError(t, err)
	dropTable(t, s, &model.AnnouncementRead{})

	// The announcement itself was deleted successfully (RowsAffected=1), then
	// the cascade cleanup of read records hits the missing table: the error
	// must propagate, never treating "partially deleted" as success.
	assert.Error(t, s.Delete(ctx, uint(created.ID)), "确认记录表缺表时级联清理失败须透传")
}

func TestAnnouncementService_BindingHelpersDirect(t *testing.T) {
	s := newFixture(t)
	ctx := context.Background()
	created, err := s.Create(ctx, &CreateRequest{Title: "helper 公告", ContentMd: "body", Audience: "all"})
	require.NoError(t, err)

	// With the binding table missing, both helpers fail on their own and the
	// errors must not be swallowed
	dropTable(t, s, &model.AnnouncementGame{})
	assert.Error(t, s.replaceGameBindings(ctx, uint(created.ID), []string{"game-a"}), "先删后插：删即失败")
	assert.Error(t, s.replaceGameBindings(ctx, uint(created.ID), nil), "空绑定列表也先删后失败")

	bindings, err := s.gameIDsByAnnouncement(ctx, []uint{uint(created.ID)})
	assert.Error(t, err, "绑定批量查询失败须透传")
	assert.Nil(t, bindings)

	// Empty input short-circuits: no error even with the table missing (no
	// query means no failure)
	empty, err := s.gameIDsByAnnouncement(ctx, nil)
	require.NoError(t, err, "空 ids 直接短路返回，不触表")
	assert.Empty(t, empty)
}
