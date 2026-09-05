package runtime

import (
	"os"
	"path/filepath"
)

// DefaultBootstrapDataDir attempts to locate the configs directory relative to the
// current executable or working directory. It always returns a usable path even
// if the directory is missing so callers can override as needed.
func DefaultBootstrapDataDir() string {
	if dir := findConfigsDir(); dir != "" {
		return dir
	}

	if execDir, err := executableDir(); err == nil {
		return filepath.Join(execDir, "configs")
	}

	if wd, err := os.Getwd(); err == nil {
		return filepath.Join(wd, "configs")
	}

	return "configs"
}

func findConfigsDir() string {
	candidates := []string{}

	if execDir, err := executableDir(); err == nil {
		candidates = append(candidates,
			filepath.Join(execDir, "configs"),
			filepath.Join(execDir, "..", "configs"),
			filepath.Join(execDir, "..", "..", "configs"),
		)
	}

	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(wd, "configs"),
		)
	}

	for _, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return filepath.Clean(dir)
		}
	}

	return ""
}

// osExecutable 便于测试注入的 os.Executable 别名。
var osExecutable = os.Executable

func executableDir() (string, error) {
	exe, err := osExecutable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(exe), nil
}
