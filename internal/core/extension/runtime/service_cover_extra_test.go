package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	extensiongorm "github.com/cuihairu/croupier/internal/repo/gorm/extension"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func mustCreateInstallation(t *testing.T, db *gorm.DB) uint {
	t.Helper()
	item := &model.ExtensionInstallation{ExtensionID: "official.analytics", Status: ""}
	require.NoError(t, db.Create(item).Error)
	return item.ID
}

// Save（UPDATE）失败：状态回填持久化报错必须返回错误。
func TestReconcile_SaveError(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("test:update_fail", func(tx *gorm.DB) {
		_ = tx.AddError(errors.New("save boom"))
	}))
	id := mustCreateInstallation(t, db)

	svc := NewService(extensiongorm.NewInstallationRepo(db), extensiongorm.NewBindingRepo(db), extensiongorm.NewEventRepo(db))
	res, err := svc.Reconcile(context.Background(), id)
	require.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "save boom")
}

// ReplaceForInstallation 失败：事务内 binding Create 被注入错误
// （Save 走 UPDATE 回调不受影响）。
func TestReconcile_ReplaceBindingsError(t *testing.T) {
	db := setupTestDB(t)
	id := mustCreateInstallation(t, db)
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("test:create_fail", func(tx *gorm.DB) {
		_ = tx.AddError(errors.New("binding boom"))
	}))

	svc := NewService(extensiongorm.NewInstallationRepo(db), extensiongorm.NewBindingRepo(db), extensiongorm.NewEventRepo(db))
	res, err := svc.Reconcile(context.Background(), id)
	require.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "binding boom")
}
