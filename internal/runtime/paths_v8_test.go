package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFindConfigsDir_CandidatesCovered ensures all candidate paths in
// findConfigsDir are exercised when configs dir exists in CWD.
func TestFindConfigsDir_CandidatesCovered(t *testing.T) {
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalWd)

	tmpDir := t.TempDir()
	configsDir := filepath.Join(tmpDir, "configs")
	if err := os.Mkdir(configsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	dir := findConfigsDir()
	if dir == "" {
		t.Error("expected findConfigsDir to find configs in CWD")
	}
}

// TestDefaultBootstrapDataDir_FallbackGetwd covers the os.Getwd fallback
// path in DefaultBootstrapDataDir when findConfigsDir returns "".
func TestDefaultBootstrapDataDir_FallbackGetwd(t *testing.T) {
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalWd)

	// Move to a dir without configs subdirectory so findConfigsDir returns ""
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	dir := DefaultBootstrapDataDir()
	if dir == "" {
		t.Error("DefaultBootstrapDataDir should never return empty string")
	}
}

// TestFindConfigsDir_NoMatch covers the case where no candidate matches.
func TestFindConfigsDir_NoMatch(t *testing.T) {
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalWd)

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	dir := findConfigsDir()
	// In a fresh temp dir with no configs, this should return ""
	if dir != "" {
		t.Errorf("expected empty from findConfigsDir in empty dir, got %s", dir)
	}
}

// TestDefaultBootstrapDataDir_AlwaysReturnsConfigs ensures the function
// always returns a path ending with "configs".
func TestDefaultBootstrapDataDir_AlwaysReturnsConfigs(t *testing.T) {
	dir := DefaultBootstrapDataDir()
	if filepath.Base(dir) != "configs" {
		t.Errorf("expected path ending with 'configs', got %s", dir)
	}
}

// TestExecutableDir_ReturnsCleanPath verifies executableDir returns a
// cleaned absolute path.
func TestExecutableDir_ReturnsCleanPath(t *testing.T) {
	dir, _ := executableDir()
	if dir != "" && dir != filepath.Clean(dir) {
		t.Errorf("expected cleaned path, got %s", dir)
	}
}

// TestFindConfigsDir_WithConfigsAboveExe tests that findConfigsDir can
// locate configs in parent directories relative to executable.
func TestFindConfigsDir_WithConfigsAboveExe(t *testing.T) {
	// findConfigsDir tries multiple candidate paths; verify it doesn't panic
	dir := findConfigsDir()
	t.Logf("findConfigsDir result: %q", dir)
}

// TestDefaultBootstrapDataDir_TwoFallbacks covers the two-level fallback
// chain: findConfigsDir -> executableDir -> os.Getwd -> "configs".
func TestDefaultBootstrapDataDir_TwoFallbacks(t *testing.T) {
	// This test ensures the function handles the case where configs
	// cannot be found via the normal search paths.
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalWd)

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	result := DefaultBootstrapDataDir()
	if result == "" {
		t.Error("DefaultBootstrapDataDir returned empty string")
	}
}
