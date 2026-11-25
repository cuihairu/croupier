// Coverage target: missing branches of the handler layer in the functioncall
// package -- service errors (closed DB), URI binding errors for
// Detail/Cancel/Rerun, and Rerun's JSON binding error (ContentLength>0).
package functioncall

import (
	"context"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newClosedDBService returns a service whose underlying sql.DB is already
// closed, used to trigger a gorm query error.
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
	ctx.Params = nil // missing the :id URI parameter
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
	resp, err := s.List(context.TODO(), &ListRequest{Page: 1, PageSize: 10})
	require.Error(t, err)
	assert.Nil(t, resp)

	stats, err := s.Stats(context.TODO(), &ListRequest{})
	require.Error(t, err)
	assert.Nil(t, stats)
}

func TestService_Cancel_DatabaseClosed(t *testing.T) {
	s := newClosedDBService(t)
	err := s.Cancel(context.TODO(), &DetailRequest{ID: "t-1"})
	require.Error(t, err)
}

// TestHandler_Rerun_SuccessViaInjection injects a success implementation
// through the rerun injection point to drive the Rerun handler's success
// response branch (service.Rerun is an intentional feature stub that always
// returns BadRequest, so the production success path is unreachable until the
// stub is implemented; see the Handler.rerun comment).
func TestHandler_Rerun_SuccessViaInjection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := setupHandler(t)
	handler.rerun = func(ctx context.Context, req *RerunRequest) (*RerunResponse, error) {
		return &RerunResponse{TaskID: req.ID}, nil
	}
	router := newRouter(handler)

	rec := doReq(t, router, http.MethodPost, "/calls/call-42/rerun", `{}`)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "call-42")
}
