package assignment

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppendAssignmentHistory_New(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Registry: config.RegistryConfig{
				AssignmentsPath: filepath.Join(tmpDir, "assignments.json"),
			},
		},
	}

	// Test appending new entry
	entry := assignmentHistoryEntry{
		GameID:     "game1",
		Env:        "prod",
		FunctionID: "func1",
		Action:     "add",
		Count:      2,
		OperatedBy: "testuser",
	}

	err := appendAssignmentHistory(svcCtx, entry)
	require.NoError(t, err)

	// Verify file was created
	historyPath := assignmentHistoryPath(svcCtx)
	_, err = os.Stat(historyPath)
	assert.NoError(t, err)

	// Verify entry was saved with ID and timestamp
	loaded, err := loadAssignmentHistory(historyPath)
	require.NoError(t, err)
	assert.Equal(t, 1, len(loaded))
	assert.NotEmpty(t, loaded[0].ID)
	assert.NotEmpty(t, loaded[0].OperatedAt)
	assert.Equal(t, "game1", loaded[0].GameID)
	assert.Equal(t, "prod", loaded[0].Env)
}

func TestAppendAssignmentHistory_MaxEntries(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	historyPath := filepath.Join(tmpDir, "assignments_history.json")

	// Create initial history with 500 entries
	entries := make([]assignmentHistoryEntry, 500)
	for i := 0; i < 500; i++ {
		entries[i] = assignmentHistoryEntry{
			ID:         string(rune(i)),
			GameID:     "game1",
			FunctionID: "func1",
			Action:     "add",
		}
	}
	err := saveAssignmentHistory(historyPath, entries)
	require.NoError(t, err)

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Registry: config.RegistryConfig{
				AssignmentsPath: filepath.Join(tmpDir, "assignments.json"),
			},
		},
	}

	// Append new entry
	newEntry := assignmentHistoryEntry{
		GameID:     "game2",
		FunctionID: "func2",
		Action:     "add",
	}
	err = appendAssignmentHistory(svcCtx, newEntry)
	require.NoError(t, err)

	// Verify history is capped at 500
	loaded, err := loadAssignmentHistory(historyPath)
	require.NoError(t, err)
	assert.Equal(t, 500, len(loaded))
	// New entry should be first
	assert.Equal(t, "game2", loaded[0].GameID)
}

func TestAppendAssignmentHistory_PrePopulatedEntry(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Registry: config.RegistryConfig{
				AssignmentsPath: filepath.Join(tmpDir, "assignments.json"),
			},
		},
	}

	// Entry with pre-populated ID and timestamp
	entry := assignmentHistoryEntry{
		ID:         "custom-id-123",
		GameID:     "game1",
		FunctionID: "func1",
		Action:     "add",
		OperatedAt: "2024-01-01T12:00:00Z",
	}

	err := appendAssignmentHistory(svcCtx, entry)
	require.NoError(t, err)

	historyPath := assignmentHistoryPath(svcCtx)
	loaded, err := loadAssignmentHistory(historyPath)
	require.NoError(t, err)
	assert.Equal(t, "custom-id-123", loaded[0].ID)
	assert.Equal(t, "2024-01-01T12:00:00Z", loaded[0].OperatedAt)
}

func TestCollectKnownFunctions_EmptyOperations(t *testing.T) {
	t.Parallel()

	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}

	result := collectKnownFunctions(svcCtx)
	assert.Nil(t, result)
}

func TestCollectKnownFunctions_WithOperations(t *testing.T) {
	t.Parallel()

	store := registry.NewStore()

	// Add some operations
	op1 := &openapi3.Operation{
		OperationID: "game1.func1",
		Summary:     "Function 1",
		Responses:   openapi3.NewResponses(),
	}
	op2 := &openapi3.Operation{
		OperationID: "game2.func2",
		Summary:     "Function 2",
		Responses:   openapi3.NewResponses(),
	}

	store.UpsertOpenAPI("game1.func1", op1)
	store.UpsertOpenAPI("game2.func2", op2)

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
	}

	result := collectKnownFunctions(svcCtx)
	assert.NotNil(t, result)
	assert.Equal(t, 2, len(result))

	// Check all function IDs are in the result
	_, exists1 := result["game1.func1"]
	_, exists2 := result["game2.func2"]
	assert.True(t, exists1, "Function ID game1.func1 should be in result")
	assert.True(t, exists2, "Function ID game2.func2 should be in result")
}

func TestAssignmentsPath_RelativePathWithBaseDir(t *testing.T) {
	t.Parallel()

	baseDir := "/absolute/base"
	relPath := "relative/assignments.json"

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			BootstrapData: config.BootstrapDataConfig{
				BaseDir: baseDir,
			},
			Registry: config.RegistryConfig{
				AssignmentsPath: relPath,
			},
		},
	}

	result := assignmentsPath(svcCtx)
	expected := filepath.Join(baseDir, relPath)
	assert.Equal(t, expected, result)
}

func TestAssignmentsPath_AbsolutePath(t *testing.T) {
	t.Parallel()

	absPath := "/absolute/path/assignments.json"

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Registry: config.RegistryConfig{
				AssignmentsPath: absPath,
			},
		},
	}

	result := assignmentsPath(svcCtx)
	assert.Equal(t, absPath, result)
}

func TestAssignmentsPath_EmptyConfigWithBaseDir(t *testing.T) {
	t.Parallel()

	baseDir := "/custom/base"

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			BootstrapData: config.BootstrapDataConfig{
				BaseDir: baseDir,
			},
			Registry: config.RegistryConfig{
				AssignmentsPath: "",
			},
		},
	}

	result := assignmentsPath(svcCtx)
	assert.True(t, strings.HasSuffix(result, filepath.Join("data", "assignments.json")))
}

func TestLoadAssignments_InvalidJSON(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "invalid.json")

	err := os.WriteFile(path, []byte("{ invalid json"), 0644)
	require.NoError(t, err)

	_, err = loadAssignments(path)
	assert.Error(t, err)
}

func TestSaveAssignments_CreateDirectory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "subdir", "nested", "assignments.json")

	data := map[string][]string{
		"game1|prod": {"func1"},
	}

	err := saveAssignments(path, data)
	assert.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(path)
	assert.NoError(t, err)
}

func TestLoadAssignmentHistory_InvalidJSON(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "invalid.json")

	err := os.WriteFile(path, []byte("[ invalid json"), 0644)
	require.NoError(t, err)

	_, err = loadAssignmentHistory(path)
	assert.Error(t, err)
}

func TestSaveAssignmentHistory_CreateDirectory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "subdir", "history.json")

	entries := []assignmentHistoryEntry{
		{ID: "1", FunctionID: "func1", Action: "add"},
	}

	err := saveAssignmentHistory(path, entries)
	assert.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(path)
	assert.NoError(t, err)
}

func TestNewHandler(t *testing.T) {
	t.Parallel()

	service := &mockAssignmentService{}
	handler := NewHandler(service)

	assert.NotNil(t, handler)
	assert.Equal(t, service, handler.service)
}
