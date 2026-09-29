package announcement

// 覆盖率巡检补测（service.go 残余 8 块）：全为「DB 操作失败 → 原样透传」
// 的错误返回，注入口径与 svc 批一致——**缺表**（DropTable 让目标表不存在，
// gorm 立即报错且不产生副作用）。逐块对应：
//
//	List     → announcements 缺表（查询失败）
//	Create   → announcements 缺表（写入失败）
//	Update   → gameIds 非 nil 时绑定替换失败；缺省时绑定批量查询失败
//	Delete   → 公告本体删成功后，确认记录表缺表（级联清理失败）
//	用户侧   → announcement_games 缺表（绑定批量查询失败）
//
// 两个私有 helper（replaceGameBindings / gameIDsByAnnouncement）另作直调
// 断言，避免只经上层间接覆盖。

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dropTable 擦除目标表制造确定性 DB 失败（同包测试，可经 s.db() 取句柄）。
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

// 公告本体表在、绑定表缺：查询与写入各自的后半段失败（列表回填绑定 /
// 创建后写绑定），错误须透传而不是静默按「无绑定」处理。
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

// 公告本体已落库、绑定表缺：列表回填绑定失败须透传（不得静默按「无绑定」
// 返回，把数据损坏伪装成正常列表）。
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

	// 顺带锁定「查无公告时短路」：ids 为空 → 不触绑定查询 → 正常返回。
	// （无公告的库与缺表库不可兼得，此处只验证短路语义本身。）
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

	// gameIds 非 nil → 先走全量替换绑定，绑定表缺表即失败
	item, err := s.Update(ctx, uint(created.ID), &UpdateRequest{GameIds: &gameIDs})
	assert.Error(t, err, "绑定替换失败须透传（表缺）")
	assert.Nil(t, item)

	// gameIds 缺省 → 跳过替换，回读时绑定批量查询撞缺表
	item, err = s.Update(ctx, uint(created.ID), &UpdateRequest{})
	assert.Error(t, err, "回读绑定失败须透传（表缺）")
	assert.Nil(t, item)

	// 用户侧：绑定批量查询同样撞缺表（拉取失败不得静默丢绑定）
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

	// 公告本体删除已成功（RowsAffected=1），级联清理确认记录时撞缺表：
	// 错误须上抛，不得把「已部分删除」当成功。
	assert.Error(t, s.Delete(ctx, uint(created.ID)), "确认记录表缺表时级联清理失败须透传")
}

func TestAnnouncementService_BindingHelpersDirect(t *testing.T) {
	s := newFixture(t)
	ctx := context.Background()
	created, err := s.Create(ctx, &CreateRequest{Title: "helper 公告", ContentMd: "body", Audience: "all"})
	require.NoError(t, err)

	// 绑定表缺表时两个 helper 各自失败，错误不得被吞掉
	dropTable(t, s, &model.AnnouncementGame{})
	assert.Error(t, s.replaceGameBindings(ctx, uint(created.ID), []string{"game-a"}), "先删后插：删即失败")
	assert.Error(t, s.replaceGameBindings(ctx, uint(created.ID), nil), "空绑定列表也先删后失败")

	bindings, err := s.gameIDsByAnnouncement(ctx, []uint{uint(created.ID)})
	assert.Error(t, err, "绑定批量查询失败须透传")
	assert.Nil(t, bindings)

	// 空入参短路：表缺也不报错（无查询即无失败）
	empty, err := s.gameIDsByAnnouncement(ctx, nil)
	require.NoError(t, err, "空 ids 直接短路返回，不触表")
	assert.Empty(t, empty)
}
