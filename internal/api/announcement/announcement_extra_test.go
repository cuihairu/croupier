// Fill out remaining announcement branches: field-level Create/Update
// updates, List success, readSet/FirstOrCreate failures, audience filtering,
// and Active error branches.
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

	// Create with explicit Active=false (covers the req.Active != nil branch).
	// Known quirk: the model's gorm default:true makes the zero value false
	// get overridden by the DB default; that behavior is out of scope here
	// (the create-as-inactive flow is currently unusable, backlog).
	created, err := s.Create(ctx, &CreateRequest{Title: "t", ContentMd: "c", Audience: "all", Active: boolPtr(false)})
	require.NoError(t, err)
	id := uint(created.ID)

	// List success path (previously only the error branch was covered)
	list, listErr := s.List(ctx, "")
	require.NoError(t, listErr)
	require.NotEmpty(t, list.Items)

	// Update: field-level update of title + content_md + end_at
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

	// Update: audience stays role but the new role becomes an empty string
	// after rename -> validation fails
	roleAnn, err := s.Create(ctx, &CreateRequest{Title: "r", ContentMd: "c", Audience: "role", Role: "ops"})
	require.NoError(t, err)
	_, err = s.Update(ctx, uint(roleAnn.ID), &UpdateRequest{Role: strPtr("  ")})
	assert.Error(t, err)
}

// Audience filtering: a role announcement is invisible to users without the
// role and visible to users whose roles match.
func TestAnnouncementVisibleToRoleFiltering(t *testing.T) {
	s := newFixture(t)
	ctx := context.Background()
	_, err := s.Create(ctx, &CreateRequest{Title: "ops-only", ContentMd: "c", Audience: "role", Role: "ops"})
	require.NoError(t, err)

	resp, err := s.ActiveForUser(ctx, "bob", []string{"player"}, "")
	require.NoError(t, err)
	assert.Empty(t, resp.Items, "未命中角色不应可见")

	resp, err = s.ActiveForUser(ctx, "alice", []string{"ops"}, "")
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
}

// FirstOrCreate failure branches of readSetOf / Dismiss (only the reads
// table is missing).
func TestAnnouncementReadsTableFailure(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Announcement{}, &model.AnnouncementRead{}, &model.AnnouncementGame{}))
	s := NewService(&svc.ServiceContext{DB: db})
	ctx := context.Background()

	created, err := s.Create(ctx, &CreateRequest{Title: "t", ContentMd: "c", Audience: "all"})
	require.NoError(t, err)

	// Normal dismiss (FirstOrCreate takes the Create branch)
	_, err = s.Dismiss(ctx, "bob", uint(created.ID))
	require.NoError(t, err)
	// Idempotent: a second dismiss takes the Found branch
	_, err = s.Dismiss(ctx, "bob", uint(created.ID))
	require.NoError(t, err)

	// Missing reads table: readSetOf and Dismiss take the error branches
	require.NoError(t, db.Migrator().DropTable("announcement_reads"))
	_, err = s.ActiveForUser(ctx, "bob", nil, "")
	assert.Error(t, err)
	_, err = s.Dismiss(ctx, "bob", uint(created.ID))
	assert.Error(t, err)
}
