package extensiongorm

import (
	"context"
	"errors"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// 事务 Begin 失败（底层连接池已关闭）必须直接返回错误。
func TestBindingRepo_ReplaceForInstallation_BeginError(t *testing.T) {
	db := setupTestDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	repo := NewBindingRepo(db)
	err = repo.ReplaceForInstallation(context.Background(), 1, []model.ExtensionRuntimeBinding{
		{BindingKey: "k1", TargetRef: "t1"},
	})
	require.Error(t, err)
}

// Count 成功但 Find 失败：通过 query 回调按 Dest 类型注入错误，验证
// List 的 Find 错误传播（catalog）。
func TestCatalogRepo_List_FindError(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.Create(&model.ExtensionCatalog{ExtensionID: "e1", Name: "n", Kind: "source", Status: "active"}).Error)
	require.NoError(t, db.Callback().Query().After("gorm:query").Register("test:catalog_find_fail", func(tx *gorm.DB) {
		if _, ok := tx.Statement.Dest.(*[]model.ExtensionCatalog); ok {
			_ = tx.AddError(errors.New("catalog find boom"))
		}
	}))

	repo := NewCatalogRepo(db)
	items, total, err := repo.List(context.Background(), CatalogListQuery{})
	require.Error(t, err)
	assert.Nil(t, items)
	assert.Equal(t, int64(0), total)
	assert.Contains(t, err.Error(), "catalog find boom")
}

func TestEventRepo_List_FindError(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.Create(&model.ExtensionEvent{EventType: "reconcile", Level: "info", Message: "m"}).Error)
	require.NoError(t, db.Callback().Query().After("gorm:query").Register("test:event_find_fail", func(tx *gorm.DB) {
		if _, ok := tx.Statement.Dest.(*[]model.ExtensionEvent); ok {
			_ = tx.AddError(errors.New("event find boom"))
		}
	}))

	repo := NewEventRepo(db)
	items, total, err := repo.List(context.Background(), EventListQuery{})
	require.Error(t, err)
	assert.Nil(t, items)
	assert.Equal(t, int64(0), total)
}

func TestInstallationRepo_List_FindError(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.Create(&model.ExtensionInstallation{InstallationKey: "k", ExtensionID: "e", Status: "installed"}).Error)
	require.NoError(t, db.Callback().Query().After("gorm:query").Register("test:installation_find_fail", func(tx *gorm.DB) {
		if _, ok := tx.Statement.Dest.(*[]model.ExtensionInstallation); ok {
			_ = tx.AddError(errors.New("installation find boom"))
		}
	}))

	repo := NewInstallationRepo(db)
	items, total, err := repo.List(context.Background(), InstallationListQuery{})
	require.Error(t, err)
	assert.Nil(t, items)
	assert.Equal(t, int64(0), total)
}
