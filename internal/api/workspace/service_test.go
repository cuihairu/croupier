package workspace

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/cache"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupWorkspaceServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(gsqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))

	sqlDB, err := db.DB()
	require.NoError(t, err)

	_, err = sqlDB.Exec("DELETE FROM workspace_configs")
	require.NoError(t, err)
	_, err = sqlDB.Exec("DELETE FROM config_versions")
	require.NoError(t, err)

	return db
}

func setupWorkspaceServiceContext(t *testing.T, db *gorm.DB) *svc.ServiceContext {
	t.Helper()

	nullCache := cache.NewNullCache()
	cacheHelper := cache.NewCacheHelper(nullCache)

	return &svc.ServiceContext{
		DB:                   db,
		Cache:                nullCache,
		CacheHelper:          cacheHelper,
		WorkspaceConfigModel: model.NewWorkspaceConfigModel(db),
		ConfigVersionModel:   model.NewConfigVersionModel(db),
	}
}

func validWorkspaceLayout() map[string]any {
	return map[string]any{
		"type": "tabs",
		"tabs": []any{
			map[string]any{
				"key":   "players",
				"title": "玩家列表",
				"layout": map[string]any{
					"type":         "list",
					"listFunction": "player.list",
					"columns": []any{
						map[string]any{"key": "username", "title": "用户名"},
					},
				},
			},
		},
	}
}

func extractVersionConfig(t *testing.T, value string) WorkspaceConfig {
	t.Helper()

	cfg, _, err := decodeWorkspaceSnapshot(value)
	require.NoError(t, err)
	return cfg
}

func TestService_SaveConfig_PersistsFullWorkspaceSnapshot(t *testing.T) {
	db := setupWorkspaceServiceTestDB(t)
	svcCtx := setupWorkspaceServiceContext(t, db)
	service := NewService(svcCtx)

	resp, err := service.SaveConfig(context.Background(), &SaveConfigRequest{
		ObjectKey:   "player",
		Title:       "玩家工作台",
		Description: "完整配置快照",
		MenuOrder:   7,
		Status:      "archived",
		Layout:      validWorkspaceLayout(),
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "archived", resp.WorkspaceConfig.Status)
	assert.Equal(t, 7, resp.WorkspaceConfig.MenuOrder)
	assert.NotZero(t, resp.WorkspaceConfig.Version)

	stored, err := svcCtx.WorkspaceConfigModel.FindByObjectKey(context.Background(), "player")
	require.NoError(t, err)

	var snapshot WorkspaceConfig
	require.NoError(t, json.Unmarshal(stored.Config, &snapshot))
	assert.Equal(t, "player", snapshot.ObjectKey)
	assert.Equal(t, "玩家工作台", snapshot.Title)
	assert.Equal(t, "完整配置快照", snapshot.Description)
	assert.Equal(t, "archived", snapshot.Status)
	assert.Equal(t, 7, snapshot.MenuOrder)
	assert.NotNil(t, snapshot.Layout)
	assert.NotEmpty(t, snapshot.CreatedAt)
	assert.NotEmpty(t, snapshot.UpdatedAt)

	versions, err := svcCtx.ConfigVersionModel.List(context.Background(), workspaceVersionKey("player"))
	require.NoError(t, err)
	require.Len(t, versions, 1)
	versionSnapshot := extractVersionConfig(t, versions[0].Value)
	assert.Equal(t, snapshot.Title, versionSnapshot.Title)
	assert.Equal(t, snapshot.Description, versionSnapshot.Description)
	assert.Equal(t, snapshot.MenuOrder, versionSnapshot.MenuOrder)
	assert.Equal(t, snapshot.Status, versionSnapshot.Status)
}

func TestService_SaveConfig_RejectsInvalidLayout(t *testing.T) {
	db := setupWorkspaceServiceTestDB(t)
	svcCtx := setupWorkspaceServiceContext(t, db)
	service := NewService(svcCtx)

	resp, err := service.SaveConfig(context.Background(), &SaveConfigRequest{
		ObjectKey: "player",
		Title:     "非法配置",
		Layout: map[string]any{
			"type": "tabs",
			"tabs": []any{
				map[string]any{
					"key":   "invalid",
					"title": "非法列表",
					"layout": map[string]any{
						"type": "list",
					},
				},
			},
		},
	})

	require.Error(t, err)
	assert.Nil(t, resp)

	codeErr, ok := err.(*errorx.CodeError)
	require.True(t, ok)
	assert.Equal(t, "workspace_invalid_config", codeErr.ErrorCode())
	assert.Contains(t, err.Error(), "listFunction")

	versions, listErr := svcCtx.ConfigVersionModel.List(context.Background(), workspaceVersionKey("player"))
	require.NoError(t, listErr)
	assert.Empty(t, versions)
}

func TestService_Rollback_RestoresSnapshotAndVersionPointers(t *testing.T) {
	db := setupWorkspaceServiceTestDB(t)
	svcCtx := setupWorkspaceServiceContext(t, db)
	service := NewService(svcCtx)

	ctx := context.WithValue(context.Background(), "username", "tester")

	firstResp, err := service.SaveConfig(ctx, &SaveConfigRequest{
		ObjectKey:   "player",
		Title:       "版本一",
		Description: "初始版本",
		MenuOrder:   1,
		Status:      "draft",
		Layout:      validWorkspaceLayout(),
	})
	require.NoError(t, err)
	require.NotNil(t, firstResp)

	secondLayout := validWorkspaceLayout()
	secondLayout["tabs"] = []any{
		map[string]any{
			"key":   "players",
			"title": "玩家表单",
			"layout": map[string]any{
				"type":           "form",
				"submitFunction": "player.save",
				"fields": []any{
					map[string]any{"key": "username", "label": "用户名"},
				},
			},
		},
	}

	secondResp, err := service.SaveConfig(ctx, &SaveConfigRequest{
		ObjectKey:   "player",
		Title:       "版本二",
		Description: "修改后的版本",
		MenuOrder:   3,
		Status:      "draft",
		Layout:      secondLayout,
	})
	require.NoError(t, err)
	require.NotNil(t, secondResp)
	require.Greater(t, secondResp.WorkspaceConfig.Version, firstResp.WorkspaceConfig.Version)

	rollbackResp, err := service.Rollback(ctx, &RollbackRequest{
		ObjectKey: "player",
		VersionID: "1",
	})
	require.NoError(t, err)
	require.NotNil(t, rollbackResp)
	assert.Greater(t, rollbackResp.Version, 2)

	current, err := service.GetConfig(ctx, &GetConfigRequest{ObjectKey: "player"})
	require.NoError(t, err)
	require.NotNil(t, current)
	assert.Equal(t, "版本一", current.WorkspaceConfig.Title)
	assert.Equal(t, "初始版本", current.WorkspaceConfig.Description)
	assert.Equal(t, 1, current.WorkspaceConfig.MenuOrder)
	assert.Equal(t, rollbackResp.Version, current.WorkspaceConfig.Version)

	versionsResp, err := service.Versions(ctx, &VersionsRequest{ObjectKey: "player"})
	require.NoError(t, err)
	require.NotNil(t, versionsResp)
	require.Len(t, versionsResp.Items, 3)

	assert.Equal(t, rollbackResp.Version, versionsResp.Items[0].Version)
	assert.True(t, versionsResp.Items[0].IsCurrentDraft)
	assert.Equal(t, "3", versionsResp.Items[0].ID)
	assert.Equal(t, "2", versionsResp.Items[1].ID)
	assert.Equal(t, "1", versionsResp.Items[2].ID)

	versionDetail, err := service.VersionDetail(ctx, &VersionDetailRequest{
		ObjectKey: "player",
		VersionID: "1",
	})
	require.NoError(t, err)
	require.NotNil(t, versionDetail)
	detailConfig, ok := versionDetail.WorkspaceVersionRecord.Config.(WorkspaceConfig)
	require.True(t, ok)
	assert.Equal(t, "版本一", detailConfig.Title)
}
