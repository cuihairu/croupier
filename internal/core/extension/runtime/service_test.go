package runtime

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/core/extension/externalfunc"
	extmanifest "github.com/cuihairu/croupier/internal/core/extension/manifest"
	"github.com/cuihairu/croupier/internal/model"
	extensiongorm "github.com/cuihairu/croupier/internal/repo/gorm/extension"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(gsqlite.Open("file::memory:?mode=memory"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&model.ExtensionInstallation{}, &model.ExtensionRelease{}, &model.ExtensionRuntimeBinding{}, &model.ExtensionEvent{})
	require.NoError(t, err)
	return db
}

func TestNewService(t *testing.T) {
	db := setupTestDB(t)

	installationRepo := extensiongorm.NewInstallationRepo(db)
	bindingRepo := extensiongorm.NewBindingRepo(db)
	eventRepo := extensiongorm.NewEventRepo(db)

	svc := NewService(installationRepo, extensiongorm.NewReleaseRepo(db), bindingRepo, eventRepo)

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

	s := NewService(installationRepo, extensiongorm.NewReleaseRepo(db), bindingRepo, eventRepo)

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

	s := NewService(installationRepo, extensiongorm.NewReleaseRepo(db), bindingRepo, eventRepo)

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

	s := NewService(installationRepo, extensiongorm.NewReleaseRepo(db), bindingRepo, eventRepo)

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

	s := NewService(installationRepo, extensiongorm.NewReleaseRepo(db), bindingRepo, eventRepo)

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

func TestBuildRuntimeBindings_ProviderRequiresExplicitName(t *testing.T) {
	out := buildRuntimeBindings(&model.ExtensionInstallation{
		ExtensionID:    "official.external-platform",
		ReleaseVersion: "1.0.0",
		Enabled:        true,
		ConfigJSON:     model.JSON(`{}`),
	})
	require.Len(t, out, 1)
	require.Equal(t, "function", out[0].BindingType)
}

func TestBuildRuntimeBindings_ExplicitProvider(t *testing.T) {
	raw := `{
		"provider": "acme ops",
		"config": {"base_url": "http://127.0.0.1:8080", "version": "1.2.3"},
		"secret_refs": {"api_key": "secret/acme"}
	}`
	item := &model.ExtensionInstallation{
		ExtensionID:    "official.external-platform",
		ReleaseVersion: "1.0.0",
		Enabled:        true,
		ConfigJSON:     model.JSON(raw),
		SecretRefsJSON: model.JSON(`{"api_key":"secret/acme"}`),
	}
	block, err := extmanifest.ParseProviderBlock(map[string]any{
		"provider": map[string]any{
			"type":       "openapi",
			"operations": []any{"day_report", "user_live", "order_list"},
		},
	})
	require.NoError(t, err)
	out := append(buildRuntimeBindings(item), providerBindingFromInstallation(item, block)...)
	require.Len(t, out, 2)
	require.Equal(t, "function", out[0].BindingType)

	provider := out[1]
	require.Equal(t, "provider", provider.BindingType)
	require.Equal(t, "acme_ops", provider.BindingKey)
	require.Equal(t, "extension:official.external-platform", provider.TargetRef)
	require.Equal(t, "active", provider.Status)

	var spec map[string]any
	require.NoError(t, json.Unmarshal(provider.SpecJSON, &spec))
	require.Equal(t, "acme_ops", spec["provider"])
	require.Equal(t, "openapi", spec["type"])
	require.Equal(t, []any{"day_report", "user_live", "order_list"}, spec["operations"])
	require.Equal(t, true, spec["enabled"])
	require.Equal(t, "http://127.0.0.1:8080", spec["config"].(map[string]any)["base_url"])
	require.Equal(t, "secret/acme", spec["config"].(map[string]any)["api_key"])

	parsed, ok := externalfunc.ParseProviderBinding(provider.BindingKey, spec)
	require.True(t, ok)
	require.Equal(t, "acme_ops", parsed.Provider)
	require.Equal(t, "openapi", parsed.Type)
	require.Equal(t, []string{"day_report", "user_live", "order_list"}, parsed.Operations)
	require.True(t, parsed.Enabled)
	require.Equal(t, "http://127.0.0.1:8080", parsed.Config["base_url"])
	require.Equal(t, "secret/acme", parsed.Config["api_key"])
}

func TestBuildRuntimeBindings_ExplicitProviderDisabled(t *testing.T) {
	out := buildRuntimeBindings(&model.ExtensionInstallation{
		ExtensionID: "official.external-platform",
		ConfigJSON:  model.JSON(`{"provider":"acme_ops"}`),
	})
	for _, binding := range out {
		if binding.BindingType != "provider" {
			continue
		}
		require.Equal(t, "acme_ops", binding.BindingKey)
		require.Equal(t, "disabled", binding.Status)
		var spec map[string]any
		require.NoError(t, json.Unmarshal(binding.SpecJSON, &spec))
		require.Equal(t, false, spec["enabled"])
	}
}

func TestBuildRuntimeBindings_ExplicitProviderInvalidConfig(t *testing.T) {
	out := buildRuntimeBindings(&model.ExtensionInstallation{
		ExtensionID: "official.external-platform",
		ConfigJSON:  model.JSON(`not-json`),
	})
	require.Len(t, out, 1)
	require.Equal(t, "function", out[0].BindingType)
}

func TestReconcile_DerivesProviderBindingFromReleaseManifest(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.Create(&model.ExtensionRelease{
		ExtensionID:    "official.external-platform",
		Version:        "1.0.0",
		ReleaseChannel: "stable",
		ManifestJSON: model.JSON(`{
			"id":"official.external-platform",
			"provider":{"type":"webhook","operations":["day_report","user_live"]}
		}`),
	}).Error)
	installation := &model.ExtensionInstallation{
		ExtensionID:    "official.external-platform",
		ReleaseVersion: "1.0.0",
		Status:         "enabled",
		Enabled:        true,
		ConfigJSON:     model.JSON(`{"provider":"acme_ops","config":{"base_url":"http://127.0.0.1:8080"}}`),
	}
	require.NoError(t, db.Create(installation).Error)

	svc := NewService(
		extensiongorm.NewInstallationRepo(db),
		extensiongorm.NewReleaseRepo(db),
		extensiongorm.NewBindingRepo(db),
		extensiongorm.NewEventRepo(db),
	)
	_, err := svc.Reconcile(context.Background(), installation.ID)
	require.NoError(t, err)

	var provider model.ExtensionRuntimeBinding
	require.NoError(t, db.Where("installation_id = ? AND binding_type = ?", installation.ID, "provider").First(&provider).Error)
	var spec map[string]any
	require.NoError(t, json.Unmarshal(provider.SpecJSON, &spec))
	require.Equal(t, "acme_ops", spec["provider"])
	require.Equal(t, "webhook", spec["type"])
	require.Equal(t, []any{"day_report", "user_live"}, spec["operations"])
}
