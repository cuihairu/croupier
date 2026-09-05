package extensiongorm

import (
	"context"
	"fmt"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 各 List 的 Offset 分支。
func TestRepoListsWithOffset(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		require.NoError(t, db.Create(&model.ExtensionCatalog{
			ExtensionID: fmt.Sprintf("ext-catalog-offset-%d", i), Name: "n", LatestVersion: "1.0.0",
			DisplayName: "N", Kind: "tool", Status: "active",
		}).Error)
	}
	catalog := NewCatalogRepo(db)
	items, total, err := catalog.List(ctx, CatalogListQuery{Limit: 2, Offset: 1})
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, items, 2)

	events := NewEventRepo(db)
	require.NoError(t, events.Create(ctx, &model.ExtensionEvent{
		InstallationID: 1, EventType: "install", Level: "info", Message: "m",
	}))
	evItems, evTotal, err := events.List(ctx, EventListQuery{InstallationID: 1, Limit: 1, Offset: 1})
	require.NoError(t, err)
	assert.Equal(t, int64(1), evTotal)
	assert.Empty(t, evItems)

	installs := NewInstallationRepo(db)
	require.NoError(t, installs.Create(ctx, &model.ExtensionInstallation{
		InstallationKey: "k-offset", ExtensionID: "ext-offset", ReleaseVersion: "1.0.0",
		ScopeType: "global", ScopeID: "default", TargetType: "server", TargetID: "default",
		Status: "installed", DesiredState: "disabled",
	}))
	inItems, inTotal, err := installs.List(ctx, InstallationListQuery{Limit: 1, Offset: 1})
	require.NoError(t, err)
	assert.Equal(t, int64(1), inTotal)
	assert.Empty(t, inItems)
}

// ReplaceForInstallation：删除阶段失败 → 回滚并返回错误。
func TestBindingRepoReplaceDeleteError(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.Migrator().DropTable(&model.ExtensionRuntimeBinding{}))
	repo := NewBindingRepo(db)

	err := repo.ReplaceForInstallation(context.Background(), 1, []model.ExtensionRuntimeBinding{})
	require.Error(t, err)
}
