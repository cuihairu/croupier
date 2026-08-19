package fsutil

import (
	"os"
	"path/filepath"
)

// WriteFileAtomic 写文件采用「同目录临时文件 + rename」原子替换。
//
// 直接 os.WriteFile 会以 truncate 方式打开已存在文件；若旧文件属主与
// 当前进程不同（例如容器曾被 root 身份写入过），打开即 EACCES。rename
// 只要求目录可写，同时保证读者永远不会观察到半截内容。
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+"-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) //nolint:errcheck // rename 成功后清理失败无影响
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
