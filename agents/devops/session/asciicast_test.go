package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// C-010：sessionID 进 filepath.Join 前过白名单 [A-Za-z0-9_-]。
// 合法 id 照常落 dir；非法 id（含 ../、/、\、空白、分号）在 OpenCast 即拒，
// 不得在 dir 外创建文件（路径穿越潜伏面）。
func TestOpenCast_RejectsPathTraversalSessionID(t *testing.T) {
	dir := t.TempDir()
	hdr := CastHeader{Width: 80, Height: 24}

	valid := []string{"sess-1", "abc123", "A_B-c", "session_2026_11"}
	for _, id := range valid {
		w, err := OpenCast(dir, id, hdr, time.Now())
		require.NoError(t, err, "valid id %q", id)
		require.FileExists(t, filepath.Join(dir, id+".cast"))
		require.NoError(t, w.Close())
	}

	invalid := []string{
		"", "../evil", "a/b", `a\b`, "a..b", "a b", "a;b", "..", ".", "a*b", "a?b", "a:b",
	}
	for _, id := range invalid {
		w, err := OpenCast(dir, id, hdr, time.Now())
		require.Error(t, err, "invalid id %q should be rejected", id)
		assert.Nil(t, w)
		if id != "" {
			assert.Contains(t, err.Error(), "invalid characters", "id %q", id)
		}
	}

	// 路径穿越不得在 dir 之外落文件：父目录不应出现 evil.cast / b.cast。
	parent := filepath.Dir(dir)
	assert.NoFileExists(t, filepath.Join(parent, "evil.cast"))
	assert.NoFileExists(t, filepath.Join(parent, "b.cast"))
	assert.NoFileExists(t, filepath.Join(parent, "..", "evil.cast"))
}

// C-010：castFileName 直构也保持纯拼接（校验在 OpenCast 入口，不依赖调用方）。
func TestCastFileName_AppendsExtension(t *testing.T) {
	assert.Equal(t, "sess-1.cast", castFileName("sess-1"))
}

// C-010：validCastSessionID 白名单本身可独立断言（含空串拒）。
func TestValidCastSessionID_Allowlist(t *testing.T) {
	for _, ok := range []string{"sess-1", "abc", "A_B-c", "0123456789abcdef"} {
		assert.True(t, validCastSessionID.MatchString(ok), "should match %q", ok)
	}
	for _, bad := range []string{"", "../", "a/b", `a\b`, "a b", "a;b", "a.b", "a..b"} {
		assert.False(t, validCastSessionID.MatchString(bad), "should reject %q", bad)
	}
}

// C-010：OpenCast 对 dir 为空仍报错（既有语义不破）。
func TestOpenCast_RequiresDir(t *testing.T) {
	_, err := OpenCast("", "sess-1", CastHeader{Version: 2}, time.Now())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cast dir is required")
}

// C-010：合法 id 落盘后 header 行存在（回放格式不破）。
func TestOpenCast_WritesHeader(t *testing.T) {
	dir := t.TempDir()
	w, err := OpenCast(dir, "sess-hdr", CastHeader{Version: 2, Width: 80, Height: 24}, time.Now())
	require.NoError(t, err)
	require.NoError(t, w.Close())
	raw, err := os.ReadFile(filepath.Join(dir, "sess-hdr.cast"))
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"version":2`)
	assert.Contains(t, string(raw), `"width":80`)
}
