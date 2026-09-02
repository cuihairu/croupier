// 覆盖目标：固化 DefaultBootstrapDataDir/findConfigsDir 在工作目录被
// 删除（os.Getwd 失败）场景下的降级行为——executableDir 仍可用时回退
// 到可执行文件目录下的 configs，函数永不返回空串。
// 注：paths.go 中依赖 os.Executable() 本身失败的 4 条兜底语句
// （DefaultBootstrapDataDir 的 wd/"configs" 兜底与 executableDir 的
// 错误分支）在 Linux 测试进程内不可达（/proc/self/exe 总可读）。
package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultBootstrapDataDir_DeletedWorkdir(t *testing.T) {
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalWd)

	// 进入一个随即被删除的目录：os.Getwd() 将返回 ENOENT。
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(tmpDir); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Getwd(); err == nil {
		t.Skip("os.Getwd still succeeds after cwd removal; cannot exercise fallback")
	}

	dir := DefaultBootstrapDataDir()
	if dir == "" {
		t.Fatal("DefaultBootstrapDataDir returned empty string with deleted cwd")
	}
	if filepath.Base(dir) != "configs" {
		t.Errorf("expected path ending with 'configs', got %s", dir)
	}
	// executableDir 成功时兜底路径是可执行文件目录下的 configs。
	if exe, err := os.Executable(); err == nil {
		want := filepath.Join(filepath.Dir(exe), "configs")
		if dir != want {
			t.Errorf("DefaultBootstrapDataDir() = %s, want executable-relative fallback %s", dir, want)
		}
	}

	if got := findConfigsDir(); got != "" {
		t.Logf("findConfigsDir with deleted cwd = %s", got)
	}
}
