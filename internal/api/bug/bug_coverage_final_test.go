// 补齐 bug 包剩余可覆盖分支：fingerprintStack 空行、ReportCrash 的 Create/
// bump Update 失败、Create/Update 的模型写入失败。
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

// fingerprintStack：含空行/纯空白行的堆栈被跳过。
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
	t.Cleanup(func() { db.Callback().Create().Remove("test/fail_bug_create") })
}

func failBugUpdate(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("test/fail_bug_update", func(tx *gorm.DB) {
		if _, ok := tx.Statement.Model.(*model.Bug); ok {
			_ = tx.AddError(errors.New("forced bug update failure"))
		}
	}))
	t.Cleanup(func() { db.Callback().Update().Remove("test/fail_bug_update") })
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

// ReportCrash：无聚合命中时新建 Bug 失败 → 透传错误。
func TestReportCrash_CreateBugFails(t *testing.T) {
	t.Parallel()

	db := newBugTestDB(t)
	failBugCreate(t, db)
	s := NewService(&svc.ServiceContext{BugModel: model.NewBugModel(db)})

	_, err := s.ReportCrash(context.Background(), crashRequest("lua: /game/bag.lua:1: boom"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forced bug create failure")
}

// ReportCrash：命中聚合后 bump 的 Update 失败 → 透传错误。
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

// Create：模型写入失败 → 透传错误。
func TestBugService_CreateModelFails(t *testing.T) {
	t.Parallel()

	db := newBugTestDB(t)
	failBugCreate(t, db)
	s := NewService(&svc.ServiceContext{BugModel: model.NewBugModel(db)})

	_, err := s.Create(context.Background(), &BugCreateRequest{Title: "title"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forced bug create failure")
}

// Update：模型更新失败 → 透传错误。
func TestBugService_UpdateModelFails(t *testing.T) {
	t.Parallel()

	db := newBugTestDB(t)
	s := NewService(&svc.ServiceContext{BugModel: model.NewBugModel(db)})
	created, err := s.Create(context.Background(), &BugCreateRequest{Title: "title"})
	require.NoError(t, err)

	failBugUpdate(t, db)
	newStatus := model.BugStatusFixing
	_, err = s.Update(context.Background(), &BugUpdateRequest{ID: fmt.Sprintf("%d", created.Bug.Id), Status: &newStatus})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forced bug update failure")
}
