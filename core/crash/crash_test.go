package crash

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInjectProfiles(t *testing.T) {
	// none / 空 = 不注入
	inj, err := Inject(ProfileNone, "")
	require.NoError(t, err)
	assert.Empty(t, inj.Env)
	assert.Empty(t, inj.Args)
	inj, err = Inject("", "/d")
	require.NoError(t, err)
	assert.Empty(t, inj.Env)

	// go：GOTRACEBACK=crash
	inj, err = Inject(ProfileGo, "/d")
	require.NoError(t, err)
	assert.Equal(t, []string{"GOTRACEBACK=crash"}, inj.Env)
	assert.Empty(t, inj.Args)

	// node：报告关 env + 落盘目录
	inj, err = Inject(ProfileNode, "/dumps/node")
	require.NoError(t, err)
	assert.Equal(t, []string{"--report-on-fatalerror", "--report-exclude-env", "--report-directory=/dumps/node"}, inj.Args)

	// python：faulthandler
	inj, err = Inject(ProfilePython, "")
	require.NoError(t, err)
	assert.Equal(t, []string{"PYTHONFAULTHANDLER=1"}, inj.Env)

	// jvm：HeapDump 落盘目录
	inj, err = Inject(ProfileJVM, "/dumps/jvm")
	require.NoError(t, err)
	assert.Equal(t, []string{"-XX:+HeapDumpOnOutOfMemoryError", "-XX:HeapDumpPath=/dumps/jvm"}, inj.Args)

	// node/jvm 缺 dumpDir 拒绝；未知档位拒绝
	_, err = Inject(ProfileNode, "")
	assert.Error(t, err)
	_, err = Inject(ProfileJVM, "")
	assert.Error(t, err)
	_, err = Inject(Profile("rust"), "/d")
	assert.Error(t, err)
}

func TestDumpStoreListAndDirFor(t *testing.T) {
	root := t.TempDir()
	s := NewDumpStore(root)
	assert.Equal(t, root, s.Root())
	assert.Equal(t, filepath.Join(root, "sleeper"), s.DirFor("sleeper"))

	// 目录不存在 = 空表 nil 错
	snaps, err := s.List("sleeper")
	require.NoError(t, err)
	assert.Empty(t, snaps)

	dir := s.DirFor("sleeper")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "core.1"), []byte("x"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "core.2"), []byte("yy"), 0o644))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "subdir"), 0o755)) // 子目录跳过

	snaps, err = s.List("sleeper")
	require.NoError(t, err)
	require.Len(t, snaps, 2)
	for _, sn := range snaps {
		assert.NotEmpty(t, sn.Name)
		assert.Greater(t, sn.Size, int64(0))
	}
}

func TestDumpStoreSweepRetentionAndQuota(t *testing.T) {
	s := NewDumpStore(t.TempDir())
	dir := s.DirFor("sleeper")
	require.NoError(t, os.MkdirAll(dir, 0o755))

	mk := func(name string, size int64, age time.Duration) {
		p := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(p, make([]byte, size), 0o644))
		old := time.Now().Add(-age)
		require.NoError(t, os.Chtimes(p, old, old))
	}
	mk("ancient.core", 10, 100*time.Hour) // 超 72h 保留期
	mk("newest.core", 700, time.Hour)
	mk("older.core", 700, 60*time.Hour)
	// 总量 1400 + 10；配额 1000 → 最旧的 newer 文件先被删

	removed, err := s.Sweep("sleeper", 72*time.Hour, 1000)
	require.NoError(t, err)
	assert.Equal(t, 2, removed, "ancient (age) + older (quota) removed")

	snaps, err := s.List("sleeper")
	require.NoError(t, err)
	require.Len(t, snaps, 1)
	assert.Equal(t, "newest.core", snaps[0].Name)
}

func TestDumpStoreSweepDefaultsAndMissingDir(t *testing.T) {
	s := NewDumpStore(t.TempDir())
	removed, err := s.Sweep("ghost", 0, 0) // 缺目录 no-op + 默认档不炸
	require.NoError(t, err)
	assert.Equal(t, 0, removed)

	// 默认档（retention/quota 为 0 → DefaultRetention/DefaultQuota）
	dir := s.DirFor("sleeper")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "fresh.core"), []byte("x"), 0o644))
	removed, err = s.Sweep("sleeper", 0, 0)
	require.NoError(t, err)
	assert.Equal(t, 0, removed, "fresh file survives default retention")
}
