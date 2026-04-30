// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

package migrate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/svc"
)

func TestNowDurationMS(t *testing.T) {
	t.Parallel()

	start := time.Now()
	time.Sleep(10 * time.Millisecond)
	result := nowDurationMS(start)

	if result == "" {
		t.Error("nowDurationMS() should not return empty string")
	}
	// Result should contain "ms" or similar duration indicator
	if len(result) < 2 {
		t.Errorf("nowDurationMS() result too short: %q", result)
	}
}

func TestMigrateHistoryPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		ctx       *svc.ServiceContext
		prefix    string
		wantsBase bool
	}{
		{
			name:      "nil context",
			ctx:       nil,
			prefix:    "data",
			wantsBase: true,
		},
		{
			name: "context with custom base dir",
			ctx: &svc.ServiceContext{
				Config: config.Config{
					BootstrapData: config.BootstrapDataConfig{
						BaseDir: "custom/path",
					},
				},
			},
			prefix:    "custom/path",
			wantsBase: true,
		},
		{
			name: "context with empty base dir uses default",
			ctx: &svc.ServiceContext{
				Config: config.Config{
					BootstrapData: config.BootstrapDataConfig{
						BaseDir: "",
					},
				},
			},
			prefix:    "data",
			wantsBase: true,
		},
		{
			name: "context with whitespace base dir uses default",
			ctx: &svc.ServiceContext{
				Config: config.Config{
					BootstrapData: config.BootstrapDataConfig{
						BaseDir: "   ",
					},
				},
			},
			prefix:    "data",
			wantsBase: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := migrateHistoryPath(tt.ctx)
			if tt.wantsBase {
				if !filepath.HasPrefix(filepath.Clean(got), filepath.Clean(tt.prefix)) {
					t.Errorf("migrateHistoryPath() = %q, should start with %q", got, tt.prefix)
				}
			}
			if filepath.Base(got) != "migrate_history.json" {
				t.Errorf(`migrateHistoryPath() basename should be "migrate_history.json", got %q`, filepath.Base(got))
			}
		})
	}
}

func TestLoadSaveMigrateHistoryRoundtrip(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "history.json")

	original := []MigrationResult{
		{Name: "migrate1", Version: "v1", StartTime: "2025-01-01T00:00:00Z", EndTime: "2025-01-01T00:00:01Z", Status: "success"},
		{Name: "migrate2", Version: "v2", StartTime: "2025-01-02T00:00:00Z", EndTime: "2025-01-02T00:00:01Z", Status: "failed", Error: "test error"},
	}

	// Save
	err := saveMigrateHistory(path, original)
	if err != nil {
		t.Fatalf("saveMigrateHistory() error = %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("saveMigrateHistory() did not create file")
	}

	// Load
	loaded, err := loadMigrateHistory(path)
	if err != nil {
		t.Fatalf("loadMigrateHistory() error = %v", err)
	}

	// Verify
	if len(loaded) != len(original) {
		t.Errorf("loadMigrateHistory() length = %d, want %d", len(loaded), len(original))
	}
	for i := range original {
		if loaded[i].Name != original[i].Name {
			t.Errorf("loaded[%d].Name = %q, want %q", i, loaded[i].Name, original[i].Name)
		}
		if loaded[i].Status != original[i].Status {
			t.Errorf("loaded[%d].Status = %q, want %q", i, loaded[i].Status, original[i].Status)
		}
	}
}

func TestLoadMigrateHistoryNotExist(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "nonexistent.json")

	loaded, err := loadMigrateHistory(path)
	if err != nil {
		t.Fatalf("loadMigrateHistory() on nonexistent file should not error, got %v", err)
	}
	if loaded == nil {
		t.Error("loadMigrateHistory() should return empty slice, not nil")
	}
	if len(loaded) != 0 {
		t.Errorf("loadMigrateHistory() on nonexistent file should return empty slice, got %d items", len(loaded))
	}
}

func TestLoadMigrateHistoryEmptyFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "empty.json")

	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatalf("failed to create empty file: %v", err)
	}

	loaded, err := loadMigrateHistory(path)
	if err != nil {
		t.Fatalf("loadMigrateHistory() on empty file should not error, got %v", err)
	}
	if len(loaded) != 0 {
		t.Errorf("loadMigrateHistory() on empty file should return empty slice, got %d items", len(loaded))
	}
}

func TestLoadMigrateHistoryInvalidJSON(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "invalid.json")

	if err := os.WriteFile(path, []byte("{invalid json"), 0644); err != nil {
		t.Fatalf("failed to create invalid json file: %v", err)
	}

	_, err := loadMigrateHistory(path)
	if err == nil {
		t.Error("loadMigrateHistory() on invalid JSON should error")
	}
}

func TestSaveMigrateHistoryCreatesDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "subdir", "inner", "history.json")

	data := []MigrationResult{{Name: "test", Status: "success"}}
	err := saveMigrateHistory(path, data)
	if err != nil {
		t.Fatalf("saveMigrateHistory() error = %v", err)
	}

	// Verify file was created in nested directory
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("saveMigrateHistory() did not create nested directories")
	}
}

func TestSaveMigrateHistoryFormat(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "history.json")

	data := []MigrationResult{
		{Name: "migrate1", Status: "success", StartTime: "2025-01-01T00:00:00Z"},
	}
	err := saveMigrateHistory(path, data)
	if err != nil {
		t.Fatalf("saveMigrateHistory() error = %v", err)
	}

	// Read file content
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	// Verify it's valid JSON
	var parsed []MigrationResult
	if err := json.Unmarshal(content, &parsed); err != nil {
		t.Errorf("saveMigrateHistory() did not create valid JSON: %v", err)
	}

	// Verify formatted with indentation (contains spaces/newlines)
	if string(content) == jsonContentWithoutIndent(data) {
		t.Error("saveMigrateHistory() should format JSON with indentation")
	}
}

func jsonContentWithoutIndent(data []MigrationResult) string {
	b, _ := json.Marshal(data)
	return string(b)
}

func TestAppendMigrateHistory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	ctx := &svc.ServiceContext{
		Config: config.Config{
			BootstrapData: config.BootstrapDataConfig{
				BaseDir: tmpDir,
			},
		},
	}

	// First append
	results1 := []MigrationResult{
		{Name: "migrate1", Status: "success", StartTime: "2025-01-01T00:00:00Z"},
	}
	err := appendMigrateHistory(ctx, results1)
	if err != nil {
		t.Fatalf("appendMigrateHistory() first append error = %v", err)
	}

	// Second append
	results2 := []MigrationResult{
		{Name: "migrate2", Status: "success", StartTime: "2025-01-02T00:00:00Z"},
	}
	err = appendMigrateHistory(ctx, results2)
	if err != nil {
		t.Fatalf("appendMigrateHistory() second append error = %v", err)
	}

	// Load and verify - results2 should be first (prepended)
	path := migrateHistoryPath(ctx)
	loaded, err := loadMigrateHistory(path)
	if err != nil {
		t.Fatalf("loadMigrateHistory() error = %v", err)
	}

	if len(loaded) != 2 {
		t.Fatalf("loaded history length = %d, want 2", len(loaded))
	}
	// Newest should be first
	if loaded[0].Name != "migrate2" {
		t.Errorf("first entry name = %q, want migrate2", loaded[0].Name)
	}
	if loaded[1].Name != "migrate1" {
		t.Errorf("second entry name = %q, want migrate1", loaded[1].Name)
	}
}

func TestAppendMigrateHistoryTruncatesAt500(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	ctx := &svc.ServiceContext{
		Config: config.Config{
			BootstrapData: config.BootstrapDataConfig{
				BaseDir: tmpDir,
			},
		},
	}

	// Create initial history with 499 items
	initial := make([]MigrationResult, 499)
	for i := 0; i < 499; i++ {
		initial[i] = MigrationResult{Name: "old", Status: "success"}
	}
	path := migrateHistoryPath(ctx)
	if err := saveMigrateHistory(path, initial); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// Append 2 items - total should be 500 (old 499 + 2 new, truncated)
	newItems := []MigrationResult{
		{Name: "new1", Status: "success"},
		{Name: "new2", Status: "success"},
	}
	err := appendMigrateHistory(ctx, newItems)
	if err != nil {
		t.Fatalf("appendMigrateHistory() error = %v", err)
	}

	loaded, err := loadMigrateHistory(path)
	if err != nil {
		t.Fatalf("loadMigrateHistory() error = %v", err)
	}

	if len(loaded) != 500 {
		t.Errorf("after append with truncation, length = %d, want 500", len(loaded))
	}
	// Newest items should be at the beginning
	if loaded[0].Name != "new1" {
		t.Errorf("first item name = %q, want new1", loaded[0].Name)
	}
	if loaded[1].Name != "new2" {
		t.Errorf("second item name = %q, want new2", loaded[1].Name)
	}
}
