// Covers the invalid installation-config JSON branch in loadFiltersFromAnalyticsInstallation.
package agent

import (
	"context"
	"testing"

	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	"github.com/cuihairu/croupier/internal/model"
	extensiongorm "github.com/cuihairu/croupier/internal/repo/gorm/extension"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// The installation record's ConfigJSON is not a valid JSON object -> unmarshalling into a
// map fails, and the error is passed through.
func TestLoadFiltersInstallationConfigInvalidJSON(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ExtensionInstallation{}, &model.ExtensionRuntimeBinding{}, &model.ExtensionEvent{}))
	repos := extensiongorm.NewBundle(db)
	inst := extensioninstallation.NewService(repos.Installation, nil, nil)

	row := model.ExtensionInstallation{
		InstallationKey: "official:analytics:system:global:agent_group:default",
		ExtensionID:     officialAnalyticsExtensionID,
		ReleaseVersion:  "1.0.0",
		ScopeType:       "system",
		ScopeID:         "global",
		TargetType:      "agent_group",
		TargetID:        "default",
		Status:          "installed",
		DesiredState:    "installed",
		Enabled:         true,
		ConfigJSON:      model.JSON(`{invalid-json`),
	}
	require.NoError(t, db.Create(&row).Error)

	s := NewService(&svc.ServiceContext{Extensions: &svc.ExtensionServices{Installation: inst}})
	items, ok, err := s.loadFiltersFromAnalyticsInstallation(context.Background())
	require.Error(t, err)
	assert.False(t, ok)
	assert.Nil(t, items)
}
