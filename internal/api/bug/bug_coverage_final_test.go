// Covers the remaining reachable branches of the bug package: blank lines in
// fingerprintStack, ReportCrash Create / bump Update failures, and Create/Update model
// write failures.
package bug

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// fingerprintStack: stacks containing blank / whitespace-only lines skip them.
func TestFingerprintStack_SkipsBlankLines(t *testing.T) {
	t.Parallel()

	a := fingerprintStack("lua: /game/bag.lua:1: boom\n\n   \n0x1 bag.lua:2")
	b := fingerprintStack("lua: /game/bag.lua:1: boom\n0x1 bag.lua:2")
	assert.Equal(t, a, b)
	assert.NotEmpty(t, a)
}

func failBugCreate(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("test/fail_bug_create", func(tx *gorm.DB) {
		if _, ok := tx.Statement.Dest.(*model.Bug); ok {
			_ = tx.AddError(errors.New("forced bug create failure"))
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Create().Remove("test/fail_bug_create") })
}

func failBugUpdate(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("test/fail_bug_update", func(tx *gorm.DB) {
		if _, ok := tx.Statement.Model.(*model.Bug); ok {
			_ = tx.AddError(errors.New("forced bug update failure"))
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Update().Remove("test/fail_bug_update") })
}

func crashRequest(stack string) *ReportCrashRequest {
	return &ReportCrashRequest{
		GameID:   "demo",
		Env:      "prod",
		Platform: "ios",
		Stack:    stack,
		Message:  "boom",
	}
}

// ReportCrash: creating a new Bug when no aggregation hits -> the error is passed through.
func TestReportCrash_CreateBugFails(t *testing.T) {
	t.Parallel()

	db := newBugTestDB(t)
	failBugCreate(t, db)
	s := NewService(&svc.ServiceContext{BugModel: model.NewBugModel(db)})

	_, err := s.ReportCrash(context.Background(), crashRequest("lua: /game/bag.lua:1: boom"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forced bug create failure")
}

// ReportCrash: the bump Update after an aggregation hit fails -> the error is passed
// through.
func TestReportCrash_BumpUpdateFails(t *testing.T) {
	t.Parallel()

	db := newBugTestDB(t)
	s := NewService(&svc.ServiceContext{BugModel: model.NewBugModel(db)})
	stack := "lua: /game/bag.lua:1: boom"
	_, err := s.ReportCrash(context.Background(), crashRequest(stack))
	require.NoError(t, err)

	failBugUpdate(t, db)
	_, err = s.ReportCrash(context.Background(), crashRequest("lua: /game/bag.lua:9: boom"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forced bug update failure")
}

// Create: a model write failure -> the error is passed through.
func TestBugService_CreateModelFails(t *testing.T) {
	t.Parallel()

	db := newBugTestDB(t)
	failBugCreate(t, db)
	s := NewService(&svc.ServiceContext{BugModel: model.NewBugModel(db)})

	_, err := s.Create(context.Background(), &BugCreateRequest{Title: "title"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forced bug create failure")
}

// Update: a model update failure -> the error is passed through.
func TestBugService_UpdateModelFails(t *testing.T) {
	t.Parallel()

	db := newBugTestDB(t)
	s := NewService(&svc.ServiceContext{BugModel: model.NewBugModel(db)})
	created, err := s.Create(context.Background(), &BugCreateRequest{Title: "title"})
	require.NoError(t, err)

	failBugUpdate(t, db)
	newStatus := model.BugStatusFixing
	_, err = s.Update(context.Background(), &BugUpdateRequest{ID: fmt.Sprintf("%d", created.Id), Status: &newStatus})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forced bug update failure")
}
