//go:build windows

package logs

import "os"

// inodeOf returns 0 on Windows (inode is unreliable); rotation detection falls
// back to size-based logic in the tailer.
func inodeOf(info os.FileInfo) uint64 { return 0 }
