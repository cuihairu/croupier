package logs

// tailer.go / inode_unix.go 补测：Drain 对目录路径跳过臂、readLine 的
// bufio.ErrBufferFull 续读臂（超 64KB 行）、clip 的短行/超长切边/max<3
// 兜底臂，以及 inodeOf 的 *syscall.Stat_t 命中与 Sys 非该类型的回落臂。
//
// 诚实跳过：drainFile 的 Seek 错误臂（tailer.go:118-119）——正则文件上
// Seek 仅在 fd 已关闭或偏移为负时失败，而 offset 恒非负且文件刚被打开，
// 经公有入口无可注入缝；os.Open 失败臂（97-99）由 TestMissingFileSkipped
// 覆盖（路径不存在即 open 失败返回 nil）。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDrainSkipsDirectoryPath(t *testing.T) {
	dir := t.TempDir()
	tr := NewTailer([]string{dir}, 0, 0)
	require.Empty(t, tr.Drain())
}

func TestReadLineBufferFull(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	// 超 64KB 无换行前缀行 → ReadSlice 触发 ErrBufferFull 续读
	long := strings.Repeat("a", 100_000) + "\n"
	require.NoError(t, os.WriteFile(path, []byte(long), 0o644))

	got := NewTailer([]string{path}, 0, 0).Drain()
	require.Len(t, got, 1, "buffer-full line must still surface as one entry")
	require.Len(t, got[0].Message, DefaultMaxLineBytes)
}

func TestClipEdgeArms(t *testing.T) {
	// 长度未超 → 原样
	require.Equal(t, "abc", clip("abc", 5))
	// 恰好等于上限 → 原样
	require.Equal(t, "abc", clip("abc", 3))
	// 超长且 max>=3 → 切到 rune 边界 + 省略号
	require.Equal(t, "ab...", clip("abcdef", 5))
	// max<3 → 兜底为 0，仅省略号（186-188 臂）
	require.Equal(t, "...", clip("abcdef", 2))
	require.Equal(t, "...", clip("abcdef", 0))
}

func TestInodeOfArms(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.log")
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o600))
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.NotZero(t, inodeOf(info), "real file stat must yield inode")

	// Sys() 非 *syscall.Stat_t → 回落 0
	require.Zero(t, inodeOf(fakeSysFileInfo{sys: "not-a-stat"}))
	require.Zero(t, inodeOf(fakeSysFileInfo{sys: nil}))
}

// fakeSysFileInfo 仅用于让 inodeOf 走 Sys() 类型断言失败分支。
type fakeSysFileInfo struct{ sys any }

func (f fakeSysFileInfo) Name() string       { return "f" }
func (f fakeSysFileInfo) Size() int64        { return 0 }
func (f fakeSysFileInfo) Mode() os.FileMode  { return 0 }
func (f fakeSysFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeSysFileInfo) IsDir() bool        { return false }
func (f fakeSysFileInfo) Sys() any           { return f.sys }
