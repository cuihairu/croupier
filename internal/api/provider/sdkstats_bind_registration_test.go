package provider

// Coverage sweep registration (the ShouldBindQuery error branch of SdkStats in
// handler.go):
//
//	SdkStatsRequest has only two `form:"...,optional"` string fields (dto.go),
//	and gin's form binding has no failure path for all-optional strings -- no
//	missing required, no type conversion error, so any valid query binds
//	successfully. The handler's binding-error branch (response.Error; return)
//	is therefore defensively unreachable; it is registered per the repo's
//	existing convention, with no fabricated test case and no deletion of the
//	defensive branch.
//
// This file locks in the "binding never fails" premise with a proof test: if
// a required field or a strongly typed field (int/bool) is added in the
// future, that branch becomes reachable, at which point real error-path cases
// should be added and this proof removed.

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
