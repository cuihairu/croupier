package configsource

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
)

// gitSource browses a Git repository as a directory tree. Read-only:
// 改 Git 必须走项目组的 MR 流程，平台不提供应急写。
//
// Config: {"repoUrl", "branch", "subPath", "username", "password"}.
type gitSource struct {
	repoURL  string
	branch   string
	subPath  string
	username string
	password string

	mu      sync.Mutex
	cached  billy.Filesystem // 浅克隆的内存工作区
	cachedE error
	expires time.Time
}

func newGitSource(cfg map[string]interface{}) (Source, error) {
	repoURL := configString(cfg, "repoUrl", "")
	if repoURL == "" {
		return nil, fmt.Errorf("git source requires repoUrl")
	}
	return &gitSource{
		repoURL:  repoURL,
		branch:   configString(cfg, "branch", ""),
		subPath:  configString(cfg, "subPath", ""),
		username: configString(cfg, "username", ""),
		password: configString(cfg, "password", ""),
	}, nil
}

func (s *gitSource) Type() string { return "git" }

// worktree returns a cached in-memory shallow clone (TTL 60s) so consecutive
// tree/read calls do not re-clone.
func (s *gitSource) worktree(ctx context.Context) (billy.Filesystem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Now().Before(s.expires) && (s.cached != nil || s.cachedE != nil) {
		return s.cached, s.cachedE
	}
	opts := &git.CloneOptions{
		URL:   s.repoURL,
		Depth: 1,
	}
	if s.branch != "" {
		opts.ReferenceName = plumbing.NewBranchReferenceName(s.branch)
		opts.SingleBranch = true
	}
	if s.username != "" || s.password != "" {
		opts.Auth = &http.BasicAuth{Username: s.username, Password: s.password}
	}
	wt := memfs.New()
	_, err := git.CloneContext(ctx, memory.NewStorage(), wt, opts)
	if err != nil {
		s.cached, s.cachedE = nil, fmt.Errorf("git clone failed: %w", err)
		s.expires = time.Now().Add(10 * time.Second) // 失败短 TTL，避免连续重试打爆远端
		return nil, s.cachedE
	}
	s.cached, s.cachedE = wt, nil
	s.expires = time.Now().Add(60 * time.Second)
	return s.cached, nil
}

func (s *gitSource) List(ctx context.Context, dir string) ([]Entry, error) {
	dir, err := cleanPath(dir)
	if err != nil {
		return nil, err
	}
	wt, err := s.worktree(ctx)
	if err != nil {
		return nil, err
	}
	full := joinSub(s.subPath, dir)
	infos, err := wt.ReadDir(full)
	if err != nil {
		return nil, fmt.Errorf("read dir %q: %w", dir, err)
	}
	out := make([]Entry, 0, len(infos))
	for _, info := range infos {
		if info.IsDir() && info.Name() == ".git" {
			continue
		}
		rel := joinSub(dir, info.Name())
		out = append(out, Entry{
			Name:    info.Name(),
			Path:    rel,
			Dir:     info.IsDir(),
			Size:    sizeOf(info),
			ModTime: info.ModTime(),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Dir != out[j].Dir {
			return out[i].Dir
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func (s *gitSource) Read(ctx context.Context, path string) ([]byte, error) {
	path, err := cleanPath(path)
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, fmt.Errorf("path required")
	}
	wt, err := s.worktree(ctx)
	if err != nil {
		return nil, err
	}
	f, err := wt.Open(joinSub(s.subPath, path))
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", path, err)
	}
	defer f.Close()
	buf := make([]byte, 0, 64*1024)
	tmp := make([]byte, 32*1024)
	for {
		n, err := f.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if err != nil {
			break
		}
		if len(buf) > 8<<20 {
			return nil, fmt.Errorf("file too large (>8MiB)")
		}
	}
	return buf, nil
}

func joinSub(base, rel string) string {
	if base == "" {
		return rel
	}
	if rel == "" {
		return base
	}
	return strings.TrimSuffix(base, "/") + "/" + rel
}

func sizeOf(info fs.FileInfo) int64 {
	if info.IsDir() {
		return 0
	}
	return info.Size()
}
