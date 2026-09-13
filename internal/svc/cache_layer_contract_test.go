// 设计债回归：GetAdminByUsernameCached 空 username 契约对称化。
// 此前空/纯空白 username 静默返回 (nil, nil)，与「查询失败返回 error」
// 的契约不对称——调用方无法区分「参数错误」与「查询成功但无数据」。
// 修复后空 username 返回明确 error；非空 username 的正常查询路径不变。
package svc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAdminByUsernameCached_EmptyUsernameReturnsError(t *testing.T) {
	svcCtx := setupTestServiceContext(t)
	ctx := context.Background()

	for _, username := range []string{"", "   ", "\t"} {
		admin, err := svcCtx.GetAdminByUsernameCached(ctx, username)
		assert.Error(t, err, "空 username（%q）必须返回 error", username)
		assert.Nil(t, admin, "error 时不得返回非 nil admin")
		assert.Contains(t, err.Error(), "username", "错误信息应指明 username 为空")
	}

	// 非空 username 仍是正常查询路径：不存在的用户走 FindByUsername 报错。
	_, err := svcCtx.GetAdminByUsernameCached(ctx, "no-such-user")
	require.Error(t, err)
}
