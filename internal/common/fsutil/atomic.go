package fsutil

import (
	"os"
	"path/filepath"
)

// WriteFileAtomic writes files using atomic replacement via a temp file in the
// same directory plus rename.
//
// A direct os.WriteFile opens an existing file in truncate mode; if the old
// file's owner differs from the current process (for example the container was
// once written to as root), the open itself fails with EACCES. rename only
// requires the directory to be writable, while also guaranteeing that readers
// never observe partial content.
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
	defer os.Remove(tmpName) //nolint:errcheck // a cleanup failure after a successful rename has no effect
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
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
