//go:build windows

package agent

import "os"

// writeFileNoFollow is the Windows fallback. O_NOFOLLOW is unavailable; the
// preceding EvalSymlinks(absTarget) containment check remains, and the Windows
// deployment surface is documented separately.
func writeFileNoFollow(path string, content []byte, mode os.FileMode) error {
	return os.WriteFile(path, content, mode)
}
