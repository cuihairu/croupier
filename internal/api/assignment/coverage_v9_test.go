// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

package assignment

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandlerHistoryBindQueryErrorV9(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := &Handler{service: &mockAssignmentService{}}
	router := gin.New()
	router.GET("/assignments/history", handler.History)

	req, _ := http.NewRequest("GET", "/assignments/history?page=invalid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlerUpdateServiceErrorV9(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &mockAssignmentService{
		updateFunc: func(ctx context.Context, req *AssignmentsUpdateRequest) (*AssignmentsUpdateResponse, error) {
			return nil, errorx.NewInternalError("更新失败")
		},
	}

	handler := &Handler{service: service}
	router := gin.New()
	router.PUT("/assignments", handler.Update)

	body, _ := json.Marshal(AssignmentsUpdateRequest{
		GameId:    "game1",
		Env:       "prod",
		Functions: []string{"func1"},
	})

	req, _ := http.NewRequest("PUT", "/assignments", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestLoadAssignmentsReadErrorV9(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := loadAssignments(tmpDir)
	require.Error(t, err)
}

func TestLoadAssignmentsNullJSONV9(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "null.json")
	require.NoError(t, os.WriteFile(path, []byte("null"), 0o644))

	loaded, err := loadAssignments(path)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Empty(t, loaded)
}

func TestLoadAssignmentHistoryReadErrorV9(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := loadAssignmentHistory(tmpDir)
	require.Error(t, err)
}

func TestLoadAssignmentHistoryEmptyFileV9(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "empty.json")
	require.NoError(t, os.WriteFile(path, []byte{}, 0o644))

	loaded, err := loadAssignmentHistory(path)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Empty(t, loaded)
}

func TestLoadAssignmentHistoryNullJSONV9(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "null.json")
	require.NoError(t, os.WriteFile(path, []byte("null"), 0o644))

	loaded, err := loadAssignmentHistory(path)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Empty(t, loaded)
}

func TestSaveAssignmentHistoryMarshalErrorV9(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "history.json")

	entries := []assignmentHistoryEntry{
		{
			ID:      "1",
			GameID:  "game1",
			Details: map[string]interface{}{"nan": math.NaN()},
		},
	}

	err := saveAssignmentHistory(path, entries)
	require.Error(t, err)
}

func TestAppendAssignmentHistoryLoadErrorV9(t *testing.T) {
	tmpDir := t.TempDir()
	svcCtx := buildAssignmentSvcContextV9(t, tmpDir)

	historyPath := assignmentHistoryPath(svcCtx)
	require.NoError(t, os.WriteFile(historyPath, []byte("[ invalid json"), 0o644))

	err := appendAssignmentHistory(svcCtx, assignmentHistoryEntry{
		GameID: "game1",
		Env:    "prod",
		Action: "assign",
	})
	require.Error(t, err)
}

func TestServiceHistoryPageSizeClampV9(t *testing.T) {
	_, svcCtx := setupAssignmentTestDB(t)
	ctx := createAssignmentTestContext(t, svcCtx.DB)

	historyPath := assignmentHistoryPath(svcCtx)
	require.NoError(t, saveAssignmentHistory(historyPath, []assignmentHistoryEntry{
		{ID: "1", GameID: "game1", Env: "prod", Action: "assign"},
	}))

	service := NewService(svcCtx)
	resp, err := service.History(ctx, &AssignmentsHistoryRequest{
		Page:     1,
		PageSize: 500,
	})
	require.NoError(t, err)
	assert.Equal(t, 100, resp.PageSize)
	assert.Equal(t, 1, resp.Total)
}

func TestServiceHistoryFiltersAndOutOfRangePageV9(t *testing.T) {
	_, svcCtx := setupAssignmentTestDB(t)
	ctx := createAssignmentTestContext(t, svcCtx.DB)

	historyPath := assignmentHistoryPath(svcCtx)
	require.NoError(t, saveAssignmentHistory(historyPath, []assignmentHistoryEntry{
		{ID: "1", GameID: "game1", Env: "prod", Action: "assign"},
		{ID: "2", GameID: "game2", Env: "dev", Action: "remove"},
	}))

	service := NewService(svcCtx)

	resp, err := service.History(ctx, &AssignmentsHistoryRequest{GameId: "game1"})
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Total)
	assert.Equal(t, "game1", resp.Items[0].GameID)

	resp, err = service.History(ctx, &AssignmentsHistoryRequest{GameId: "game1", Page: 10, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Total)
	assert.Empty(t, resp.Items)
	assert.Equal(t, 10, resp.Page)
}

func TestServiceUpdateCloneEmptyTargetEnvV9(t *testing.T) {
	_, svcCtx := setupAssignmentTestDB(t)
	ctx := createAssignmentTestContext(t, svcCtx.DB)

	service := NewService(svcCtx)
	_, err := service.Update(ctx, &AssignmentsUpdateRequest{
		GameId:    "game1",
		Action:    "clone",
		TargetEnv: "   ",
		Functions: []string{"func1"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "targetEnv不能为空")
}

func TestServiceUpdateCloneModelNotInitializedV9(t *testing.T) {
	_, svcCtx := setupAssignmentTestDB(t)
	ctx := createAssignmentTestContext(t, svcCtx.DB)
	svcCtx.GameModel = nil

	service := NewService(svcCtx)
	_, err := service.Update(ctx, &AssignmentsUpdateRequest{
		GameId:    "game1",
		Action:    "clone",
		TargetEnv: "prod",
		Functions: []string{"func1"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "游戏环境模型未初始化")
}

func TestServiceUpdateCloneTargetEnvNotBoundV9(t *testing.T) {
	_, svcCtx := setupAssignmentTestDB(t)
	ctx := createAssignmentTestContext(t, svcCtx.DB)
	ctx = svc.WithGameScope(ctx, svc.GameScope{GameID: "game1", Env: "prod"})

	game := &model.Game{GameID: "game1", Name: "Game One"}
	require.NoError(t, svcCtx.GameModel.Create(ctx, game))
	require.NoError(t, svcCtx.GameModel.AddEnvBinding(ctx, "game1", "prod", "game1_prod", "", ""))

	service := NewService(svcCtx)
	_, err := service.Update(ctx, &AssignmentsUpdateRequest{
		GameId:    "game1",
		Action:    "clone",
		TargetEnv: "ghost-env",
		Functions: []string{"func1"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "目标环境不存在")
}

func TestServiceUpdateLoadAssignmentsErrorV9(t *testing.T) {
	_, svcCtx := setupAssignmentTestDB(t)
	ctx := createAssignmentTestContext(t, svcCtx.DB)

	require.NoError(t, os.WriteFile(svcCtx.Config.Registry.AssignmentsPath, []byte("{ invalid json"), 0o644))

	service := NewService(svcCtx)
	_, err := service.Update(ctx, &AssignmentsUpdateRequest{
		GameId:    "game1",
		Env:       "prod",
		Functions: []string{"func1"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "读取分配数据失败")
}

func TestServiceUpdateSaveAssignmentsErrorV9(t *testing.T) {
	_, svcCtx := setupAssignmentTestDB(t)
	ctx := createAssignmentTestContext(t, svcCtx.DB)

	// Readable directory with a valid assignments file: load succeeds, but
	// the write-protected directory makes saveAssignments fail.
	dir := t.TempDir()
	assignmentsFile := filepath.Join(dir, "assignments.json")
	require.NoError(t, os.WriteFile(assignmentsFile, []byte("{}"), 0o644))
	require.NoError(t, os.Chmod(dir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	svcCtx.Config.Registry.AssignmentsPath = assignmentsFile

	service := NewService(svcCtx)
	_, err := service.Update(ctx, &AssignmentsUpdateRequest{
		GameId:    "game1",
		Env:       "prod",
		Functions: []string{"func1"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "保存分配数据失败")
}

func buildAssignmentSvcContextV9(t *testing.T, tmpDir string) *svc.ServiceContext {
	t.Helper()
	return &svc.ServiceContext{
		Config: config.Config{
			Registry: config.RegistryConfig{
				AssignmentsPath: filepath.Join(tmpDir, "assignments.json"),
			},
		},
	}
}
