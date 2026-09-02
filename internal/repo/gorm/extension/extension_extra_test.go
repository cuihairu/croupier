// 覆盖目标：binding 事务失败回滚、各 repo List 的过滤分支与存储错误分支。
package extensiongorm

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBindingRepo_ReplaceForInstallation_Success(t *testing.T) {
	db := setupTestDB(t)
	repo := NewBindingRepo(db)
	ctx := context.Background()

	require.NoError(t, repo.ReplaceForInstallation(ctx, 1, []model.ExtensionRuntimeBinding{
		{BindingType: "source", BindingKey: "cfg", Status: "active"},
		{BindingType: "sink", BindingKey: "evt", Status: "active"},
	}))
	items, err := repo.ListByInstallationID(ctx, 1)
	require.NoError(t, err)
	assert.Len(t, items, 2)

	// 替换后旧绑定被清空
	require.NoError(t, repo.ReplaceForInstallation(ctx, 1, []model.ExtensionRuntimeBinding{
		{BindingType: "source", BindingKey: "cfg2", Status: "active"},
	}))
	items, err = repo.ListByInstallationID(ctx, 1)
	require.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, "cfg2", items[0].BindingKey)
}

func TestBindingRepo_ReplaceForInstallation_CreateConflictRollsBack(t *testing.T) {
	db := setupTestDB(t)
	repo := NewBindingRepo(db)
	ctx := context.Background()

	// 同一 installation 下重复 bindingKey 触发唯一索引冲突 → 事务回滚
	err := repo.ReplaceForInstallation(ctx, 2, []model.ExtensionRuntimeBinding{
		{BindingType: "source", BindingKey: "dup", Status: "active"},
		{BindingType: "sink", BindingKey: "dup", Status: "active"},
	})
	require.Error(t, err)

	// 回滚后不应留下任何记录
	items, listErr := repo.ListByInstallationID(ctx, 2)
	require.NoError(t, listErr)
	assert.Empty(t, items)
}

func TestBindingRepo_ListByInstallationID_StoreError(t *testing.T) {
	db := setupTestDB(t)
	repo := NewBindingRepo(db)
	require.NoError(t, db.Migrator().DropTable("extension_runtime_bindings"))

	_, err := repo.ListByInstallationID(context.Background(), 1)
	require.Error(t, err)
}

func TestCatalogRepo_List_StoreError(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCatalogRepo(db)
	require.NoError(t, db.Migrator().DropTable("extension_catalogs"))

	_, _, err := repo.List(context.Background(), CatalogListQuery{})
	require.Error(t, err)
}

func TestEventRepo_List_FiltersAndError(t *testing.T) {
	db := setupTestDB(t)
	repo := NewEventRepo(db)
	ctx := context.Background()

	require.NoError(t, db.Create(&model.ExtensionEvent{InstallationID: 1, EventType: "install", Level: "info", Message: "m1"}).Error)
	require.NoError(t, db.Create(&model.ExtensionEvent{InstallationID: 2, EventType: "error", Level: "error", Message: "boom happened"}).Error)

	items, total, err := repo.List(ctx, EventListQuery{Level: "error", Keyword: "boom"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	assert.Equal(t, "boom happened", items[0].Message)

	// ListByInstallationID 组合路径
	items2, total2, err := repo.ListByInstallationID(ctx, 1, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total2)
	require.Len(t, items2, 1)

	require.NoError(t, db.Migrator().DropTable("extension_events"))
	_, _, err = repo.List(ctx, EventListQuery{})
	require.Error(t, err)
}

func TestInstallationRepo_List_FiltersAndError(t *testing.T) {
	db := setupTestDB(t)
	repo := NewInstallationRepo(db)
	ctx := context.Background()

	enabled := true
	require.NoError(t, db.Create(&model.ExtensionInstallation{InstallationKey: "k1", ExtensionID: "e1", ReleaseVersion: "1.0", ScopeType: "global", ScopeID: "g", TargetType: "cluster", TargetID: "t1", Status: "active", DesiredState: "installed", Enabled: enabled}).Error)
	require.NoError(t, db.Create(&model.ExtensionInstallation{InstallationKey: "k2", ExtensionID: "e2", ReleaseVersion: "1.0", ScopeType: "game", ScopeID: "demo", TargetType: "cluster", TargetID: "t2", Status: "disabled", DesiredState: "installed"}).Error)

	items, total, err := repo.List(ctx, InstallationListQuery{
		ExtensionID: "e1",
		ScopeType:   "global",
		ScopeID:     "g",
		TargetType:  "cluster",
		TargetID:    "t1",
		Status:      "active",
		Enabled:     &enabled,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	assert.Equal(t, "e1", items[0].ExtensionID)

	// 范围过滤无命中
	_, total2, err := repo.List(ctx, InstallationListQuery{Status: "pending"})
	require.NoError(t, err)
	assert.Equal(t, int64(0), total2)

	require.NoError(t, db.Migrator().DropTable("extension_installations"))
	_, _, err = repo.List(ctx, InstallationListQuery{})
	require.Error(t, err)
}

func TestReleaseRepo_ListByExtensionID_FoundAndError(t *testing.T) {
	db := setupTestDB(t)
	repo := NewReleaseRepo(db)
	ctx := context.Background()

	require.NoError(t, db.Create(&model.ExtensionRelease{ExtensionID: "ext-a", Version: "1.0.0", ReleaseChannel: "stable", ManifestJSON: model.JSON(`{}`)}).Error)
	require.NoError(t, db.Create(&model.ExtensionRelease{ExtensionID: "ext-a", Version: "1.1.0", ReleaseChannel: "stable", ManifestJSON: model.JSON(`{}`)}).Error)

	items, err := repo.ListByExtensionID(ctx, "ext-a")
	require.NoError(t, err)
	assert.Len(t, items, 2)

	_, err = repo.ListByExtensionID(ctx, "missing")
	require.NoError(t, err)

	require.NoError(t, db.Migrator().DropTable("extension_releases"))
	_, err = repo.ListByExtensionID(ctx, "ext-a")
	require.Error(t, err)
}
