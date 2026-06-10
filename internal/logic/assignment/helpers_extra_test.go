package assignment

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
)

func TestCollectKnownFunctions_WithPopulatedStore(t *testing.T) {
	store := registry.NewStore()

	// Register some operations
	op1 := &openapi3.Operation{
		Summary: "Function 1",
		Responses: openapi3.NewResponses(
			openapi3.WithName("200", openapi3.NewResponse()),
		),
	}
	op2 := &openapi3.Operation{
		Summary: "Function 2",
		Responses: openapi3.NewResponses(
			openapi3.WithName("200", openapi3.NewResponse()),
		),
	}

	require.NoError(t, store.UpsertOpenAPI("player.get", op1))
	require.NoError(t, store.UpsertOpenAPI("player.ban", op2))

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
	}

	result := collectKnownFunctions(svcCtx)
	assert.NotNil(t, result)
	assert.Len(t, result, 2)
	assert.Contains(t, result, "player.get")
	assert.Contains(t, result, "player.ban")
}

func TestCollectKnownFunctions_EmptyStoreOperations(t *testing.T) {
	store := registry.NewStore()
	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
	}

	result := collectKnownFunctions(svcCtx)
	assert.Nil(t, result)
}

func TestLoadAssignments_NullAssignmentsInJSON(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "assignments.json")

	// Write JSON with null assignments
	err := os.WriteFile(testFile, []byte("null"), 0644)
	require.NoError(t, err)

	result, err := loadAssignments(testFile)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestLoadAssignments_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "assignments.json")

	err := os.WriteFile(testFile, []byte{}, 0644)
	require.NoError(t, err)

	result, err := loadAssignments(testFile)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestLoadAssignmentHistory_NullEntriesInJSON(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "history.json")

	// Write JSON with null entries
	err := os.WriteFile(testFile, []byte("null"), 0644)
	require.NoError(t, err)

	result, err := loadAssignmentHistory(testFile)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestSaveAssignments_InvalidPath(t *testing.T) {
	// Use a file as a directory path to cause error
	tmpDir := t.TempDir()
	blockingFile := filepath.Join(tmpDir, "blocking_file")
	err := os.WriteFile(blockingFile, []byte("test"), 0644)
	require.NoError(t, err)

	// Try to save with the file as a directory component
	invalidPath := filepath.Join(blockingFile, "subdir", "assignments.json")
	err = saveAssignments(invalidPath, map[string][]string{"key": {"func1"}})
	assert.Error(t, err)
}

func TestSaveAssignmentHistory_InvalidPath(t *testing.T) {
	// Use a file as a directory path to cause error
	tmpDir := t.TempDir()
	blockingFile := filepath.Join(tmpDir, "blocking_file")
	err := os.WriteFile(blockingFile, []byte("test"), 0644)
	require.NoError(t, err)

	// Try to save with the file as a directory component
	invalidPath := filepath.Join(blockingFile, "subdir", "history.json")
	entries := []assignmentHistoryEntry{
		{GameID: "game1", Env: "dev", FunctionID: "func1", Action: "assign"},
	}
	err = saveAssignmentHistory(invalidPath, entries)
	assert.Error(t, err)
}

func TestSaveAssignments_ValidData(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "assignments.json")

	data := map[string][]string{
		"game1|dev":  {"func1", "func2"},
		"game1|prod": {"func3"},
	}

	err := saveAssignments(testFile, data)
	assert.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(testFile)
	assert.NoError(t, err)

	// Verify content
	loaded, err := loadAssignments(testFile)
	assert.NoError(t, err)
	assert.Len(t, loaded, 2)
	assert.Equal(t, []string{"func1", "func2"}, loaded["game1|dev"])
	assert.Equal(t, []string{"func3"}, loaded["game1|prod"])
}

func TestSaveAssignmentHistory_ValidData(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "history.json")

	entries := []assignmentHistoryEntry{
		{GameID: "game1", Env: "dev", FunctionID: "func1", Action: "assign", Count: 2},
		{GameID: "game1", Env: "prod", FunctionID: "func2", Action: "remove", Count: 1},
	}

	err := saveAssignmentHistory(testFile, entries)
	assert.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(testFile)
	assert.NoError(t, err)

	// Verify content
	loaded, err := loadAssignmentHistory(testFile)
	assert.NoError(t, err)
	assert.Len(t, loaded, 2)
	assert.Equal(t, "game1", loaded[0].GameID)
	assert.Equal(t, "dev", loaded[0].Env)
}

func TestAppendAssignmentHistory_AutoGenerateIDAndTimestamp(t *testing.T) {
	tmpDir := t.TempDir()
	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			BootstrapData: config.BootstrapDataConfig{BaseDir: tmpDir},
			Registry:      config.RegistryConfig{AssignmentsPath: "assignments.json"},
		},
	}

	entry := assignmentHistoryEntry{
		GameID:     "game1",
		Env:        "dev",
		FunctionID: "func1",
		Action:     "assign",
		Count:      1,
		OperatedBy: "tester",
		// ID and OperatedAt are empty - should be auto-generated
	}

	err := appendAssignmentHistory(svcCtx, entry)
	assert.NoError(t, err)

	entries, err := loadAssignmentHistory(assignmentHistoryPath(svcCtx))
	assert.NoError(t, err)
	assert.Len(t, entries, 1)

	// ID should be auto-generated (non-empty)
	assert.NotEmpty(t, entries[0].ID)
	// OperatedAt should be auto-generated (non-empty)
	assert.NotEmpty(t, entries[0].OperatedAt)
	assert.Equal(t, "game1", entries[0].GameID)
	assert.Equal(t, "dev", entries[0].Env)
	assert.Equal(t, "func1", entries[0].FunctionID)
}

func TestAppendAssignmentHistory_WithProvidedIDAndTimestamp(t *testing.T) {
	tmpDir := t.TempDir()
	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			BootstrapData: config.BootstrapDataConfig{BaseDir: tmpDir},
			Registry:      config.RegistryConfig{AssignmentsPath: "assignments.json"},
		},
	}

	entry := assignmentHistoryEntry{
		ID:         "custom-id-123",
		GameID:     "game1",
		Env:        "dev",
		FunctionID: "func1",
		Action:     "assign",
		Count:      1,
		OperatedBy: "tester",
		OperatedAt: "2024-01-01T00:00:00Z",
	}

	err := appendAssignmentHistory(svcCtx, entry)
	assert.NoError(t, err)

	entries, err := loadAssignmentHistory(assignmentHistoryPath(svcCtx))
	assert.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "custom-id-123", entries[0].ID)
	assert.Equal(t, "2024-01-01T00:00:00Z", entries[0].OperatedAt)
}

func TestAppendAssignmentHistory_MaxEntriesLimit(t *testing.T) {
	tmpDir := t.TempDir()
	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			BootstrapData: config.BootstrapDataConfig{BaseDir: tmpDir},
			Registry:      config.RegistryConfig{AssignmentsPath: "assignments.json"},
		},
	}

	// Add 505 entries (exceeds the 500 limit)
	for i := 0; i < 505; i++ {
		entry := assignmentHistoryEntry{
			GameID:     "game1",
			Env:        "dev",
			FunctionID: "func1",
			Action:     "assign",
			Count:      i,
			OperatedBy: "tester",
		}
		err := appendAssignmentHistory(svcCtx, entry)
		require.NoError(t, err)
	}

	entries, err := loadAssignmentHistory(assignmentHistoryPath(svcCtx))
	assert.NoError(t, err)
	// Should be capped at 500
	assert.Len(t, entries, 500)

	// The newest entry should be first (count=504)
	assert.Equal(t, 504, entries[0].Count)
}

func TestAppendAssignmentHistory_LoadError(t *testing.T) {
	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "assignments_history.json")

	// Write invalid JSON to cause load error
	err := os.WriteFile(historyFile, []byte("{invalid json"), 0644)
	require.NoError(t, err)

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			BootstrapData: config.BootstrapDataConfig{BaseDir: tmpDir},
			Registry:      config.RegistryConfig{AssignmentsPath: "assignments.json"},
		},
	}

	entry := assignmentHistoryEntry{
		GameID:     "game1",
		Env:        "dev",
		FunctionID: "func1",
		Action:     "assign",
		Count:      1,
		OperatedBy: "tester",
	}

	err = appendAssignmentHistory(svcCtx, entry)
	assert.Error(t, err)
}

func TestFilterAssignments_EmptyFilters(t *testing.T) {
	data := map[string][]string{
		"game1|dev":  {"func1", "func2"},
		"game1|prod": {"func3"},
		"game2|dev":  {"func4"},
	}

	result := filterAssignments(data, "", "")
	assert.Len(t, result, 3)
	// Should be a clone, not the same reference
	assert.Equal(t, data["game1|dev"], result["game1|dev"])
}

func TestFilterAssignments_GameIDFilter(t *testing.T) {
	data := map[string][]string{
		"game1|dev":  {"func1", "func2"},
		"game1|prod": {"func3"},
		"game2|dev":  {"func4"},
	}

	result := filterAssignments(data, "game1", "")
	assert.Len(t, result, 2)
	assert.Contains(t, result, "game1|dev")
	assert.Contains(t, result, "game1|prod")
	assert.NotContains(t, result, "game2|dev")
}

func TestFilterAssignments_EnvFilter(t *testing.T) {
	data := map[string][]string{
		"game1|dev":  {"func1", "func2"},
		"game1|prod": {"func3"},
		"game2|dev":  {"func4"},
	}

	result := filterAssignments(data, "", "dev")
	assert.Len(t, result, 2)
	assert.Contains(t, result, "game1|dev")
	assert.Contains(t, result, "game2|dev")
	assert.NotContains(t, result, "game1|prod")
}

func TestFilterAssignments_GameIDAndEnvFilter(t *testing.T) {
	data := map[string][]string{
		"game1|dev":  {"func1", "func2"},
		"game1|prod": {"func3"},
		"game2|dev":  {"func4"},
	}

	result := filterAssignments(data, "game1", "dev")
	assert.Len(t, result, 1)
	assert.Contains(t, result, "game1|dev")
}

func TestFilterAssignments_CaseInsensitive(t *testing.T) {
	data := map[string][]string{
		"Game1|Dev": {"func1"},
	}

	result := filterAssignments(data, "game1", "dev")
	assert.Len(t, result, 1)
	assert.Contains(t, result, "Game1|Dev")
}

func TestFilterAssignments_NoMatch(t *testing.T) {
	data := map[string][]string{
		"game1|dev": {"func1"},
	}

	result := filterAssignments(data, "game2", "prod")
	assert.Empty(t, result)
}

func TestSplitAssignmentKey_ValidKey(t *testing.T) {
	gameID, env := splitAssignmentKey("game1|dev")
	assert.Equal(t, "game1", gameID)
	assert.Equal(t, "dev", env)
}

func TestSplitAssignmentKey_NoSeparator(t *testing.T) {
	gameID, env := splitAssignmentKey("game1")
	assert.Equal(t, "game1", gameID)
	assert.Equal(t, "", env)
}

func TestSplitAssignmentKey_WithWhitespace(t *testing.T) {
	gameID, env := splitAssignmentKey("  game1  |  dev  ")
	assert.Equal(t, "game1", gameID)
	assert.Equal(t, "dev", env)
}

func TestSplitAssignmentKey_EmptyString(t *testing.T) {
	gameID, env := splitAssignmentKey("")
	assert.Equal(t, "", gameID)
	assert.Equal(t, "", env)
}

func TestBuildAssignmentKey_Normal(t *testing.T) {
	key := buildAssignmentKey("game1", "dev")
	assert.Equal(t, "game1|dev", key)
}

func TestBuildAssignmentKey_WithWhitespace(t *testing.T) {
	key := buildAssignmentKey("  game1  ", "  dev  ")
	assert.Equal(t, "game1|dev", key)
}

func TestCloneAssignments_WithData(t *testing.T) {
	original := map[string][]string{
		"game1|dev":  {"func1", "func2"},
		"game1|prod": {"func3"},
	}

	cloned := cloneAssignments(original)

	// Should have same data
	assert.Len(t, cloned, 2)
	assert.Equal(t, []string{"func1", "func2"}, cloned["game1|dev"])
	assert.Equal(t, []string{"func3"}, cloned["game1|prod"])

	// Modifying clone should not affect original
	cloned["game1|dev"] = append(cloned["game1|dev"], "func4")
	assert.Len(t, original["game1|dev"], 2)
	assert.Len(t, cloned["game1|dev"], 3)
}

func TestCloneAssignments_NilInput(t *testing.T) {
	cloned := cloneAssignments(nil)
	assert.NotNil(t, cloned)
	assert.Empty(t, cloned)
}

func TestAssignmentsPath_NilContext(t *testing.T) {
	path := assignmentsPath(nil)
	assert.Equal(t, filepath.Join("data", "assignments.json"), path)
}

func TestAssignmentsPath_EmptyConfig(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	path := assignmentsPath(svcCtx)
	assert.Equal(t, filepath.Join("data", "assignments.json"), path)
}

func TestAssignmentsPath_WithConfig(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Registry: config.RegistryConfig{
				AssignmentsPath: filepath.Join("custom", "path", "assignments.json"),
			},
		},
	}
	path := assignmentsPath(svcCtx)
	assert.Equal(t, filepath.Join("custom", "path", "assignments.json"), path)
}

func TestAssignmentsPath_RelativePathWithBaseDir(t *testing.T) {
	baseDir := t.TempDir()
	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			BootstrapData: config.BootstrapDataConfig{
				BaseDir: baseDir,
			},
			Registry: config.RegistryConfig{
				AssignmentsPath: filepath.Join("relative", "assignments.json"),
			},
		},
	}
	path := assignmentsPath(svcCtx)
	assert.Equal(t, filepath.Join(baseDir, "relative", "assignments.json"), path)
}

func TestAssignmentHistoryPath_DerivedFromAssignmentsPath(t *testing.T) {
	baseDir := t.TempDir()
	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			BootstrapData: config.BootstrapDataConfig{
				BaseDir: baseDir,
			},
			Registry: config.RegistryConfig{
				AssignmentsPath: filepath.Join("data", "assignments.json"),
			},
		},
	}
	path := assignmentHistoryPath(svcCtx)
	assert.Equal(t, filepath.Join(baseDir, "data", "assignments_history.json"), path)
}

func TestLoadAllAssignments_WithValidData(t *testing.T) {
	tmpDir := t.TempDir()
	assignmentsFile := filepath.Join(tmpDir, "assignments.json")

	// Write test data
	data := map[string][]string{
		"game1|dev": {"func1", "func2"},
	}
	bytes, err := json.Marshal(data)
	require.NoError(t, err)
	err = os.WriteFile(assignmentsFile, bytes, 0644)
	require.NoError(t, err)

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Registry: config.RegistryConfig{
				AssignmentsPath: assignmentsFile,
			},
		},
	}

	result, err := LoadAllAssignments(svcCtx)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, []string{"func1", "func2"}, result["game1|dev"])
}

func TestLoadAllAssignments_NonExistentFile(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Registry: config.RegistryConfig{
				AssignmentsPath: "/nonexistent/path/assignments.json",
			},
		},
	}

	result, err := LoadAllAssignments(svcCtx)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result)
}
