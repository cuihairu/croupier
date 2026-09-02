package announcement

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newFixture(t *testing.T) *Service {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Announcement{}, &model.AnnouncementRead{}))
	return NewService(&svc.ServiceContext{DB: db})
}

// 生命周期：创建（默认生效）→ 按受众可见 → 弹窗直至确认 → 下线不可见。
func TestAnnouncementLifecycle(t *testing.T) {
	s := newFixture(t)
	ctx := context.Background()

	created, err := s.Create(ctx, &CreateRequest{
		Title: "维护公告", ContentMd: "# 停服维护\n今晚 22:00-24:00", Audience: "all", Popup: true,
	})
	require.NoError(t, err)
	assert.True(t, created.Active)

	roleA, err := s.Create(ctx, &CreateRequest{
		Title: "管理员专属", ContentMd: "仅 admin", Audience: "role", Role: "admin", Popup: false,
	})
	require.NoError(t, err)

	// 全员用户：可见维护公告（shouldPopup=未确认），不可见角色公告
	resp, err := s.ActiveForUser(ctx, "alice", nil)
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, created.ID, resp.Items[0].ID)
	assert.True(t, resp.Items[0].ShouldPopup, "未确认前应弹窗")

	// admin 角色用户：两条都可见
	resp, err = s.ActiveForUser(ctx, "bob", []string{"admin"})
	require.NoError(t, err)
	assert.Len(t, resp.Items, 2)

	// 确认后不再弹（列表仍可见）
	_, err = s.Dismiss(ctx, "alice", uint(created.ID))
	require.NoError(t, err)
	_, err = s.Dismiss(ctx, "alice", uint(created.ID)) // 幂等
	require.NoError(t, err)
	resp, err = s.ActiveForUser(ctx, "alice", nil)
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.False(t, resp.Items[0].ShouldPopup, "确认后不再弹窗")

	// 下线后全员不可见
	off := false
	_, err = s.Update(ctx, uint(created.ID), &UpdateRequest{Active: &off})
	require.NoError(t, err)
	resp, err = s.ActiveForUser(ctx, "bob", []string{"admin"})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, roleA.ID, resp.Items[0].ID)

	// 删除 + 确认记录级联清理
	require.NoError(t, s.Delete(ctx, uint(roleA.ID)))
	resp, err = s.ActiveForUser(ctx, "bob", []string{"admin"})
	require.NoError(t, err)
	assert.Empty(t, resp.Items)
}

// 发布窗口：未开始不可见，已结束不可见。
func TestAnnouncementWindow(t *testing.T) {
	s := newFixture(t)
	ctx := context.Background()

	past := time.Now().Add(-2 * time.Hour)
	pastEnd := time.Now().Add(-1 * time.Hour)
	future := time.Now().Add(1 * time.Hour)

	ended, err := s.Create(ctx, &CreateRequest{Title: "已结束", ContentMd: "x", StartAt: &past, EndAt: &pastEnd})
	require.NoError(t, err)
	pending, err := s.Create(ctx, &CreateRequest{Title: "未开始", ContentMd: "x", StartAt: &future})
	require.NoError(t, err)

	resp, err := s.ActiveForUser(ctx, "alice", nil)
	require.NoError(t, err)
	for _, item := range resp.Items {
		assert.NotEqual(t, ended.ID, item.ID, "已结束公告不可见")
		assert.NotEqual(t, pending.ID, item.ID, "未开始公告不可见")
	}
}

// 受众校验：audience=role 缺 role 拒绝；非法 audience 拒绝。
func TestAnnouncementAudienceValidation(t *testing.T) {
	s := newFixture(t)
	ctx := context.Background()

	_, err := s.Create(ctx, &CreateRequest{Title: "t", ContentMd: "c", Audience: "role"})
	assert.ErrorContains(t, err, "role")

	_, err = s.Create(ctx, &CreateRequest{Title: "t", ContentMd: "c", Audience: "bogus"})
	assert.ErrorContains(t, err, "all 或 role")
}
