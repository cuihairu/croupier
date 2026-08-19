package catalog

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	extensiongorm "github.com/cuihairu/croupier/internal/repo/gorm/extension"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupCatalogDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open("file::memory:?mode=memory"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.ExtensionCatalog{},
		&model.ExtensionRelease{},
	))
	return db
}

func newCatalogService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db := setupCatalogDB(t)
	return NewService(extensiongorm.NewCatalogRepo(db), extensiongorm.NewReleaseRepo(db)), db
}

func TestService_List_WithData(t *testing.T) {
	svc, db := newCatalogService(t)
	ctx := context.Background()

	require.NoError(t, db.Create(&model.ExtensionCatalog{ExtensionID: "ext1", Name: "Ext One", Kind: "source", Status: "active"}).Error)
	require.NoError(t, db.Create(&model.ExtensionCatalog{ExtensionID: "ext2", Name: "Ext Two", Kind: "validator", Status: "active"}).Error)

	t.Run("list all", func(t *testing.T) {
		items, total, err := svc.List(ctx, ListQuery{})
		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
		assert.Len(t, items, 2)
	})

	t.Run("filter kind", func(t *testing.T) {
		items, total, err := svc.List(ctx, ListQuery{Kind: "source"})
		require.NoError(t, err)
		assert.Equal(t, int64(1), total)
		assert.Equal(t, "ext1", items[0].ExtensionID)
	})

	t.Run("filter keyword", func(t *testing.T) {
		items, total, err := svc.List(ctx, ListQuery{Keyword: "Two"})
		require.NoError(t, err)
		assert.Equal(t, int64(1), total)
		assert.Equal(t, "ext2", items[0].ExtensionID)
	})

	t.Run("filter status", func(t *testing.T) {
		items, total, err := svc.List(ctx, ListQuery{Status: "active"})
		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
		assert.Len(t, items, 2)
	})

	t.Run("with limit", func(t *testing.T) {
		items, total, err := svc.List(ctx, ListQuery{Limit: 1})
		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
		assert.Len(t, items, 1)
	})
}

func TestService_List_RepoError(t *testing.T) {
	svc, db := newCatalogService(t)
	require.NoError(t, db.Migrator().DropTable(&model.ExtensionCatalog{}))

	_, _, err := svc.List(context.Background(), ListQuery{})
	require.Error(t, err)
}

func TestService_Get(t *testing.T) {
	svc, db := newCatalogService(t)
	ctx := context.Background()

	require.NoError(t, db.Create(&model.ExtensionCatalog{ExtensionID: "ext1", Name: "Ext One", Kind: "source", Status: "active"}).Error)
	require.NoError(t, db.Create(&model.ExtensionRelease{ExtensionID: "ext1", Version: "1.0.0", ManifestJSON: []byte("{}")}).Error)
	require.NoError(t, db.Create(&model.ExtensionRelease{ExtensionID: "ext1", Version: "2.0.0", ManifestJSON: []byte("{}")}).Error)

	item, releases, err := svc.Get(ctx, "ext1")
	require.NoError(t, err)
	require.NotNil(t, item)
	assert.Equal(t, "Ext One", item.Name)
	assert.Len(t, releases, 2)
}

func TestService_Get_NotFound(t *testing.T) {
	svc, _ := newCatalogService(t)

	item, releases, err := svc.Get(context.Background(), "missing")
	require.Error(t, err)
	assert.Nil(t, item)
	assert.Nil(t, releases)
}

func TestService_Get_ReleaseListError(t *testing.T) {
	svc, db := newCatalogService(t)
	ctx := context.Background()
	require.NoError(t, db.Create(&model.ExtensionCatalog{ExtensionID: "ext1", Name: "Ext", Kind: "source"}).Error)
	require.NoError(t, db.Migrator().DropTable(&model.ExtensionRelease{}))

	_, _, err := svc.Get(ctx, "ext1")
	require.Error(t, err)
}
