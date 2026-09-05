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

// restoreExecutable 在测试结束后恢复 osExecutable 注入点与工作目录。
func restoreExecutable(t *testing.T) {
	t.Helper()
	orig := osExecutable
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		osExecutable = orig
		_ = os.Chdir(origWd)
	})
}

func TestFindConfigsDirFromExecutableParents(t *testing.T) {
	restoreExecutable(t)

	// exe 位于 tmp/x/bin/，configs 位于 tmp/configs → 命中 ../.. 级候选
	root := t.TempDir()
	binDir := filepath.Join(root, "x", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "configs"), 0o755); err != nil {
		t.Fatal(err)
	}
	osExecutable = func() (string, error) { return filepath.Join(binDir, "croupier-test"), nil }
	if err := os.Chdir(t.TempDir()); err != nil { // wd 无 configs，隔离干扰
		t.Fatal(err)
	}

	got := findConfigsDir()
	want := filepath.Clean(filepath.Join(root, "configs"))
	if got != want {
		t.Fatalf("findConfigsDir() = %q, want %q", got, want)
	}
}

func TestFindConfigsDirExecutableErrorUsesWD(t *testing.T) {
	restoreExecutable(t)

	// os.Executable 失败：仅探测 wd 候选
	osExecutable = func() (string, error) { return "", errTestInject }
	wd := t.TempDir()
	if err := os.Chdir(wd); err != nil {
		t.Fatal(err)
	}
	if got := findConfigsDir(); got != "" {
		t.Fatalf("findConfigsDir() = %q, want empty", got)
	}
	if dir := DefaultBootstrapDataDir(); dir != filepath.Join(wd, "configs") {
		t.Fatalf("DefaultBootstrapDataDir() = %q, want wd fallback %q", dir, filepath.Join(wd, "configs"))
	}
}

func TestDefaultBootstrapDataDirFinalFallback(t *testing.T) {
	restoreExecutable(t)

	// os.Executable 与 os.Getwd 双双失败 → 最终字面量 "configs"
	osExecutable = func() (string, error) { return "", errTestInject }
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	// 工作目录被删除后 getcwd 失败（Linux 上直接系统调用返回 ENOENT）
	if err := os.Remove(tmp); err != nil {
		t.Fatal(err)
	}
	if got := DefaultBootstrapDataDir(); got != "configs" {
		t.Fatalf("DefaultBootstrapDataDir() = %q, want literal %q", got, "configs")
	}
}

func TestExecutableDirError(t *testing.T) {
	restoreExecutable(t)
	osExecutable = func() (string, error) { return "", errTestInject }
	dir, err := executableDir()
	if err == nil {
		t.Fatal("executableDir() should fail with injected error")
	}
	if dir != "" {
		t.Fatalf("executableDir() = %q, want empty", dir)
	}
}

var errTestInject = errTest{}

type errTest struct{}

func (errTest) Error() string { return "injected test error" }
