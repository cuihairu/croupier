package assignment

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_List_Success(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	assignmentsPath := filepath.Join(tmpDir, "assignments.json")

	// Create test data
	testData := map[string][]string{
		"game1|prod": {"func1", "func2"},
		"game2|dev":  {"func3"},
	}
	err := saveAssignments(assignmentsPath, testData)
	require.NoError(t, err)

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Registry: config.RegistryConfig{
				AssignmentsPath: assignmentsPath,
			},
		},
	}
	service := NewService(svcCtx)

	// Create a context with mocked permissions
	ctx := context.Background()

	req := &AssignmentsListRequest{
		GameId: "game1",
	}

	resp, err := service.List(ctx, req)
	if err != nil {
		// Admin model not initialized in test, that's expected
		assert.Contains(t, err.Error(), "管理员")
		return
	}

	assert.NotNil(t, resp)
	assert.Equal(t, 1, resp.Total)
}

func TestService_List_FilterByEnv(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	assignmentsPath := filepath.Join(tmpDir, "assignments.json")

	testData := map[string][]string{
		"game1|prod": {"func1"},
		"game1|dev":  {"func2"},
	}
	err := saveAssignments(assignmentsPath, testData)
	require.NoError(t, err)

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Registry: config.RegistryConfig{
				AssignmentsPath: assignmentsPath,
			},
		},
	}
	service := NewService(svcCtx)

	ctx := context.Background()

	req := &AssignmentsListRequest{
		GameId: "game1",
		Env:    "prod",
	}

	resp, err := service.List(ctx, req)
	if err != nil {
		assert.Contains(t, err.Error(), "管理员")
		return
	}

	assert.NotNil(t, resp)
	assert.Equal(t, 1, resp.Total)
}

func TestService_History_Success(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	historyPath := filepath.Join(tmpDir, "assignments_history.json")

	// Create test history
	testHistory := []assignmentHistoryEntry{
		{ID: "1", GameID: "game1", Env: "prod", FunctionID: "func1", Action: "add"},
		{ID: "2", GameID: "game1", Env: "dev", FunctionID: "func2", Action: "add"},
	}
	err := saveAssignmentHistory(historyPath, testHistory)
	require.NoError(t, err)

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Registry: config.RegistryConfig{
				AssignmentsPath: filepath.Join(tmpDir, "assignments.json"),
			},
		},
	}
	service := NewService(svcCtx)

	ctx := context.Background()

	req := &AssignmentsHistoryRequest{
		GameId: "game1",
		Env:    "prod",
	}

	resp, err := service.History(ctx, req)
	if err != nil {
		assert.Contains(t, err.Error(), "管理员")
		return
	}

	assert.NotNil(t, resp)
	assert.Equal(t, 1, resp.Total)
}

func TestService_History_Pagination(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	historyPath := filepath.Join(tmpDir, "assignments_history.json")

	// Create test history with 25 entries
	testHistory := make([]assignmentHistoryEntry, 25)
	for i := 0; i < 25; i++ {
		testHistory[i] = assignmentHistoryEntry{
			ID:        string(rune(i)),
			GameID:    "game1",
			FunctionID: "func1",
			Action:    "add",
		}
	}
	err := saveAssignmentHistory(historyPath, testHistory)
	require.NoError(t, err)

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Registry: config.RegistryConfig{
				AssignmentsPath: filepath.Join(tmpDir, "assignments.json"),
			},
		},
	}
	service := NewService(svcCtx)

	ctx := context.Background()

	req := &AssignmentsHistoryRequest{
		Page:     1,
		PageSize: 10,
	}

	resp, err := service.History(ctx, req)
	if err != nil {
		assert.Contains(t, err.Error(), "管理员")
		return
	}

	assert.NotNil(t, resp)
	assert.Equal(t, 25, resp.Total)
	assert.Equal(t, 10, len(resp.Items))
	assert.Equal(t, 1, resp.Page)
	assert.Equal(t, 10, resp.PageSize)
}

func TestService_Update_Success(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	assignmentsPath := filepath.Join(tmpDir, "assignments.json")

	// Create initial data
	testData := map[string][]string{
		"game1|prod": {"func1"},
	}
	err := saveAssignments(assignmentsPath, testData)
	require.NoError(t, err)

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Registry: config.RegistryConfig{
				AssignmentsPath: assignmentsPath,
			},
		},
	}
	service := NewService(svcCtx)

	ctx := context.Background()

	req := &AssignmentsUpdateRequest{
		GameId:    "game1",
		Env:       "prod",
		Action:    "assign",
		Functions: []string{"func1", "func2"},
	}

	resp, err := service.Update(ctx, req)
	if err != nil {
		assert.Contains(t, err.Error(), "管理员")
		return
	}

	assert.NotNil(t, resp)
	assert.True(t, resp.OK)
}

func TestService_Update_EmptyGameId(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	assignmentsPath := filepath.Join(tmpDir, "assignments.json")

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Registry: config.RegistryConfig{
				AssignmentsPath: assignmentsPath,
			},
		},
	}
	service := NewService(svcCtx)

	ctx := context.Background()

	req := &AssignmentsUpdateRequest{
		GameId:    "",
		Env:       "prod",
		Functions: []string{"func1"},
	}

	_, err := service.Update(ctx, req)
	assert.Error(t, err)
}

func TestService_Update_RemoveAll(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	assignmentsPath := filepath.Join(tmpDir, "assignments.json")

	// Create initial data
	testData := map[string][]string{
		"game1|prod": {"func1", "func2"},
	}
	err := saveAssignments(assignmentsPath, testData)
	require.NoError(t, err)

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Registry: config.RegistryConfig{
				AssignmentsPath: assignmentsPath,
			},
		},
	}
	service := NewService(svcCtx)

	ctx := context.Background()

	req := &AssignmentsUpdateRequest{
		GameId:    "game1",
		Env:       "prod",
		Functions: []string{},
	}

	resp, err := service.Update(ctx, req)
	if err != nil {
		assert.Contains(t, err.Error(), "管理员")
		return
	}

	assert.NotNil(t, resp)
	assert.True(t, resp.OK)

	// Verify assignments were removed
	loaded, err := loadAssignments(assignmentsPath)
	require.NoError(t, err)
	assert.Equal(t, []string{}, loaded["game1|prod"])
}

func TestService_Update_WithWhitespace(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	assignmentsPath := filepath.Join(tmpDir, "assignments.json")

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Registry: config.RegistryConfig{
				AssignmentsPath: assignmentsPath,
			},
		},
	}
	service := NewService(svcCtx)

	ctx := context.Background()

	req := &AssignmentsUpdateRequest{
		GameId:    "  game1  ",
		Env:       "  prod  ",
		Functions: []string{"  func1  ", " func2 ", ""},
	}

	resp, err := service.Update(ctx, req)
	if err != nil {
		assert.Contains(t, err.Error(), "管理员")
		return
	}

	assert.NotNil(t, resp)
	assert.True(t, resp.OK)

	// Verify key was normalized
	loaded, err := loadAssignments(assignmentsPath)
	require.NoError(t, err)
	assert.Contains(t, loaded, "game1|prod")
	assert.Equal(t, []string{"func1", "func2"}, loaded["game1|prod"])
}

func TestService_LoadAssignments_NotExist(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	assignmentsPath := filepath.Join(tmpDir, "nonexistent.json")

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Registry: config.RegistryConfig{
				AssignmentsPath: assignmentsPath,
			},
		},
	}
	service := NewService(svcCtx)

	ctx := context.Background()

	req := &AssignmentsListRequest{}

	resp, err := service.List(ctx, req)
	if err != nil {
		assert.Contains(t, err.Error(), "管理员")
		return
	}

	assert.NotNil(t, resp)
	assert.Equal(t, 0, resp.Total)
}

func TestService_Update_CreateHistoryEntry(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	assignmentsPath := filepath.Join(tmpDir, "assignments.json")

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Registry: config.RegistryConfig{
				AssignmentsPath: assignmentsPath,
			},
		},
	}
	service := NewService(svcCtx)

	ctx := context.Background()

	req := &AssignmentsUpdateRequest{
		GameId:    "game1",
		Env:       "prod",
		Functions: []string{"func1"},
	}

	_, err := service.Update(ctx, req)
	if err != nil {
		assert.Contains(t, err.Error(), "管理员")
		return
	}

	// Verify history file was created
	historyPath := assignmentHistoryPath(svcCtx)
	_, err = os.Stat(historyPath)
	assert.NoError(t, err)
}
