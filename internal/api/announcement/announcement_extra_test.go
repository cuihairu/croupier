// 补齐 announcement 分支：Create/Update 字段级更新、List 成功、
// readSet/FirstOrCreate 故障、受众过滤、Active 错误分支。
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

func TestAnnouncementFieldLevelUpdatesAndList(t *testing.T) {
	s := newFixture(t)
	ctx := context.Background()

	// Create 显式 Active=false（覆盖 req.Active != nil 分支）。
	// 已知怪癖：model gorm default:true 使零值 false 被 DB 默认值覆盖，
	// 该行为不在本次测试范围（创建即下线场景当前不可用，backlog）。
	created, err := s.Create(ctx, &CreateRequest{Title: "t", ContentMd: "c", Audience: "all", Active: boolPtr(false)})
	require.NoError(t, err)
	id := uint(created.ID)

	// List 成功路径（此前仅覆盖错误分支）
	list, listErr := s.List(ctx)
	require.NoError(t, listErr)
	require.NotEmpty(t, list.Items)

	// Update：title + content_md + end_at 字段级更新
	end := time.Now().Add(2 * time.Hour)
	upd, err := s.Update(ctx, id, &UpdateRequest{
		Title:     strPtr("新标题"),
		ContentMd: strPtr("**new**"),
		EndAt:     &end,
	})
	require.NoError(t, err)
	assert.Equal(t, "新标题", upd.Title)
	assert.Equal(t, "**new**", upd.ContentMd)
	require.NotNil(t, upd.EndAt)

	// Update：role 换名但沿用的 audience=role 且新 role 为空串 → 校验失败
	roleAnn, err := s.Create(ctx, &CreateRequest{Title: "r", ContentMd: "c", Audience: "role", Role: "ops"})
	require.NoError(t, err)
	_, err = s.Update(ctx, uint(roleAnn.ID), &UpdateRequest{Role: strPtr("  ")})
	assert.Error(t, err)
}

// 受众过滤：role 公告对无角色用户不可见，对命中角色用户可见。
func TestAnnouncementVisibleToRoleFiltering(t *testing.T) {
	s := newFixture(t)
	ctx := context.Background()
	_, err := s.Create(ctx, &CreateRequest{Title: "ops-only", ContentMd: "c", Audience: "role", Role: "ops"})
	require.NoError(t, err)

	resp, err := s.ActiveForUser(ctx, "bob", []string{"player"})
	require.NoError(t, err)
	assert.Empty(t, resp.Items, "未命中角色不应可见")

	resp, err = s.ActiveForUser(ctx, "alice", []string{"ops"})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
}

// readSetOf / Dismiss 的 FirstOrCreate 故障分支（只缺 reads 表）。
func TestAnnouncementReadsTableFailure(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Announcement{}, &model.AnnouncementRead{}))
	s := NewService(&svc.ServiceContext{DB: db})
	ctx := context.Background()

	created, err := s.Create(ctx, &CreateRequest{Title: "t", ContentMd: "c", Audience: "all"})
	require.NoError(t, err)

	// 正常 dismiss（FirstOrCreate 走 Create 分支）
	_, err = s.Dismiss(ctx, "bob", uint(created.ID))
	require.NoError(t, err)
	// 幂等：再次 dismiss 走 Found 分支
	_, err = s.Dismiss(ctx, "bob", uint(created.ID))
	require.NoError(t, err)

	// 缺 reads 表：readSetOf 与 Dismiss 走错误分支
	require.NoError(t, db.Migrator().DropTable("announcement_reads"))
	_, err = s.ActiveForUser(ctx, "bob", nil)
	assert.Error(t, err)
	_, err = s.Dismiss(ctx, "bob", uint(created.ID))
	assert.Error(t, err)
}
