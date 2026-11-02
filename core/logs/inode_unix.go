//go:build !windows

package logs

import (
	"os"
	"syscall"
)

// inodeOf returns the file's inode (unix). Used for logrotate detection.
func inodeOf(info os.FileInfo) uint64 {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return uint64(st.Ino)
	}
	return 0
}
