// 补齐 loadFiltersFromAnalyticsInstallation 中安装配置 JSON 非法分支。
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

// 安装记录的 ConfigJSON 本身不是合法 JSON 对象 → unmarshal map 失败，透传错误。
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
