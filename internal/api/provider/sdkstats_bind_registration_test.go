package provider

// 覆盖率巡检登记（handler.go SdkStats 的 ShouldBindQuery 错误分支）：
//
//	SdkStatsRequest 仅两个 `form:"...,optional"` string 字段（dto.go），
//	gin form 绑定对全可选字符串不存在失败路径——无 required 缺失、无类型
//	转换错误，任何合法 query 都绑定成功。故 handler 的绑定错误分支
//	（response.Error; return）为防御性不可达，按仓库既有口径登记，
//	不造假用例、不删防御分支。
//
// 本文件以证明性用例锁定「绑定永不失败」这一前提：若未来字段引入
// required 或强类型（int/bool），该分支变为可达，届时应补真实错误
// 路径用例并删除本证明。

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSdkStatsRequest_BindingNeverFailsOnArbitraryQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, q := range []string{
		"",
		"?metaKey=",
		"?metaKey=region&metaValue=cn",
		"?junk=1&unknown=2",
		"?metaKey=%E5%9C%B0%E5%9F%9F",
	} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/providers/sdk-stats"+q, nil)

		var req SdkStatsRequest
		require.NoError(t, c.ShouldBindQuery(&req), "query %q 不应产生绑定错误（分支不可达前提）", q)
	}
}
