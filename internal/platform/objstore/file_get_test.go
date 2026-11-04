package objstore

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFileStore_Get_RoundTrip 验证 Get 读路径返回 Put 写入的原始内容。
func TestFileStore_Get_RoundTrip(t *testing.T) {
	st := newFileStore(t)
	ctx := context.Background()
	payload := []byte("SELECT 1;\n-- file round-trip\n")

	require.NoError(t, st.Put(ctx, "backups/demo.sql", strings.NewReader(string(payload)), int64(len(payload)), "application/sql"))

	r, err := st.Get(ctx, "backups/demo.sql")
	require.NoError(t, err)
	defer r.Close()
	got, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Equal(t, payload, got)
}

// TestFileStore_Get_NotFound 验证不存在的对象返回包装 os.ErrNotExist 的错误。
func TestFileStore_Get_NotFound(t *testing.T) {
	st := newFileStore(t)
	_, err := st.Get(context.Background(), "missing.sql")
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err), "expected os.ErrNotExist, got %v", err)
}

// TestFileStore_Get_PathTraversalNeutralized 验证 Get 与 Put/Delete 同一道
// 路径闸：sanitizeKey 把 ".." 段解析回 base 目录内（../../etc/passwd →
// etc/passwd），系统上真实存在的 /etc/passwd 不会被读取。若穿越生效，
// os.Open 会成功读到真实文件；此处必须失败且为 not-exist。
func TestFileStore_Get_PathTraversalNeutralized(t *testing.T) {
	st := newFileStore(t)
	_, err := st.Get(context.Background(), "../../etc/passwd")
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err), "traversal key must resolve inside base (not-exist), got %v", err)
}
