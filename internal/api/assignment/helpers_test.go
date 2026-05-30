// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

package assignment

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/svc"
)

func TestSplitAssignmentKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		key        string
		wantGameID string
		wantEnv    string
	}{
		{
			name:       "full key with game and env",
			key:        "game1|production",
			wantGameID: "game1",
			wantEnv:    "production",
		},
		{
			name:       "key with only game",
			key:        "game2",
			wantGameID: "game2",
			wantEnv:    "",
		},
		{
			name:       "key with empty env",
			key:        "game3|",
			wantGameID: "game3",
			wantEnv:    "",
		},
		{
			name:       "key with whitespace",
			key:        " game4 | dev ",
			wantGameID: "game4",
			wantEnv:    "dev",
		},
		{
			name:       "empty key",
			key:        "",
			wantGameID: "",
			wantEnv:    "",
		},
		{
			name:       "multiple separators - only first split",
			key:        "game5|dev|extra",
			wantGameID: "game5",
			wantEnv:    "dev|extra",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gameID, env := splitAssignmentKey(tt.key)
			if gameID != tt.wantGameID {
				t.Errorf("gameID = %q, want %q", gameID, tt.wantGameID)
			}
			if env != tt.wantEnv {
				t.Errorf("env = %q, want %q", env, tt.wantEnv)
			}
		})
	}
}

func TestBuildAssignmentKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		gameID  string
		env     string
		wantKey string
	}{
		{
			name:    "normal key",
			gameID:  "game1",
			env:     "production",
			wantKey: "game1|production",
		},
		{
			name:    "empty env",
			gameID:  "game2",
			env:     "",
			wantKey: "game2|",
		},
		{
			name:    "whitespace trimmed",
			gameID:  " game3 ",
			env:     " dev ",
			wantKey: "game3|dev",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildAssignmentKey(tt.gameID, tt.env)
			if got != tt.wantKey {
				t.Errorf("buildAssignmentKey() = %q, want %q", got, tt.wantKey)
			}
		})
	}
}

func TestCloneAssignments(t *testing.T) {
	t.Parallel()

	original := map[string][]string{
		"game1|prod": {"func1", "func2"},
		"game2|dev":  {"func3"},
	}

	cloned := cloneAssignments(original)

	// Check values are equal
	if len(cloned) != len(original) {
		t.Errorf("clone length = %d, want %d", len(cloned), len(original))
	}

	// Modify clone and check original is unchanged
	cloned["game1|prod"][0] = "modified"
	if original["game1|prod"][0] != "func1" {
		t.Error("Modifying clone should not affect original")
	}

	// Add to clone and check original
	cloned["new"] = []string{"newfunc"}
	if _, ok := original["new"]; ok {
		t.Error("Adding to clone should not affect original")
	}
}

func TestNormalizeFunctions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "already normalized",
			input:    []string{"func1", "func2"},
			expected: []string{"func1", "func2"},
		},
		{
			name:     "whitespace trimmed",
			input:    []string{" func1 ", "  func2  "},
			expected: []string{"func1", "func2"},
		},
		{
			name:     "empty strings removed",
			input:    []string{"func1", "", "func2", "  "},
			expected: []string{"func1", "func2"},
		},
		{
			name:     "duplicates removed",
			input:    []string{"func1", "func2", "func1", "func3", "func2"},
			expected: []string{"func1", "func2", "func3"},
		},
		{
			name:     "empty input",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "all empty",
			input:    []string{"", "  ", "\t"},
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
			got := normalizeFunctions(tt.input)
			if len(got) != len(tt.expected) {
				t.Errorf("normalizeFunctions() length = %d, want %d", len(got), len(tt.expected))
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("normalizeFunctions()[%d] = %q, want %q", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestSplitKnownAndUnknown(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		functions    []string
		known        map[string]struct{}
		wantAccepted []string
		wantUnknown  []string
	}{
		{
			name:         "all known",
			functions:    []string{"func1", "func2"},
			known:        map[string]struct{}{"func1": {}, "func2": {}},
			wantAccepted: []string{"func1", "func2"},
			wantUnknown:  nil,
		},
		{
			name:         "all unknown",
			functions:    []string{"func1", "func2"},
			known:        map[string]struct{}{},
			wantAccepted: []string{"func1", "func2"},
			wantUnknown:  nil,
		},
		{
			name:         "mixed known and unknown",
			functions:    []string{"func1", "unknown1", "func2", "unknown2"},
			known:        map[string]struct{}{"func1": {}, "func2": {}},
			wantAccepted: []string{"func1", "func2"},
			wantUnknown:  []string{"unknown1", "unknown2"},
		},
		{
			name:         "empty known map returns all as accepted",
			functions:    []string{"z_func", "a_func", "m_func"},
			known:        map[string]struct{}{},
			wantAccepted: []string{"z_func", "a_func", "m_func"},
			wantUnknown:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accepted, unknown := splitKnownAndUnknown(tt.functions, tt.known)

			// Check accepted
			if len(accepted) != len(tt.wantAccepted) {
				t.Errorf("accepted length = %d, want %d", len(accepted), len(tt.wantAccepted))
			} else {
				for i := range accepted {
					if accepted[i] != tt.wantAccepted[i] {
						t.Errorf("accepted[%d] = %q, want %q", i, accepted[i], tt.wantAccepted[i])
					}
				}
			}

			// Check unknown
			if len(tt.wantUnknown) == 0 {
				if unknown != nil {
					t.Errorf("unknown = %v, want nil", unknown)
				}
			} else {
				if len(unknown) != len(tt.wantUnknown) {
					t.Errorf("unknown length = %d, want %d", len(unknown), len(tt.wantUnknown))
				} else {
					for i := range unknown {
						if unknown[i] != tt.wantUnknown[i] {
							t.Errorf("unknown[%d] = %q, want %q", i, unknown[i], tt.wantUnknown[i])
						}
					}
				}
			}
		})
	}
}

func TestDiffFunctions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		before      []string
		after       []string
		wantAdded   []string
		wantRemoved []string
	}{
		{
			name:        "no changes",
			before:      []string{"func1", "func2"},
			after:       []string{"func1", "func2"},
			wantAdded:   nil,
			wantRemoved: nil,
		},
		{
			name:        "functions added",
			before:      []string{"func1"},
			after:       []string{"func1", "func2", "func3"},
			wantAdded:   []string{"func2", "func3"},
			wantRemoved: nil,
		},
		{
			name:        "functions removed",
			before:      []string{"func1", "func2", "func3"},
			after:       []string{"func1"},
			wantAdded:   nil,
			wantRemoved: []string{"func2", "func3"},
		},
		{
			name:        "both added and removed",
			before:      []string{"func1", "func2"},
			after:       []string{"func1", "func3"},
			wantAdded:   []string{"func3"},
			wantRemoved: []string{"func2"},
		},
		{
			name:        "complete replacement",
			before:      []string{"func1", "func2"},
			after:       []string{"func3", "func4"},
			wantAdded:   []string{"func3", "func4"},
			wantRemoved: []string{"func1", "func2"},
		},
		{
			name:        "empty before",
			before:      []string{},
			after:       []string{"func1"},
			wantAdded:   []string{"func1"},
			wantRemoved: nil,
		},
		{
			name:        "empty after",
			before:      []string{"func1"},
			after:       []string{},
			wantAdded:   nil,
			wantRemoved: []string{"func1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			added, removed := diffFunctions(tt.before, tt.after)

			// Check added
			if len(tt.wantAdded) == 0 {
				if added != nil {
					t.Errorf("added = %v, want nil", added)
				}
			} else {
				if len(added) != len(tt.wantAdded) {
					t.Errorf("added length = %d, want %d", len(added), len(tt.wantAdded))
				} else {
					for i := range added {
						if added[i] != tt.wantAdded[i] {
							t.Errorf("added[%d] = %q, want %q", i, added[i], tt.wantAdded[i])
						}
					}
				}
			}

			// Check removed
			if len(tt.wantRemoved) == 0 {
				if removed != nil {
					t.Errorf("removed = %v, want nil", removed)
				}
			} else {
				if len(removed) != len(tt.wantRemoved) {
					t.Errorf("removed length = %d, want %d", len(removed), len(tt.wantRemoved))
				} else {
					for i := range removed {
						if removed[i] != tt.wantRemoved[i] {
							t.Errorf("removed[%d] = %q, want %q", i, removed[i], tt.wantRemoved[i])
						}
					}
				}
			}
		})
	}
}

func TestFilterAssignments(t *testing.T) {
	t.Parallel()

	data := map[string][]string{
		"game1|prod": {"func1", "func2"},
		"game1|dev":  {"func3"},
		"game2|prod": {"func4"},
		"game3|dev":  {"func5"},
	}

	tests := []struct {
		name          string
		gameID        string
		env           string
		expectedCount int
		expectedKeys  []string
	}{
		{
			name:          "no filter",
			gameID:        "",
			env:           "",
			expectedCount: 4,
			expectedKeys:  []string{"game1|prod", "game1|dev", "game2|prod", "game3|dev"},
		},
		{
			name:          "filter by game",
			gameID:        "game1",
			env:           "",
			expectedCount: 2,
			expectedKeys:  []string{"game1|prod", "game1|dev"},
		},
		{
			name:          "filter by env",
			gameID:        "",
			env:           "prod",
			expectedCount: 2,
			expectedKeys:  []string{"game1|prod", "game2|prod"},
		},
		{
			name:          "filter by both",
			gameID:        "game1",
			env:           "prod",
			expectedCount: 1,
			expectedKeys:  []string{"game1|prod"},
		},
		{
			name:          "case insensitive match",
			gameID:        "GAME1",
			env:           "PROD",
			expectedCount: 1,
			expectedKeys:  []string{"game1|prod"},
		},
		{
			name:          "no matches",
			gameID:        "nonexistent",
			env:           "prod",
			expectedCount: 0,
			expectedKeys:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterAssignments(data, tt.gameID, tt.env)
			if len(result) != tt.expectedCount {
				t.Errorf("filterAssignments() count = %d, want %d", len(result), tt.expectedCount)
			}
			for _, key := range tt.expectedKeys {
				if _, ok := result[key]; !ok {
					t.Errorf("filterAssignments() missing key %q", key)
				}
			}
		})
	}
}

func TestFilterAssignmentsClone(t *testing.T) {
	t.Parallel()

	data := map[string][]string{
		"game1|prod": {"func1"},
	}

	result := filterAssignments(data, "", "")

	// Modify result and check original is not affected
	result["game1|prod"][0] = "modified"
	if data["game1|prod"][0] != "func1" {
		t.Error("Modifying filtered result should not affect original")
	}
}

func TestAssignmentsPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ctx      *svc.ServiceContext
		expected string
	}{
		{
			name:     "nil context",
			ctx:      nil,
			expected: filepath.Join("data", "assignments.json"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := assignmentsPath(tt.ctx)
			if got != tt.expected {
				t.Errorf("assignmentsPath() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestAssignmentsPathWithConfig(t *testing.T) {
	t.Parallel()

	ctx := &svc.ServiceContext{
		Config: config.Config{
			Registry: config.RegistryConfig{
				AssignmentsPath: "custom/path/assignments.json",
			},
		},
	}

	got := assignmentsPath(ctx)
	if !strings.Contains(got, "custom") || !strings.Contains(got, "assignments.json") {
		t.Errorf("assignmentsPath() with config = %q, should contain custom path", got)
	}
}

func TestAssignmentHistoryPath(t *testing.T) {
	t.Parallel()

	ctx := &svc.ServiceContext{
		Config: config.Config{
			Registry: config.RegistryConfig{
				AssignmentsPath: "/absolute/path/assignments.json",
			},
		},
	}

	got := assignmentHistoryPath(ctx)
	if !strings.Contains(got, "assignments_history.json") {
		t.Errorf("assignmentHistoryPath() = %q, should contain assignments_history.json", got)
	}
	// Should be in same directory as assignments.json
	if filepath.Dir(got) != filepath.Dir("/absolute/path/assignments.json") {
		t.Errorf("assignmentHistoryPath() dir = %q, should be same as assignments.json dir", filepath.Dir(got))
	}
}

func TestLoadSaveAssignmentsRoundtrip(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "assignments.json")

	// Save
	original := map[string][]string{
		"game1|prod": {"func1", "func2"},
		"game2|dev":  {"func3"},
	}
	if err := saveAssignments(path, original); err != nil {
		t.Fatalf("saveAssignments() error = %v", err)
	}

	// Load
	loaded, err := loadAssignments(path)
	if err != nil {
		t.Fatalf("loadAssignments() error = %v", err)
	}

	// Verify
	if len(loaded) != len(original) {
		t.Errorf("loaded count = %d, want %d", len(loaded), len(original))
	}
	for key, functions := range original {
		if got, ok := loaded[key]; !ok {
			t.Errorf("loaded missing key %q", key)
		} else if len(got) != len(functions) {
			t.Errorf("loaded[%q] length = %d, want %d", key, len(got), len(functions))
		}
	}
}

func TestLoadAssignmentsNotExist(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "nonexistent.json")

	loaded, err := loadAssignments(path)
	if err != nil {
		t.Fatalf("loadAssignments() on nonexistent file should not error, got %v", err)
	}
	if loaded == nil {
		t.Error("loadAssignments() should return empty map, not nil")
	}
	if len(loaded) != 0 {
		t.Errorf("loadAssignments() on nonexistent file should return empty map, got %d items", len(loaded))
	}
}

func TestLoadAssignmentsEmptyFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "empty.json")

	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatalf("failed to create empty file: %v", err)
	}

	loaded, err := loadAssignments(path)
	if err != nil {
		t.Fatalf("loadAssignments() on empty file should not error, got %v", err)
	}
	if len(loaded) != 0 {
		t.Errorf("loadAssignments() on empty file should return empty map, got %d items", len(loaded))
	}
}

func TestLoadAssignmentHistoryRoundtrip(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "history.json")

	// Save
	original := []assignmentHistoryEntry{
		{ID: "1", FunctionID: "func1", Action: "add"},
		{ID: "2", FunctionID: "func2", Action: "remove"},
	}
	if err := saveAssignmentHistory(path, original); err != nil {
		t.Fatalf("saveAssignmentHistory() error = %v", err)
	}

	// Load
	loaded, err := loadAssignmentHistory(path)
	if err != nil {
		t.Fatalf("loadAssignmentHistory() error = %v", err)
	}

	// Verify
	if len(loaded) != len(original) {
		t.Errorf("loaded count = %d, want %d", len(loaded), len(original))
	}
}

func TestLoadAssignmentHistoryNotExist(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "nonexistent.json")

	loaded, err := loadAssignmentHistory(path)
	if err != nil {
		t.Fatalf("loadAssignmentHistory() on nonexistent file should not error, got %v", err)
	}
	if loaded == nil {
		t.Error("loadAssignmentHistory() should return empty slice, not nil")
	}
	if len(loaded) != 0 {
		t.Errorf("loadAssignmentHistory() on nonexistent file should return empty slice, got %d items", len(loaded))
	}
}
