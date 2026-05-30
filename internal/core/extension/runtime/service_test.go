package runtime

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	extensiongorm "github.com/cuihairu/croupier/internal/repo/gorm/extension"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(gsqlite.Open("file::memory:?mode=memory"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&model.ExtensionInstallation{}, &model.ExtensionRuntimeBinding{}, &model.ExtensionEvent{})
	require.NoError(t, err)
	return db
}

func TestNewService(t *testing.T) {
	db := setupTestDB(t)

	installationRepo := extensiongorm.NewInstallationRepo(db)
	bindingRepo := extensiongorm.NewBindingRepo(db)
	eventRepo := extensiongorm.NewEventRepo(db)

	svc := NewService(installationRepo, bindingRepo, eventRepo)

	require.NotNil(t, svc)
	require.NotNil(t, svc.installationRepo)
	require.NotNil(t, svc.bindingRepo)
	require.NotNil(t, svc.eventRepo)
}

func TestReconcile_NilService(t *testing.T) {
	var s *Service
	_, err := s.Reconcile(context.Background(), 1)
	require.Error(t, err)
}

func TestReconcile_NilRepos(t *testing.T) {
	s := &Service{}
	_, err := s.Reconcile(context.Background(), 1)
	require.Error(t, err)
}

func TestReconcile_Success_Analytics(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Create installation
	installation := &model.ExtensionInstallation{
		ExtensionID: "official.analytics",
		Status:      "",
	}
	require.NoError(t, db.Create(installation).Error)

	installationRepo := extensiongorm.NewInstallationRepo(db)
	bindingRepo := extensiongorm.NewBindingRepo(db)
	eventRepo := extensiongorm.NewEventRepo(db)

	s := NewService(installationRepo, bindingRepo, eventRepo)

	result, err := s.Reconcile(ctx, installation.ID)
	require.NoError(t, err)
	require.Equal(t, installation.ID, result.InstallationID)
	require.Equal(t, "installed", result.Status)
	require.GreaterOrEqual(t, result.Applied, 8) // analytics has at least 8 bindings
	require.Equal(t, 0, result.Failed)
	require.Equal(t, "reconciled", result.Message)

	// Verify bindings were created
	var bindings []model.ExtensionRuntimeBinding
	require.NoError(t, db.Where("installation_id = ?", installation.ID).Find(&bindings).Error)
	require.GreaterOrEqual(t, len(bindings), 8)

	// Verify status was updated
	var updated model.ExtensionInstallation
	require.NoError(t, db.First(&updated, installation.ID).Error)
	require.Equal(t, "installed", updated.Status)

	// Verify event was created
	var events []model.ExtensionEvent
	require.NoError(t, db.Where("installation_id = ?", installation.ID).Find(&events).Error)
	require.Greater(t, len(events), 0)
	require.Equal(t, "reconcile", events[0].EventType)
}

func TestReconcile_WithExistingStatus(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Create installation with existing status
	installation := &model.ExtensionInstallation{
		ExtensionID: "official.alerting",
		Status:      "active",
	}
	require.NoError(t, db.Create(installation).Error)

	installationRepo := extensiongorm.NewInstallationRepo(db)
	bindingRepo := extensiongorm.NewBindingRepo(db)
	eventRepo := extensiongorm.NewEventRepo(db)

	s := NewService(installationRepo, bindingRepo, eventRepo)

	result, err := s.Reconcile(ctx, installation.ID)
	require.NoError(t, err)
	require.Equal(t, "active", result.Status)
}

func TestReconcile_NotFound(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	installationRepo := extensiongorm.NewInstallationRepo(db)
	bindingRepo := extensiongorm.NewBindingRepo(db)
	eventRepo := extensiongorm.NewEventRepo(db)

	s := NewService(installationRepo, bindingRepo, eventRepo)

	_, err := s.Reconcile(ctx, 999)
	require.Error(t, err)
}

func TestReconcile_AllOfficialExtensions(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	extensions := []string{
		"official.analytics",
		"official.alerting",
		"official.notification",
		"official.approval",
		"official.backup-advanced",
	}

	installationRepo := extensiongorm.NewInstallationRepo(db)
	bindingRepo := extensiongorm.NewBindingRepo(db)
	eventRepo := extensiongorm.NewEventRepo(db)

	s := NewService(installationRepo, bindingRepo, eventRepo)

	for i, extID := range extensions {
		t.Run(extID, func(t *testing.T) {
			installation := &model.ExtensionInstallation{
				ExtensionID:     extID,
				InstallationKey: extID + "-" + string(rune(i)), // Unique key
			}
			require.NoError(t, db.Create(installation).Error)

			result, err := s.Reconcile(ctx, installation.ID)
			require.NoError(t, err)
			require.NotZero(t, result.Applied)

			// Clean up for next test - delete bindings first due to FK
			db.Where("installation_id = ?", installation.ID).Delete(&model.ExtensionRuntimeBinding{})
			db.Where("installation_id = ?", installation.ID).Delete(&model.ExtensionEvent{})
			db.Delete(installation)
		})
	}
}

func TestBuildRuntimeBindings_NilItem(t *testing.T) {
	out := buildRuntimeBindings(nil)
	require.Empty(t, out)
}

func TestBuildRuntimeBindings_TrimmedExtensionID(t *testing.T) {
	out := buildRuntimeBindings(&model.ExtensionInstallation{
		ExtensionID: "  official.analytics  ",
	})
	require.GreaterOrEqual(t, len(out), 4)
	// Check that target ref is correctly formed (trimmed)
	require.Equal(t, "extension:official.analytics", out[0].TargetRef)
}

func TestBuildRuntimeBindings_Default(t *testing.T) {
	out := buildRuntimeBindings(&model.ExtensionInstallation{
		ExtensionID: "custom.extension",
	})
	require.Len(t, out, 1)
	require.Equal(t, "custom.extension.default", out[0].BindingKey)
	require.Equal(t, "extension:custom.extension", out[0].TargetRef)
}
