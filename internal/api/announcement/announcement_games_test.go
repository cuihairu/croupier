// 公告↔游戏多对多绑定回归（#45）：一公告可绑定多游戏，未绑定任何
// 游戏 = 全服可见。锁定绑定写路径（create/update 全量替换语义）、
// 管理列表 gameId 过滤、用户侧 active 的 X-Game-ID 过滤。
package announcement

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func gameIDsOf(t *testing.T, s *Service, id uint) []string {
	t.Helper()
	var rows []model.AnnouncementGame
	require.NoError(t, s.db().Where("announcement_id = ?", id).Find(&rows).Error)
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.GameID)
	}
	return out
}

func TestAnnouncementGameBindingsLifecycle(t *testing.T) {
	s := newFixture(t)
	ctx := context.Background()

	// 创建带绑定：入参含空串/重复 → 归一（trim、去重、丢空）
	created, err := s.Create(ctx, &CreateRequest{
		Title: "版本更新", ContentMd: "v2 上线", Audience: "all",
		GameIds: []string{" demo ", "", "battle", "demo"},
	})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"demo", "battle"}, gameIDsOf(t, s, uint(created.ID)))
	assert.Equal(t, []string{"demo", "battle"}, created.GameIds)

	// 未绑定（缺省）= 全服可见
	allSrv, err := s.Create(ctx, &CreateRequest{Title: "全服公告", ContentMd: "hi", Audience: "all"})
	require.NoError(t, err)
	assert.Empty(t, gameIDsOf(t, s, uint(allSrv.ID)))

	// 更新：gameIds 缺省（nil）→ 绑定不动
	kept, err := s.Update(ctx, uint(created.ID), &UpdateRequest{Title: strPtr("版本更新 v2")})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"demo", "battle"}, kept.GameIds)

	// 更新：gameIds=[]（非 nil 空数组）→ 清空绑定 → 全服可见
	cleared, err := s.Update(ctx, uint(created.ID), &UpdateRequest{GameIds: &[]string{}})
	require.NoError(t, err)
	assert.Empty(t, cleared.GameIds)
	assert.Empty(t, gameIDsOf(t, s, uint(created.ID)))

	// 更新：非空数组 → 全量替换
	replaced, err := s.Update(ctx, uint(created.ID), &UpdateRequest{GameIds: &[]string{"rpg"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"rpg"}, replaced.GameIds)
}

func TestAnnouncementListGameFilter(t *testing.T) {
	s := newFixture(t)
	ctx := context.Background()

	all, err := s.Create(ctx, &CreateRequest{Title: "全服", ContentMd: "a", Audience: "all"})
	require.NoError(t, err)
	demoOnly, err := s.Create(ctx, &CreateRequest{Title: "demo 专属", ContentMd: "b", Audience: "all",
		GameIds: []string{"demo"}})
	require.NoError(t, err)
	rpgOnly, err := s.Create(ctx, &CreateRequest{Title: "rpg 专属", ContentMd: "c", Audience: "all",
		GameIds: []string{"rpg"}})
	require.NoError(t, err)
	multi, err := s.Create(ctx, &CreateRequest{Title: "双游戏", ContentMd: "d", Audience: "all",
		GameIds: []string{"demo", "rpg"}})
	require.NoError(t, err)
	_ = multi

	// 不过滤：全部可见，且各项带绑定
	list, err := s.List(ctx, "")
	require.NoError(t, err)
	assert.Len(t, list.Items, 4)

	// gameId=demo：全服 + demo 绑定（含多游戏绑定）
	demoList, err := s.List(ctx, "demo")
	require.NoError(t, err)
	titles := titlesOf(demoList.Items)
	assert.Contains(t, titles, all.Title)
	assert.Contains(t, titles, demoOnly.Title)
	assert.Contains(t, titles, "双游戏")
	assert.NotContains(t, titles, rpgOnly.Title)

	// 无效 gameId：仅全服可见
	none, err := s.List(ctx, "no-such-game")
	require.NoError(t, err)
	require.Len(t, none.Items, 1)
	assert.Equal(t, all.ID, none.Items[0].ID)
}

func TestAnnouncementActiveForUserGameFilter(t *testing.T) {
	s := newFixture(t)
	ctx := context.Background()

	_, err := s.Create(ctx, &CreateRequest{Title: "全服弹窗", ContentMd: "a", Audience: "all", Popup: true})
	require.NoError(t, err)
	_, err = s.Create(ctx, &CreateRequest{Title: "demo 专属", ContentMd: "b", Audience: "all",
		GameIds: []string{"demo"}})
	require.NoError(t, err)

	// 有游戏上下文（X-Game-ID=demo）：全服 + demo
	inGame, err := s.ActiveForUser(ctx, "alice", nil, "demo")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"全服弹窗", "demo 专属"}, titlesOfActive(inGame.Items))

	// 其他游戏：只有全服
	otherGame, err := s.ActiveForUser(ctx, "alice", nil, "rpg")
	require.NoError(t, err)
	assert.Equal(t, []string{"全服弹窗"}, titlesOfActive(otherGame.Items))

	// 无游戏上下文（scope 未选择）：绑定公告不可见（严格语义）
	noGame, err := s.ActiveForUser(ctx, "alice", nil, "")
	require.NoError(t, err)
	assert.Equal(t, []string{"全服弹窗"}, titlesOfActive(noGame.Items))
}

func TestAnnouncementDeleteCascadesGameBindings(t *testing.T) {
	s := newFixture(t)
	ctx := context.Background()

	created, err := s.Create(ctx, &CreateRequest{Title: "待删", ContentMd: "x", Audience: "all",
		GameIds: []string{"demo", "rpg"}})
	require.NoError(t, err)
	require.NoError(t, s.Delete(ctx, uint(created.ID)))
	assert.Empty(t, gameIDsOf(t, s, uint(created.ID)))
}

func titlesOf(items []AdminAnnouncementItem) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.Title)
	}
	return out
}

func titlesOfActive(items []ActiveItem) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.Title)
	}
	return out
}
