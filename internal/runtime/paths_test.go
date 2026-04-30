// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExecutableDir(t *testing.T) {
	dir, err := executableDir()
	if err != nil {
		t.Logf("executableDir() error = %v (expected in some test environments)", err)
	}
	if dir == "" && err == nil {
		t.Error("executableDir() returned empty string with no error")
	}
	if dir != "" {
		t.Logf("executableDir() = %s", dir)
		if !filepath.IsAbs(dir) {
			t.Errorf("executableDir() = %s, want absolute path", dir)
		}
	}
}

func TestFindConfigsDir(t *testing.T) {
	// This test may find an existing configs directory or return empty
	dir := findConfigsDir()
	t.Logf("findConfigsDir() = %s", dir)

	if dir != "" {
		// If a directory was found, it should be valid
		if info, err := os.Stat(dir); err != nil {
			t.Errorf("findConfigsDir() returned invalid path: %v", err)
		} else if !info.IsDir() {
			t.Error("findConfigsDir() returned path that is not a directory")
		}
		if !filepath.IsAbs(dir) {
			t.Errorf("findConfigsDir() = %s, want absolute path", dir)
		}
	}
}

func TestDefaultBootstrapDataDir(t *testing.T) {
	dir := DefaultBootstrapDataDir()
	t.Logf("DefaultBootstrapDataDir() = %s", dir)

	if dir == "" {
		t.Error("DefaultBootstrapDataDir() returned empty string")
	}

	// The result should always be a valid path format
	// (may not exist, but should be well-formed)
	if filepath.Clean(dir) != dir {
		t.Errorf("DefaultBootstrapDataDir() = %s, not cleaned", dir)
	}

	// Should end with "configs"
	if filepath.Base(dir) != "configs" {
		t.Errorf("DefaultBootstrapDataDir() = %s, should end with 'configs'", dir)
	}
}

func TestDefaultBootstrapDataDirInTempDir(t *testing.T) {
	// Create a temporary directory with a configs subdirectory
	tmpDir := t.TempDir()
	configsDir := filepath.Join(tmpDir, "configs")
	if err := os.Mkdir(configsDir, 0755); err != nil {
		t.Fatalf("Failed to create test configs dir: %v", err)
	}

	// Change to the temp directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp dir: %v", err)
	}

	// Note: DefaultBootstrapDataDir uses os.Executable which we can't easily mock,
	// so we just verify the function doesn't panic
	dir := DefaultBootstrapDataDir()
	if dir == "" {
		t.Error("DefaultBootstrapDataDir() returned empty string")
	}
	t.Logf("DefaultBootstrapDataDir() in temp dir = %s", dir)
}
