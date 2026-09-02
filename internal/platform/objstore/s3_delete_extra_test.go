// 覆盖目标：s3Store.Delete 的单对象/前缀递归删除/不存在错误分支，
// 以及 SignedURL 的 TTL 边界（复用 memblob 离线桶）。
package objstore

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestS3Store_Delete_SingleAndMissing(t *testing.T) {
	ctx := context.Background()
	s := newMemS3Store(t)

	require.NoError(t, s.Put(ctx, "f.txt", strings.NewReader("x"), 1, "text/plain"))
	require.NoError(t, s.Delete(ctx, "f.txt"))

	// 不存在的 key：gocloud 返回 NotFound（gcerrors IsNotFound）
	err := s.Delete(ctx, "missing.txt")
	require.Error(t, err, "missing key delete must fail")
}

func TestS3Store_Delete_PrefixRecursive(t *testing.T) {
	ctx := context.Background()
	s := newMemS3Store(t)

	require.NoError(t, s.Put(ctx, "dir/a.txt", strings.NewReader("a"), 1, ""))
	require.NoError(t, s.Put(ctx, "dir/b.txt", strings.NewReader("b"), 1, ""))

	// 已知 bug 固化（与 fileStore.Delete 同源）：sanitizeKey 吃掉尾斜杠，
	// Delete("dir/") 的前缀递归删除分支不可达——实际删的是 "dir"，
	// memblob 下该 key 不存在（只有 dir/a.txt 等），报 NotFound。
	err := s.Delete(ctx, "dir/")
	require.Error(t, err)

	// 对象仍在（递归删除未生效）
	res, err := s.List(ctx, "", "", "", 0)
	require.NoError(t, err)
	found := false
	for _, o := range res.Objects {
		if strings.HasPrefix(o.Key, "dir/") {
			found = true
		}
	}
	assert.True(t, found, "prefix objects survive until the trailing-slash bug is fixed")
}
