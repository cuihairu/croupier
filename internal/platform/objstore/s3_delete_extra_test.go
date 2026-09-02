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

	// 尾斜杠键前缀递归删除（sanitizeKey 前保留目录语义）
	require.NoError(t, s.Delete(ctx, "dir/"))

	res, err := s.List(ctx, "", "", "", 0)
	require.NoError(t, err)
	for _, o := range res.Objects {
		assert.NotContains(t, o.Key, "dir/", "prefix objects must be deleted, got %s", o.Key)
	}
}
