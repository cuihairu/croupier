// 覆盖目标：functioncall 包 handler 层缺失分支——service 错误（DB 关闭）、
// Detail/Cancel/Rerun 的 URI 绑定错误、Rerun 的 JSON 绑定错误（ContentLength>0）。
package functioncall

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newClosedDBService 返回底层 sql.DB 已关闭的 service，用于触发 gorm 查询错误。
func newClosedDBService(t *testing.T) *Service {
	t.Helper()
	svcCtx := setupSvcCtx(t)
	sqlDB, err := svcCtx.DB.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	return NewService(svcCtx)
}

func TestHandler_List_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(newClosedDBService(t))
	router := newRouter(handler)

	rec := doReq(t, router, http.MethodGet, "/calls", "")
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assertErrorShape(t, rec)
}

func TestHandler_Stats_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(newClosedDBService(t))
	router := newRouter(handler)

	rec := doReq(t, router, http.MethodGet, "/calls/stats", "")
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assertErrorShape(t, rec)
}

func TestHandler_Detail_URIbindError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)

	ctx, rec := newTestContext(http.MethodGet, "/calls", "")
	ctx.Params = nil // 缺少 :id URI 参数
	handler.Detail(ctx)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_Cancel_URIbindError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)

	ctx, rec := newTestContext(http.MethodPost, "/calls//cancel", "")
	ctx.Params = nil
	handler.Cancel(ctx)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_Cancel_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(newClosedDBService(t))
	router := newRouter(handler)

	rec := doReq(t, router, http.MethodPost, "/calls/whatever/cancel", "")
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandler_Rerun_MalformedJSONWithBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)
	router := newRouter(handler)

	rec := doReq(t, router, http.MethodPost, "/calls/some-id/rerun", "{not-json")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assertErrorShape(t, rec)
}

func TestHandler_Rerun_URIbindError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)

	ctx, rec := newTestContext(http.MethodPost, "/calls//rerun", "{}")
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = nil
	handler.Rerun(ctx)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestService_List_DatabaseClosed(t *testing.T) {
	s := newClosedDBService(t)
	resp, err := s.List(nil, &ListRequest{Page: 1, PageSize: 10})
	require.Error(t, err)
	assert.Nil(t, resp)

	stats, err := s.Stats(nil, &ListRequest{})
	require.Error(t, err)
	assert.Nil(t, stats)
}

func TestService_Cancel_DatabaseClosed(t *testing.T) {
	s := newClosedDBService(t)
	err := s.Cancel(nil, &DetailRequest{ID: "t-1"})
	require.Error(t, err)
}
