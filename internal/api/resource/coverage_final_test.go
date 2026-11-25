package resource

import (
	"net/http"
	"testing"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// List: sorting is a stable sort by Key only (the Category.Order field was
// removed and has no persisted source).
func TestFinalList_SortTieBreaksByKey(t *testing.T) {
	svcCtx, ctx := newResourceTestServiceContext(t, reg.NewStore(), "resources:read")
	seedPlayerResource(t, svcCtx, ctx)

	resp, err := NewService(svcCtx).List(ctx, &ResourceListRequest{})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(resp.Items), 2)
	for i := 1; i < len(resp.Items); i++ {
		assert.Less(t, resp.Items[i-1].Key, resp.Items[i].Key)
	}
}

// List: contract query fails while expanding capabilities.
func TestFinalList_ContractQueryError(t *testing.T) {
	svcCtx, ctx := newResourceTestServiceContext(t, reg.NewStore(), "resources:read")
	seedPlayerResource(t, svcCtx, ctx)
	require.NoError(t, svcCtx.DB.Migrator().DropTable("function_contracts"))

	resp, err := NewService(svcCtx).List(ctx, &ResourceListRequest{})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// Detail: semantics query fails (not NotFound).
func TestFinalDetail_SemanticsQueryError(t *testing.T) {
	svcCtx, ctx := newResourceTestServiceContext(t, reg.NewStore(), "resources:read")
	seedPlayerResource(t, svcCtx, ctx)
	require.NoError(t, svcCtx.DB.Migrator().DropTable("capability_semantics"))

	resp, err := NewService(svcCtx).Detail(ctx, &ResourceDetailRequest{ResourceKey: "player"})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// Detail: a contract query failure is wrapped as ResourceNotFoundError -> 404.
func TestFinalHandler_Detail_ContractQueryError(t *testing.T) {
	ginCtx, rec, svcCtx, ctx := newResourceHandlerContext(t, http.MethodGet, "/api/v1/resources/player", "resources:read")
	seedPlayerResource(t, svcCtx, ctx)
	ginCtx.Params = gin.Params{{Key: "resourceKey", Value: "player"}}
	require.NoError(t, svcCtx.DB.Migrator().DropTable("function_contracts"))

	NewHandler(NewService(svcCtx)).Detail(ginCtx)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// Detail: no permission -> 403 through response.Error (not the 404 branch).
func TestFinalHandler_Detail_Forbidden(t *testing.T) {
	ginCtx, rec, svcCtx, _ := newResourceHandlerContext(t, http.MethodGet, "/api/v1/resources/player")
	ginCtx.Params = gin.Params{{Key: "resourceKey", Value: "player"}}

	NewHandler(NewService(svcCtx)).Detail(ginCtx)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// Operations: no permission -> 403 through response.Error (not the 404
// branch).
func TestFinalHandler_Operations_Forbidden(t *testing.T) {
	ginCtx, rec, svcCtx, _ := newResourceHandlerContext(t, http.MethodGet, "/api/v1/resources/player/operations")
	ginCtx.Params = gin.Params{{Key: "resourceKey", Value: "player"}}

	NewHandler(NewService(svcCtx)).Operations(ginCtx)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// humanizeKey: empty input returns an empty string.
func TestFinalHumanizeKey_Empty(t *testing.T) {
	assert.Equal(t, "", humanizeKey(""))
	assert.Equal(t, "", humanizeKey(" ._- "))
}
