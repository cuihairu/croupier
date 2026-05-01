package extensiongorm

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(gsqlite.Open("file::memory:?mode=memory"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(
		&model.ExtensionCatalog{},
		&model.ExtensionRelease{},
		&model.ExtensionInstallation{},
		&model.ExtensionRuntimeBinding{},
		&model.ExtensionEvent{},
	)
	require.NoError(t, err)
	return db
}

// Bundle tests
func TestNewBundle(t *testing.T) {
	db := setupTestDB(t)
	bundle := NewBundle(db)

	assert.NotNil(t, bundle)
	assert.NotNil(t, bundle.Catalog)
	assert.NotNil(t, bundle.Release)
	assert.NotNil(t, bundle.Installation)
	assert.NotNil(t, bundle.Binding)
	assert.NotNil(t, bundle.Event)
}

// CatalogRepo tests
func TestNewCatalogRepo(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCatalogRepo(db)
	assert.NotNil(t, repo)
	assert.Equal(t, db, repo.db)
}

func TestCatalogRepo_List(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCatalogRepo(db)
	ctx := context.Background()

	// Insert test data
	catalog1 := &model.ExtensionCatalog{ExtensionID: "ext1", Name: "Extension 1", Kind: "source", Status: "active"}
	catalog2 := &model.ExtensionCatalog{ExtensionID: "ext2", Name: "Extension 2", Kind: "validator", Status: "active"}
	catalog3 := &model.ExtensionCatalog{ExtensionID: "ext3", Name: "Extension 3", Kind: "source", Status: "inactive"}
	require.NoError(t, db.Create(catalog1).Error)
	require.NoError(t, db.Create(catalog2).Error)
	require.NoError(t, db.Create(catalog3).Error)

	t.Run("list all", func(t *testing.T) {
		items, total, err := repo.List(ctx, CatalogListQuery{})
		require.NoError(t, err)
		assert.Equal(t, int64(3), total)
		assert.Len(t, items, 3)
	})

	t.Run("filter by keyword", func(t *testing.T) {
		items, total, err := repo.List(ctx, CatalogListQuery{Keyword: "Extension 1"})
		require.NoError(t, err)
		assert.Equal(t, int64(1), total)
		assert.Len(t, items, 1)
		assert.Equal(t, "ext1", items[0].ExtensionID)
	})

	t.Run("filter by kind", func(t *testing.T) {
		items, total, err := repo.List(ctx, CatalogListQuery{Kind: "source"})
		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
		assert.Len(t, items, 2)
	})

	t.Run("filter by status", func(t *testing.T) {
		items, total, err := repo.List(ctx, CatalogListQuery{Status: "active"})
		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
		assert.Len(t, items, 2)
	})

	t.Run("with limit and offset", func(t *testing.T) {
		items, total, err := repo.List(ctx, CatalogListQuery{Limit: 2, Offset: 0})
		require.NoError(t, err)
		assert.Equal(t, int64(3), total)
		assert.Len(t, items, 2)
	})
}

func TestCatalogRepo_GetByExtensionID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCatalogRepo(db)
	ctx := context.Background()

	catalog := &model.ExtensionCatalog{ExtensionID: "test-ext", Name: "Test Extension", Kind: "source"}
	require.NoError(t, db.Create(catalog).Error)

	t.Run("found", func(t *testing.T) {
		found, err := repo.GetByExtensionID(ctx, "test-ext")
		require.NoError(t, err)
		assert.Equal(t, "test-ext", found.ExtensionID)
		assert.Equal(t, "Test Extension", found.Name)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := repo.GetByExtensionID(ctx, "non-existent")
		assert.Error(t, err)
	})
}

// ReleaseRepo tests
func TestNewReleaseRepo(t *testing.T) {
	db := setupTestDB(t)
	repo := NewReleaseRepo(db)
	assert.NotNil(t, repo)
	assert.Equal(t, db, repo.db)
}

func TestReleaseRepo_ListByExtensionID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewReleaseRepo(db)
	ctx := context.Background()

	release1 := &model.ExtensionRelease{ExtensionID: "ext1", Version: "1.0.0", ManifestJSON: []byte("{}")}
	release2 := &model.ExtensionRelease{ExtensionID: "ext1", Version: "2.0.0", ManifestJSON: []byte("{}")}
	release3 := &model.ExtensionRelease{ExtensionID: "ext2", Version: "1.0.0", ManifestJSON: []byte("{}")}
	require.NoError(t, db.Create(release1).Error)
	require.NoError(t, db.Create(release2).Error)
	require.NoError(t, db.Create(release3).Error)

	t.Run("list by extension ID", func(t *testing.T) {
		items, err := repo.ListByExtensionID(ctx, "ext1")
		require.NoError(t, err)
		assert.Len(t, items, 2)
	})

	t.Run("empty result", func(t *testing.T) {
		items, err := repo.ListByExtensionID(ctx, "non-existent")
		require.NoError(t, err)
		assert.Len(t, items, 0)
	})
}

// InstallationRepo tests
func TestNewInstallationRepo(t *testing.T) {
	db := setupTestDB(t)
	repo := NewInstallationRepo(db)
	assert.NotNil(t, repo)
	assert.Equal(t, db, repo.db)
}

func TestInstallationRepo_Create(t *testing.T) {
	db := setupTestDB(t)
	repo := NewInstallationRepo(db)
	ctx := context.Background()

	item := &model.ExtensionInstallation{
		InstallationKey: "create-test-key",
		ExtensionID:     "test-ext",
		ScopeType:       "game",
		ScopeID:         "game1",
		TargetType:      "agent",
		TargetID:        "agent1",
		Status:          "pending",
	}

	err := repo.Create(ctx, item)
	require.NoError(t, err)
	assert.NotZero(t, item.ID)
}

func TestInstallationRepo_Save(t *testing.T) {
	db := setupTestDB(t)
	repo := NewInstallationRepo(db)
	ctx := context.Background()

	item := &model.ExtensionInstallation{
		InstallationKey: "save-test-key",
		ExtensionID:     "test-ext",
		Status:          "pending",
	}
	require.NoError(t, db.Create(item).Error)

	item.Status = "installed"
	err := repo.Save(ctx, item)
	require.NoError(t, err)

	var found model.ExtensionInstallation
	require.NoError(t, db.First(&found, item.ID).Error)
	assert.Equal(t, "installed", found.Status)
}

func TestInstallationRepo_GetByID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewInstallationRepo(db)
	ctx := context.Background()

	item := &model.ExtensionInstallation{
		InstallationKey: "getby-test-key",
		ExtensionID:     "test-ext",
		Status:          "pending",
	}
	require.NoError(t, db.Create(item).Error)

	t.Run("found", func(t *testing.T) {
		found, err := repo.GetByID(ctx, item.ID)
		require.NoError(t, err)
		assert.Equal(t, item.ID, found.ID)
		assert.Equal(t, "test-ext", found.ExtensionID)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := repo.GetByID(ctx, 999)
		assert.Error(t, err)
	})
}

func TestInstallationRepo_List(t *testing.T) {
	db := setupTestDB(t)
	repo := NewInstallationRepo(db)
	ctx := context.Background()

	enabled := true
	disabled := false

	item1 := &model.ExtensionInstallation{InstallationKey: "key1", ExtensionID: "ext1", ScopeType: "game", ScopeID: "game1", Status: "installed", Enabled: true}
	item2 := &model.ExtensionInstallation{InstallationKey: "key2", ExtensionID: "ext2", ScopeType: "game", ScopeID: "game1", Status: "pending", Enabled: false}
	item3 := &model.ExtensionInstallation{InstallationKey: "key3", ExtensionID: "ext1", ScopeType: "game", ScopeID: "game2", Status: "installed", Enabled: true}
	require.NoError(t, db.Create(item1).Error)
	require.NoError(t, db.Create(item2).Error)
	require.NoError(t, db.Create(item3).Error)

	t.Run("list all", func(t *testing.T) {
		items, total, err := repo.List(ctx, InstallationListQuery{})
		require.NoError(t, err)
		assert.Equal(t, int64(3), total)
		assert.Len(t, items, 3)
	})

	t.Run("filter by extension ID", func(t *testing.T) {
		items, total, err := repo.List(ctx, InstallationListQuery{ExtensionID: "ext1"})
		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
		assert.Len(t, items, 2)
	})

	t.Run("filter by scope", func(t *testing.T) {
		items, total, err := repo.List(ctx, InstallationListQuery{ScopeType: "game", ScopeID: "game1"})
		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
		assert.Len(t, items, 2)
	})

	t.Run("filter by status", func(t *testing.T) {
		items, total, err := repo.List(ctx, InstallationListQuery{Status: "installed"})
		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
		assert.Len(t, items, 2)
	})

	t.Run("filter by enabled", func(t *testing.T) {
		items, total, err := repo.List(ctx, InstallationListQuery{Enabled: &enabled})
		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
		assert.Len(t, items, 2)
	})

	t.Run("filter by disabled", func(t *testing.T) {
		items, total, err := repo.List(ctx, InstallationListQuery{Enabled: &disabled})
		require.NoError(t, err)
		assert.Equal(t, int64(1), total)
		assert.Len(t, items, 1)
	})

	t.Run("with limit", func(t *testing.T) {
		items, total, err := repo.List(ctx, InstallationListQuery{Limit: 2})
		require.NoError(t, err)
		assert.Equal(t, int64(3), total)
		assert.Len(t, items, 2)
	})
}

// BindingRepo tests
func TestNewBindingRepo(t *testing.T) {
	db := setupTestDB(t)
	repo := NewBindingRepo(db)
	assert.NotNil(t, repo)
	assert.Equal(t, db, repo.db)
}

func TestBindingRepo_ReplaceForInstallation(t *testing.T) {
	db := setupTestDB(t)
	repo := NewBindingRepo(db)
	ctx := context.Background()

	installation := &model.ExtensionInstallation{
		InstallationKey: "replace-binding-key",
		ExtensionID:     "test-ext",
		Status:          "installed",
	}
	require.NoError(t, db.Create(installation).Error)

	// Create initial bindings
	binding1 := model.ExtensionRuntimeBinding{BindingKey: "key1", TargetRef: "target1"}
	binding2 := model.ExtensionRuntimeBinding{BindingKey: "key2", TargetRef: "target2"}

	err := repo.ReplaceForInstallation(ctx, installation.ID, []model.ExtensionRuntimeBinding{binding1, binding2})
	require.NoError(t, err)

	// Verify bindings were created
	var bindings []model.ExtensionRuntimeBinding
	require.NoError(t, db.Where("installation_id = ?", installation.ID).Find(&bindings).Error)
	assert.Len(t, bindings, 2)

	// Replace with different bindings
	newBinding1 := model.ExtensionRuntimeBinding{BindingKey: "key3", TargetRef: "target3"}
	err = repo.ReplaceForInstallation(ctx, installation.ID, []model.ExtensionRuntimeBinding{newBinding1})
	require.NoError(t, err)

	// Verify old bindings were deleted and new one was created
	require.NoError(t, db.Where("installation_id = ?", installation.ID).Find(&bindings).Error)
	assert.Len(t, bindings, 1)
	assert.Equal(t, "key3", bindings[0].BindingKey)
}

func TestBindingRepo_ListByInstallationID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewBindingRepo(db)
	ctx := context.Background()

	installation := &model.ExtensionInstallation{
		InstallationKey: "list-binding-key",
		ExtensionID:     "test-ext",
		Status:          "installed",
	}
	require.NoError(t, db.Create(installation).Error)

	binding1 := &model.ExtensionRuntimeBinding{InstallationID: installation.ID, BindingKey: "key1", TargetRef: "target1"}
	binding2 := &model.ExtensionRuntimeBinding{InstallationID: installation.ID, BindingKey: "key2", TargetRef: "target2"}
	require.NoError(t, db.Create(binding1).Error)
	require.NoError(t, db.Create(binding2).Error)

	items, err := repo.ListByInstallationID(ctx, installation.ID)
	require.NoError(t, err)
	assert.Len(t, items, 2)

	// Verify order (id desc)
	assert.Equal(t, "key2", items[0].BindingKey)
	assert.Equal(t, "key1", items[1].BindingKey)
}

// EventRepo tests
func TestNewEventRepo(t *testing.T) {
	db := setupTestDB(t)
	repo := NewEventRepo(db)
	assert.NotNil(t, repo)
	assert.Equal(t, db, repo.db)
}

func TestEventRepo_Create(t *testing.T) {
	db := setupTestDB(t)
	repo := NewEventRepo(db)
	ctx := context.Background()

	installation := &model.ExtensionInstallation{
		InstallationKey: "create-event-key",
		ExtensionID:     "test-ext",
		Status:          "installed",
	}
	require.NoError(t, db.Create(installation).Error)

	event := &model.ExtensionEvent{
		InstallationID: installation.ID,
		EventType:      "reconcile",
		Level:          "info",
		Message:        "Test event",
	}

	err := repo.Create(ctx, event)
	require.NoError(t, err)
	assert.NotZero(t, event.ID)
}

func TestEventRepo_List(t *testing.T) {
	db := setupTestDB(t)
	repo := NewEventRepo(db)
	ctx := context.Background()

	installation1 := &model.ExtensionInstallation{InstallationKey: "evt-key1", ExtensionID: "ext1", Status: "installed"}
	installation2 := &model.ExtensionInstallation{InstallationKey: "evt-key2", ExtensionID: "ext2", Status: "installed"}
	require.NoError(t, db.Create(installation1).Error)
	require.NoError(t, db.Create(installation2).Error)

	event1 := &model.ExtensionEvent{InstallationID: installation1.ID, EventType: "reconcile", Level: "info", Message: "Success"}
	event2 := &model.ExtensionEvent{InstallationID: installation1.ID, EventType: "error", Level: "error", Message: "Failed"}
	event3 := &model.ExtensionEvent{InstallationID: installation2.ID, EventType: "reconcile", Level: "info", Message: "Success"}
	require.NoError(t, db.Create(event1).Error)
	require.NoError(t, db.Create(event2).Error)
	require.NoError(t, db.Create(event3).Error)

	t.Run("list all", func(t *testing.T) {
		items, total, err := repo.List(ctx, EventListQuery{})
		require.NoError(t, err)
		assert.Equal(t, int64(3), total)
		assert.Len(t, items, 3)
	})

	t.Run("filter by installation ID", func(t *testing.T) {
		items, total, err := repo.List(ctx, EventListQuery{InstallationID: installation1.ID})
		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
		assert.Len(t, items, 2)
	})

	t.Run("filter by level", func(t *testing.T) {
		items, total, err := repo.List(ctx, EventListQuery{Level: "error"})
		require.NoError(t, err)
		assert.Equal(t, int64(1), total)
		assert.Len(t, items, 1)
		assert.Equal(t, "error", items[0].Level)
	})

	t.Run("filter by keyword", func(t *testing.T) {
		items, total, err := repo.List(ctx, EventListQuery{Keyword: "Failed"})
		require.NoError(t, err)
		assert.Equal(t, int64(1), total)
		assert.Len(t, items, 1)
	})

	t.Run("with limit", func(t *testing.T) {
		items, total, err := repo.List(ctx, EventListQuery{Limit: 2})
		require.NoError(t, err)
		assert.Equal(t, int64(3), total)
		assert.Len(t, items, 2)
	})
}

func TestEventRepo_ListByInstallationID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewEventRepo(db)
	ctx := context.Background()

	installation := &model.ExtensionInstallation{
		InstallationKey: "listbyid-event-key",
		ExtensionID:     "test-ext",
		Status:          "installed",
	}
	require.NoError(t, db.Create(installation).Error)

	event1 := &model.ExtensionEvent{InstallationID: installation.ID, EventType: "reconcile", Level: "info", Message: "Event 1"}
	event2 := &model.ExtensionEvent{InstallationID: installation.ID, EventType: "error", Level: "error", Message: "Event 2"}
	require.NoError(t, db.Create(event1).Error)
	require.NoError(t, db.Create(event2).Error)

	items, total, err := repo.ListByInstallationID(ctx, installation.ID, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, items, 2)
}
