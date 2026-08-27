package configsource

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// newGitFixture creates a local repo:
// gameplay/item.json, gameplay/hero.json, runtime/switch.yaml, README.md
func newGitFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"gameplay/item.json":  `{"id":1}`,
		"gameplay/hero.json":  `{"id":2}`,
		"runtime/switch.yaml": "on: true",
		"README.md":           "# demo",
	}
	for path, content := range files {
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := wt.Add(path); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := wt.Commit("init", &git.CommitOptions{
		Author: &object.Signature{Name: "t", Email: "t@t"},
	}); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestGitSource_ListRead(t *testing.T) {
	dir := newGitFixture(t)
	src, err := New(testBinding("git", fmt.Sprintf(`{"repoUrl":%q}`, dir)))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	root, err := src.List(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	dirs := map[string]bool{}
	for _, e := range root {
		if e.Dir {
			dirs[e.Name] = true
		}
	}
	if !dirs["gameplay"] || !dirs["runtime"] {
		t.Errorf("root = %+v", root)
	}

	sub, err := src.List(ctx, "gameplay")
	if err != nil {
		t.Fatal(err)
	}
	if len(sub) != 2 {
		t.Errorf("gameplay = %+v", sub)
	}

	val, err := src.Read(ctx, "runtime/switch.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if string(val) != "on: true" {
		t.Errorf("read = %q", val)
	}

	// git 永远只读
	if IsWritable(src) {
		t.Errorf("git source must not be writable")
	}

	// 路径穿越拒绝
	if _, err := src.Read(ctx, "../../etc/passwd"); err == nil {
		t.Errorf("traversal must be rejected")
	}
}

func TestGitSource_RequireRepoURL(t *testing.T) {
	if _, err := New(testBinding("git", `{}`)); err == nil {
		t.Errorf("repoUrl required")
	}
}

func TestValidateGitURL(t *testing.T) {
	ok := []string{
		"https://github.com/org/repo.git",
		"http://git.internal/game/configs.git",
		"file:///mnt/nas/configs",
		"/data/repos/configs", // 本地路径（空 scheme）
	}
	for _, u := range ok {
		if err := validateGitURL(u); err != nil {
			t.Errorf("validateGitURL(%q) = %v, want nil", u, err)
		}
	}
	bad := []struct {
		url  string
		want string
	}{
		{"ssh://git@github.com/org/repo.git", "scheme"},
		{"ftp://x/repo.git", "scheme"},
		{"https://user:pass@github.com/org/repo.git", "userinfo"},
		{"https://", "host"},
	}
	for _, c := range bad {
		err := validateGitURL(c.url)
		if err == nil {
			t.Errorf("validateGitURL(%q) = nil, want error containing %q", c.url, c.want)
		}
	}
}
