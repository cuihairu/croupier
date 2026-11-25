package function

import (
	"context"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Regression: POST /api/v1/functions/:id/disable used to pass through
// ShouldBindJSON with an empty body and get io.EOF -> 400 bad_request("EOF"),
// and FunctionDisableRequest lacked a uri tag so the path :id was never bound
// into FunctionId. After the fix an empty body is legal and functionId is
// bound from the path, while malformed JSON still returns 400.

func TestBindFunctionRequestEmptyBodyTolerant(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	ctx, _ := newFunctionTestContext(http.MethodPost, "/api/v1/functions/inventory.consume/disable", "")
	ctx.Params = gin.Params{{Key: "id", Value: "inventory.consume"}}
	var req FunctionDisableRequest
	require.NoError(t, bindFunctionRequest(ctx, &req))
	assert.Equal(t, "inventory.consume", req.FunctionId)
}

func TestBindFunctionRequestChunkedEmptyBodyTolerant(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	// ContentLength = -1 (chunked / no declared length) + empty payload:
	// ShouldBindJSON reports io.EOF, which is also tolerated.
	ctx, _ := newFunctionTestContext(http.MethodPost, "/api/v1/functions/fn.a/enable", "")
	ctx.Request.ContentLength = -1
	ctx.Params = gin.Params{{Key: "id", Value: "fn.a"}}
	var req FunctionEnableRequest
	require.NoError(t, bindFunctionRequest(ctx, &req))
	assert.Equal(t, "fn.a", req.FunctionId)
}

func TestBindFunctionRequestMalformedJSONStill400(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	ctx, _ := newFunctionTestContext(http.MethodPost, "/api/v1/functions/fn.a/disable", "{invalid")
	ctx.Params = gin.Params{{Key: "id", Value: "fn.a"}}
	var req FunctionDisableRequest
	require.Error(t, bindFunctionRequest(ctx, &req))
}

func TestBindFunctionRequestJSONBodyStillWins(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	// The legacy client usage with a body is unaffected; an explicit body
	// value takes precedence over the path value.
	ctx, _ := newFunctionTestContext(http.MethodPost, "/api/v1/functions/from-path/disable", `{"functionId":"from-body"}`)
	ctx.Params = gin.Params{{Key: "id", Value: "from-path"}}
	var req FunctionDisableRequest
	require.NoError(t, bindFunctionRequest(ctx, &req))
	assert.Equal(t, "from-body", req.FunctionId)
}

// toggleRecordingService records the functionId received by disable/enable to
// verify the handler chain (route param -> bind -> service) end to end.
type toggleRecordingService struct {
	*Service
	gotDisable string
	gotEnable  string
}

func (s *toggleRecordingService) FunctionDisable(_ context.Context, req *FunctionDisableRequest) error {
	s.gotDisable = req.FunctionId
	return nil
}

func (s *toggleRecordingService) FunctionEnable(_ context.Context, req *FunctionEnableRequest) error {
	s.gotEnable = req.FunctionId
	return nil
}

func TestHandlerDisableEmptyBodyEndToEnd(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svc := &toggleRecordingService{Service: NewService(nil)}
	h := &Handler{service: svc}

	ctx, rec := newFunctionTestContext(http.MethodPost, "/api/v1/functions/inventory.consume/disable", "")
	ctx.Params = gin.Params{{Key: "id", Value: "inventory.consume"}}
	h.FunctionDisable(ctx)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "inventory.consume", svc.gotDisable)

	ctx2, rec2 := newFunctionTestContext(http.MethodPost, "/api/v1/functions/fn.b/enable", "")
	ctx2.Params = gin.Params{{Key: "id", Value: "fn.b"}}
	h.FunctionEnable(ctx2)

	require.Equal(t, http.StatusOK, rec2.Code, rec2.Body.String())
	assert.Equal(t, "fn.b", svc.gotEnable)
}
