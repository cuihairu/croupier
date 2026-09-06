// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

package helper

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
)

func TestResolveAnalyticsFiltersPath(t *testing.T) {
	tests := []struct {
		name        string
		schemasDir  string
		filtersPath string
		wantAbs     bool
		wantEnd     string
	}{
		{
			name:        "explicit absolute path",
			schemasDir:  "",
			filtersPath: "/absolute/path/filters.json",
			wantAbs:     true,
			wantEnd:     "/absolute/path/filters.json",
		},
		{
			name:        "relative to schemas dir",
			schemasDir:  "/schemas",
			filtersPath: "",
			wantAbs:     true,
			wantEnd:     "/schemas/analytics_filters.json",
		},
		{
			name:        "empty filters path and schemas dir",
			schemasDir:  "",
			filtersPath: "",
			wantAbs:     false,
			wantEnd:     "analytics_filters.json",
		},
		{
			name:        "relative path becomes absolute",
			schemasDir:  "",
			filtersPath: "relative/filters.json",
			wantAbs:     true,
			wantEnd:     "filters.json", // The abs path will vary by system
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Config{}
			cfg.Schemas.Dir = tt.schemasDir
			cfg.Registry.AnalyticsFiltersPath = tt.filtersPath

			result := ResolveAnalyticsFiltersPath(cfg)

			if tt.wantAbs && !filepath.IsAbs(result) {
				t.Errorf("ResolveAnalyticsFiltersPath() = %v, want absolute path", result)
			}
			if tt.wantEnd != "" && result != tt.wantEnd {
				// For relative paths, just check if it ends with expected
				if filepath.IsAbs(tt.wantEnd) {
					if result != tt.wantEnd {
						t.Errorf("ResolveAnalyticsFiltersPath() = %v, want %v", result, tt.wantEnd)
					}
				}
			}
		})
	}
}

func TestReadAnalyticsFiltersFile(t *testing.T) {
	t.Run("non-existent file returns default", func(t *testing.T) {
		data, err := ReadAnalyticsFiltersFile("/nonexistent/path/file.json")
		if err != nil {
			t.Fatalf("ReadAnalyticsFiltersFile() error = %v", err)
		}
		if string(data) != `{"items":[]}` {
			t.Errorf("ReadAnalyticsFiltersFile() = %s, want '{\"items\":[]}'", string(data))
		}
	})

	t.Run("empty path returns empty", func(t *testing.T) {
		data, err := ReadAnalyticsFiltersFile("")
		if err != nil {
			t.Fatalf("ReadAnalyticsFiltersFile() error = %v", err)
		}
		if len(data) != 0 {
			t.Errorf("ReadAnalyticsFiltersFile() = %v, want empty slice", data)
		}
	})

	t.Run("whitespace path returns empty", func(t *testing.T) {
		data, err := ReadAnalyticsFiltersFile("   ")
		if err != nil {
			t.Fatalf("ReadAnalyticsFiltersFile() error = %v", err)
		}
		if len(data) != 0 {
			t.Errorf("ReadAnalyticsFiltersFile() = %v, want empty slice", data)
		}
	})

	t.Run("read existing file", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.json")
		testContent := `{"items":[{"id":"test"}]}`

		if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		data, err := ReadAnalyticsFiltersFile(testFile)
		if err != nil {
			t.Fatalf("ReadAnalyticsFiltersFile() error = %v", err)
		}
		if string(data) != testContent {
			t.Errorf("ReadAnalyticsFiltersFile() = %s, want %s", string(data), testContent)
		}
	})
}

func TestWriteAnalyticsFiltersFile(t *testing.T) {
	t.Run("write and read back", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "write_test.json")
		testContent := []byte(`{"test":"data"}`)

		err := WriteAnalyticsFiltersFile(testFile, testContent)
		if err != nil {
			t.Fatalf("WriteAnalyticsFiltersFile() error = %v", err)
		}

		data, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("Failed to read written file: %v", err)
		}

		if string(data) != string(testContent) {
			t.Errorf("File content = %s, want %s", string(data), string(testContent))
		}
	})

	t.Run("empty path does nothing", func(t *testing.T) {
		err := WriteAnalyticsFiltersFile("", []byte(`{}`))
		if err != nil {
			t.Errorf("WriteAnalyticsFiltersFile() empty path should not error, got = %v", err)
		}
	})

	t.Run("whitespace path does nothing", func(t *testing.T) {
		err := WriteAnalyticsFiltersFile("   ", []byte(`{}`))
		if err != nil {
			t.Errorf("WriteAnalyticsFiltersFile() whitespace path should not error, got = %v", err)
		}
	})

	t.Run("creates parent directories", func(t *testing.T) {
		tmpDir := t.TempDir()
		nestedFile := filepath.Join(tmpDir, "parent", "child", "test.json")
		testContent := []byte(`{"nested":true}`)

		err := WriteAnalyticsFiltersFile(nestedFile, testContent)
		if err != nil {
			t.Fatalf("WriteAnalyticsFiltersFile() error = %v", err)
		}

		data, err := os.ReadFile(nestedFile)
		if err != nil {
			t.Fatalf("Failed to read nested file: %v", err)
		}

		if string(data) != string(testContent) {
			t.Errorf("Nested file content = %s, want %s", string(data), string(testContent))
		}
	})
}

func TestLoadAndSaveAnalyticsFilters(t *testing.T) {
	t.Run("load and save roundtrip", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "roundtrip.json")
		original := []byte(`{"items":[1,2,3]}`)

		// Save
		err := SaveAnalyticsFilters(testFile, original)
		if err != nil {
			t.Fatalf("SaveAnalyticsFilters() error = %v", err)
		}

		// Load
		loaded, err := LoadAnalyticsFilters(testFile)
		if err != nil {
			t.Fatalf("LoadAnalyticsFilters() error = %v", err)
		}

		if string(loaded) != string(original) {
			t.Errorf("LoadAnalyticsFilters() = %s, want %s", string(loaded), string(original))
		}
	})
}

func TestAnalyticsFiltersFileIOErrors(t *testing.T) {
	// Read：路径是目录（EISDIR，非 NotExist）→ 原样返回错误
	if _, err := ReadAnalyticsFiltersFile(t.TempDir()); err == nil {
		t.Fatal("expected read error for directory path")
	}

	// Write：父目录是一个已存在的普通文件 → MkdirAll 失败
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteAnalyticsFiltersFile(filepath.Join(blocker, "sub", "f.json"), []byte("{}")); err == nil {
		t.Fatal("expected MkdirAll failure")
	}
}
