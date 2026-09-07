package resource

import (
	"net/http"
	"testing"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// List：两个资源 Category.Order 相同时按 Key 排序。
func TestFinalList_SortTieBreaksByKey(t *testing.T) {
	svcCtx, ctx := newResourceTestServiceContext(t, reg.NewStore(), "resources:read")
	seedPlayerResource(t, svcCtx, ctx)

	resp, err := NewService(svcCtx).List(ctx, &ResourceListRequest{})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(resp.Items), 2)
	for i := 1; i < len(resp.Items); i++ {
		prev, cur := resp.Items[i-1], resp.Items[i]
		if prev.Category.Order == cur.Category.Order {
			assert.Less(t, prev.Key, cur.Key)
		}
	}
}

// List：capability 展开时契约查询失败。
func TestFinalList_ContractQueryError(t *testing.T) {
	svcCtx, ctx := newResourceTestServiceContext(t, reg.NewStore(), "resources:read")
	seedPlayerResource(t, svcCtx, ctx)
	require.NoError(t, svcCtx.DB.Migrator().DropTable("function_contracts"))

	resp, err := NewService(svcCtx).List(ctx, &ResourceListRequest{})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// Detail：语义查询失败（非 NotFound）。
func TestFinalDetail_SemanticsQueryError(t *testing.T) {
	svcCtx, ctx := newResourceTestServiceContext(t, reg.NewStore(), "resources:read")
	seedPlayerResource(t, svcCtx, ctx)
	require.NoError(t, svcCtx.DB.Migrator().DropTable("capability_semantics"))

	resp, err := NewService(svcCtx).Detail(ctx, &ResourceDetailRequest{ResourceKey: "player"})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// Detail：契约查询失败被包装为 ResourceNotFoundError → 404。
func TestFinalHandler_Detail_ContractQueryError(t *testing.T) {
	ginCtx, rec, svcCtx, ctx := newResourceHandlerContext(t, http.MethodGet, "/api/v1/resources/player", "resources:read")
	seedPlayerResource(t, svcCtx, ctx)
	ginCtx.Params = gin.Params{{Key: "resourceKey", Value: "player"}}
	require.NoError(t, svcCtx.DB.Migrator().DropTable("function_contracts"))

	NewHandler(NewService(svcCtx)).Detail(ginCtx)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// Detail：无权限 → 403 走 response.Error（非 404 分支）。
func TestFinalHandler_Detail_Forbidden(t *testing.T) {
	ginCtx, rec, svcCtx, _ := newResourceHandlerContext(t, http.MethodGet, "/api/v1/resources/player")
	ginCtx.Params = gin.Params{{Key: "resourceKey", Value: "player"}}

	NewHandler(NewService(svcCtx)).Detail(ginCtx)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// Operations：无权限 → 403 走 response.Error（非 404 分支）。
func TestFinalHandler_Operations_Forbidden(t *testing.T) {
	ginCtx, rec, svcCtx, _ := newResourceHandlerContext(t, http.MethodGet, "/api/v1/resources/player/operations")
	ginCtx.Params = gin.Params{{Key: "resourceKey", Value: "player"}}

	NewHandler(NewService(svcCtx)).Operations(ginCtx)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// humanizeKey：空输入返回空串。
func TestFinalHumanizeKey_Empty(t *testing.T) {
	assert.Equal(t, "", humanizeKey(""))
	assert.Equal(t, "", humanizeKey(" ._- "))
}
