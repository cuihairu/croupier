package runtime

// coverage_v9_test.go 补充 paths.go 的候选目录判定边界：
// 名为 configs 的普通文件应被跳过、符号链接指向目录应被接受、
// executableDir 结果应稳定且为绝对路径。
//
// 注：paths.go 中依赖 os.Executable() 返回错误的 4 条兜底语句
// （DefaultBootstrapDataDir 的 Getwd/"configs" 兜底与 executableDir 的
// 错误分支）在 Linux 测试进程内不可达：Go 1.26 的 os.Executable 即使
// 在二进制被删除后也仅裁剪 " (deleted)" 后缀而不报错，且 /proc/self/exe
// 始终可读。因此本包覆盖率的实际上限为 80.0%（16/20）。

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFindConfigsDir_IgnoresFileNamedConfigsV9 确认名为 configs 的普通文件
// 不会被视为配置目录，函数继续回退。
func TestFindConfigsDir_IgnoresFileNamedConfigsV9(t *testing.T) {
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalWd)

	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "configs"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	if got := findConfigsDir(); got != "" {
		t.Errorf("findConfigsDir() = %q, want empty when configs is a regular file", got)
	}
}

// TestFindConfigsDir_FollowsSymlinkV9 确认指向目录的符号链接被接受。
func TestFindConfigsDir_FollowsSymlinkV9(t *testing.T) {
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalWd)

	tmpDir := t.TempDir()
	realDir := filepath.Join(tmpDir, "real-configs")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realDir, filepath.Join(tmpDir, "configs")); err != nil {
		t.Skipf("symlink creation failed: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	got := findConfigsDir()
	if got == "" {
		t.Fatal("findConfigsDir() = empty, want symlinked configs dir to be accepted")
	}
	if filepath.Base(got) != "configs" {
		t.Errorf("findConfigsDir() = %q, want base name 'configs'", got)
	}
}

// TestDefaultBootstrapDataDir_FileConfigsFallsBackV9 确认 configs 为普通文件时
// DefaultBootstrapDataDir 回退到可执行文件目录。
func TestDefaultBootstrapDataDir_FileConfigsFallsBackV9(t *testing.T) {
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalWd)

	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "configs"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	dir := DefaultBootstrapDataDir()
	if dir == "" {
		t.Fatal("DefaultBootstrapDataDir() returned empty string")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Skipf("os.Executable failed: %v", err)
	}
	want := filepath.Join(filepath.Dir(exe), "configs")
	if dir != want {
		t.Errorf("DefaultBootstrapDataDir() = %q, want executable fallback %q", dir, want)
	}
}

// TestExecutableDir_IdempotentV9 确认重复调用结果一致且为绝对路径。
func TestExecutableDir_IdempotentV9(t *testing.T) {
	first, err := executableDir()
	if err != nil {
		t.Skipf("executableDir() error = %v", err)
	}
	second, err := executableDir()
	if err != nil {
		t.Fatalf("second executableDir() error = %v", err)
	}
	if first != second {
		t.Errorf("executableDir() not stable: %q vs %q", first, second)
	}
	if !filepath.IsAbs(first) {
		t.Errorf("executableDir() = %q, want absolute path", first)
	}
}
