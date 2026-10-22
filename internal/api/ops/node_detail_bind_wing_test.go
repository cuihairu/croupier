// 覆盖率巡检第二十一轮（wt-api）：handler.go OpsNodeDetail 的
// bindOpsRequest 错误翼（352-354）登记防御性不可达——
// OpsNodeDetailRequest 仅一个可选 string 字段（form+uri tag、无
// binding 校验约束），GET 走 BindQueryCompat 的 query 兼容绑定无失败
// 路径（第四轮 provider SdkStats 同构证明）。本文件以证明性用例锁定
// 「任意 query 绑定永不失败」前提：若未来字段引入 required/强类型，
// 分支转可达，届时补真实错误路径用例。
package ops

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpsNodeDetailBindNeverFails(t *testing.T) {
	for _, q := range []string{
		"",
		"?nodeId=%zz",
		"?nodeId=<script>",
		"?nodeId=a&nodeId=b",
	} {
		ctx, _ := newOpsTestContext(http.MethodGet, "/api/v1/ops/nodes"+q, "")
		var req OpsNodeDetailRequest
		require.NoError(t, bindOpsRequest(ctx, &req), "query %q 不应产生绑定错误（分支不可达前提）", q)
	}
}
