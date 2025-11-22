package svc

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	workRootOnce sync.Once
	workRootPath string

	serverRootOnce sync.Once
	serverRootPath string
)

func workspaceRoot() string {
	workRootOnce.Do(func() {
		workRootPath = detectWorkspaceRoot()
	})
	return workRootPath
}

func detectWorkspaceRoot() string {
	candidates := []string{}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, wd)
	}
	if exe, err := os.Executable(); err == nil {
		if exedir, err := filepath.EvalSymlinks(filepath.Dir(exe)); err == nil {
			candidates = append(candidates, exedir)
		} else {
			candidates = append(candidates, filepath.Dir(exe))
		}
	}
	for _, start := range candidates {
		dir := start
		for {
			if dir == "" {
				break
			}
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				return dir
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return ""
}

func serverRoot() string {
	serverRootOnce.Do(func() {
		if root := workspaceRoot(); root != "" {
			candidate := filepath.Join(root, "services", "server")
			if fi, err := os.Stat(candidate); err == nil && fi.IsDir() {
				serverRootPath = candidate
				return
			}
		}
		if wd, err := os.Getwd(); err == nil {
			serverRootPath = wd
			return
		}
		serverRootPath = "."
	})
	return serverRootPath
}

// ResolveWorkspacePath returns an absolute path rooted at the repository root.
func ResolveWorkspacePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	if root := workspaceRoot(); root != "" {
		return filepath.Clean(filepath.Join(root, p))
	}
	if abs, err := filepath.Abs(p); err == nil {
		return filepath.Clean(abs)
	}
	return filepath.Clean(p)
}

// ResolveServerPath returns an absolute path rooted at the server package directory.
func ResolveServerPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	root := serverRoot()
	return filepath.Clean(filepath.Join(root, p))
}
