//go:build !windows

package agent

import (
	"os"
	"syscall"
)

// writeFileNoFollow writes content to path without following a symlink at the
// final component. O_NOFOLLOW makes the open fail with ELOOP if the target is
// (or becomes, in the check→write window) a symlink, closing the TOCTOU gap
// that would otherwise let a pre-planted link redirect the write outside the
// PushBaseDir.
func writeFileNoFollow(path string, content []byte, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC|syscall.O_NOFOLLOW, mode)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = f.Write(content)
	return err
}
