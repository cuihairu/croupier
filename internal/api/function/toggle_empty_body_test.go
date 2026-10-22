package function

import (
	"context"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 回归：POST /api/v1/functions/:id/disable 曾因空 body 走 ShouldBindJSON
// 得 io.EOF → 400 bad_request("EOF")，且 FunctionDisableRequest 缺 uri
// tag 导致路径 :id 从未绑进 FunctionId。修复后空 body 合法、functionId
// 从路径绑定，非法 JSON 仍 400。

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

	// ContentLength = -1（chunked/无长度声明）+ 空 payload：ShouldBindJSON
	// 报 io.EOF，同样容忍。
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

	// 带 body 的旧客户端用法不受影响；body 显式值优先于路径值。
	ctx, _ := newFunctionTestContext(http.MethodPost, "/api/v1/functions/from-path/disable", `{"functionId":"from-body"}`)
	ctx.Params = gin.Params{{Key: "id", Value: "from-path"}}
	var req FunctionDisableRequest
	require.NoError(t, bindFunctionRequest(ctx, &req))
	assert.Equal(t, "from-body", req.FunctionId)
}

// toggleRecordingService 记录 disable/enable 收到的 functionId，
// 验证 handler 链路（路由参数 → bind → service）端到端成立。
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
