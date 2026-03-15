package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/api/analytics"
	"github.com/cuihairu/croupier/internal/config"
	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	"github.com/cuihairu/croupier/internal/model"
	extensiongorm "github.com/cuihairu/croupier/internal/repo/gorm/extension"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestGetAnalyticsFiltersFallsBackToFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "analytics_filters.json")
	data, err := analytics.SaveAnalyticsFiltersJSON([]analytics.AnalyticsFilters{
		{GameId: "file", Filters: map[string]any{"env": "prod"}},
	})
	if err != nil {
		t.Fatalf("SaveAnalyticsFiltersJSON() error = %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write filters file failed: %v", err)
	}

	s := NewService(&svc.ServiceContext{
		Config: config.Config{
			Registry: config.RegistryConfig{
				AnalyticsFiltersPath: path,
			},
		},
	})
	resp, err := s.GetAnalyticsFilters(context.Background(), &GetAnalyticsFiltersRequest{})
	if err != nil {
		t.Fatalf("GetAnalyticsFilters() error = %v", err)
	}
	if resp.Count != 1 || len(resp.Items) != 1 || resp.Items[0].GameId != "file" {
		t.Fatalf("unexpected fallback response: %#v", resp)
	}
}

func TestGetAnalyticsFiltersPrefersInstallationConfig(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.ExtensionInstallation{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	repos := extensiongorm.NewBundle(db)
	installationSvc := extensioninstallation.NewService(repos.Installation, repos.Event, repos.Binding)
	_, err = installationSvc.Install(context.Background(), extensioninstallation.InstallRequest{
		ExtensionID:    "official.analytics",
		ReleaseVersion: "1.0.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent_group",
		TargetID:       "default",
		Config: map[string]any{
			"filters": []map[string]any{
				{"gameId": "ext", "filters": map[string]any{"env": "stage"}},
			},
		},
		Operator: "tester",
	})
	if err != nil {
		t.Fatalf("install analytics extension failed: %v", err)
	}

	if err := db.AutoMigrate(&model.ExtensionEvent{}); err != nil {
		t.Fatalf("auto migrate extension event failed: %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "analytics_filters.json")
	fileData, err := analytics.SaveAnalyticsFiltersJSON([]analytics.AnalyticsFilters{
		{GameId: "file", Filters: map[string]any{"env": "prod"}},
	})
	if err != nil {
		t.Fatalf("SaveAnalyticsFiltersJSON() error = %v", err)
	}
	if err := os.WriteFile(path, fileData, 0o644); err != nil {
		t.Fatalf("write filters file failed: %v", err)
	}

	s := NewService(&svc.ServiceContext{
		Config: config.Config{
			Registry: config.RegistryConfig{
				AnalyticsFiltersPath: path,
			},
		},
		Extensions: &svc.ExtensionServices{
			Installation: installationSvc,
		},
	})
	resp, err := s.GetAnalyticsFilters(context.Background(), &GetAnalyticsFiltersRequest{})
	if err != nil {
		t.Fatalf("GetAnalyticsFilters() error = %v", err)
	}
	if resp.Count != 1 || len(resp.Items) != 1 || resp.Items[0].GameId != "ext" {
		t.Fatalf("expected installation config to win over file fallback, got %#v", resp)
	}
}
