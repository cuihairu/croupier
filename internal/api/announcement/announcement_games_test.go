// Announcement <-> game many-to-many binding regression (#45): one
// announcement can bind multiple games; binding no games means visible
// server-wide. Pins down the binding write path (full-replacement semantics
// on create/update), the admin list gameId filter, and the user-side active
// X-Game-ID filter.
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

	// Create with bindings: input contains blanks/duplicates -> normalized
	// (trimmed, deduplicated, empties dropped)
	created, err := s.Create(ctx, &CreateRequest{
		Title: "版本更新", ContentMd: "v2 上线", Audience: "all",
		GameIds: []string{" demo ", "", "battle", "demo"},
	})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"demo", "battle"}, gameIDsOf(t, s, uint(created.ID)))
	assert.Equal(t, []string{"demo", "battle"}, created.GameIds)

	// No bindings (omitted) = visible server-wide
	allSrv, err := s.Create(ctx, &CreateRequest{Title: "全服公告", ContentMd: "hi", Audience: "all"})
	require.NoError(t, err)
	assert.Empty(t, gameIDsOf(t, s, uint(allSrv.ID)))

	// Update: gameIds omitted (nil) -> bindings left untouched
	kept, err := s.Update(ctx, uint(created.ID), &UpdateRequest{Title: strPtr("版本更新 v2")})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"demo", "battle"}, kept.GameIds)

	// Update: gameIds=[] (non-nil empty slice) -> bindings cleared ->
	// visible server-wide
	cleared, err := s.Update(ctx, uint(created.ID), &UpdateRequest{GameIds: &[]string{}})
	require.NoError(t, err)
	assert.Empty(t, cleared.GameIds)
	assert.Empty(t, gameIDsOf(t, s, uint(created.ID)))

	// Update: non-empty slice -> full replacement
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

	// No filter: everything visible, each item carrying its bindings
	list, err := s.List(ctx, "")
	require.NoError(t, err)
	assert.Len(t, list.Items, 4)

	// gameId=demo: server-wide + demo-bound items (including multi-game
	// bindings)
	demoList, err := s.List(ctx, "demo")
	require.NoError(t, err)
	titles := titlesOf(demoList.Items)
	assert.Contains(t, titles, all.Title)
	assert.Contains(t, titles, demoOnly.Title)
	assert.Contains(t, titles, "双游戏")
	assert.NotContains(t, titles, rpgOnly.Title)

	// Invalid gameId: only the server-wide announcement
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

	// With a game context (X-Game-ID=demo): server-wide + demo
	inGame, err := s.ActiveForUser(ctx, "alice", nil, "demo")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"全服弹窗", "demo 专属"}, titlesOfActive(inGame.Items))

	// Another game: only the server-wide announcement
	otherGame, err := s.ActiveForUser(ctx, "alice", nil, "rpg")
	require.NoError(t, err)
	assert.Equal(t, []string{"全服弹窗"}, titlesOfActive(otherGame.Items))

	// No game context (no scope selected): bound announcements are invisible
	// (strict semantics)
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
