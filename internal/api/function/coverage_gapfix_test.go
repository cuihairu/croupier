package function

import (
	"context"
	"errors"
	"net/http"
	"testing"

	logicfunction "github.com/cuihairu/croupier/internal/logic/function"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errHandlerServiceBoom = errors.New("handler service boom")

// errInjectingService wraps *Service, replacing eight stub methods with ones
// that return an injected error, to cover the handler layer's service error
// branches.
type errInjectingService struct {
	*Service
}

func (s *errInjectingService) FunctionsPending(ctx context.Context, req *FunctionsPendingRequest) (*FunctionsPendingResponse, error) {
	return nil, errHandlerServiceBoom
}

func (s *errInjectingService) FunctionCopy(ctx context.Context, req *FunctionCopyRequest) (*FunctionCopyResponse, error) {
	return nil, errHandlerServiceBoom
}

func (s *errInjectingService) FunctionPublish(ctx context.Context, req *FunctionPublishRequest) (*FunctionPublishResponse, error) {
	return nil, errHandlerServiceBoom
}

func (s *errInjectingService) FunctionInstances(ctx context.Context, req *FunctionInstancesRequest) (*FunctionInstancesResponse, error) {
	return nil, errHandlerServiceBoom
}

func (s *errInjectingService) FunctionInstancesAll(ctx context.Context, req *FunctionInstancesAllRequest) (*FunctionInstancesAllResponse, error) {
	return nil, errHandlerServiceBoom
}

func (s *errInjectingService) FunctionWarnings(ctx context.Context, req *FunctionWarningsRequest) (*FunctionWarningsResponse, error) {
	return nil, errHandlerServiceBoom
}

func (s *errInjectingService) BatchCopyFunctions(ctx context.Context, req *BatchCopyFunctionsRequest) (*BatchCopyFunctionsResponse, error) {
	return nil, errHandlerServiceBoom
}

func (s *errInjectingService) BatchDeleteFunctions(ctx context.Context, req *BatchDeleteFunctionsRequest) (*BatchDeleteFunctionsResponse, error) {
	return nil, errHandlerServiceBoom
}

func TestHandlers_ServiceErrorResponds500(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := &Handler{service: &errInjectingService{Service: NewService(&svc.ServiceContext{})}}
	cases := []struct {
		name   string
		fn     func(*gin.Context)
		target string
	}{
		{name: "FunctionsPending", fn: h.FunctionsPending, target: "/api/v1/functions/pending"},
		{name: "FunctionCopy", fn: h.FunctionCopy, target: "/api/v1/functions/copy"},
		{name: "FunctionPublish", fn: h.FunctionPublish, target: "/api/v1/functions/publish"},
		{name: "FunctionInstances", fn: h.FunctionInstances, target: "/api/v1/functions/instances"},
		{name: "FunctionInstancesAll", fn: h.FunctionInstancesAll, target: "/api/v1/functions/instances-all"},
		{name: "FunctionWarnings", fn: h.FunctionWarnings, target: "/api/v1/functions/warnings"},
		{name: "BatchCopyFunctions", fn: h.BatchCopyFunctions, target: "/api/v1/functions/batch-copy"},
		{name: "BatchDeleteFunctions", fn: h.BatchDeleteFunctions, target: "/api/v1/functions/batch-delete"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, rec := newFunctionTestContext(http.MethodPost, tc.target, "{}")
			tc.fn(ctx)

			require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
			// AUD-047 opaque: the raw unknown error only goes to the log; the
			// body contains only the stable message plus a tracking number.
			assert.Contains(t, rec.Body.String(), "internal_error")
			assert.NotContains(t, rec.Body.String(), errHandlerServiceBoom.Error())
		})
	}
}

func TestFunctionHistory_NilItemsNormalized(t *testing.T) {
	orig := fetchFunctionHistoryPage
	fetchFunctionHistoryPage = func(ctx context.Context, svcCtx *svc.ServiceContext, req *logicfunction.FunctionHistoryRequest) ([]logicfunction.FunctionHistoryItem, int, error) {
		assert.Equal(t, "fn.nil.items", req.ID)
		return nil, 7, nil
	}
	t.Cleanup(func() { fetchFunctionHistoryPage = orig })

	resp, err := functionHistory(context.Background(), &svc.ServiceContext{}, &FunctionHistoryRequest{ID: "fn.nil.items"})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Items)
	assert.Empty(t, resp.Items)
	assert.EqualValues(t, 7, resp.Total)
}
