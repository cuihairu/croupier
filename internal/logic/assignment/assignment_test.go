package assignment

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupTestContext creates a test context with in-memory database
func setupTestContext(t *testing.T) (*svc.ServiceContext, context.Context) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = model.AutoMigrate(db)
	require.NoError(t, err)

	baseDir := t.TempDir()
	svcCtx := &svc.ServiceContext{
		DB: db,
		Config: config.Config{
			BootstrapData: config.BootstrapDataConfig{BaseDir: baseDir},
			Registry:      config.RegistryConfig{AssignmentsPath: "assignments.json"},
		},
		AdminModel:      model.NewAdminModel(db),
		RoleModel:       model.NewRoleModel(db),
		PermissionModel: model.NewPermissionModel(db),
		RegistryStore:   nil, // No registry store for basic tests
	}

	// Create test admin and role
	admin := &model.Admin{Username: "testadmin", Status: 1}
	err = svcCtx.AdminModel.Create(context.Background(), admin, "password")
	require.NoError(t, err)

	role := &model.Role{Name: "admin", Description: "Admin role"}
	err = svcCtx.RoleModel.Create(context.Background(), role)
	require.NoError(t, err)

	err = svcCtx.AdminModel.AssignRole(context.Background(), admin.ID, role.ID)
	require.NoError(t, err)

	err = svcCtx.RoleModel.ReplacePermissions(context.Background(), role.ID, []string{"admin:all"})
	require.NoError(t, err)

	ctx := context.WithValue(context.Background(), "username", "testadmin")
	return svcCtx, ctx
}

func TestAssignmentsPath(t *testing.T) {
	// Use platform-specific paths for cross-platform compatibility
	var absBaseDir, absPath string
	if filepath.IsAbs("C:\\data") || filepath.IsAbs("/data") {
		// Unix-like or Windows with drive letter
		if filepath.IsAbs("/data") {
			absBaseDir = "/data"
			absPath = "/absolute/assignments.json"
		} else {
			// Windows: need drive letter for absolute path
			absBaseDir = filepath.Join(os.Getenv("TEMP"), "data")
			absPath = filepath.Join(os.Getenv("TEMP"), "absolute", "assignments.json")
		}
	} else {
		// Fallback: use temp directory
		absBaseDir = filepath.Join(os.Getenv("TEMP"), "data")
		absPath = filepath.Join(os.Getenv("TEMP"), "absolute", "assignments.json")
	}

	tests := []struct {
		name     string
		baseDir  string
		assigns  string
		expected string
	}{
		{
			name:     "absolute path",
			baseDir:  absBaseDir,
			assigns:  absPath,
			expected: absPath,
		},
		{
			name:     "relative path with base dir",
			baseDir:  absBaseDir,
			assigns:  "assignments.json",
			expected: filepath.Join(absBaseDir, "assignments.json"),
		},
		{
			name:     "empty assignments path",
			baseDir:  absBaseDir,
			assigns:  "",
			expected: filepath.Join(absBaseDir, "data", "assignments.json"),
		},
		{
			name:     "nil context",
			baseDir:  "",
			assigns:  "",
			expected: filepath.Join("data", "assignments.json"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var svcCtx *svc.ServiceContext
			if tt.baseDir != "" {
				svcCtx = &svc.ServiceContext{
					Config: config.Config{
						BootstrapData: config.BootstrapDataConfig{BaseDir: tt.baseDir},
						Registry:      config.RegistryConfig{AssignmentsPath: tt.assigns},
					},
				}
			}

			result := assignmentsPath(svcCtx)
			// Normalize paths for comparison on all platforms
			resultNorm := filepath.ToSlash(result)
			expectedNorm := filepath.ToSlash(tt.expected)
			if resultNorm != expectedNorm {
				t.Errorf("assignmentsPath() = %v (normalized: %v), want %v (normalized: %v)", result, resultNorm, tt.expected, expectedNorm)
			}
		})
	}
}

func TestLoadAssignments(t *testing.T) {
	t.Run("nonexistent file returns empty map", func(t *testing.T) {
		result, err := loadAssignments("/nonexistent/path/assignments.json")
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Empty(t, result)
	})

	t.Run("empty file returns empty map", func(t *testing.T) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, "empty.json")
		err := os.WriteFile(path, []byte{}, 0o644)
		require.NoError(t, err)

		result, err := loadAssignments(path)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Empty(t, result)
	})

	t.Run("valid JSON file", func(t *testing.T) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, "valid.json")
		content := `{"game1|prod": ["fn1", "fn2"], "game2|dev": ["fn3"]}`
		err := os.WriteFile(path, []byte(content), 0o644)
		require.NoError(t, err)

		result, err := loadAssignments(path)
		assert.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, []string{"fn1", "fn2"}, result["game1|prod"])
	})
}

func TestSaveAssignments(t *testing.T) {
	t.Run("save and load", func(t *testing.T) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, "test.json")

		data := map[string][]string{
			"game1|prod": {"fn1", "fn2"},
			"game2|dev":  {"fn3"},
		}

		err := saveAssignments(path, data)
		assert.NoError(t, err)

		// Verify file exists
		_, err = os.Stat(path)
		assert.NoError(t, err)

		// Load and verify
		loaded, err := loadAssignments(path)
		assert.NoError(t, err)
		assert.Equal(t, data, loaded)
	})

	t.Run("creates directory if needed", func(t *testing.T) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, "subdir", "test.json")

		err := saveAssignments(path, map[string][]string{})
		assert.NoError(t, err)

		_, err = os.Stat(path)
		assert.NoError(t, err)
	})
}

func TestFilterAssignments(t *testing.T) {
	data := map[string][]string{
		"game1|prod":    {"fn1", "fn2"},
		"game1|dev":     {"fn3"},
		"game2|prod":    {"fn4"},
		"game2|staging": {"fn5"},
	}

	t.Run("no filters returns all", func(t *testing.T) {
		result := filterAssignments(data, "", "")
		assert.Len(t, result, 4)
	})

	t.Run("filter by gameID", func(t *testing.T) {
		result := filterAssignments(data, "game1", "")
		assert.Len(t, result, 2)
		assert.Contains(t, result, "game1|prod")
		assert.Contains(t, result, "game1|dev")
	})

	t.Run("filter by env", func(t *testing.T) {
		result := filterAssignments(data, "", "prod")
		assert.Len(t, result, 2)
		assert.Contains(t, result, "game1|prod")
		assert.Contains(t, result, "game2|prod")
	})

	t.Run("filter by both", func(t *testing.T) {
		result := filterAssignments(data, "game1", "prod")
		assert.Len(t, result, 1)
		assert.Contains(t, result, "game1|prod")
	})

	t.Run("case insensitive filter", func(t *testing.T) {
		result := filterAssignments(data, "GAME1", "PROD")
		assert.Len(t, result, 1)
		assert.Contains(t, result, "game1|prod")
	})

	t.Run("no matches", func(t *testing.T) {
		result := filterAssignments(data, "game3", "prod")
		assert.Len(t, result, 0)
	})
}

func TestSplitAssignmentKey(t *testing.T) {
	tests := []struct {
		name           string
		key            string
		expectedGameID string
		expectedEnv    string
	}{
		{
			name:           "valid key with env",
			key:            "game1|prod",
			expectedGameID: "game1",
			expectedEnv:    "prod",
		},
		{
			name:           "key without env",
			key:            "game1",
			expectedGameID: "game1",
			expectedEnv:    "",
		},
		{
			name:           "key with extra separators",
			key:            "game1|prod|extra",
			expectedGameID: "game1",
			expectedEnv:    "prod|extra",
		},
		{
			name:           "key with whitespace",
			key:            " game1 | prod ",
			expectedGameID: "game1",
			expectedEnv:    "prod",
		},
		{
			name:           "empty key",
			key:            "",
			expectedGameID: "",
			expectedEnv:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gameID, env := splitAssignmentKey(tt.key)
			assert.Equal(t, tt.expectedGameID, gameID)
			assert.Equal(t, tt.expectedEnv, env)
		})
	}
}

func TestBuildAssignmentKey(t *testing.T) {
	tests := []struct {
		name     string
		gameID   string
		env      string
		expected string
	}{
		{
			name:     "both values",
			gameID:   "game1",
			env:      "prod",
			expected: "game1|prod",
		},
		{
			name:     "with whitespace",
			gameID:   " game1 ",
			env:      " prod ",
			expected: "game1|prod",
		},
		{
			name:     "empty env",
			gameID:   "game1",
			env:      "",
			expected: "game1|",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildAssignmentKey(tt.gameID, tt.env)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCloneAssignments(t *testing.T) {
	original := map[string][]string{
		"game1|prod": {"fn1", "fn2"},
		"game2|dev":  {"fn3"},
	}

	cloned := cloneAssignments(original)

	// Verify deep copy
	assert.Equal(t, original, cloned)

	// Modify original
	original["game1|prod"][0] = "modified"

	// Clone should be unchanged
	assert.Equal(t, "fn1", cloned["game1|prod"][0])
}

func TestNormalizeFunctions(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "normal functions",
			input:    []string{"fn1", "fn2", "fn3"},
			expected: []string{"fn1", "fn2", "fn3"},
		},
		{
			name:     "with whitespace",
			input:    []string{" fn1 ", " fn2 ", " fn3 "},
			expected: []string{"fn1", "fn2", "fn3"},
		},
		{
			name:     "empty strings removed",
			input:    []string{"fn1", "", "fn2", "", "fn3"},
			expected: []string{"fn1", "fn2", "fn3"},
		},
		{
			name:     "duplicates removed",
			input:    []string{"fn1", "fn2", "fn1", "fn3", "fn2"},
			expected: []string{"fn1", "fn2", "fn3"},
		},
		{
			name:     "all empty",
			input:    []string{"", "", ""},
			expected: []string{},
		},
		{
			name:     "nil input",
			input:    nil,
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeFunctions(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSplitKnownAndUnknown(t *testing.T) {
	known := map[string]struct{}{
		"fn1": {},
		"fn2": {},
		"fn3": {},
	}

	t.Run("all known", func(t *testing.T) {
		functions := []string{"fn1", "fn2", "fn3"}
		accepted, unknown := splitKnownAndUnknown(functions, known)
		assert.ElementsMatch(t, []string{"fn1", "fn2", "fn3"}, accepted)
		assert.Nil(t, unknown)
	})

	t.Run("some unknown", func(t *testing.T) {
		functions := []string{"fn1", "unknown1", "fn2", "unknown2"}
		accepted, unknown := splitKnownAndUnknown(functions, known)
		assert.ElementsMatch(t, []string{"fn1", "fn2"}, accepted)
		assert.ElementsMatch(t, []string{"unknown1", "unknown2"}, unknown)
	})

	t.Run("all unknown", func(t *testing.T) {
		functions := []string{"unknown1", "unknown2"}
		accepted, unknown := splitKnownAndUnknown(functions, known)
		assert.Nil(t, accepted)
		assert.ElementsMatch(t, []string{"unknown1", "unknown2"}, unknown)
	})

	t.Run("nil known map returns all as accepted", func(t *testing.T) {
		functions := []string{"fn1", "unknown1"}
		accepted, unknown := splitKnownAndUnknown(functions, nil)
		assert.ElementsMatch(t, []string{"fn1", "unknown1"}, accepted)
		assert.Nil(t, unknown)
	})

	t.Run("empty known map returns all as accepted", func(t *testing.T) {
		functions := []string{"fn1", "unknown1"}
		accepted, unknown := splitKnownAndUnknown(functions, map[string]struct{}{})
		assert.ElementsMatch(t, []string{"fn1", "unknown1"}, accepted)
		assert.Nil(t, unknown)
	})

	t.Run("unknown are sorted", func(t *testing.T) {
		functions := []string{"fn1", "zebra", "fn2", "apple"}
		_, unknown := splitKnownAndUnknown(functions, known)
		if unknown != nil {
			assert.Equal(t, []string{"apple", "zebra"}, unknown)
		}
	})
}

func TestDiffFunctionsExtended(t *testing.T) {
	t.Run("additions and removals", func(t *testing.T) {
		before := []string{"a", "b", "c"}
		after := []string{"b", "c", "d"}
		added, removed := diffFunctions(before, after)
		assert.ElementsMatch(t, []string{"d"}, added)
		assert.ElementsMatch(t, []string{"a"}, removed)
	})

	t.Run("only additions", func(t *testing.T) {
		before := []string{"a", "b"}
		after := []string{"a", "b", "c"}
		added, removed := diffFunctions(before, after)
		assert.ElementsMatch(t, []string{"c"}, added)
		assert.Empty(t, removed)
	})

	t.Run("only removals", func(t *testing.T) {
		before := []string{"a", "b", "c"}
		after := []string{"a", "b"}
		added, removed := diffFunctions(before, after)
		assert.Empty(t, added)
		assert.ElementsMatch(t, []string{"c"}, removed)
	})

	t.Run("no change", func(t *testing.T) {
		before := []string{"a", "b", "c"}
		after := []string{"a", "b", "c"}
		added, removed := diffFunctions(before, after)
		assert.Empty(t, added)
		assert.Empty(t, removed)
	})

	t.Run("empty slices", func(t *testing.T) {
		before := []string{}
		after := []string{}
		added, removed := diffFunctions(before, after)
		assert.Empty(t, added)
		assert.Empty(t, removed)
	})

	t.Run("from empty to some", func(t *testing.T) {
		before := []string{}
		after := []string{"a", "b"}
		added, removed := diffFunctions(before, after)
		assert.ElementsMatch(t, []string{"a", "b"}, added)
		assert.Empty(t, removed)
	})

	t.Run("duplicates handled", func(t *testing.T) {
		before := []string{"a", "a", "b"}
		after := []string{"b", "b", "c"}
		added, removed := diffFunctions(before, after)
		assert.ElementsMatch(t, []string{"c"}, added)
		// removed contains all elements from before not in afterSet
		// Since before has "a" twice and "a" is not in afterSet, removed will have "a" twice
		assert.Contains(t, removed, "a")
		assert.NotContains(t, removed, "b")
	})
}

func TestCollectKnownFunctions(t *testing.T) {
	t.Run("nil context returns nil", func(t *testing.T) {
		result := collectKnownFunctions(nil)
		assert.Nil(t, result)
	})

	t.Run("context without registry store returns nil", func(t *testing.T) {
		svcCtx := &svc.ServiceContext{}
		result := collectKnownFunctions(svcCtx)
		assert.Nil(t, result)
	})

	t.Run("context with nil registry store returns nil", func(t *testing.T) {
		svcCtx := &svc.ServiceContext{RegistryStore: nil}
		result := collectKnownFunctions(svcCtx)
		assert.Nil(t, result)
	})
}

func TestAssignmentsHistoryPath(t *testing.T) {
	t.Run("relative assignments path", func(t *testing.T) {
		base := t.TempDir()
		svcCtx := &svc.ServiceContext{
			Config: config.Config{
				BootstrapData: config.BootstrapDataConfig{BaseDir: base},
				Registry:      config.RegistryConfig{AssignmentsPath: "assignments.json"},
			},
		}
		historyPath := assignmentHistoryPath(svcCtx)
		assert.Equal(t, filepath.Join(base, "assignments_history.json"), historyPath)
	})

	t.Run("absolute assignments path", func(t *testing.T) {
		// Use temp directory to ensure cross-platform absolute path
		tmpDir := t.TempDir()
		absPath := filepath.Join(tmpDir, "absolute", "path", "assignments.json")
		// Create the directory structure to ensure it's a valid path
		os.MkdirAll(filepath.Dir(absPath), 0755)

		svcCtx := &svc.ServiceContext{
			Config: config.Config{
				Registry: config.RegistryConfig{AssignmentsPath: absPath},
			},
		}
		historyPath := assignmentHistoryPath(svcCtx)
		expectedHistoryPath := filepath.Join(tmpDir, "absolute", "path", "assignments_history.json")
		assert.Equal(t, expectedHistoryPath, historyPath)
	})

	t.Run("nested directory", func(t *testing.T) {
		base := t.TempDir()
		svcCtx := &svc.ServiceContext{
			Config: config.Config{
				BootstrapData: config.BootstrapDataConfig{BaseDir: base},
				Registry:      config.RegistryConfig{AssignmentsPath: filepath.Join("x", "y", "assignments.json")},
			},
		}
		historyPath := assignmentHistoryPath(svcCtx)
		expectedDir := filepath.Join(base, "x", "y")
		assert.Equal(t, filepath.Join(expectedDir, "assignments_history.json"), historyPath)
	})
}

func TestLoadAssignmentHistory(t *testing.T) {
	t.Run("nonexistent file returns empty slice", func(t *testing.T) {
		result, err := loadAssignmentHistory("/nonexistent/history.json")
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Empty(t, result)
	})

	t.Run("empty file returns empty slice", func(t *testing.T) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, "empty.json")
		err := os.WriteFile(path, []byte{}, 0o644)
		require.NoError(t, err)

		result, err := loadAssignmentHistory(path)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Empty(t, result)
	})
}

func TestSaveAssignmentHistory(t *testing.T) {
	t.Run("save and load", func(t *testing.T) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, "history.json")

		entries := []assignmentHistoryEntry{
			{ID: "1", GameID: "game1", Action: "assign"},
			{ID: "2", GameID: "game2", Action: "remove"},
		}

		err := saveAssignmentHistory(path, entries)
		assert.NoError(t, err)

		loaded, err := loadAssignmentHistory(path)
		assert.NoError(t, err)
		assert.Len(t, loaded, 2)
		assert.Equal(t, "game1", loaded[0].GameID)
		assert.Equal(t, "game2", loaded[1].GameID)
	})
}

func TestAssignmentsListLogic(t *testing.T) {
	svcCtx, ctx := setupTestContext(t)

	t.Run("successful list", func(t *testing.T) {
		logic := NewAssignmentsListLogic(ctx, svcCtx)
		resp, err := logic.AssignmentsList(&AssignmentsListRequest{
			GameId:   "game1",
			Env:      "prod",
			Page:     1,
			PageSize: 10,
		})
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, 0, resp.Code)
		assert.Equal(t, "OK", resp.Message)
	})
}

func TestAssignmentsHistoryLogic(t *testing.T) {
	svcCtx, ctx := setupTestContext(t)

	t.Run("successful history retrieval", func(t *testing.T) {
		// First add some history
		err := appendAssignmentHistory(svcCtx, assignmentHistoryEntry{
			GameID:     "game1",
			Env:        "prod",
			FunctionID: "all",
			Action:     "assign",
			Count:      2,
			OperatedBy: "testadmin",
		})
		require.NoError(t, err)

		logic := NewAssignmentsHistoryLogic(ctx, svcCtx)
		resp, err := logic.AssignmentsHistory(&AssignmentsHistoryRequest{
			GameId:   "game1",
			Env:      "prod",
			Action:   "assign",
			Page:     1,
			PageSize: 10,
		})
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, 0, resp.Code)
	})

	t.Run("pagination defaults", func(t *testing.T) {
		logic := NewAssignmentsHistoryLogic(ctx, svcCtx)
		resp, err := logic.AssignmentsHistory(&AssignmentsHistoryRequest{
			Page:     0, // Should default to 1
			PageSize: 0, // Should default to 20
		})
		assert.NoError(t, err)
		data, ok := resp.Data.(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, 1, data["page"])
		assert.Equal(t, 20, data["pageSize"])
	})

	t.Run("max page size", func(t *testing.T) {
		logic := NewAssignmentsHistoryLogic(ctx, svcCtx)
		resp, err := logic.AssignmentsHistory(&AssignmentsHistoryRequest{
			PageSize: 200, // Should be capped at 100
		})
		assert.NoError(t, err)
		data, ok := resp.Data.(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, 100, data["pageSize"])
	})
}

func TestLoadAllAssignments(t *testing.T) {
	svcCtx, _ := setupTestContext(t)

	// Create test assignments
	path := assignmentsPath(svcCtx)
	assignments := map[string][]string{
		"game1|prod": {"fn1", "fn2"},
	}
	err := saveAssignments(path, assignments)
	require.NoError(t, err)

	result, err := LoadAllAssignments(svcCtx)
	assert.NoError(t, err)
	assert.Equal(t, assignments, result)
}
