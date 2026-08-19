package resource

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	dashboardservice "github.com/cuihairu/croupier/internal/service"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedPlayerResource(t *testing.T, svcCtx *svc.ServiceContext, ctx context.Context) {
	t.Helper()
	contractService := dashboardservice.NewContractService(svcCtx.DB)
	require.NoError(t, contractService.RebuildContractFromFunctionMeta(ctx, "demo-game", "development", "sdk", dashboardservice.FunctionMetaInput{
		ID:         "player.list",
		Version:    "1.0.0",
		Enabled:    true,
		Summary:    "List players",
		Resource:   "player",
		Risk:       "safe",
		Operation:  "list",
		Capability: "collection_query",
		Execution:  "sync",
		Permission: "player:list",
	}))
	require.NoError(t, contractService.RebuildContractFromFunctionMeta(ctx, "demo-game", "development", "sdk", dashboardservice.FunctionMetaInput{
		ID:         "guild.info",
		Version:    "1.0.0",
		Enabled:    true,
		Summary:    "Guild info",
		Resource:   "guild",
		Risk:       "safe",
		Operation:  "get",
		Capability: "item_query",
		Execution:  "sync",
		Permission: "guild:info",
	}))
	require.NoError(t, contractService.RebuildResourceCapability(ctx, "demo-game", "development", "player"))
	require.NoError(t, contractService.RebuildResourceCapability(ctx, "demo-game", "development", "guild"))
}

func TestServiceList_FiltersCategoryAndQuery(t *testing.T) {
	svcCtx, ctx := newResourceTestServiceContext(t, reg.NewStore(), "resources:read")
	seedPlayerResource(t, svcCtx, ctx)

	svcAPI := NewService(svcCtx)

	resp, err := svcAPI.List(ctx, &ResourceListRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Items, 2)
	// Sorted by category key: guild before player.
	assert.Equal(t, "guild", resp.Items[0].Key)
	assert.Equal(t, "player", resp.Items[1].Key)

	resp, err = svcAPI.List(ctx, &ResourceListRequest{Category: "player"})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "player", resp.Items[0].Key)

	resp, err = svcAPI.List(ctx, &ResourceListRequest{Category: "nope"})
	require.NoError(t, err)
	assert.Empty(t, resp.Items)

	// Query matches the localized label.
	resp, err = svcAPI.List(ctx, &ResourceListRequest{Query: "guild"})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "guild", resp.Items[0].Key)

	// Query matches neither key nor labels.
	resp, err = svcAPI.List(ctx, &ResourceListRequest{Query: "nothing-matches"})
	require.NoError(t, err)
	assert.Empty(t, resp.Items)

	// Blank query matches everything.
	resp, err = svcAPI.List(ctx, &ResourceListRequest{Query: "   "})
	require.NoError(t, err)
	assert.Len(t, resp.Items, 2)
}

func TestServiceList_ScopeMissing(t *testing.T) {
	svcCtx, _ := newResourceTestServiceContext(t, reg.NewStore(), "resources:read")
	// Authorized context without a game scope.
	ctx := context.WithValue(context.Background(), "username", "resource_tester")

	_, err := NewService(svcCtx).List(ctx, &ResourceListRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "X-Game-ID")

	_, err = NewService(svcCtx).Detail(ctx, &ResourceDetailRequest{ResourceKey: "player"})
	require.Error(t, err)
}

func TestServiceList_DBError(t *testing.T) {
	svcCtx, ctx := newResourceTestServiceContext(t, reg.NewStore(), "resources:read")
	require.NoError(t, svcCtx.DB.Migrator().DropTable("resource_capabilities"))

	_, err := NewService(svcCtx).List(ctx, &ResourceListRequest{})
	require.Error(t, err)
}

func TestServiceDetail_DBError(t *testing.T) {
	svcCtx, ctx := newResourceTestServiceContext(t, reg.NewStore(), "resources:read")
	require.NoError(t, svcCtx.DB.Migrator().DropTable("resource_capabilities"))

	_, err := NewService(svcCtx).Detail(ctx, &ResourceDetailRequest{ResourceKey: "player"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resource not found")
}

func TestResourceSpecFromCapability_Nil(t *testing.T) {
	svcCtx, ctx := newResourceTestServiceContext(t, reg.NewStore(), "resources:read")
	_, err := NewService(svcCtx).resourceSpecFromCapability(ctx, "demo-game", "development", nil)
	require.Error(t, err)
}

func TestRequireResourceDiagnose(t *testing.T) {
	svcCtx, ctx := newResourceTestServiceContext(t, reg.NewStore(), "resources:diagnose")
	require.NoError(t, NewService(svcCtx).requireResourceDiagnose(ctx))

	svcCtxNoPerm, _ := newResourceTestServiceContext(t, reg.NewStore())
	err := NewService(svcCtxNoPerm).requireResourceDiagnose(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权生成页面候选")
}

// --- HTTP handler coverage ---

func newResourceHandlerContext(t *testing.T, method, target string, perms ...string) (*gin.Context, *httptest.ResponseRecorder, *svc.ServiceContext, context.Context) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	svcCtx, ctx := newResourceTestServiceContext(t, reg.NewStore(), perms...)

	rec := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(method, target, nil)
	ginCtx.Request = req
	ginCtx.Request = req.WithContext(ctx)
	return ginCtx, rec, svcCtx, ctx
}

func decodeResourceBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]json.RawMessage {
	t.Helper()
	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body
}

func TestHandler_List_Success(t *testing.T) {
	ginCtx, rec, svcCtx, ctx := newResourceHandlerContext(t, http.MethodGet, "/api/v1/resources?category=player", "resources:read")
	seedPlayerResource(t, svcCtx, ctx)

	NewHandler(NewService(svcCtx)).List(ginCtx)

	require.Equal(t, http.StatusOK, rec.Code)
	body := decodeResourceBody(t, rec)
	var items []spec.ResourceSpec
	require.NoError(t, json.Unmarshal(body["items"], &items))
	require.Len(t, items, 1)
	assert.Equal(t, "player", items[0].Key)
}

func TestHandler_List_Unauthorized(t *testing.T) {
	ginCtx, rec, svcCtx, _ := newResourceHandlerContext(t, http.MethodGet, "/api/v1/resources")

	NewHandler(NewService(svcCtx)).List(ginCtx)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestHandler_Detail_SuccessAndNotFound(t *testing.T) {
	ginCtx, rec, svcCtx, ctx := newResourceHandlerContext(t, http.MethodGet, "/api/v1/resources/player", "resources:read")
	seedPlayerResource(t, svcCtx, ctx)
	ginCtx.Params = gin.Params{{Key: "resourceKey", Value: "player"}}

	h := NewHandler(NewService(svcCtx))
	h.Detail(ginCtx)
	require.Equal(t, http.StatusOK, rec.Code)
	body := decodeResourceBody(t, rec)
	var resource spec.ResourceSpec
	require.NoError(t, json.Unmarshal(body["resource"], &resource))
	assert.Equal(t, "player", resource.Key)

	// Not-found maps to 404 through ResourceNotFoundError.
	rec2 := httptest.NewRecorder()
	ginCtx2, _ := gin.CreateTestContext(rec2)
	ginCtx2.Request = httptest.NewRequest(http.MethodGet, "/api/v1/resources/ghost", nil).WithContext(ctx)
	ginCtx2.Params = gin.Params{{Key: "resourceKey", Value: "ghost"}}
	h.Detail(ginCtx2)
	assert.Equal(t, http.StatusNotFound, rec2.Code)
}

func TestHandler_Detail_MissingURIParam(t *testing.T) {
	ginCtx, rec, svcCtx, _ := newResourceHandlerContext(t, http.MethodGet, "/api/v1/resources/", "resources:read")

	NewHandler(NewService(svcCtx)).Detail(ginCtx)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_Operations_SuccessAndNotFound(t *testing.T) {
	ginCtx, rec, svcCtx, ctx := newResourceHandlerContext(t, http.MethodGet, "/api/v1/resources/player/operations", "resources:read")
	seedPlayerResource(t, svcCtx, ctx)
	ginCtx.Params = gin.Params{{Key: "resourceKey", Value: "player"}}

	h := NewHandler(NewService(svcCtx))
	h.Operations(ginCtx)
	require.Equal(t, http.StatusOK, rec.Code)
	body := decodeResourceBody(t, rec)
	var ops []spec.OperationSpec
	require.NoError(t, json.Unmarshal(body["items"], &ops))
	require.Len(t, ops, 1)
	assert.Equal(t, "player.list", ops[0].FunctionID)

	rec2 := httptest.NewRecorder()
	ginCtx2, _ := gin.CreateTestContext(rec2)
	ginCtx2.Request = httptest.NewRequest(http.MethodGet, "/api/v1/resources/ghost/operations", nil).WithContext(ctx)
	ginCtx2.Params = gin.Params{{Key: "resourceKey", Value: "ghost"}}
	h.Operations(ginCtx2)
	assert.Equal(t, http.StatusNotFound, rec2.Code)
}

func TestHandler_Operations_MissingURIParam(t *testing.T) {
	ginCtx, rec, svcCtx, _ := newResourceHandlerContext(t, http.MethodGet, "/api/v1/resources/", "resources:read")
	NewHandler(NewService(svcCtx)).Operations(ginCtx)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestResourceNotFoundError_Message(t *testing.T) {
	err := ErrResourceNotFound("gold")
	require.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "resource not found: gold"))
}
