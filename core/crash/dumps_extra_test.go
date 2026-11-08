package crash

// dumps.go 补测：List 的 ReadDir 非「不存在」错误臂（根目录位被文件占据
// → ENOTDIR，不落 IsNotExist 短路，错误透传）。
//
// 诚实跳过：List 的 e.Info() 失败臂（dumps.go:52，轮询间隙被清理的竞态
// 容错）——需在 ReadDir 返回与逐条 Info 之间的窗口内删除条目，无确定性
// 注入缝，竞态注入不稳定性反而违背测试可信度。

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDumpStoreListReadDirError(t *testing.T) {
	occupied := filepath.Join(t.TempDir(), "occupied")
	require.NoError(t, os.WriteFile(occupied, []byte("x"), 0o600))

	s := NewDumpStore(occupied)
	snaps, err := s.List("proc")
	require.Error(t, err, "ENOTDIR must surface instead of the not-exist short-circuit")
	require.Nil(t, snaps)
}
